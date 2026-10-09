// Package testserver supplies trusted HTTPS fixtures for offline provider tests.
package testserver

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"
)

// New starts an HTTPS server and temporarily trusts its certificate in the
// default transport, including clones made by the provider. It preserves TLS
// verification and redirect handling. Callers must not run in parallel because
// the default transport is process-global.
func New(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	previous := http.DefaultTransport
	transport, ok := previous.(*http.Transport)
	if !ok {
		server.Close()
		t.Fatalf("default transport has type %T, want *http.Transport", previous)
	}
	trusted := transport.Clone()
	if trusted.TLSClientConfig == nil {
		trusted.TLSClientConfig = &tls.Config{}
	}
	if trusted.TLSClientConfig.RootCAs == nil {
		trusted.TLSClientConfig.RootCAs = x509.NewCertPool()
	} else {
		trusted.TLSClientConfig.RootCAs = trusted.TLSClientConfig.RootCAs.Clone()
	}
	trusted.TLSClientConfig.RootCAs.AddCert(server.Certificate())
	http.DefaultTransport = trusted
	t.Cleanup(func() {
		server.Close()
		trusted.CloseIdleConnections()
		http.DefaultTransport = previous
	})
	return server
}
