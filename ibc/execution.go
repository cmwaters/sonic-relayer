package ibc

import (
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfer "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	channel "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
)

var (
	ErrMsgNotSupported = errors.New("IBC message not supported")

	// To be used when an IBC message updates state but doesn't
	// produce any output packet that needs to be sent out
	ErrExecutionOnly = errors.New("IBC message is execute only")
)

func (h Handler) executeIBCMessage(msg sdk.Msg) (channel.Packet, string, error) {
	// TODO: add more IBC message types (both core and application)
	switch m := msg.(type) {
	case *transfer.MsgTransfer:
		return h.processTransferMsg(m)
	default:
		return channel.Packet{}, "", ErrMsgNotSupported
	}
}

// processTransferMsg simulates the execution of a transfer message, caching the state
// changes to IBC and producing the packet that needs to be sent to another chain
func (h Handler) processTransferMsg(msg *transfer.MsgTransfer) (channel.Packet, string, error) {
	return channel.Packet{}, "", nil
}