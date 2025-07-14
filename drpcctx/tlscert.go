package drpcctx

import (
	"context"
	"crypto/x509"
)

// TLSPeerCertKey is used to store TLS info in the context.
type TLSPeerCertKey struct{}

// WithPeerCertificate associates the peer certificate of the TLS connection
// with the context.
func WithPeerCertificate(ctx context.Context, certificate *x509.Certificate) context.Context {
	return context.WithValue(ctx, TLSPeerCertKey{}, certificate)
}

// GetPeerCertificate returns the TLS peer certificate associated with the context
// and a bool indicating if they existed.
func GetPeerCertificate(ctx context.Context) (*x509.Certificate, bool) {
	tlsInfo, ok := ctx.Value(TLSPeerCertKey{}).(*x509.Certificate)
	return tlsInfo, ok
}
