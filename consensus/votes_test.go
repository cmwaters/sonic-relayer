package consensus_test

import (
	cs "github.com/tendermint/tendermint/consensus"
	p2pmocks "github.com/tendermint/tendermint/p2p/mocks"

	tm "github.com/tendermint/tendermint/types"
)

func (s *TestSuite) sendProposal() tm.BlockID {
	blockID := makeRandomBlockID()
	proposal := s.genProposal(0, 1, 0, blockID)

	proposalMsg := &cs.ProposalMessage{Proposal: proposal}
	msg, err := cs.MsgToProto(proposalMsg)
	s.Require().NoError(err)

	bz, err := msg.Marshal()
	s.Require().NoError(err)

	s.consensus.Receive(cs.DataChannel, &p2pmocks.Peer{}, bz)
	return blockID
}

func (s *TestSuite) TestTallyVotes() {
	blockID := s.sendProposal()

	testCases := []struct {
		name       string
		votes      []*tm.Vote
		assertions func(suite *TestSuite)
	}{
		{
			name:  "receive a good single vote",
			votes: []*tm.Vote{s.genVote(2, 1, 0, blockID)},
			assertions: func(suite *TestSuite) {
				tally := suite.consensus.Tally(0)
				vote := tally.GetByIndex(2)
				suite.Require().NotNil(vote)
			},
		},
		{
			name: "received quorum without block",
			votes: []*tm.Vote{
				s.genVote(0, 1, 0, blockID),
				s.genVote(1, 1, 0, blockID),
				s.genVote(2, 1, 0, blockID),
			},
			assertions: func(suite *TestSuite) {
				tally := suite.consensus.Tally(0)
				suite.Require().True(tally.HasTwoThirdsMajority())
			},
		},
		{
			name:  "received vote for a different height",
			votes: []*tm.Vote{s.genVote(0, 2, 0, blockID)},
			assertions: func(suite *TestSuite) {
				return
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			for _, vote := range tc.votes {
				voteMessage := &cs.VoteMessage{Vote: vote}
				msg, err := cs.MsgToProto(voteMessage)
				s.Require().NoError(err)
				bz, err := msg.Marshal()
				s.Require().NoError(err)
				s.consensus.Receive(cs.VoteChannel, &p2pmocks.Peer{}, bz)
			}
			tc.assertions(s)
		})
	}
}
