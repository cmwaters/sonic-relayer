package relayer

import (
	"context"
	"errors"

	ibcclient "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"

	"github.com/plural-labs/sonic-relayer/consensus"
	"github.com/plural-labs/sonic-relayer/ibc"
	"github.com/plural-labs/sonic-relayer/provider"
	"github.com/plural-labs/sonic-relayer/tx"
)

// Relay is the top level function taking a context and config and
// relaying packets across multiple chains
func Relay(ctx context.Context, cfg *Config) error {
	var err error

	signer, err := NewSigner(cfg)
	if err != nil {
		return err
	}

	// And God said to Noah, gimme two of everything

	providerA := provider.NewRPCClient([]string{cfg.ChainA.RPC})
	providerB := provider.NewRPCClient([]string{cfg.ChainB.RPC})

	mempoolA := tx.NewMempool()
	mempoolB := tx.NewMempool()

	infos, err := signer.List()
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		return errors.New("no keys present in keyring")
	}
	address := infos[0].GetAddress()

	accountant, err := ibc.NewAccountant(signer, address, nil)
	if err != nil {
		return err
	}

	// get the latest two validator sets and heights
	nextValSetA, heightA, err := providerA.ValidatorSet(ctx, nil)
	if err != nil {
		return err
	}
	currValSetA, heightA, err := providerA.ValidatorSet(ctx, &heightA)
	if err != nil {
		return err
	}
	nextValSetB, heightB, err := providerB.ValidatorSet(ctx, nil)
	if err != nil {
		return err
	}
	currValSetB, heightB, err := providerB.ValidatorSet(ctx, &heightB)
	if err != nil {
		return err
	}

	currValSetProtoA, err := currValSetA.ToProto()
	if err != nil {
		return err
	}
	currValSetProtoB, err := currValSetB.ToProto()
	if err != nil {
		return err
	}
	ibcStateA := &ibc.State{
		ChainID: cfg.ChainA.ID,
		Client: ibc.ClientState{
			ChainID: cfg.ChainB.ID,
			LastTrustedHeight: ibcclient.Height{
				RevisionNumber: cfg.ChainB.RevisionNumber,
				RevisionHeight: uint64(heightB),
			},
			LastTrustedValidators: currValSetProtoB,
		},
		Endpoint: ibc.Endpoint{
			ClientID:     cfg.ChainA.ClientID,
			ConnectionID: cfg.ChainA.ConnectionID,
			Channel: ibc.Channel{
				ChannelID: cfg.ChainA.ChannelID,
				PortID:    cfg.ChainA.PortID,
				Version:   cfg.ChainA.ChannelVersion,
			},
		},
	}
	ibcStateB := &ibc.State{
		ChainID: cfg.ChainB.ID,
		Client: ibc.ClientState{
			ChainID: cfg.ChainA.ID,
			LastTrustedHeight: ibcclient.Height{
				RevisionNumber: cfg.ChainA.RevisionNumber,
				RevisionHeight: uint64(heightA),
			},
			LastTrustedValidators: currValSetProtoA,
		},
		Endpoint: ibc.Endpoint{
			ClientID:     cfg.ChainB.ClientID,
			ConnectionID: cfg.ChainB.ConnectionID,
			Channel: ibc.Channel{
				ChannelID: cfg.ChainB.ChannelID,
				PortID:    cfg.ChainB.PortID,
				Version:   cfg.ChainB.ChannelVersion,
			},
		},
	}

	ibcHandlerA := ibc.NewHandler(mempoolB, accountant, ibcStateA, ibcStateB)
	ibcHandlerB := ibc.NewHandler(mempoolA, accountant, ibcStateB, ibcStateA)

	consensusA := consensus.NewService(cfg.ChainA.ID, heightA, ibcHandlerA, providerA, currValSetA, nextValSetA)
	consensusB := consensus.NewService(cfg.ChainB.ID, heightB, ibcHandlerB, providerB, currValSetB, nextValSetB)

	errCh := make(chan error, 2)
	go func() {
		err := runNetwork(ctx, cfg, cfg.ChainA, consensusA, mempoolA)
		if err != nil {
			errCh <- err
		}
	}()
	go func() {
		err := runNetwork(ctx, cfg, cfg.ChainB, consensusB, mempoolB)
		if err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}
