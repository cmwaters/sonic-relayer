package relayer

import (
	"context"
	"path/filepath"

	"github.com/tendermint/tendermint/config"
	cs "github.com/tendermint/tendermint/consensus"
	mpl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/p2p/pex"
	"github.com/tendermint/tendermint/version"

	"github.com/plural-labs/sonic-relayer/consensus"
	"github.com/plural-labs/sonic-relayer/router"
)

func runNetwork(
	ctx context.Context,
	cfg *Config,
	chain ChainConfig,
	consensus *consensus.Service,
	mempool *router.Mempool,
) error {
	chainDir := filepath.Join(cfg.RootDir, chain.Name)
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
		Version:       version.TMCoreSemVer,
		Channels: []byte{
			cs.DataChannel, cs.VoteChannel, cs.VoteSetBitsChannel,
			mpl.MempoolChannel,
		},
		Moniker: cfg.Moniker,
	}
	transport := p2p.NewMultiplexTransport(nodeInfo, *nodeKey, conn.DefaultMConnConfig())
	p2p.MultiplexTransportMaxIncomingConnections(20)(transport)
	network := p2p.NewSwitch(&config.P2PConfig{
		ListenAddress:   chain.ListenAddress,
		ExternalAddress: chain.ExternalAddress,
	}, transport)
	network.SetNodeInfo(nodeInfo)
	network.SetNodeKey(nodeKey)
	addrBook := pex.NewAddrBook(addressBookFile, false)
	network.SetAddrBook(addrBook)

	network.AddReactor("CONSENSUS", consensus)
	network.AddReactor("MEMPOOL", mempool)

	if err := network.Start(); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		err := network.Stop()
		if err != nil {
			return err
		}
		// wait for all services to shut down
		<- network.Quit()
		return nil
	}
}
