package consensus_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/mocks"

	cs "github.com/plural-labs/sonic-relayer/consensus"
)

func TestPeerList(t *testing.T) {
	peerList := cs.NewPeerList()

	require.True(t, peerList.IsEmpty())
	require.Nil(t, peerList.Next())

	peerA := &mocks.Peer{}
	peerA.On("ID").Return(p2p.ID("A"))

	peerB := &mocks.Peer{}
	peerB.On("ID").Return(p2p.ID("B"))

	peerList.Add(peerA)

	require.False(t, peerList.IsEmpty())

	require.Equal(t, peerA, peerList.Next())
	require.Equal(t, peerA, peerList.Next())

	peerList.Add(peerB)

	require.Equal(t, peerB, peerList.Next())
	require.Equal(t, peerA, peerList.Next())
	require.Equal(t, peerB, peerList.Next())

	peerList.Remove(peerA)

	require.Equal(t, peerB, peerList.Next())

	peerList.Remove(peerB)

	require.Nil(t, peerList.Next())
	require.True(t, peerList.IsEmpty())

	peerList.Remove(peerB)
}
