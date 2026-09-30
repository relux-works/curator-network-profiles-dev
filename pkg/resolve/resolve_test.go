package resolve_test

import (
	"strings"
	"testing"

	"github.com/relux-works/curator-network-profiles/pkg/netprofile"
	"github.com/relux-works/curator-network-profiles/pkg/refusal"
	"github.com/relux-works/curator-network-profiles/pkg/resolve"
)

const catalog = `schema = "relux-network-profiles-v1"
default = "egress-b"

[networks.egress-a]
kind         = "external-http-proxy"
endpoint     = "http://127.0.0.1:18081"
bypass_hosts = ["localhost", "127.0.0.1", "::1", "engine.local"]

[networks.egress-b]
kind         = "external-http-proxy"
endpoint     = "http://127.0.0.1:18082"
bypass_hosts = ["localhost", "127.0.0.1", "::1"]
`

func file(t *testing.T) *netprofile.File {
	t.Helper()
	f, err := netprofile.Parse([]byte(catalog))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// ledgerAll confirms every profile of f at its current digest.
func ledgerAll(f *netprofile.File) resolve.Confirmations {
	return resolve.ConfirmedFunc(func(name, digest string) bool {
		p, ok := f.Lookup(name)
		return ok && netprofile.Digest(p) == digest
	})
}

func TestResolveTable(t *testing.T) {
	t.Parallel()
	f := file(t)
	all := ledgerAll(f)
	cases := []struct {
		name      string
		file      *netprofile.File
		ledger    resolve.Confirmations
		req       resolve.Request
		unmanaged bool
		ref       string
		origin    resolve.Origin
		code      string
		detail    string
	}{
		{name: "explicit wins", file: f, ledger: all, req: resolve.Request{Explicit: "egress-a", RuntimeDefault: "egress-b", ProjectDefault: "egress-b"}, ref: "egress-a", origin: resolve.OriginExplicit},
		{name: "runtime default", file: f, ledger: all, req: resolve.Request{RuntimeDefault: "egress-a", ProjectDefault: "egress-b"}, ref: "egress-a", origin: resolve.OriginRuntimeDefault},
		{name: "project default", file: f, ledger: all, req: resolve.Request{ProjectDefault: "egress-a"}, ref: "egress-a", origin: resolve.OriginProjectDefault},
		{name: "operator default", file: f, ledger: all, req: resolve.Request{}, ref: "egress-b", origin: resolve.OriginOperatorDefault},
		{name: "whitespace is no selection", file: f, ledger: all, req: resolve.Request{Explicit: "  "}, ref: "egress-b", origin: resolve.OriginOperatorDefault},
		{name: "unmanaged without any selection", file: mustParse(t, "schema = \"relux-network-profiles-v1\"\n"), ledger: all, req: resolve.Request{}, unmanaged: true},
		{name: "unmanaged with missing file", file: nil, ledger: nil, req: resolve.Request{}, unmanaged: true},
		{name: "unknown with missing file", file: nil, ledger: nil, req: resolve.Request{Explicit: "egress-a"}, code: refusal.CodeProfileUnknown},
		{name: "unknown name", file: f, ledger: all, req: resolve.Request{Explicit: "egress-z"}, code: refusal.CodeProfileUnknown},
		{name: "unknown runtime default is refused not skipped", file: f, ledger: all, req: resolve.Request{RuntimeDefault: "egress-z", ProjectDefault: "egress-a"}, code: refusal.CodeProfileUnknown},
		{name: "allowed set admits", file: f, ledger: all, req: resolve.Request{Explicit: "egress-a", Allowed: []string{"egress-a"}}, ref: "egress-a", origin: resolve.OriginExplicit},
		{name: "allowed set denies explicit", file: f, ledger: all, req: resolve.Request{Explicit: "egress-a", Allowed: []string{"egress-b"}}, code: refusal.CodeProfileDenied, detail: "allowed set"},
		{name: "allowed set denies operator default", file: f, ledger: all, req: resolve.Request{Allowed: []string{"egress-a"}}, code: refusal.CodeProfileDenied},
		{name: "empty allowed set denies everything", file: f, ledger: all, req: resolve.Request{Explicit: "egress-a", Allowed: []string{}}, code: refusal.CodeProfileDenied},
		{name: "allowed check precedes existence", file: f, ledger: all, req: resolve.Request{Explicit: "egress-z", Allowed: []string{"egress-a"}}, code: refusal.CodeProfileDenied},
		{name: "unconfirmed is denied", file: f, ledger: resolve.None, req: resolve.Request{Explicit: "egress-a"}, code: refusal.CodeProfileDenied, detail: "unconfirmed"},
		{name: "nil ledger confirms nothing", file: f, ledger: nil, req: resolve.Request{Explicit: "egress-a"}, code: refusal.CodeProfileDenied, detail: "unconfirmed"},
		{name: "confirmed at an old digest is denied", file: f, ledger: resolve.ConfirmedFunc(func(name, digest string) bool { return name == "egress-a" && digest == "sha256:old" }), req: resolve.Request{Explicit: "egress-a"}, code: refusal.CodeProfileDenied, detail: "unconfirmed"},
		{name: "enforced refused", file: f, ledger: all, req: resolve.Request{Explicit: "egress-a", RequiredAssurance: "enforced"}, code: refusal.CodeScopeUnsupported, detail: "cooperative only"},
		{name: "bogus assurance refused", file: f, ledger: all, req: resolve.Request{Explicit: "egress-a", RequiredAssurance: "magic"}, code: refusal.CodeScopeUnsupported},
		{name: "engine hosts covered", file: f, ledger: all, req: resolve.Request{Explicit: "egress-a", EngineHosts: []string{"127.0.0.1", "LOCALHOST", "[::1]:11434", "Engine.Local"}}, ref: "egress-a", origin: resolve.OriginExplicit},
		{name: "engine host not covered", file: f, ledger: all, req: resolve.Request{Explicit: "egress-b", EngineHosts: []string{"127.0.0.1", "engine.local"}}, code: refusal.CodeConfigurationConflict, detail: "engine host engine.local"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res, err := resolve.Resolve(tc.file, tc.ledger, tc.req)
			if tc.code != "" {
				if err == nil {
					t.Fatalf("expected %s, got result %+v", tc.code, res)
				}
				if c, _ := refusal.CodeOf(err); c != tc.code {
					t.Fatalf("code = %s (%v), want %s", c, err, tc.code)
				}
				if tc.detail != "" && !strings.Contains(err.Error(), tc.detail) {
					t.Fatalf("detail %q not in %v", tc.detail, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.unmanaged {
				if res.Managed || res.Selection.ProfileRef != "" || res.Digest != "" {
					t.Fatalf("expected unmanaged, got %+v", res)
				}
				if res.Selection.RequiredAssurance != "cooperative" {
					t.Fatalf("assurance = %q", res.Selection.RequiredAssurance)
				}
				return
			}
			if !res.Managed || res.Selection.ProfileRef != tc.ref || res.Selection.Origin != tc.origin {
				t.Fatalf("got %+v, want %s/%s", res.Selection, tc.ref, tc.origin)
			}
			if res.Profile.Name != tc.ref || res.Digest != netprofile.Digest(res.Profile) || res.Assurance != "cooperative" {
				t.Fatalf("result = %+v", res)
			}
		})
	}
}

func mustParse(t *testing.T, doc string) *netprofile.File {
	t.Helper()
	f, err := netprofile.Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestSelectOnly(t *testing.T) {
	t.Parallel()
	f := file(t)
	sel, ok := resolve.Select(f, resolve.Request{Explicit: "does-not-exist"})
	if !ok || sel.ProfileRef != "does-not-exist" || sel.Origin != resolve.OriginExplicit {
		t.Fatalf("Select = %+v, %v", sel, ok)
	}
	if sel, ok := resolve.Select(nil, resolve.Request{}); ok || sel.ProfileRef != "" {
		t.Fatalf("Select(nil) = %+v, %v", sel, ok)
	}
	if sel, ok := resolve.Select(f, resolve.Request{RequiredAssurance: "enforced"}); !ok || sel.RequiredAssurance != "enforced" || sel.Origin != resolve.OriginOperatorDefault {
		t.Fatalf("Select = %+v, %v", sel, ok)
	}
}
