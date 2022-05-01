package ibc

import (
	client "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// Endpoint represents a channel endpoint and its associated
// client and connections. It contains client, connection, and channel
// configuration parameters.
type Endpoint struct {
	ChainID      string
	ClientID     string
	ConnectionID string
	Channel      Channel

	// RevisionNumber is used as part of the height in packets.
	// It usually doesn't change unless a hard fork happens
	RevisionNumber uint64
}

type ClientState struct {
	// LastTrustedHeight is the last known trusted height
	LastTrustedHeight     client.Height
	LastTrustedValidators *tmproto.ValidatorSet
}

type Channel struct {
	ChannelID string
	PortID    string
	Version   string
}
