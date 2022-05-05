package consensus

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/rs/zerolog/log"
	cs "github.com/tendermint/tendermint/consensus"
	cstypes "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/p2p"
	csproto "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tm "github.com/tendermint/tendermint/types"
)

const (
	maxMsgSize             = 1048576
	stateBroadcastInterval = 2000 * time.Millisecond
	voteBroadcastInterval  = 2000 * time.Millisecond
)

//go:generate mockery --case underscore --name Provider|Handler
type Provider interface {
	ValidatorSet(ctx context.Context, height *int64) (*tm.ValidatorSet, int64, error)
}

type Handler interface {
	Process(block *tm.Block) error
	Commit(blockID []byte, commit *tmproto.SignedHeader, valSet *tmproto.ValidatorSet)
}

var _ p2p.Reactor = &Service{}

// Service tracks consensus state for a single chain. In particular
// Service monitors for new blocks and tallies votes
type Service struct {
	p2p.BaseReactor

	chainID  string
	ibc      Handler
	provider Provider
	closer   chan struct{}

	mtx               sync.Mutex
	height            int64
	round             int32
	proposals         map[int32]tm.BlockID
	partSets          map[string]*tm.PartSet
	proposedBlocks    map[string]*tm.Block
	roundVoteSets     map[int32]*tm.VoteSet
	currentValidators *tm.ValidatorSet
	nextValidators    *tm.ValidatorSet

	peerList *PeerList
}

func NewService(chainID string, height int64, handler Handler, provider Provider, currentValidators, nextValidators *tm.ValidatorSet) *Service {
	service := &Service{
		chainID:           chainID,
		height:            height,
		round:             0,
		ibc:               handler,
		provider:          provider,
		closer:            make(chan struct{}),
		currentValidators: currentValidators,
		nextValidators:    nextValidators,
		proposedBlocks:    make(map[string]*tm.Block),
		proposals:         make(map[int32]tm.BlockID),
		roundVoteSets:     make(map[int32]*tm.VoteSet),
		partSets:          make(map[string]*tm.PartSet),
		peerList:          NewPeerList(),
	}
	service.BaseReactor = *p2p.NewBaseReactor("RelayerConsensus", service)
	return service
}

func (s *Service) OnStart() error {
	go s.broadcastRoutine()
	return nil
}

func (s *Service) OnStop() {
	close(s.closer)
}

// BroadcastRoutine continually loops through to send peers
// the nodes current voteSetBits for each round and block
// that the node has. This basically informs connected peers
// which votes are remaining that need to be sent
func (s Service) broadcastRoutine() {
	stateTicker := time.NewTimer(stateBroadcastInterval)
	voteTicker := time.NewTimer(voteBroadcastInterval)
LOOP:
	for {
		select {
		case <-s.closer:
			log.Info().Msg("closing consensus broadcast routine")
			return
		case <-stateTicker.C:
			stateTicker.Reset(stateBroadcastInterval)
			peer := s.peerList.Next()
			if peer == nil {
				continue
			}
			log.Info().Msg("sending state update msg")
			s.mtx.Lock()
			peer.Send(cs.StateChannel, s.stateMsg())
			s.mtx.Unlock()
		case <-voteTicker.C:
			voteTicker.Reset(voteBroadcastInterval)
			if s.peerList.IsEmpty() {
				log.Info().Msg("no peers available, sleeping...")
				continue
			}

			for peer := s.peerList.Next(); peer != nil; peer = s.peerList.Next() {
				s.mtx.Lock()
				// check if there's any vote state to send
				if len(s.roundVoteSets) == 0 {
					s.mtx.Unlock()
					continue LOOP
				}
				// loop through all rounds and all received proposals constructing
				// the voteset bits for each and gossiping them to individual peers
				for round, roundVoteSet := range s.roundVoteSets {
					for _, blockID := range s.proposals {
						bitArray := roundVoteSet.BitArrayByBlockID(blockID)
						if bitArray == nil {
							// no bit array for this block ID
							s.mtx.Unlock()
							continue
						}

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
							continue LOOP
						}
						log.Info().Msg("Sending vote set bits message")
						// non-blocking. We don't check to see if the peers
						// queue is full
						peer.Send(cs.VoteSetBitsChannel, bz)
					}
				}
				// we've broadcasted all our vote set bits for a height
				// return the lock and wait for the next interval
				s.mtx.Unlock()
				continue LOOP
			}

		}
	}
}

