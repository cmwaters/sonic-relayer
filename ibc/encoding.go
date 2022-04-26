package ibc

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/gogo/protobuf/proto"
)

func decodeRawTx(txBytes []byte) ([]sdk.Msg, error) {
	var Tx tx.Tx
	if err := proto.Unmarshal(txBytes, &Tx); err != nil {
		return []sdk.Msg{}, err
	}

	return Tx.GetMsgs(), nil
}

func encodeTx(msg sdk.Msg) ([]byte, error) {
	var Tx tx.Tx

	if err := proto.Marshal(msg, &Tx); err != nil {
		return tx.Tx{}, err
	}

	return Tx, nil
}
