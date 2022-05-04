package relayer

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type Config struct {
	RootDir string `toml:"root_dir"`
	Moniker string `toml:"moniker"`
	// temporary: for prototyping
	Mnemonic string      `toml:"mnemonic"`
	MaxPeers int         `toml:"max_peers"`
	ChainA   ChainConfig `toml:"chain_a"`
	ChainB   ChainConfig `toml:"chain_b"`
}

type ChainConfig struct {
	Name            string `toml:"name"`
	ID              string `toml:"id"`
	ListenAddress   string `toml:"listen_address"`
	ExternalAddress string `toml:"external_address"`
	// comma separated list of tendermint peers
	Peers string `toml:"peers"`
	// This is used to query for information on other nodes
	RPC            string `toml:"rpc"`
	AppVersion     uint64 `toml:"app_version"`
	RevisionNumber uint64 `toml:"revision_number"`
	ClientID       string `toml:"client_id"`
	ConnectionID   string `toml:"connection_id"`
	ChannelID      string `toml:"channel_id"`
	PortID         string `toml:"port_id"`
	ChannelVersion string `toml:"channel_version"`
}

func LoadConfig(file string) (Config, error) {
	var config Config
	_, err := toml.DecodeFile(file, &config)
	if err != nil {
		return config, fmt.Errorf("failed to load config from %q: %w", file, err)
	}
	return config, nil
}
