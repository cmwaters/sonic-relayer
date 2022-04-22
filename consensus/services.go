package consensus

import (
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tm "github.com/tendermint/tendermint/types"
)

type Provider interface {
	ValidatorSet(height int64) *tm.ValidatorSet
}

type Handler interface {
	Process(block *tm.Block) error
	Commit(blockID []byte, commit *tmproto.SignedHeader, valSet *tmproto.ValidatorSet)
}
