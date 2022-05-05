package consensus

import (
	"github.com/rs/zerolog/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tm "github.com/tendermint/tendermint/types"
)

// addVote tallys new votes as they come in from peers
func (s *Service) addVote(vote *tm.Vote) {
	if vote.Height != s.height {
		log.Debug().Int64("state_height", s.height).Int64("vote_height", vote.Height).Msg("vote is for a different height")
		return
	}

	// ignore prevote messages
	if vote.Type != tmproto.PrecommitType {
		return
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	voteSet, ok := s.roundVoteSets[vote.Round]
	if !ok {
		// if we don't have a voteset for this round we need to create a new one
		voteSet = tm.NewVoteSet(s.chainID, s.height, vote.Round, tmproto.PrecommitType, s.currentValidators)
		s.roundVoteSets[vote.Round] = voteSet
	}

	if !vote.BlockID.IsZero() {
		if _, ok := s.partSets[vote.BlockID.Hash.String()]; !ok {
			log.Info().Msg("no proposal received for vote, creating a new part set header from vote")
			s.partSets[vote.BlockID.Hash.String()] = tm.NewPartSetFromHeader(vote.BlockID.PartSetHeader)
			s.proposals[vote.Round] = vote.BlockID
		}
	}

	// add the new vote to the vote set. This verifies the signature and tallys the voting power
	added, err := voteSet.AddVote(vote)
	if added {
		log.Info().Str("block_id", vote.BlockID.Hash.String()).Str("validator", string(vote.ValidatorAddress)).Msg("added vote")
	}
	if err != nil {
		log.Error().Err(err).Msg("adding vote")
		return
	}

	// check to see if the vote caused us to reach majority
	blockID, ok := voteSet.TwoThirdsMajority()
	if !ok {
		return
	}

	// TODO: we should also track polka's and prune prior rounds accrodingly so we don't
	// have a state explosion in the event that consensus has too many rounds

	// we have finalized a block!
	s.commit(blockID, voteSet)
}
