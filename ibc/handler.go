package ibc

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types/tx"
	ibcclient "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	"github.com/gogo/protobuf/proto"
	"github.com/plural-labs/sonic-relayer/router"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tm "github.com/tendermint/tendermint/types"
)

// The handler is the central component responsible for processing
// blocks from the consensus of a single network and routing the
// outbound IBC messages to the corresponding destination chains
// when a block is committed.
type Handler struct {
	// pendingTxs is a queue of outbound transactions
	// transactions will be ready to submit to the counterparty Mempool once 2/3 votes have been tallied
	pendingTxs []sdk.Tx

	// the IBC handler has write access to the counterparty Mempool
	counterpartyMempool router.Mempool

	// Each handler has write access to the Endpoint of the chain
	// it is receiving blocks on and read access to the counterparty chains Endpoint
	EndpointA Endpoint
	EndpointB CounterpartyReader
}

func NewHandler() *Handler {
	return &Handler{}
}

// Process takes a proposed block and scans for IBC messages,
// predicting the modules state transition if the block were to
// be committed and producing the packets needed to be sent
// to the respective chains
func (h Handler) Process(block *tm.Block) error {
	// var outboundTxs []sdk.Tx
	// proofCommitment := block.Hash().Bytes()

	// decode raw tx into sdk.Msg
	for _, rawTx := range block.Data.Txs {
		msgs, err := decodeRawTx(rawTx)
		if err != nil {
			return err
		}

		// parse IBC messages
		for _, msg := range msgs {
			fmt.Println(msg)
		}
	}

	// h.pendingTxs[string(proofCommitment)] = outboundTxs
	return nil
}

// Commit is called the moment a block is committed. The handler will
// retrieve the cached outbound IBC messages, generate the corresponding
// proof for that height, bundle this into a sdk transaction, sign it
// marshal it into bytes and deliver it to the router to be submitted
// on the destination chain. It will then make any updates to the handlers
// own state if necessary. This should never error.
func (h *Handler) Commit(blockID []byte, commit *tmproto.SignedHeader, valSet *tmproto.ValidatorSet) {
	/*
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
			}
		}

		// Update the IBC state of this specific chain from the pending transactions
		h.UpdateIBCState(blockID)
		Order     channeltypes.Order
	*/
}

func (h Handler) BroadcastPackets(header *ibcclient.Header, chainID string, tx sdk.Tx) error {
	// TODO: refactor to use Mempool directly
	/*
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
	*/
	return nil
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
