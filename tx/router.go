package tx

import (
	"errors"

	tm "github.com/tendermint/tendermint/types"
)

var ErrRouteDoesNotExist = errors.New("no route for transaction to specified chain")

// TxRouter is a simple wrapper around multiple mempools
type Router struct {
	mempools map[string]*Mempool
}

func New(mempools map[string]*Mempool) *Router {
	return &Router{
		mempools: mempools,
	}
}

func (r Router) HasRoute(chainID string) bool {
	_, ok := r.mempools[chainID]
	return ok
}

func (r Router) Send(chainID string, txs []tm.Tx) error {
	mempool, ok := r.mempools[chainID]
	if !ok {
		return ErrRouteDoesNotExist
	}
	for _, tx := range txs {
		mempool.BroadcastTx(tx)
	}
	return nil
}
