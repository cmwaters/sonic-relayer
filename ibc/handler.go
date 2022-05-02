package ibc

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/rs/zerolog/log"

	transfer "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	client "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	channel "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"

	ibcclient "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	"github.com/gogo/protobuf/proto"
	transaction "github.com/plural-labs/sonic-relayer/tx"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tm "github.com/tendermint/tendermint/types"
)

// The handler is the central component responsible for processing
// blocks from the consensus of a single network and routing the
// outbound IBC messages to the corresponding destination chains
// when a block is committed.
type Handler struct {
	// pendingActions maps proposed blocks to actions which will be triggered
	// once one of the blocks is committed. This includes outbound transactions
	// and client state updates which are derived from the transactions in that
	// block
	pendingActions map[string]*BlockActions

	// The keyring is used to sign outbound transactions before
	// they are routed to the respective mempool
	accountant *Accountant

	// the IBC handler has write access to the counterparty Mempool
	counterpartyMempool *transaction.Mempool

	// information on the ibc state of the source chain
	sourceChain *State

	// information on the ibc state of the counterparty chain. This should be read only
	counterpartyChain *State
}

func NewHandler(counterpartyMempool *transaction.Mempool, accountant *Accountant,
	sourceChain, counterpartyChain *State) *Handler {
	return &Handler{
		counterpartyMempool: counterpartyMempool,
		pendingActions:      make(map[string]*BlockActions),
		sourceChain:         sourceChain,
		counterpartyChain:   counterpartyChain,
		accountant:          accountant,
	}
}

// Process takes a proposed block and scans for IBC messages,
// predicting the modules state transition if the block were to
// be committed and producing the transactions that will be passed to the counterparty chains Mempool
func (h Handler) Process(block *tm.Block) error {
	blockActions := NewBlockActions(h.sourceChain.NextPacketSeq, block.Hash().Bytes(), block.Height)

	// decode raw tx into sdk.Msg
	for _, rawTx := range block.Data.Txs {
		msgs, err := decodeRawTx(rawTx)
		if err != nil {
			return err
		}

		// parse IBC messages
		var outboundMsg sdk.Msg
		for _, msg := range msgs {
			switch m := msg.(type) {
			case *transfer.MsgTransfer:
				outboundMsg, err = h.processTransferMsg(m, blockActions)
			case *client.MsgUpdateClient:
				err = h.processUpdateClientMsg(m, blockActions)
				outboundMsg = nil
			case *channel.MsgRecvPacket:
				outboundMsg, err = h.processOnRecvPacketMsg(m, blockActions)
			default:
				continue
			}
			// perform error handling
			switch err {
			case nil:
				if outboundMsg == nil {
					blockActions.OutboundTxs = append(blockActions.OutboundTxs, outboundMsg)
				}
			case ErrChannelNotConfigured, ErrClientNotSupported:
				log.Error().Err(err)
				continue
			default:
				return err
			}
		}
	}

	h.pendingActions[string(blockActions.ProofCommitment())] = blockActions
	return nil
}

// processUpdateClientMsg takes an UpdateClientMsg as input and updates the ClientState accordingly
func (h Handler) processUpdateClientMsg(msg *client.MsgUpdateClient, block *BlockActions) error {
	if msg.ClientId != h.counterpartyChain.Endpoint.ClientID {
		return ErrClientNotSupported
	}

	// check that the client state has not already been updated by an earlier transaction in the same block
	if block.ClientState != nil {
		return ErrClientAlreadyUpdated
	}

	// unmarhsal the header into a concrete type. Note we are assuming this is a tendermint header.
	// TODO: we may want to add better error handling
	var header ibcclient.Header
	if err := proto.Unmarshal(msg.Header.Value, &header); err != nil {
		return err
	}

	// TODO: we could possibly do some basic verification but currently we trust the message

	// update the client state
	block.ClientState = &ClientState{
		ChainID: h.sourceChain.Client.ChainID,
		LastTrustedHeight: client.Height{
			// We assume this doesn't change. TODO: check how this may change
			RevisionNumber: h.sourceChain.Client.LastTrustedHeight.RevisionNumber,
			RevisionHeight: uint64(header.Header.Height),
		},
		LastTrustedValidators: header.ValidatorSet,
	}
	return nil
}

