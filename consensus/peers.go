package consensus

import (
	"sync"

	"github.com/tendermint/tendermint/p2p"
)

// PeerList is a concurrent list implmenetation for
// safely cycling through connected peers
// TODO: consider moving to a generic libs directory
type PeerList struct {
	mtx    sync.Mutex
	peers  []p2p.Peer
	cursor int
}

func NewPeerList() *PeerList {
	return &PeerList{
		peers:  make([]p2p.Peer, 0),
		cursor: 0,
	}
}

func (l *PeerList) Add(peer p2p.Peer) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.peers = append(l.peers, peer)
}

func (l *PeerList) Remove(peer p2p.Peer) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	for i := 0; i < len(l.peers); i++ {
		if l.peers[i].ID() == peer.ID() {
			l.peers = append(l.peers[:i], l.peers[i+1:]...)
			if l.cursor > 0 && i >= l.cursor {
				l.cursor--
			}
			return
		}
	}
}

func (l *PeerList) IsEmpty() bool {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	return len(l.peers) == 0
}

func (l *PeerList) Next() p2p.Peer {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	if len(l.peers) == 0 {
		return nil
	}
	l.cursor = (l.cursor + 1) % len(l.peers)
	return l.peers[l.cursor]
}
