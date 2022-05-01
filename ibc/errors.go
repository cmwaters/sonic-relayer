package ibc

import "errors"

var (
	ErrMsgNotSupported      = errors.New("IBC message not supported.")
	ErrChannelNotConfigured = errors.New("this channel has not been configured for relaying.")
	ErrBroadcastingPackets  = errors.New("unable to broadcast packets")
)
