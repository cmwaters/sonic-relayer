package relayer

type Config struct {
	RootDir string
	Moniker string
	// temporary: for prototyping
	Mnemonic string
	MaxPeers int
	ChainA   ChainConfig
	ChainB   ChainConfig
}

type ChainConfig struct {
	Name            string
	ID              string
	ListenAddress   string
	ExternalAddress string
	RPC             string
	AppVersion      uint64
}
