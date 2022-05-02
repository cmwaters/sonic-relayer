package tx

import (
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	proto "github.com/tendermint/tendermint/proto/tendermint/mempool"
)

var _ p2p.Reactor = &Mempool{}

// Mempool is a simple send-only in memory pool for
// broadcasting transactions to connected peers on this chain
// It does not communicate to an application using `CheckTx`
// but rather assumes that transactions passed to it have
// been correctly constructed
type Mempool struct {
	p2p.BaseReactor

	mtx      sync.Mutex
	peerList map[string]p2p.Peer
}

func NewMempool() *Mempool {
	mempool := &Mempool{
		peerList: make(map[string]p2p.Peer),
	}
	mempool.BaseReactor = *p2p.NewBaseReactor("RelayerMempool", mempool)
	return mempool
}

// AddPeer implements the Reactor interface
func (m *Mempool) AddPeer(peer p2p.Peer) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.peerList[string(peer.ID())] = peer
}

// RemovePeer implements the Reactor interface
func (m *Mempool) RemovePeer(peer p2p.Peer, reason interface{}) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	delete(m.peerList, string(peer.ID()))
}

// Receive implements the Reactor interface. It is a no-op because
// the mempool is send only. In the future it would be nice to communicate
// to nodes whether it accepts inbound transactions or not to save on bandwidth
func (m *Mempool) Receive(chID byte, src p2p.Peer, msgBytes []byte) {}

// BroadcastTx loops through all peers in the mempools current directory
// and send each of them the marshalled transaction. It does not send
// the same transaction multiple times.
// NOTE: we intentionally only send one transaction at a time. Tendermint
// has temporarily disabled batch broadcasts.
func (m *Mempool) BroadcastTx(tx []byte) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	msg := proto.Message{
		Sum: &proto.Message_Txs{
			Txs: &proto.Txs{Txs: [][]byte{tx}},
		},
	}

	bz, err := msg.Marshal()
	if err != nil {
		panic(err)
	}

	for _, peer := range m.peerList {
		sent := peer.Send(mempool.MempoolChannel, bz)
		if !sent {
			log.Error().Str("peer", string(peer.ID())).Msg("peer buffer full - transaction not sent")
		}
	}
	log.Debug().Msg("broadcasted tx to all peers")
}
