package ibc

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	client "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// State is a consolidates on-chain representation of the IBC module needed for
// relaying packets
// TODO: This state only tracks data corresponding to a single other ibc based chain
type State struct {
	ChainID       string
	Client        ClientState
	Endpoint      Endpoint
	NextPacketSeq uint64
}

// Endpoint represents a channel endpoint and its associated
// client and connections. It contains client, connection, and channel
// configuration parameters.
type Endpoint struct {
	ClientID     string
	ConnectionID string
	Channel      Channel
}

// ClientState tracks the state of the on-chain tendermint light client which
// is tracking the validator set of another chain
type ClientState struct {
	ChainID               string
	LastTrustedHeight     client.Height
	LastTrustedValidators *tmproto.ValidatorSet
}

// Channel tracks the state of a single channel across a connection between two
// ibc based chains
type Channel struct {
	ChannelID string
	PortID    string
	Version   string
}

// BlockActions is a set of actions produced from processing a proposed block.
// These actions are triggered once a block is finalized
type BlockActions struct {
	// The set of transactions to be sent on other chains
	OutboundTxs []sdk.Msg

	// If the chains client which tracks the validator set
	// of another chain is updated we process it here
	ClientState *ClientState

	// For a particular block track the NextPacketSeq
	NextPacketSeq uint64

	// the hash and height of the block (used in the proofs) (immutable)
	hash   []byte
	height uint64
}

func NewBlockActions(nextPacketSeq uint64, blockHash []byte, height int64) *BlockActions {
	return &BlockActions{
		NextPacketSeq: nextPacketSeq,
		hash:          blockHash,
		height:        uint64(height),
	}
}

func (a *BlockActions) ProofCommitment() []byte {
	return a.hash
}

func (a *BlockActions) Height() uint64 {
	return a.height
}
