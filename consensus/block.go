package consensus

import (
	"context"
	"io/ioutil"

	"github.com/gogo/protobuf/proto"
	"github.com/rs/zerolog/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tm "github.com/tendermint/tendermint/types"
)

func (s *Service) addBlockPart(height int64, round int32, part *tm.Part) {
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
	partSet, ok := s.partSets[proposedBlockID.Hash.String()]
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
			return
		}

		var pbb = new(tmproto.Block)
		err = proto.Unmarshal(bz, pbb)
		if err != nil {
			log.Error().Err(err)
			return
		}

		block, err := tm.BlockFromProto(pbb)
		if err != nil {
			log.Error().Err(err)
			return
		}

		// Do a basic validation check. Note we don't actually properly verify the block
		// but trust that the honest majority will vote for a valid block
		if err := block.ValidateBasic(); err != nil {
			log.Error().Err(err).Msg("invalid proposed block")
			return
		}

		// sanity check that the block ID is the same
		if block.ChainID != s.chainID {
			log.Error().Str("ChainID", block.ChainID).Str("BlockID", block.Hash().String()).Msg("unexpected chain id for block")
			return
		}

		// save the block
		s.proposedBlocks[proposedBlockID.Hash.String()] = block
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

func (s *Service) handleProposal(proposal *tm.Proposal) {
	if proposal.Height != s.height {
		log.Debug().Msg("received proposal from a different height")
		return
	}

	if _, ok := s.partSets[proposal.BlockID.Hash.String()]; ok {
		log.Debug().Msg("received a duplicate proposal")
	}

	// Verify that the proposal's signature came from the expected proposer
	p := proposal.ToProto()
	var val *tm.ValidatorSet
	if proposal.Round == 0 {
		val = s.currentValidators.Copy()
	} else {
		val = s.currentValidators.CopyIncrementProposerPriority(proposal.Round)
	}
	if !val.GetProposer().PubKey.VerifySignature(
		tm.ProposalSignBytes(s.chainID, p), proposal.Signature,
	) {
		log.Error().Msg("received proposal with invalid signature")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	// create partset from the proposal and throw out the proposal
	s.partSets[proposal.BlockID.Hash.String()] = tm.NewPartSetFromHeader(proposal.BlockID.PartSetHeader)
	s.proposals[proposal.Round] = proposal.BlockID

	if s.nextValidators == nil {
		nextHeight := s.height + 1
		var err error
		s.nextValidators, _, err = s.provider.ValidatorSet(context.TODO(), &nextHeight)
		if err != nil {
			log.Error().Err(err)
		}
	}
}
