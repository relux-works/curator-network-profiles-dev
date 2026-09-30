package binding_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/relux-works/curator-network-profiles/pkg/binding"
	"github.com/relux-works/curator-network-profiles/pkg/envpatch"
	"github.com/relux-works/curator-network-profiles/pkg/refusal"
)

var (
	idA    = binding.AdapterIdentity{Adapter: "generic-env-v1", Harness: "exec", Build: "", Entrypoint: "codex"}
	idB    = binding.AdapterIdentity{Adapter: "generic-env-v1", Harness: "exec", Build: "1.2.3", Entrypoint: "codex"}
	digest = "sha256:" + strings.Repeat("a", 64)
	other  = "sha256:" + strings.Repeat("b", 64)
)

func bind(ref, dig string, id binding.AdapterIdentity, assurance string) binding.Binding {
	return binding.Binding{ProfileRef: ref, ProfileDigest: dig, AdapterIdentity: id, Assurance: assurance,
		ResolvedEndpoint: "http://127.0.0.1:1", EnvPatch: envpatch.Patch{Set: []envpatch.Pair{{Name: "HTTP_PROXY", Value: "x"}}}}
}

func TestEqual(t *testing.T) {
	t.Parallel()
	base := bind("egress-a", digest, idA, binding.AssuranceCooperative)
	cases := []struct {
		name string
		b    binding.Binding
		want bool
	}{
		{"same", base, true},
		{"different name same digest", bind("egress-copy", digest, idA, binding.AssuranceCooperative), true},
		{"different endpoint and patch only", func() binding.Binding {
			b := base
			b.ResolvedEndpoint = "http://127.0.0.1:2"
			b.EnvPatch = envpatch.Patch{}
			return b
		}(), true},
		{"different digest", bind("egress-a", other, idA, binding.AssuranceCooperative), false},
		{"different adapter build", bind("egress-a", digest, idB, binding.AssuranceCooperative), false},
		{"different assurance", bind("egress-a", digest, idA, binding.AssuranceEnforced), false},
		{"empty digest never equal", bind("egress-a", "", idA, binding.AssuranceCooperative), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := binding.Equal(base, tc.b); got != tc.want {
				t.Fatalf("Equal = %v, want %v", got, tc.want)
			}
			if got := binding.RecordEqual(base.Record("explicit", nil), tc.b); got != tc.want {
				t.Fatalf("RecordEqual = %v, want %v", got, tc.want)
			}
		})
	}
	if binding.Equal(bind("a", "", idA, "cooperative"), bind("a", "", idA, "cooperative")) {
		t.Fatal("two empty digests compared equal")
	}
	if !idA.Equal(idA) || idA.Equal(idB) {
		t.Fatal("AdapterIdentity.Equal")
	}
}

func TestRecordIsManifestSafe(t *testing.T) {
	t.Parallel()
	b := bind("egress-a", digest, idA, binding.AssuranceCooperative)
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	rec := b.Record("runtime-default", &binding.ProbeRecord{TCP: "ok", Connect: "skipped", TLS: "skipped", CheckedAt: at})
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, forbidden := range []string{"HTTP_PROXY", "127.0.0.1", "env_patch", "\"unset\"", "\"set\"", "endpoint"} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("record leaks %q: %s", forbidden, s)
		}
	}
	want := `{"profile_ref":"egress-a","profile_digest":"` + digest + `","adapter_identity":{"adapter":"generic-env-v1","harness":"exec","build":"","entrypoint":"codex"},"assurance":"cooperative","origin":"runtime-default","probe":{"tcp":"ok","connect":"skipped","tls":"skipped","checked_at":"2026-10-01T12:00:00Z"}}`
	if s != want {
		t.Fatalf("record JSON =\n%s\nwant\n%s", s, want)
	}
	if raw2, _ := json.Marshal(b.Record("explicit", nil)); strings.Contains(string(raw2), "probe") {
		t.Fatal("nil probe rendered")
	}
}

func TestCheckReattachTable(t *testing.T) {
	t.Parallel()
	recorded := bind("egress-a", digest, idA, binding.AssuranceCooperative).Record("explicit", nil)
	cases := []struct {
		name    string
		fresh   binding.Binding
		event   binding.Event
		perSess bool
		outcome binding.Outcome
		code    string
	}{
		{"new launch", bind("egress-a", other, idB, "cooperative"), binding.EventNewLaunch, false, binding.OutcomeLaunch, ""},
		{"assignment equal", bind("egress-a", digest, idA, "cooperative"), binding.EventNewAssignment, false, binding.OutcomeAttach, ""},
		{"assignment equal under another name", bind("egress-copy", digest, idA, "cooperative"), binding.EventNewAssignment, false, binding.OutcomeAttach, ""},
		{"assignment differs with per-session route", bind("egress-b", other, idA, "cooperative"), binding.EventNewAssignment, true, binding.OutcomeSetPerSession, ""},
		{"assignment differs without per-session route", bind("egress-b", other, idA, "cooperative"), binding.EventNewAssignment, false, "", refusal.CodeScopeUnsupported},
		{"reattach equal", bind("egress-a", digest, idA, "cooperative"), binding.EventReattach, false, binding.OutcomeReattach, ""},
		{"reattach same name changed content", bind("egress-a", other, idA, "cooperative"), binding.EventReattach, true, "", refusal.CodeProfileDrift},
		{"reattach other profile", bind("egress-b", other, idA, "cooperative"), binding.EventReattach, true, "", refusal.CodeScopeUnsupported},
		{"reattach different adapter build", bind("egress-a", digest, idB, "cooperative"), binding.EventReattach, true, "", refusal.CodeScopeUnsupported},
		{"unknown event", bind("egress-a", digest, idA, "cooperative"), binding.Event("bogus"), true, "", refusal.CodeScopeUnsupported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, err := binding.CheckReattach(recorded, tc.fresh, tc.event, tc.perSess)
			if tc.code == "" {
				if err != nil || out != tc.outcome {
					t.Fatalf("got %q, %v; want %q", out, err, tc.outcome)
				}
				return
			}
			if out != "" {
				t.Fatalf("outcome %q beside error", out)
			}
			if c, _ := refusal.CodeOf(err); c != tc.code {
				t.Fatalf("code = %q (%v), want %q", c, err, tc.code)
			}
			if !strings.Contains(err.Error(), tc.fresh.ProfileRef) {
				t.Fatalf("subject missing: %v", err)
			}
		})
	}
}
