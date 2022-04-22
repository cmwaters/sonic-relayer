package router

import (
	"errors"

	tm "github.com/tendermint/tendermint/types"
)

var ErrTxRouteDoesNotExist = errors.New("no route for transaction to specified chain")

// TxRouter is a simple wrapper around multiple mempools
type TxRouter struct {
	mempools map[string]*Mempool
}

func New(mempools map[string]*Mempool) *TxRouter {
	return &TxRouter{
		mempools: mempools,
	}
}

func (r TxRouter) HasRoute(chainID string) bool {
	_, ok := r.mempools[chainID]
	return ok
}

func (r TxRouter) Send(chainID string, txs []tm.Tx) error {
	mempool, ok := r.mempools[chainID]
	if !ok {
		return ErrTxRouteDoesNotExist
	}
	for _, tx := range txs {
		if err := mempool.BroadcastTx(tx); err != nil {
			return err
		}
	}
	return nil
}
