package drpcctx

import (
	"context"
	"crypto/x509"
)

// PeerConnectionInfo contains TLS peer connection information.
type PeerConnectionInfo struct {
	Certificates []*x509.Certificate
}

// peerConnInfoKey is used to store TLS peer connection info in the context.
type peerConnInfoKey struct{}

// WithPeerConnectionInfo associates the peer connection information of the TLS connection
// with the context.
func WithPeerConnectionInfo(ctx context.Context, info PeerConnectionInfo) context.Context {
	return context.WithValue(ctx, peerConnInfoKey{}, info)
}

// GetPeerConnectionInfo returns the TLS peer connection information associated with the
// context and a bool indicating if it existed.
func GetPeerConnectionInfo(ctx context.Context) (PeerConnectionInfo, bool) {
	peerConnectionInfo, ok := ctx.Value(peerConnInfoKey{}).(PeerConnectionInfo)
	return peerConnectionInfo, ok
}
