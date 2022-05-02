package relayer

import (
	"context"
	"errors"

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

	endpointA := &ibc.State{}
	endpointB := &ibc.State{}

	ibcHandlerA := ibc.NewHandler(mempoolB, accountant, endpointA, endpointB)
	ibcHandlerB := ibc.NewHandler(mempoolA, accountant, endpointA, endpointB)

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
