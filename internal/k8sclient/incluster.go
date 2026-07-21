// Package k8sclient provides a minimal in-cluster Kubernetes API client built on
// net/http — deliberately avoids client-go, matching the rest of fusion-index's
// direct-REST approach to the K8s API (see internal/api/middleware/auth.go).
package k8sclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
)

const (
	saTokenPath     = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	saCAPath        = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	saNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

	// APIBase is the in-cluster Kubernetes API server address.
	APIBase = "https://kubernetes.default.svc"
)

// NewHTTPClient returns an HTTP client that trusts the in-cluster CA.
func NewHTTPClient() (*http.Client, error) {
	ca, err := os.ReadFile(saCAPath)
	if err != nil {
		return nil, fmt.Errorf("read cluster CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("parse cluster CA certificate")
	}
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool},
		},
	}, nil
}

// ReadToken reads this pod's own service account token. Always re-read from disk
// rather than cached — kubelet rotates projected tokens.
func ReadToken() (string, error) {
	b, err := os.ReadFile(saTokenPath)
	if err != nil {
		return "", fmt.Errorf("read own SA token: %w", err)
	}
	return string(b), nil
}

// ReadNamespace returns the namespace this pod is running in, from the same
// projected service account volume as ReadToken/NewHTTPClient (mounted automatically
// whenever automountServiceAccountToken isn't disabled) — no downward API env var
// needed.
func ReadNamespace() (string, error) {
	b, err := os.ReadFile(saNamespacePath)
	if err != nil {
		return "", fmt.Errorf("read own namespace: %w", err)
	}
	return string(b), nil
}
