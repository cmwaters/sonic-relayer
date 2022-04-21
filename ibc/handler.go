package ibc

import (
	codec "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types/tx"
	transfer "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	channel "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/gogo/protobuf/proto"
	tm "github.com/tendermint/tendermint/types"
)

type Handler struct {
	signer keyring.Keyring
}

func NewHandler(signer keyring.Keyring) *Handler {
	return &Handler{
		signer: signer,
	}
}

func (h Handler) Execute(block tm.Block) (map[string]tm.Tx, error) {
	outboundTxs := make(map[string]sdk.Tx)
	for _, rawTx := range block.Data.Txs {
		msgs, err := DecodeRawTx(rawTx)
		if err != nil {
			return nil, err
		}

		for _, msg := range msgs {
			switch m := msg.(type) {
			case *transfer.MsgTransfer:
				packet, chainID, err := h.processTransferMsg(m)
				if err != nil {
					return nil, err
				}

				anyMsg, err := codec.NewAnyWithValue(packet)
				if err != nil {
					return nil, err
				}

				queue, ok := outboundTxs[chainID]
				if !ok {
					outboundTxs[chainID] = sdk.Tx{Body: &sdk.TxBody{Messages: []*codec.Any{anyMsg}}}
				} else {
					queue.Body.Messages = append(queue.Body.Messages, anyMsg)
				}

			}
		}
	}
	result := make(map[string]tm.Tx)
	for chain, tx := range outboundTxs {
		signedTx, err := h.Sign(tx)
		if err != nil {
			return nil, err
		}

		if err := tx.ValidateBasic(); err != nil {
			return nil, err
		}

		result[chain], err = h.PrepareTx(signedTx)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (h *Handler) Commit(blockID []byte) error {
	return nil
}

func (h Handler) Sign(tx sdk.Tx) (sdk.Tx, error) {
	return tx, nil
}

func (h Handler) PrepareTx(tx sdk.Tx) (tm.Tx, error) {
	bodyBytes, err := tx.Body.Marshal()
	if err != nil {
		return nil, err
	}
	authInfoBytes, err := tx.AuthInfo.Marshal()
	if err != nil {
		return nil, err
	}

	raw := &sdk.TxRaw{
		BodyBytes:     bodyBytes,
		AuthInfoBytes: authInfoBytes,
		Signatures:    tx.Signatures,
	}
	txBytes, err := proto.Marshal(raw)
	if err != nil {
		return nil, err
	}
	return txBytes, nil
}

func (h Handler) processTransferMsg(msg *transfer.MsgTransfer) (*channel.MsgRecvPacket, string, error) {
	return &channel.MsgRecvPacket{}, "", nil
}
