package dsl

import "github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack"

// L2CLNode wraps a stack.L2CLNode interface for DSL operations
type L2CLNode struct {
	commonImpl
	inner   stack.L2CLNode
	control stack.ControlPlane
}

// NewL2CLNode creates a new L2CLNode DSL wrapper
func NewL2CLNode(inner stack.L2CLNode, control stack.ControlPlane) *L2CLNode {
	return &L2CLNode{
		commonImpl: commonFromT(inner.T()),
		inner:      inner,
		control:    control,
	}
}

func (cl *L2CLNode) String() string {
	return cl.inner.ID().String()
}

// Escape returns the underlying stack.L2CLNode
func (cl *L2CLNode) Escape() stack.L2CLNode {
	return cl.inner
}

func (cl *L2CLNode) Restart() {
	cl.control.L2CLNodeState(cl.inner.ID(), stack.Start)
}

func (cl *L2CLNode) Stop() {
	cl.control.L2CLNodeState(cl.inner.ID(), stack.Stop)
}

func (cl *L2CLNode) WithP2PConnect(peer *L2CLNode) {
	peerInfo, err := peer.inner.P2PAPI().Self(cl.ctx)
	cl.require.NoError(err, "failed to fetch peer info")
	cl.require.NoError(cl.inner.P2PAPI().ConnectPeer(cl.ctx, peerInfo.Addresses[0]), "failed to connect peer")
	peerDump, err := cl.inner.P2PAPI().Peers(cl.ctx, true)
	cl.require.NoError(err, "failed to get peers")
	multiAddr := peerInfo.PeerID.String()
	_, ok := peerDump.Peers[multiAddr]
	cl.require.True(ok, "failed to connect peer")
}
