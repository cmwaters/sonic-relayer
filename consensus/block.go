package consensus

import (
	"io/ioutil"

	"github.com/gogo/protobuf/proto"
	"github.com/rs/zerolog/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tm "github.com/tendermint/tendermint/types"
)

func (s *State) addBlockPart(height int64, round int32, part *tm.Part) {
	// ignore block parts from a different height
	if height != s.height {
		log.Debug().Msg("received block part from a different height")
		return
	}

	proposedBlockID, ok := s.proposals[round]
	if !ok {
		log.Debug().Msg("block part is for a proposal we haven't received yet")
		return
	}
	partSet, ok := s.partSets[proposedBlockID]
	if !ok {
		log.Error().Msg("don't have part set for corresponding proposal")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	added, err := partSet.AddPart(part)
	if err != nil {
		log.Error().Err(err)
		return
	}
	if !added {
		return
	}
	log.Info().Msg("added block part")

	if partSet.IsComplete() {
		bz, err := ioutil.ReadAll(partSet.GetReader())
		if err != nil {
			log.Error().Err(err)
		}

		var pbb = new(tmproto.Block)
		err = proto.Unmarshal(bz, pbb)
		if err != nil {
			log.Error().Err(err)
		}

		block, err := tm.BlockFromProto(pbb)
		if err != nil {
			log.Error().Err(err)
		}

		// save the block
		s.proposedBlocks[proposedBlockID] = block
		// get the ibc handler to preprocess the block
		err = s.ibc.Process(block)
		if err != nil {
			log.Error().Err(err)
		}

		for _, voteSet := range s.roundVoteSets {
			blockID, ok := voteSet.TwoThirdsMajority()
			if !ok {
				continue
			}
			// one of the rounds recieved 2/3 precommits. We can instanly commit
			s.commit(blockID, voteSet)
			return
		}
	}

}

func (s *State) handleProposal(proposal *tm.Proposal) {
	if proposal.Height != s.height {
		log.Debug().Msg("received proposal from a different height")
		return
	}

	if _, ok := s.partSets[proposal.BlockID.Hash.String()]; ok {
		log.Debug().Msg("received a duplicate proposal")
	}

	// Verify that the proposal's signature came from the expected proposer
	p := proposal.ToProto()
	val := s.currentValidators.CopyIncrementProposerPriority(proposal.Round)
	if !val.GetProposer().PubKey.VerifySignature(
		tm.ProposalSignBytes(s.chainID, p), proposal.Signature,
	) {
		log.Error().Msg("received proposal with invalid signature")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	// create partset from the proposal and throw out the proposal
	s.partSets[proposal.BlockID.Hash.String()] = tm.NewPartSetFromHeader(proposal.BlockID.PartSetHeader)
	s.proposals[proposal.Round] = proposal.BlockID.Hash.String()
}
