package consensus_test

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	cs "github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	p2pmocks "github.com/tendermint/tendermint/p2p/mocks"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	tm "github.com/tendermint/tendermint/types"

	"github.com/plural-labs/sonic-relayer/consensus"
	"github.com/plural-labs/sonic-relayer/consensus/mocks"
)

const (
	numVals   = 4
	testChain = "test-chain"
)

type TestSuite struct {
	suite.Suite

	consensus         *consensus.Service
	provider          *mocks.Provider
	handler           *mocks.Handler
	keys              map[string]tm.PrivValidator // address -> privval
	currentValidators *tm.ValidatorSet
}

func (s *TestSuite) SetupTest() {
	mockProvider := &mocks.Provider{}
	mockHandler := &mocks.Handler{}

	keys := make(map[string]tm.PrivValidator, numVals)
	valz := make([]*tm.Validator, numVals)
	for i := 0; i < numVals; i++ {
		newKey := tm.NewMockPV()
		newVal := newKey.ExtractIntoValidator(100)
		keys[string(newVal.Address)] = newKey
		valz[i] = newVal
	}
	currVals := tm.NewValidatorSet(valz)

	service := consensus.NewService("test-chain", 1, mockHandler, mockProvider, currVals, currVals.CopyIncrementProposerPriority(1))

	s.provider = mockProvider
	s.handler = mockHandler
	s.consensus = service
	s.keys = keys
	s.currentValidators = currVals
}

func (s *TestSuite) TestCompleteBlock() {
	payload := []byte("transactions")
	block := s.genBlock(1, s.currentValidators.CopyIncrementProposerPriority(1), payload)
	partSet := block.MakePartSet(tm.BlockPartSizeBytes)
	blockID := tm.BlockID{Hash: block.Hash(), PartSetHeader: partSet.Header()}
	s.Require().True(blockID.IsComplete(), blockID)
	s.Require().Equal(uint32(1), partSet.Total())

	// set up mock handler
	s.handler.On("Process", mock.AnythingOfType("*types.Block")).Return(nil)
	s.handler.On("Commit", block.Hash().Bytes(), mock.AnythingOfType("*types.SignedHeader"), mock.AnythingOfType("*types.ValidatorSet")).Return()

	// create and submit proposal
	proposal := s.genProposal(0, 1, 0, blockID)
	msg, err := cs.MsgToProto(&cs.ProposalMessage{Proposal: proposal})
	s.Require().NoError(err)
	bz, err := msg.Marshal()
	s.Require().NoError(err)
	s.consensus.Receive(cs.DataChannel, &p2pmocks.Peer{}, bz)
	s.Require().Len(s.consensus.Proposals(), 1)

	// send block parts
	msg, err = cs.MsgToProto(&cs.BlockPartMessage{
		Height: 1,
		Round:  0,
		Part:   partSet.GetPart(0),
	})
	s.Require().NoError(err)
	bz, err = msg.Marshal()
	s.Require().NoError(err)
	s.consensus.Receive(cs.DataChannel, &p2pmocks.Peer{}, bz)
	ps := s.consensus.PartSet(block.Hash().String())
	s.Require().NotNil(ps)
	s.Require().True(ps.IsComplete())

	// send votes
	for idx := 0; idx < 3; idx++ {
		vote := s.genVote(idx, 1, 0, blockID)
		msg, err = cs.MsgToProto(&cs.VoteMessage{Vote: vote})
		s.Require().NoError(err)
		bz, err = msg.Marshal()
		s.Require().NoError(err)
		s.consensus.Receive(cs.VoteChannel, &p2pmocks.Peer{}, bz)
	}
	s.Require().Equal(int64(2), s.consensus.Height())
	s.Require().True(s.handler.AssertExpectations(s.T()))
}

func makeRandomBlockID() tm.BlockID {
	var (
		blockHash   = make([]byte, 32)
		partSetHash = make([]byte, 32)
	)
	rand.Read(blockHash)   //nolint: errcheck // ignore errcheck for read
	rand.Read(partSetHash) //nolint: errcheck // ignore errcheck for read
	return tm.BlockID{blockHash, tm.PartSetHeader{123, partSetHash}}
}

func (s *TestSuite) genVote(valIdx int, height int64, round int32, blockID tm.BlockID) *tm.Vote {
	if valIdx >= len(s.currentValidators.Validators) {
		s.T().Fatal("incorrect valIdx exceeds size")
	}

	val := s.currentValidators.Validators[valIdx]
	signer := s.keys[string(val.Address)]

	vote := &tm.Vote{
		ValidatorAddress: val.Address,
		ValidatorIndex:   int32(valIdx),
		Height:           height,
		Round:            round,
		Timestamp:        time.Now(),
		Type:             tmproto.PrecommitType,
		BlockID:          blockID,
	}

	v := vote.ToProto()

	s.Require().NoError(signer.SignVote(testChain, v))
	vote.Signature = v.Signature
	return vote
}

func (s *TestSuite) genBlock(height int64, nextVals *tm.ValidatorSet, tx []byte) *tm.Block {
	lastBlockID := makeRandomBlockID()
	lastCommit := &tm.Commit{
		Height:  height - 1,
		Round:   0,
		BlockID: lastBlockID,
		Signatures: []tm.CommitSig{
			tm.NewCommitSigAbsent(),
		},
	}
	data := tm.Data{
		Txs: []tm.Tx{tx},
	}
	evidence := tm.EvidenceData{}
	header := tm.Header{
		Version:            tmversion.Consensus{Block: 11, App: 2},
		ChainID:            testChain,
		Height:             height,
		Time:               time.Now(),
		LastBlockID:        lastBlockID,
		LastCommitHash:     lastCommit.Hash(),
		DataHash:           data.Hash(),
		ValidatorsHash:     tmhash.Sum([]byte("validators_hash")),
		NextValidatorsHash: nextVals.Hash(),
		ConsensusHash:      tmhash.Sum([]byte("consensus_hash")),
		AppHash:            tmhash.Sum([]byte("app_hash")),
		LastResultsHash:    tmhash.Sum([]byte("last_results_hash")),
		EvidenceHash:       evidence.Hash(),
		ProposerAddress:    crypto.AddressHash([]byte("proposer_address")),
	}

	block := &tm.Block{
		Header:     header,
		Evidence:   evidence,
		Data:       data,
		LastCommit: lastCommit,
	}
	return block
}

func (s *TestSuite) genProposal(valIdx int, height int64, round int32, blockID tm.BlockID) *tm.Proposal {
	if valIdx >= len(s.currentValidators.Validators) {
		s.T().Fatal("incorrect valIdx exceeds size")
	}

	val := s.currentValidators.Validators[valIdx]
	signer := s.keys[string(val.Address)]

	proposal := &tm.Proposal{
		Type:      tmproto.ProposalType,
		Height:    height,
		Round:     round,
		POLRound:  round,
		BlockID:   blockID,
		Timestamp: time.Now(),
	}

	p := proposal.ToProto()
	s.Require().NoError(signer.SignProposal(testChain, p))
	proposal.Signature = p.Signature
	return proposal
}

func TestMain(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
