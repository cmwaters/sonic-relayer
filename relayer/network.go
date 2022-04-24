package relayer

import (
	"path/filepath"

	"github.com/tendermint/tendermint/config"
	cs "github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/p2p/pex"
	"github.com/tendermint/tendermint/version"
)

type Network struct {
	transport p2p.Transport
	sw        *p2p.Switch
	addrBook  pex.AddrBook
}

func NewNetwork(cfg GlobalConfig, chain ChainConfig) (*Network, error) {
	chainDir := filepath.Join(cfg.RootDir, chain.Name)
	addressBookFile := filepath.Join(chainDir, "peers.json")
	nodeKeyFile := filepath.Join(chainDir, "node.json")

	nodeKey, err := p2p.LoadOrGenNodeKey(nodeKeyFile)
	if err != nil {
		return nil, err
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
			mempool.MempoolChannel,
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

	return &Network{
		transport: transport,
		sw:        network,
		addrBook:  addrBook,
	}, nil
}
