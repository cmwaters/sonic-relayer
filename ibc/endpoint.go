package ibc

import (
	client "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// CounterpartyReader exposes reading only functionality of an Endpoint
type CounterpartyReader interface {
	GetClientID() string
	GerRevisionNumber() uint64
}

// Endpoint represents a channel endpoint and its associated
// client and connections. It contains client, connection, and channel
// configuration parameters. Endpoint functions will utilize the parameters
// set in the configuration structs when executing IBC messages.
type Endpoint struct {
	ClientID     string
	ConnectionID string
	Channel      Channel

	// RevisionNumber is used as part of the height in packets.
	// It usually doesn't change unless a hard fork happens
	RevisionNumber uint64

	// The last trusted height and validator set
	LastTrustedHeight     client.Height
	LastTrustedValidators *tmproto.ValidatorSet
}

type Channel struct {
	ChannelID string
	PortID    string
	Version   string
}

func (e Endpoint) GetClientID() string {
	return e.ClientID
}

func (e Endpoint) GetRevisionNumber() uint64 {
	return e.RevisionNumber
}