// Receive implements the p2p Reactor. This is the entry method for all
// inbound consensus messages from peers. They are processed into two
// types: block messages and vote messages. Eventually, these messages
// will lead to a commited block which is passed down to the handler
// before moving state to the next height.
func (s *Service) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	log.Info().Msg("Received consensus message")
	pb := &csproto.Message{}
	if err := proto.Unmarshal(msgBytes, pb); err != nil {
		log.Error().Err(err).Msg("unmarshalling consensus message")
		return
	}

	msg, err := cs.MsgFromProto(pb)
	if err != nil {
		log.Error().Err(err).Msg("converting message from proto")
		return
	}

	if err := msg.ValidateBasic(); err != nil {
		log.Error().Err(err).Msg("invalid consensus message")
		return
	}

	switch chID {
	case cs.DataChannel:
		switch msg := msg.(type) {
		case *cs.BlockPartMessage:
			log.Info().Msg("Received block part message")
			s.addBlockPart(msg.Height, msg.Round, msg.Part)
		case *cs.ProposalMessage:
			log.Info().Msg("Received proposal message")
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
		log.Info().Msg("Received vote")

		s.addVote(msg.Vote)

	case cs.VoteSetBitsChannel:
		log.Info().Msg("Received vote set bit message")

	case cs.StateChannel:
		log.Info().Msg("Received consensus state update")
		// ignore state updates
		// TODO: In the future we may want to track maj23
		// to better signal what round we are at
		return
	default:
		log.Error().Str("msg type", fmt.Sprintf("%T", msg)).Int("chID", int(chID)).Msg("unknown msg from unsolicited channel")
		return
	}
}

// commits a block, calling the handler and advancing to the next height
func (s *Service) commit(blockID tm.BlockID, voteSet *tm.VoteSet) {
	// retrieve the block that corresponds to the committed block
	block, ok := s.proposedBlocks[blockID.Hash.String()]
	if !ok {
		log.Info().Msg("received 2/3+ precommits for a block we don't have")
		return
	}

	// Once a block is committed we then use the hashes in the header to validate
	// that the validator set we received from the provider is correct
	if nextValsHash := s.nextValidators.Hash(); !bytes.Equal(nextValsHash, block.NextValidatorsHash) {
		log.Info().Msg("received invalid validator set from proposer. Fetching a new one")
		nextHeight := s.height + 1
		var err error
		s.nextValidators, _, err = s.provider.ValidatorSet(context.Background(), &nextHeight)
		if err != nil {
			log.Error().Err(err).Msg("retrieving validator set for the next height")
			return
		}
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
func (s *Service) advance() {
	s.height++
	if s.nextValidators != nil {
		s.currentValidators = s.nextValidators
	} else {
		var err error
		s.currentValidators, _, err = s.provider.ValidatorSet(context.Background(), &s.height)
		if err != nil {
			log.Error().Err(err).Msg("retrieving next valiator set")
		}
	}

	// reset all tally and block structs
	s.proposedBlocks = make(map[string]*tm.Block)
	s.proposals = make(map[int32]tm.BlockID)
	s.roundVoteSets = make(map[int32]*tm.VoteSet)
	s.partSets = make(map[string]*tm.PartSet)

	// reset the next validator set. We will retrieve it when we start to see the first proposal
	// We can't do this immediately as it's likely nodes haven't persisted the new validator set
	s.nextValidators = nil

	log.Info().Int64("height", s.height).Msg("advancing to the next height")
}

// NOTE: values copied across from the tendermint consensus reactor
func (s *Service) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                  cs.StateChannel,
			Priority:            6,
			SendQueueCapacity:   100,
			RecvMessageCapacity: maxMsgSize,
		},
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

func (s *Service) AddPeer(peer p2p.Peer) {
	log.Info().Msg("added new peer")
	s.peerList.Add(peer)

	// send out a round step message
	log.Info().Int("channel", int(cs.StateChannel)).Msg("Sending round step message")
	peer.Send(cs.StateChannel, s.stateMsg())
}

// Contract: Need to have a lock around state
func (s Service) stateMsg() []byte {
	roundStep := &cs.NewRoundStepMessage{
		Height: s.height,
		Round:  s.round,
		Step:   cstypes.RoundStepNewHeight,
		// TODO: track start time. I'm not sure if this is necessary
		SecondsSinceStartTime: time.Now().Unix(),
		// TODO: We should probably get the information for this
		LastCommitRound: 0,
	}
	msg, err := cs.MsgToProto(roundStep)
	if err != nil {
		panic(err)
	}
	bz, err := msg.Marshal()
	if err != nil {
		panic(err)
	}
	return bz
}

func (s *Service) RemovePeer(peer p2p.Peer, reason interface{}) {
	s.peerList.Remove(peer)
}
