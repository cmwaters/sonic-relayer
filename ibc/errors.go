package ibc

import "errors"

var (
	ErrMsgNotSupported      = errors.New("IBC message not supported.")
	ErrChannelNotConfigured = errors.New("this channel has not been configured for relaying.")
	ErrClientNotSupported   = errors.New("client is not supported")
	ErrBroadcastingPackets  = errors.New("unable to broadcast packets")
	ErrClientAlreadyUpdated = errors.New("client has already been updated in the same block")
)
