package ibc

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"

	transfer "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	client "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	channel "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"

	ibcclient "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	"github.com/gogo/protobuf/proto"
	broadcast "github.com/plural-labs/sonic-relayer/tx"
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
	pendingTxs map[string][]sdk.Msg

	// The keyring is used to sign outbound transactions before
	// they are routed to the respective mempool
	accountant *Accountant

	// the IBC handler has write access to the counterparty Mempool
	counterpartyMempool *broadcast.Mempool

	// information on the two endpoints
	sourceChain       Endpoint
	counterpartyChain Endpoint

	//
	clientState   ClientState
	nextPacketSeq uint64
}

func NewHandler(counterpartyMempool *broadcast.Mempool, accountant *Accountant, sourceEndpoint, destinationEndpoint Endpoint) *Handler {
	return &Handler{
		counterpartyMempool: counterpartyMempool,
		pendingTxs:          make(map[string][]sdk.Msg),
		sourceChain:         sourceEndpoint,
		counterpartyChain:   destinationEndpoint,
		accountant:          accountant,
	}
}

// Process takes a proposed block and scans for IBC messages,
// predicting the modules state transition if the block were to
// be committed and producing the transactions that will be passed to the counterparty chains Mempool
func (h Handler) Process(block *tm.Block) error {
	var outboundMsgs []sdk.Msg
	proofCommitment := block.Hash().Bytes()

	// decode raw tx into sdk.Msg
	for _, rawTx := range block.Data.Txs {
		msgs, err := decodeRawTx(rawTx)
		if err != nil {
			return err
		}

		// parse IBC messages
		for _, msg := range msgs {
			switch m := msg.(type) {
			case *transfer.MsgTransfer:
				msg, err := h.processTransferMsg(m, block)
				if err != nil {
					return err
				}

				outboundMsgs = append(outboundMsgs, msg)
				return nil
			case *client.MsgUpdateClient:
				// h.processUpdateClientMsg()
			case *channel.MsgRecvPacket:
				// h.processOnRecvPacketMsg()
			default:
				return nil
			}
		}
	}

	h.pendingTxs[string(proofCommitment)] = outboundMsgs
	return nil
}

// processUpdateClientMsg takes an UpdateClientMsg as input and updates the ClientState accordingly
func (h Handler) processUpdateClientMsg(msg *client.MsgUpdateClient, block *tm.Block) error {
	// update h.ClientState
	return nil
}

// processOnRecvPacketMsg takes an MsgOnRecvPacket as input and returns the correspending msgOnAcknowledgementPacket
func (h Handler) processOnRecvPacketMsg(msg *channel.MsgRecvPacket, block *tm.Block) (*channel.MsgAcknowledgement, error) {
	// build and return msgOnAcknowledgementPacket
	return &channel.MsgAcknowledgement{}, nil
}

// processTransferMsg builds and returns the correspending msgOnRecvPacket for a given msgTransfer
func (h Handler) processTransferMsg(msg *transfer.MsgTransfer, block *tm.Block) (*channel.MsgRecvPacket, error) {
	// we want to just relayer between two chains for the time being (channels are decided at config level)
	if msg.SourceChannel != h.sourceChain.Channel.ChannelID {
		return nil, ErrChannelNotConfigured
	}

	// create MsgOnRecvPacket
	fullDenomPath := msg.Token.Denom
	packetData := transfer.NewFungibleTokenPacketData(
		fullDenomPath, msg.Token.Amount.String(), msg.Sender, msg.Receiver,
	)

	nextSeqSend := h.nextPacketSeq

	// incremement the send sequence for the next packet
	h.nextPacketSeq++

	packet := channel.NewPacket(
		packetData.GetBytes(),
		nextSeqSend,
		h.sourceChain.Channel.PortID,
		h.sourceChain.Channel.ChannelID,
		h.counterpartyChain.Channel.PortID,
		h.counterpartyChain.Channel.ChannelID,
		msg.TimeoutHeight,
		msg.TimeoutTimestamp,
	)

	address, err := h.accountant.HumanAddress(h.counterpartyChain.ChainID)
	if err != nil {
		return nil, err
	}

	recvMsg := &channel.MsgRecvPacket{
		Packet:          packet,
		ProofCommitment: block.Hash().Bytes(),
		ProofHeight: client.Height{
			RevisionNumber: h.clientState.LastTrustedHeight.RevisionNumber,
			RevisionHeight: uint64(block.Height),
		},
		Signer: address,
	}

	return recvMsg, nil
}

// Commit is called the moment a block is committed. The handler will
// retrieve the cached outbound IBC messages, generate the corresponding
// proof for that height, bundle this into a sdk transaction, sign it
// marshal it into bytes and deliver it to the counterparty chains Mempool
func (h *Handler) Commit(blockID []byte, commit *tmproto.SignedHeader, valSet *tmproto.ValidatorSet) {
	outboundTxs, ok := h.pendingTxs[string(blockID)]
	if !ok {
		panic(fmt.Sprintf("unexpected block committed (hash: %X)", blockID))
	}

	header := &ibcclient.Header{
		SignedHeader:      commit,
		ValidatorSet:      valSet,
		TrustedHeight:     h.clientState.LastTrustedHeight,
		TrustedValidators: h.clientState.LastTrustedValidators,
	}

	// broadcast pendingTxs to counterparty Mempool
	if err := h.BroadcastPackets(header, outboundTxs); err != nil {
		return
	}
}

func (h Handler) BroadcastPackets(header *ibcclient.Header, msgs []sdk.Msg) error {
	mempool := h.counterpartyMempool

	// retrieve the relayers account address on the destination chain
	address, err := h.accountant.HumanAddress(h.counterpartyChain.ChainID)
	if err != nil {
		return err
	}

	updateMsg, err := client.NewMsgUpdateClient(h.counterpartyChain.ClientID, header, address)
	if err != nil {
		return err
	}

	// put updateMsg at the front of the array
	msgs = append([]sdk.Msg{updateMsg}, msgs...)

	signedTx, err := h.accountant.PrepareAndSign(msgs, h.sourceChain.ChainID)
	if err != nil {
		return err
	}

	completedTx, err := h.PrepareTx(signedTx)
	if err != nil {
		return err
	}

	// broadcast tx
	mempool.BroadcastTx(completedTx)

	return nil
}

// PrepareTx marshals a sdk transaction into bytes
// so it can be routed to the respective mempool
func (h Handler) PrepareTx(t tx.Tx) (tm.Tx, error) {
	bodyBytes, err := t.Body.Marshal()
	if err != nil {
		return nil, err
	}
	authInfoBytes, err := t.AuthInfo.Marshal()
	if err != nil {
		return nil, err
	}

	raw := &tx.TxRaw{
		BodyBytes:     bodyBytes,
		AuthInfoBytes: authInfoBytes,
		Signatures:    t.Signatures,
	}

	txBytes, err := proto.Marshal(raw)
	if err != nil {
		return nil, err
	}

	return txBytes, nil
}
