// Package gateway is the backend boundary of spec/network-profiles.md N6
// and N14. Slice A has one backend, the operator-configured external HTTP
// proxy, which only returns the configured endpoint. The managed gateway
// of N-D (acquire / inspect / release, runtime-owned leases, generations)
// is declared here as the Managed interface and not implemented.
package gateway

import (
	"context"

	"github.com/relux-works/curator-network-profiles/pkg/netprofile"
	"github.com/relux-works/curator-network-profiles/pkg/refusal"
)

// Backend resolves the proxy endpoint a launch connects through.
type Backend interface {
	// Kind is the profile kind this backend serves.
	Kind() string
	// Endpoint returns the `http://host:port` endpoint for the profile.
	// For the external backend this is the configured endpoint; a managed
	// backend would return the endpoint of an acquired lease.
	Endpoint(ctx context.Context, p netprofile.Profile) (string, error)
}

// Lease is the N-D placeholder: the lease belongs to the owner of the
// actual runtime, not to a short-lived launcher that handed the session
// on; one agent finishing must not stop the others' proxy; a
// configuration change creates a new generation and never moves active
// sessions silently.
type Lease struct {
	ID         string `json:"id"`
	Owner      string `json:"owner"`
	Generation uint64 `json:"generation"`
	Endpoint   string `json:"endpoint"`
}

// Managed is the N-D backend surface, declared so callers can be written
// against it before a backend exists. No implementation ships in N-A.
type Managed interface {
	Backend
	Acquire(ctx context.Context, p netprofile.Profile, owner string) (Lease, error)
	Inspect(ctx context.Context, leaseID string) (Lease, error)
	Release(ctx context.Context, leaseID string) error
}

// External is the external-http-proxy backend of slice A.
type External struct{}

// Kind returns external-http-proxy.
func (External) Kind() string { return netprofile.KindExternalHTTPProxy }

// Endpoint returns the profile's configured endpoint. A profile of
// another kind is refused as network_scope_unsupported.
func (External) Endpoint(_ context.Context, p netprofile.Profile) (string, error) {
	if p.Kind != netprofile.KindExternalHTTPProxy {
		return "", refusal.New(refusal.CodeScopeUnsupported, p.Name, "profile kind is not served by the external-http-proxy backend")
	}
	if p.Endpoint == "" {
		return "", refusal.New(refusal.CodeProfileInvalid, p.Name, "profile has no endpoint")
	}
	return p.Endpoint, nil
}

// ForKind returns the backend serving kind; only external-http-proxy
// exists in N-A.
func ForKind(kind string) (Backend, error) {
	if kind == netprofile.KindExternalHTTPProxy {
		return External{}, nil
	}
	return nil, refusal.New(refusal.CodeScopeUnsupported, kind, "no backend serves this profile kind")
}
