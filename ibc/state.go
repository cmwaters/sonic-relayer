package ibc

import (
	client "github.com/cosmos/ibc-go/modules/core/02-client/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// StateReader exposes reading only functionality of State
type StateReader interface {
	GetClientID() string
}

// State represents a reduced version of an IBC module
// of a specific chain. It contains the neessary fields
// for successfully relaying packets
type State struct {
	// clientID is the id of the chains IBC client that is used
	// to update the client
	ClientID string

	// trusted fields used to update the client
	TrustedHeight     client.Height
	TrustedValidators *tmproto.ValidatorSet
}

func (s State) GetClientID() string {
	return s.ClientID
}
