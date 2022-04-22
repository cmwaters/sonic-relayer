package consensus

import (
	"fmt"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/rs/zerolog/log"
	cs "github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/p2p"
	csproto "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tm "github.com/tendermint/tendermint/types"
)

const (
	maxMsgSize = 1048576
)

// State tracks consensus state for a single chain. In particular
// State monitors for new blocks and
type State struct {
	chainID  string
	height   int64
	ibc      Handler
	provider Provider

	mtx               sync.Mutex
	proposals         map[int32]string // round -> blockID
	partSets          map[string]*tm.PartSet
	proposedBlocks    map[string]*tm.Block
	roundVoteSets     map[int32]*tm.VoteSet
	currentValidators *tm.ValidatorSet
	nextValidators    *tm.ValidatorSet

	peerMtx  sync.Mutex
	peerList map[string]p2p.Peer
}

func NewState(chainID string, height int64, handler Handler, provider Provider) *State {
	return &State{
		chainID:        chainID,
		height:         height,
		ibc:            handler,
		provider:       provider,
		proposedBlocks: make(map[string]*tm.Block),
		proposals:      make(map[int32]string),
		roundVoteSets:  make(map[int32]*tm.VoteSet),
		partSets:       make(map[string]*tm.PartSet),
		peerList:       make(map[string]p2p.Peer),
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
	s.proposals = make(map[int32]string)
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
	s.peerMtx.Lock()
	defer s.peerMtx.Unlock()
	s.peerList[string(peer.ID())] = peer
}

func (s *State) RemovePeer(peer p2p.Peer, reason interface{}) {
	s.peerMtx.Lock()
	defer s.peerMtx.Unlock()
	delete(s.peerList, string(peer.ID()))
}
