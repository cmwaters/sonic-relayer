package relayer

type Config struct {
	GlobalConfig
	Chains []ChainConfig
}

type GlobalConfig struct {
	RootDir  string
	Moniker  string
	MaxPeers int
}

type ChainConfig struct {
	Name            string
	ID              string
	ListenAddress   string
	ExternalAddress string
	Endpoints       []string
	AppVersion      uint64
}
