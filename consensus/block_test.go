package consensus_test

import (
	cs "github.com/tendermint/tendermint/consensus"
	p2pmocks "github.com/tendermint/tendermint/p2p/mocks"
)

func (s *TestSuite) TestReceiveProposal() {
	blockID := makeRandomBlockID()
	proposal := s.genProposal(0, 1, 0, blockID)

	proposalMsg := &cs.ProposalMessage{Proposal: proposal}
	msg, err := cs.MsgToProto(proposalMsg)
	s.Require().NoError(err)

	bz, err := msg.Marshal()
	s.Require().NoError(err)

	s.consensus.Receive(cs.DataChannel, &p2pmocks.Peer{}, bz)

	proposals := s.consensus.Proposals()
	s.Require().Len(proposals, 1)
	proposedBlockID, ok := proposals[int32(0)]
	s.Require().True(ok)
	s.Require().Equal(blockID, proposedBlockID)

}
