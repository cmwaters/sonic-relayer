package tx_test

import (
	"testing"
	"time"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/proxy"

	"github.com/plural-labs/sonic-relayer/tx"
)

func TestBroadcastTx(t *testing.T) {
	N := 4
	reactors := make([]p2p.Reactor, N)
	mempools := make([]*mempool.CListMempool, N)
	logger := log.TestingLogger()
	for i := 0; i < N; i++ {
		app := kvstore.NewApplication()
		cc := proxy.NewLocalClientCreator(app)
		conn, _ := cc.NewABCIClient()
		cfg := config.TestMempoolConfig()
		mempools[i] = mempool.NewCListMempool(cfg, conn, 0)

		reactors[i] = mempool.NewReactor(cfg, mempools[i])
		reactors[i].SetLogger(logger)
	}
	relayerMempool := tx.NewMempool()
	relayerMempool.SetLogger(logger)

	p2p.MakeConnectedSwitches(config.TestP2PConfig(), N, func(idx int, s *p2p.Switch) *p2p.Switch {
		if idx == 0 {
			s.AddReactor("MEMPOOL", relayerMempool)
		} else {
			s.AddReactor("MEMPOOL", reactors[idx-1])
		}
		return s
	}, p2p.Connect2Switches)

	payload := []byte("transaction")
	relayerMempool.BroadcastTx(payload)

	ticker := time.NewTicker(100 * time.Millisecond)
	timeout := time.After(10 * time.Second)
	for i := 1; i < N; i++ {
		select {
		case <-ticker.C:
			txs := mempools[i].ReapMaxTxs(1)
			if len(txs) == 1 {
				continue
			}
		case <-timeout:
			t.Fatalf("Timed out waiting to receive transactions")
		}
	}

}
