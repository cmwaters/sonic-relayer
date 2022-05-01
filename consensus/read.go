package consensus

import (
	tm "github.com/tendermint/tendermint/types"
)

func (s Service) ChainID() string {
	return s.chainID
}

func (s Service) Height() int64 {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.height
}

func (s Service) Proposals() map[int32]tm.BlockID {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.proposals
}

func (s Service) Tally(round int32) *tm.VoteSet {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	tally, ok := s.roundVoteSets[round]
	if !ok {
		return nil
	}
	return tally
}

func (s Service) PartSet(blockID string) *tm.PartSet {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	partSet, ok := s.partSets[blockID]
	if !ok {
		return nil
	}
	return partSet
}
