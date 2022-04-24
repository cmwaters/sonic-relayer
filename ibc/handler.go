package ibc

import (
	"fmt"
	"log"

	codec "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types/tx"
	client "github.com/cosmos/ibc-go/modules/core/02-client/types"
	ibcclient "github.com/cosmos/ibc-go/modules/light-clients/07-tendermint/types"
	v3client "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	channel "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/gogo/protobuf/proto"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tm "github.com/tendermint/tendermint/types"

	"github.com/plural-labs/sonic-relayer/router"
)

// The handler is the central component responsible for processing
// blocks from the consensus of a single network and routing the
// outbound IBC messages to the corresponding destination chains
// when a block is committed.
type Handler struct {
	// The keyring is used to sign outbound transactions before
	// they are routed to the respective mempool
	// TODO: This could be abstracted out as a separate component
	signer keyring.Keyring

	// revisionNumber is used as part of the height in packets.
	// It usually doesn't change unless a hard fork happens
	revisionNumber uint64

	// Multiple blocks can be proposed in any given height
	// The handler stores all of them indexed based on the
	// string encoding of the block's hash. When one of these
	// is eventually committed
	pendingTxs map[string]TxSet

	// TxRouter wraps multiple mempools allowing for each hander
	// to fire completed transactions across to other chains
	txRouter router.TxRouter

	// Each handler has write access to the IBC state of the chain
	// it is receiving blocks on and read access to all the IBC
	// states of counterparty chains.
	ibcState           State
	counterpartyStates map[string]StateReader
}

// A set of transactions paired with the respective
// IDs of the chains they should be submitted on
type TxSet map[string]sdk.Tx

func NewHandler() *Handler {
	return &Handler{}
}

// Process takes a perspective block and scans for IBC messages,
// predicting the modules state transition if the block were to
// be committed and producing the packets needed to be sent
// to the respective chains
func (h Handler) Process(block *tm.Block) error {
	outboundTxs := make(map[string]sdk.Tx)
	proofCommitment := block.Hash().Bytes()
	for _, rawTx := range block.Data.Txs {
		msgs, err := decodeRawTx(rawTx)
		if err != nil {
			return err
		}

		for _, msg := range msgs {
			packet, chainID, err := h.executeIBCMessage(msg)
			if err == ErrExecutionOnly {
				continue
			}
			if err != nil {
				// TODO: not sure if it's better to log an error and continue
				return err
			}

			msg := &channel.MsgRecvPacket{
				Packet:          packet,
				ProofCommitment: proofCommitment,
				ProofHeight: v3client.Height{
					RevisionNumber: h.revisionNumber,
					RevisionHeight: uint64(block.Height),
				},
				Signer: "",
			}

			anyMsg, err := codec.NewAnyWithValue(msg)
			if err != nil {
				return err
			}

			queue, ok := outboundTxs[chainID]
			if !ok {
				outboundTxs[chainID] = sdk.Tx{Body: &sdk.TxBody{Messages: []*codec.Any{anyMsg}}}
			} else {
				queue.Body.Messages = append(queue.Body.Messages, anyMsg)
			}
		}
	}
	h.pendingTxs[string(proofCommitment)] = outboundTxs
	return nil
}

// Commit is called the moment a block is committed. The handler will
// retrieve the cached outbound IBC messages, generate the corresponding
// proof for that height, bundle this into a sdk transaction, sign it
// marshal it into bytes and deliver it to the router to be submitted
// on the destination chain. It will then make any updates to the handlers
// own state if necessary. This should never error.
func (h *Handler) Commit(blockID []byte, commit *tmproto.SignedHeader, valSet *tmproto.ValidatorSet) {
	outboundTxs, ok := h.pendingTxs[string(blockID)]
	if !ok {
		panic(fmt.Sprintf("unexpected block committed (hash: %X)", blockID))
	}

	header := &ibcclient.Header{
		SignedHeader:      commit,
		ValidatorSet:      valSet,
		TrustedHeight:     h.ibcState.TrustedHeight,
		TrustedValidators: h.ibcState.TrustedValidators,
	}

	// Get all the pending outbound packets from that block and broadcast them
	// to the nodes of that network
	for chainID, tx := range outboundTxs {
		if err := h.BroadcastPackets(header, chainID, tx); err != nil {
			log.Printf("error broadcasting packet to %s, %w", chainID, err)
		}
	}

	// Update the IBC state of this specific chain from the pending transactions
	h.UpdateIBCState(blockID)
}

func (h Handler) BroadcastPackets(header *ibcclient.Header, chainID string, tx sdk.Tx) error {
	counterpartyState, ok := h.counterpartyStates[chainID]
	if !ok {
		panic(fmt.Sprintf("unknown ibc state for %s", chainID))
	}

	updateMsg, err := client.NewMsgUpdateClient(counterpartyState.GetClientID(), header, "")
	if err != nil {
		return err
	}

	updateAnyMsg, err := codec.NewAnyWithValue(updateMsg)
	if err != nil {
		return err
	}

	// add the client proof to the front of the messages.
	// This should execute first
	tx.Body.Messages = append([]*codec.Any{updateAnyMsg}, tx.Body.Messages...)

	signedTx, err := h.Sign(tx)
	if err != nil {
		return err
	}

	completedTx, err := h.PrepareTx(signedTx)
	if err != nil {
		return err
	}

	return h.txRouter.Send(chainID, []tm.Tx{completedTx})
}

// TODO: When processing a block we should cache the transactions that will update
// the part of the ibc state that is important for the relayer to function. When
// this function is called we should then commit the cached state.
func (h *Handler) UpdateIBCState(blockID []byte) {

}

// Sign takes a transaction and signs it appending the signature
// to the transaction.
func (h Handler) Sign(tx sdk.Tx) (sdk.Tx, error) {
	return tx, nil
}

// PrepareTx marshals a sdk transaction into bytes
// so it can be routed to the respective mempool
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
