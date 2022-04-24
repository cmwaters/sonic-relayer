package relayer

import (
	"context"

	"github.com/plural-labs/sonic-relayer/router"
)

// Relay is the top level function taking a context and config and
// relaying packets across multiple chains
func Relay(ctx context.Context, cfg *Config) error {
	var err error

	networks := make(map[string]*Network, len(cfg.Chains))
	mempools := make(map[string]*router.Mempool, len(cfg.Chains))
	for _, chain := range cfg.Chains {
		networks[chain.ID], err = NewNetwork(cfg.GlobalConfig, chain)
		if err != nil {
			return err
		}
		mempools[chain.ID] = router.NewMempool()
	}
	// router := router.New(mempools)

	// engines := make(map[string]*consensus.Service)
	// for _, chain := range cfg.Chains {
	// 	handler := ibc.NewHandler()
	// 	engines[chain.ID] = consensus.NewService(chain.ID, 1, handler)
	// }

	return nil
}
