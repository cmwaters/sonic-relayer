package relayer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tendermint/tendermint/config"
	cs "github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/libs/log"
	mpl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/p2p/pex"
	"github.com/tendermint/tendermint/version"

	"github.com/plural-labs/sonic-relayer/consensus"
	"github.com/plural-labs/sonic-relayer/tx"
)

func runNetwork(
	ctx context.Context,
	cfg *Config,
	chain ChainConfig,
	consensus *consensus.Service,
	mempool *tx.Mempool,
) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	chainDir := filepath.Join(home, cfg.RootDir, chain.Name)
	_, err = os.Stat(chainDir)
	if errors.Is(err, os.ErrNotExist) {
		os.MkdirAll(chainDir, 0777)
	}
	addressBookFile := filepath.Join(chainDir, "peers.json")
	nodeKeyFile := filepath.Join(chainDir, "node.json")

	nodeKey, err := p2p.LoadOrGenNodeKey(nodeKeyFile)
	if err != nil {
		return err
	}
	nodeInfo := p2p.DefaultNodeInfo{
		ProtocolVersion: p2p.NewProtocolVersion(
			version.P2PProtocol,
			version.BlockProtocol,
			chain.AppVersion,
		),
		DefaultNodeID: nodeKey.ID(),
		Network:       chain.ID,
		ListenAddr:    chain.ListenAddress,
		Version:       version.TMCoreSemVer,
		Channels: []byte{
			cs.StateChannel, cs.DataChannel, cs.VoteChannel, cs.VoteSetBitsChannel,
			mpl.MempoolChannel,
		},
		Moniker: cfg.Moniker,
	}
	if err = nodeInfo.Validate(); err != nil {
		return err
	}

	transport := p2p.NewMultiplexTransport(nodeInfo, *nodeKey, conn.DefaultMConnConfig())
	p2p.MultiplexTransportMaxIncomingConnections(20)(transport)
	network := p2p.NewSwitch(&config.P2PConfig{
		ListenAddress:   chain.ListenAddress,
		ExternalAddress: chain.ExternalAddress,
	}, transport)
	network.SetNodeInfo(nodeInfo)
	network.SetNodeKey(nodeKey)
	addr, err := p2p.NewNetAddressString(p2p.IDAddressString(nodeKey.ID(), chain.ListenAddress))
	if err != nil {
		return err
	}

	addrBook := pex.NewAddrBook(addressBookFile, false)
	addrBook.AddOurAddress(addr)

	netAddress, err := p2p.NewNetAddressString(chain.Peers)
	if err != nil {
		return err
	}
	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))
	if err := addrBook.AddAddress(netAddress, netAddress); err != nil {
		logger.Error("adding address", "err", err)
	}
	network.SetAddrBook(addrBook)

	consensus.SetLogger(logger)
	addrBook.SetLogger(logger)
	mempool.SetLogger(logger)
	network.SetLogger(logger)

	network.AddReactor("CONSENSUS", consensus)
	network.AddReactor("MEMPOOL", mempool)
	network.AddPersistentPeers([]string{chain.Peers})

	if err := transport.Listen(*addr); err != nil {
		return err
	}

	if err := network.Start(); err != nil {
		return err
	}
	logger.Info("Started relayer on chain", "chain", chain.Name)

	err = network.DialPeersAsync([]string{chain.Peers})
	if err != nil {
		return fmt.Errorf("could not dial peers: %w", err)
	}

	select {
	case <-ctx.Done():
		err := network.Stop()
		if err != nil {
			return err
		}
		// wait for all services to shut down
		<-network.Quit()
		return nil
	}
}