// processOnRecvPacketMsg takes an MsgOnRecvPacket as input and returns the correspending msgOnAcknowledgementPacket
func (h Handler) processOnRecvPacketMsg(msg *channel.MsgRecvPacket, block *BlockActions) (*channel.MsgAcknowledgement, error) {
	// check that the onRecvMsg is to a channel that the handler can handle
	if msg.Packet.DestinationChannel != h.sourceChain.Endpoint.Channel.ChannelID {
		return nil, ErrChannelNotConfigured
	}

	// TODO: Currently, we can only execute transfer messages. In the future we may want to provide greater support to IBC modules
	// For now we assume that if this is the same channel ID then it is a msg of a transfer type.

	ack := channel.NewResultAcknowledgement([]byte{byte(1)})

	address, err := h.accountant.HumanAddress(h.counterpartyChain.Client.ChainID)
	if err != nil {
		return nil, err
	}

	// build and return msgOnAcknowledgementPacket
	return &channel.MsgAcknowledgement{
		Packet:          msg.Packet,
		Acknowledgement: ack.Acknowledgement(),
		ProofAcked:      nil,
		ProofHeight: client.Height{
			RevisionNumber: h.sourceChain.Client.LastTrustedHeight.RevisionNumber,
			RevisionHeight: block.Height(),
		},
		Signer: address,
	}, nil
}

// processTransferMsg builds and returns the correspending msgOnRecvPacket for a given msgTransfer
func (h Handler) processTransferMsg(msg *transfer.MsgTransfer, block *BlockActions) (*channel.MsgRecvPacket, error) {
	// we want to just relayer between two chains for the time being (channels are decided at config level)
	if msg.SourceChannel != h.sourceChain.Endpoint.Channel.ChannelID {
		return nil, ErrChannelNotConfigured
	}

	// create MsgOnRecvPacket
	fullDenomPath := msg.Token.Denom
	packetData := transfer.NewFungibleTokenPacketData(
		fullDenomPath, msg.Token.Amount.String(), msg.Sender, msg.Receiver,
	)

	packet := channel.NewPacket(
		packetData.GetBytes(),
		block.NextPacketSeq,
		h.sourceChain.Endpoint.Channel.PortID,
		h.sourceChain.Endpoint.Channel.ChannelID,
		h.counterpartyChain.Endpoint.Channel.PortID,
		h.counterpartyChain.Endpoint.Channel.ChannelID,
		msg.TimeoutHeight,
		msg.TimeoutTimestamp,
	)

	address, err := h.accountant.HumanAddress(h.counterpartyChain.Client.ChainID)
	if err != nil {
		return nil, err
	}

	recvMsg := &channel.MsgRecvPacket{
		Packet:          packet,
		ProofCommitment: block.ProofCommitment(),
		ProofHeight: client.Height{
			RevisionNumber: h.counterpartyChain.Client.LastTrustedHeight.RevisionNumber,
			RevisionHeight: block.Height(),
		},
		Signer: address,
	}

	// increment packet sequence
	block.NextPacketSeq++

	return recvMsg, nil
}

// Commit is called the moment a block is committed. The handler will
// retrieve the cached outbound IBC messages, generate the corresponding
// proof for that height, bundle this into a sdk transaction, sign it
// marshal it into bytes and deliver it to the counterparty chains Mempool
func (h *Handler) Commit(blockID []byte, commit *tmproto.SignedHeader, valSet *tmproto.ValidatorSet) {
	actions, ok := h.pendingActions[string(blockID)]
	if !ok {
		panic(fmt.Sprintf("unexpected block committed (hash: %X)", blockID))
	}

	header := &ibcclient.Header{
		SignedHeader:      commit,
		ValidatorSet:      valSet,
		TrustedHeight:     h.counterpartyChain.Client.LastTrustedHeight,
		TrustedValidators: h.counterpartyChain.Client.LastTrustedValidators,
	}

	// broadcast pendingTxs to counterparty Mempool
	if err := h.BroadcastPackets(header, actions.OutboundTxs); err != nil {
		return
	}

	// update state
	if actions.ClientState != nil {
		h.sourceChain.Client = *actions.ClientState
	}
	h.sourceChain.NextPacketSeq = actions.NextPacketSeq
	h.pendingActions = make(map[string]*BlockActions)
}

func (h Handler) BroadcastPackets(header *ibcclient.Header, msgs []sdk.Msg) error {
	mempool := h.counterpartyMempool

	// retrieve the relayers account address on the destination chain
	address, err := h.accountant.HumanAddress(h.counterpartyChain.ChainID)
	if err != nil {
		return err
	}

	updateMsg, err := client.NewMsgUpdateClient(h.counterpartyChain.Endpoint.ClientID, header, address)
	if err != nil {
		return err
	}

	// put updateMsg at the front of the array
	msgs = append([]sdk.Msg{updateMsg}, msgs...)

	signedTx, err := h.accountant.PrepareAndSign(msgs, h.counterpartyChain.ChainID)
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
