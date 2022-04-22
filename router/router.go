package router

import (
	tm "github.com/tendermint/tendermint/types"
)

type TxRouter struct {

}

func New() *TxRouter {
	return &TxRouter{}
}

func (r TxRouter) HasRoute(chainID string) bool {
	return false
}

func (r TxRouter) Send(chainID string, txs []tm.Tx) error {
	return nil
}