package consensus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/rs/zerolog/log"
	cs "github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/libs/clist"
	"github.com/tendermint/tendermint/p2p"
	csproto "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tm "github.com/tendermint/tendermint/types"
)

const (
	maxMsgSize        = 1048576
	broadcastInterval = 100 * time.Millisecond
)

// State tracks consensus state for a single chain. In particular
// State monitors for new blocks and
type State struct {
	chainID  string
	height   int64
	ibc      Handler
	provider Provider

	mtx               sync.Mutex
	proposals         map[int32]tm.BlockID
	partSets          map[string]*tm.PartSet
	proposedBlocks    map[string]*tm.Block
	roundVoteSets     map[int32]*tm.VoteSet
	currentValidators *tm.ValidatorSet
	nextValidators    *tm.ValidatorSet

	peerList *clist.CList
}

func NewState(chainID string, height int64, handler Handler, provider Provider, currentValidators, nextValidators *tm.ValidatorSet) *State {
	return &State{
		chainID:           chainID,
		height:            height,
		ibc:               handler,
		provider:          provider,
		currentValidators: currentValidators,
		nextValidators:    nextValidators,
		proposedBlocks:    make(map[string]*tm.Block),
		proposals:         make(map[int32]tm.BlockID),
		roundVoteSets:     make(map[int32]*tm.VoteSet),
		partSets:          make(map[string]*tm.PartSet),
		peerList:          clist.New(),
	}
}

// BroadcastRoutine continually loops through to send peers
// the nodes current voteSetBits for each round and block
// that the node has. This basically informs connected peers
// which votes are remaining that need to be sent
func (s State) BroadcastRoutine(ctx context.Context) {
	ticker := time.NewTicker(broadcastInterval)
	var next *clist.CElement
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mtx.Lock()
			// check if there's any vote state to send
			if len(s.roundVoteSets) == 0 {
				s.mtx.Unlock()
				continue
			}
			// loop through all rounds and all received proposals constructing
			// the voteset bits for each and gossiping them to individual peers
			for round, roundVoteSet := range s.roundVoteSets {
				for _, blockID := range s.proposals {
					// next is nil at either the start or when the list has been
					// exhausted in which case we get from the front
					if next == nil {
						next := s.peerList.Front()
						if next == nil {
							s.mtx.Unlock()
							// we have no peers so sleep
							continue
						}
					}
					bitArray := roundVoteSet.BitArrayByBlockID(blockID)
					if bitArray == nil {
						// no bit array for this block ID
						s.mtx.Unlock()
						continue
					}
					
					peer := next.Value.(p2p.Peer)
					msg := &csproto.VoteSetBits{
						Height:  s.height,
						Round:   round,
						Type:    tmproto.PrecommitType,
						BlockID: blockID.ToProto(),
						Votes:   *bitArray.ToProto(),
					}
					bz, err := msg.Marshal()
					if err != nil {
						log.Error().Err(err)
						s.mtx.Unlock()
						continue
					}
					// non-blocking. We don't check to see if the peers
					// queue is full
					_ = peer.TrySend(cs.VoteSetBitsChannel, bz)
					next = next.Next()
				}
			}
			s.mtx.Unlock()
		}
	}
}

// Receive implements the p2p Reactor. This is the entry method for all
// inbound consensus messages from peers. They are processed into two
// types: block messages and vote messages. Eventually, these messages
// will lead to a commited block which is passed down to the handler
// before moving state to the next height.
func (s *State) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	pb := &csproto.Message{}
	if err := proto.Unmarshal(msgBytes, pb); err != nil {
		log.Error().Err(err)
		return
	}

	msg, err := cs.MsgFromProto(pb)
	if err != nil {
		log.Error().Err(err)
		return
	}

	if err := msg.ValidateBasic(); err != nil {
		log.Error().Err(err)
		return
	}

	switch chID {
	case cs.DataChannel:
		switch msg := msg.(type) {
		case *cs.BlockPartMessage:
			s.addBlockPart(msg.Height, msg.Round, msg.Part)
		case *cs.ProposalMessage:
			s.handleProposal(msg.Proposal)
		case *cs.ProposalPOLMessage:
			return
		default:
			log.Error().Str("msg type", fmt.Sprintf("%T", msg)).Msg("unknown message type in data channel")
			return
		}
	case cs.VoteChannel:
		msg, ok := msg.(*cs.VoteMessage)
		if !ok {
			return
		}

		s.addVote(msg.Vote)

	case cs.VoteSetBitsChannel:
		// ignore inbound vote set bits
		return
	default:
		log.Error().Str("msg type", fmt.Sprintf("%T", msg)).Int("chID", int(chID)).Msg("unknown msg from unsolicited channel")
		return
	}
}

// commits a block, calling the handler and advancing to the next height
func (s *State) commit(blockID tm.BlockID, voteSet *tm.VoteSet) {
	// retrieve the block that corresponds to the committed block
	block, ok := s.proposedBlocks[blockID.Hash.String()]
	if !ok {
		log.Info().Msg("received 2/3+ precommits for a block we don't have")
		return
	}

	// create the signed header for the new block. This is used by IBC as
	// the commitment proof
	commit := tm.SignedHeader{
		Header: &block.Header,
		Commit: voteSet.MakeCommit(),
	}
	vals, err := s.currentValidators.ToProto()
	if err != nil {
		log.Error().Err(err).Msg("marhalling validator set to proto")
	}

	// Trigger the commit method on the ibc handler. This will prepare
	// the outbound transactions and fire them through the router
	// to the respective destination chains
	s.ibc.Commit(blockID.Hash.Bytes(), commit.ToProto(), vals)

	// advance state to the next height
	s.advance()
}

// advance signals the state that a block has been committed and to advance to the
// next height. This clears all state
func (s *State) advance() {
	s.height++
	s.currentValidators = s.nextValidators

	// reset all tally and block structs
	s.proposedBlocks = make(map[string]*tm.Block)
	s.proposals = make(map[int32]tm.BlockID)
	s.roundVoteSets = make(map[int32]*tm.VoteSet)
	s.partSets = make(map[string]*tm.PartSet)

	// retrieve the next validator set
	// TODO: we may want to call this later on when we're sure most of
	// the network has the validator set at the next height
	s.nextValidators = s.provider.ValidatorSet(s.height)
}

func (s *State) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                  cs.DataChannel,
			Priority:            10,
			SendQueueCapacity:   100,
			RecvBufferCapacity:  50 * 4096,
			RecvMessageCapacity: maxMsgSize,
		},
		{
			ID:                  cs.VoteChannel,
			Priority:            7,
			SendQueueCapacity:   100,
			RecvBufferCapacity:  100 * 100,
			RecvMessageCapacity: maxMsgSize,
		},
		{
			ID:                  cs.VoteSetBitsChannel,
			Priority:            1,
			SendQueueCapacity:   2,
			RecvBufferCapacity:  1024,
			RecvMessageCapacity: maxMsgSize,
		},
	}
}

func (s *State) AddPeer(peer p2p.Peer) {
	s.peerList.PushBack(peer)
}

func (s *State) RemovePeer(peer p2p.Peer, reason interface{}) {
	for e := s.peerList.Front(); e != nil; e = e.Next() {
		p := e.Value.(p2p.Peer)
		if p.ID() == peer.ID() {
			s.peerList.Remove(e)
			return
		}
	}
}
