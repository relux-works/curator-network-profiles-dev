// SPDX-License-Identifier: Apache-2.0
package adapterprobe

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/relux-works/curator-network-profiles/pkg/refusal"
)

// Fixtures here are in-memory JSON documents and digests only. No test
// creates or executes binaries or scripts.

func qualifiedEntry(t *testing.T, digest, os, arch string) string {
	t.Helper()
	return `{"harness":"claude-code","adapter":"generic-env-v1","entrypoint":"exec","recipe":"claude-exec-v1","binary_sha256":"` + digest + `","os":"` + os + `","arch":"` + arch + `","evidence_sha256":"` + strings.Repeat("e", 64) + `","run_url":"https://example.invalid/runs/1"}`
}

func qualifiedDoc(schema string, entries ...string) []byte {
	return []byte(`{"schema":"` + schema + `","builds":[` + strings.Join(entries, ",") + `]}`)
}

func assertQualifiedErr(t *testing.T, err error, reason string) {
	t.Helper()
	var unsupported *Unsupported
	code, _ := refusal.CodeOf(err)
	if err == nil || !errors.Is(err, ErrUnsupported) || !errors.As(err, &unsupported) || unsupported.Reason != reason || code != refusal.CodeScopeUnsupported {
		t.Fatalf("error=%v; want %s", err, reason)
	}
	if strings.Contains(err.Error(), "example.invalid") || strings.Contains(err.Error(), strings.Repeat("a", 8)) {
		t.Fatalf("refusal leaks document bytes: %v", err)
	}
}

func TestParseQualified(t *testing.T) {
	good := strings.Repeat("a", 64)
	other := strings.Repeat("b", 64)
	builds, err := ParseQualified(qualifiedDoc(QualifiedSchema, qualifiedEntry(t, good, "linux", "amd64"), qualifiedEntry(t, other, "darwin", "arm64"), qualifiedEntry(t, good, "linux", "amd64")))
	if err != nil {
		t.Fatal(err)
	}
	if len(builds) != 3 {
		t.Fatalf("parsed %d builds; want 3", len(builds))
	}
	first := builds[0]
	if first.Harness != "claude-code" || first.Adapter != "generic-env-v1" || first.Entrypoint != "exec" || first.Recipe != "claude-exec-v1" || first.BinarySHA256 != good || first.OS != "linux" || first.Arch != "amd64" || first.EvidenceSHA256 != strings.Repeat("e", 64) || first.RunURL != "https://example.invalid/runs/1" {
		t.Fatalf("entry=%+v", first)
	}
	empty, err := ParseQualified(qualifiedDoc(QualifiedSchema))
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("empty list parsed to %+v", empty)
	}
}

func TestParseQualifiedRejects(t *testing.T) {
	good := strings.Repeat("a", 64)
	upper := strings.Repeat("A", 64)
	entry := qualifiedEntry(t, good, "linux", "amd64")
	entryOf := func(inner string) string { return `{"schema":"` + QualifiedSchema + `","builds":[` + inner + `]}` }
	for _, tc := range []struct {
		name   string
		data   string
		reason string
	}{
		{"empty", "", "qualified_corrupt"},
		{"blank", "  \n ", "qualified_corrupt"},
		{"not JSON", "{bad", "qualified_corrupt"},
		{"null", "null", "qualified_corrupt"},
		{"missing schema", `{"builds":[]}`, "qualified_corrupt"},
		{"missing list", `{"schema":"relux-adapter-qualified-v1"}`, "qualified_corrupt"},
		{"null list", `{"schema":"relux-adapter-qualified-v1","builds":null}`, "qualified_corrupt"},
		{"null schema", `{"schema":null,"builds":[]}`, "qualified_corrupt"},
		{"empty schema", `{"schema":"","builds":[]}`, "qualified_corrupt"},
		{"unknown version", string(qualifiedDoc("relux-adapter-qualified-v2")), "qualified_version_unsupported"},
		{"unknown version with entry", string(qualifiedDoc("relux-adapter-qualified-v2", entry)), "qualified_version_unsupported"},
		{"unknown version with null entry", `{"schema":"relux-adapter-qualified-v2","builds":[null]}`, "qualified_corrupt"},
		{"unknown version with invalid digest", string(qualifiedDoc("relux-adapter-qualified-v2", qualifiedEntry(t, "not-a-digest", "linux", "amd64"))), "qualified_corrupt"},
		{"unknown version with wrong adapter", `{"schema":"relux-adapter-qualified-v2","builds":[{"harness":"claude-code","adapter":"codex-env-v1","entrypoint":"exec","recipe":"claude-exec-v1","binary_sha256":"` + good + `","os":"linux","arch":"amd64","evidence_sha256":"` + good + `","run_url":"https://example.invalid/runs/1"}]}`, "qualified_corrupt"},
		{"escaped duplicate schema", `{"schema":"relux-adapter-qualified-v1","builds":[],"` + "\\u0073" + `chema":"relux-adapter-qualified-v1"}`, "qualified_corrupt"},
		{"escaped duplicate schema reversed", `{"` + "\\u0073" + `chema":"relux-adapter-qualified-v1","schema":"relux-adapter-qualified-v1","builds":[]}`, "qualified_corrupt"},
		{"escaped duplicate list", `{"schema":"relux-adapter-qualified-v1","builds":[],"build` + "\\u0073" + `":[]}`, "qualified_corrupt"},
		{"escaped duplicate list reversed", `{"schema":"relux-adapter-qualified-v1","build` + "\\u0073" + `":[],"builds":[]}`, "qualified_corrupt"},
		{"unknown key", `{"schema":"relux-adapter-qualified-v1","builds":[],"extra":1}`, "qualified_corrupt"},
		{"trailing data", `{"schema":"relux-adapter-qualified-v1","builds":[]} []`, "qualified_corrupt"},
		{"wrong list type", `{"schema":"relux-adapter-qualified-v1","builds":"` + good + `"}`, "qualified_corrupt"},
		{"null element", `{"schema":"relux-adapter-qualified-v1","builds":[null]}`, "qualified_corrupt"},
		{"string element", `{"schema":"relux-adapter-qualified-v1","builds":["` + good + `"]}`, "qualified_corrupt"},
		{"duplicate list erases entries", `{"schema":"relux-adapter-qualified-v1","builds":[` + entry + `],"builds":[]}`, "qualified_corrupt"},
		{"duplicate list erases entries reversed", `{"schema":"relux-adapter-qualified-v1","builds":[],"builds":[` + entry + `]}`, "qualified_corrupt"},
		{"duplicate schema hides version", `{"schema":"relux-adapter-qualified-v2","schema":"relux-adapter-qualified-v1","builds":[]}`, "qualified_corrupt"},
		{"duplicate schema hides version reversed", `{"schema":"relux-adapter-qualified-v1","schema":"relux-adapter-qualified-v2","builds":[]}`, "qualified_corrupt"},
		{"case alias list", `{"schema":"relux-adapter-qualified-v1","BUILDS":[]}`, "qualified_corrupt"},
		{"case alias schema", `{"Schema":"relux-adapter-qualified-v1","builds":[]}`, "qualified_corrupt"},
		{"case alias with canonical", `{"schema":"relux-adapter-qualified-v1","builds":[],"BUILDS":[]}`, "qualified_corrupt"},
		{"entry missing member", entryOf(`{"harness":"claude-code","adapter":"generic-env-v1","entrypoint":"exec","recipe":"claude-exec-v1","binary_sha256":"` + good + `","os":"linux","arch":"amd64","evidence_sha256":"` + good + `"}`), "qualified_corrupt"},
		{"entry null member", entryOf(`{"harness":"claude-code","adapter":"generic-env-v1","entrypoint":"exec","recipe":null,"binary_sha256":"` + good + `","os":"linux","arch":"amd64","evidence_sha256":"` + good + `","run_url":"https://example.invalid/runs/1"}`), "qualified_corrupt"},
		{"entry wrong-typed member", entryOf(`{"harness":"claude-code","adapter":"generic-env-v1","entrypoint":"exec","recipe":"claude-exec-v1","binary_sha256":"` + good + `","os":1,"arch":"amd64","evidence_sha256":"` + good + `","run_url":"https://example.invalid/runs/1"}`), "qualified_corrupt"},
		{"entry unknown member", entryOf(`{"harness":"claude-code","adapter":"generic-env-v1","entrypoint":"exec","recipe":"claude-exec-v1","binary_sha256":"` + good + `","os":"linux","arch":"amd64","evidence_sha256":"` + good + `","run_url":"https://example.invalid/runs/1","version":"2.1.290"}`), "qualified_corrupt"},
		{"entry duplicate member", entryOf(`{"harness":"claude-code","adapter":"generic-env-v1","entrypoint":"exec","recipe":"claude-exec-v1","binary_sha256":"` + good + `","os":"linux","arch":"amd64","evidence_sha256":"` + good + `","run_url":"https://example.invalid/runs/1","run_url":"https://example.invalid/runs/2"}`), "qualified_corrupt"},
		{"entry escaped duplicate member", entryOf(`{"harness":"claude-code","adapter":"generic-env-v1","entrypoint":"exec","recipe":"claude-exec-v1","binary_sha256":"` + good + `","os":"linux","arch":"amd64","evidence_sha256":"` + good + `","run_url":"https://example.invalid/runs/1","run_` + "\\u0075" + `rl":"https://example.invalid/runs/2"}`), "qualified_corrupt"},
		{"entry case alias member", entryOf(`{"harness":"claude-code","adapter":"generic-env-v1","entrypoint":"exec","recipe":"claude-exec-v1","binary_sha256":"` + good + `","OS":"linux","arch":"amd64","evidence_sha256":"` + good + `","run_url":"https://example.invalid/runs/1"}`), "qualified_corrupt"},
		{"bad harness syntax", entryOf(`{"harness":"bad harness!","adapter":"generic-env-v1","entrypoint":"exec","recipe":"claude-exec-v1","binary_sha256":"` + good + `","os":"linux","arch":"amd64","evidence_sha256":"` + good + `","run_url":"https://example.invalid/runs/1"}`), "qualified_corrupt"},
		{"specialized adapter", entryOf(`{"harness":"claude-code","adapter":"codex-env-v1","entrypoint":"exec","recipe":"claude-exec-v1","binary_sha256":"` + good + `","os":"linux","arch":"amd64","evidence_sha256":"` + good + `","run_url":"https://example.invalid/runs/1"}`), "qualified_corrupt"},
		{"bad entrypoint syntax", entryOf(`{"harness":"claude-code","adapter":"generic-env-v1","entrypoint":"","recipe":"claude-exec-v1","binary_sha256":"` + good + `","os":"linux","arch":"amd64","evidence_sha256":"` + good + `","run_url":"https://example.invalid/runs/1"}`), "qualified_corrupt"},
		{"bad recipe syntax", entryOf(`{"harness":"claude-code","adapter":"generic-env-v1","entrypoint":"exec","recipe":"BadRecipe","binary_sha256":"` + good + `","os":"linux","arch":"amd64","evidence_sha256":"` + good + `","run_url":"https://example.invalid/runs/1"}`), "qualified_corrupt"},
		{"short digest", entryOf(strings.Replace(entry, good, "abc", 1)), "qualified_corrupt"},
		{"uppercase digest", entryOf(strings.Replace(entry, good, upper, 1)), "qualified_corrupt"},
		{"non-hex digest", entryOf(strings.Replace(entry, good, strings.Repeat("z", 64), 1)), "qualified_corrupt"},
		{"build ID form", entryOf(strings.Replace(entry, good, "sha256-"+good, 1)), "qualified_corrupt"},
		{"bad os spelling", entryOf(strings.Replace(entry, `"os":"linux"`, `"os":"Linux"`, 1)), "qualified_corrupt"},
		{"empty os", entryOf(strings.Replace(entry, `"os":"linux"`, `"os":""`, 1)), "qualified_corrupt"},
		{"bad arch spelling", entryOf(strings.Replace(entry, `"arch":"amd64"`, `"arch":"x86_64"`, 1)), "qualified_corrupt"},
		{"bad evidence digest", entryOf(strings.Replace(entry, strings.Repeat("e", 64), "not-a-digest", 1)), "qualified_corrupt"},
		{"empty run URL", entryOf(strings.Replace(entry, "https://example.invalid/runs/1", "", 1)), "qualified_corrupt"},
		{"http run URL", entryOf(strings.Replace(entry, "https://example.invalid/runs/1", "http://example.invalid/runs/1", 1)), "qualified_corrupt"},
		{"hostless run URL", entryOf(strings.Replace(entry, "https://example.invalid/runs/1", "https:///runs/1", 1)), "qualified_corrupt"},
		{"run URL with userinfo", entryOf(strings.Replace(entry, "https://example.invalid/runs/1", "https://user@example.invalid/runs/1", 1)), "qualified_corrupt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseQualified([]byte(tc.data))
			assertQualifiedErr(t, err, tc.reason)
		})
	}
}

func TestEmbeddedQualifiedShippedEmpty(t *testing.T) {
	builds, err := ParseQualified(embeddedQualifiedJSON)
	if err != nil {
		t.Fatalf("shipped list does not parse: %v", err)
	}
	if len(builds) != 0 {
		t.Fatalf("shipped list is not empty: %+v", builds)
	}
	if got := EmbeddedQualified(); len(got) != 0 {
		t.Fatalf("EmbeddedQualified() = %+v; want empty", got)
	}
}

func foreignPlatform(t *testing.T) (os, arch string) {
	t.Helper()
	os, arch = "linux", "amd64"
	if runtime.GOOS == "linux" {
		os = "darwin"
	}
	if runtime.GOARCH == "amd64" {
		arch = "arm64"
	}
	if os == runtime.GOOS && arch == runtime.GOARCH {
		t.Fatalf("no foreign platform for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return os, arch
}

func TestEmbeddedQualifiedMerge(t *testing.T) {
	good := strings.Repeat("a", 64)
	other := strings.Repeat("b", 64)
	foreignOS, foreignArch := foreignPlatform(t)
	fixture, err := ParseQualified(qualifiedDoc(QualifiedSchema,
		qualifiedEntry(t, good, foreignOS, foreignArch),
		qualifiedEntry(t, good, runtime.GOOS, runtime.GOARCH),
		qualifiedEntry(t, other, runtime.GOOS, runtime.GOARCH),
		qualifiedEntry(t, other, foreignOS, runtime.GOARCH),
		qualifiedEntry(t, good, runtime.GOOS, foreignArch)))
	if err != nil {
		t.Fatal(err)
	}
	prev := embeddedQualifiedBuilds
	embeddedQualifiedBuilds = fixture
	defer func() { embeddedQualifiedBuilds = prev }()
	allow := EmbeddedQualified()
	if len(allow) != 2 {
		t.Fatalf("merged %d entries; want 2 keyed by digest after platform filtering", len(allow))
	}
	first := allow[0]
	wantBuild := "sha256-" + good
	if first.Adapter.Adapter != "generic-env-v1" || first.Adapter.Harness != "claude-code" || first.Adapter.Entrypoint != "exec" || first.BuildID != wantBuild || first.Recipe != "claude-exec-v1" || first.Platform != runtime.GOOS || first.Scope != TransportScope {
		t.Fatalf("entry=%+v", first)
	}
	if allow[1].BuildID != "sha256-"+other || allow[1].Platform != runtime.GOOS {
		t.Fatalf("second=%+v", allow[1])
	}
}

func TestEvaluateEmbeddedQualifiedStrictAndSensitive(t *testing.T) {
	req := claudeRequest()
	build, digest := digestOf(t, req)
	fixture, err := ParseQualified(qualifiedDoc(QualifiedSchema, qualifiedEntry(t, digest, runtime.GOOS, runtime.GOARCH)))
	if err != nil {
		t.Fatal(err)
	}
	prev := embeddedQualifiedBuilds
	embeddedQualifiedBuilds = fixture
	defer func() { embeddedQualifiedBuilds = prev }()
	dec, err := Evaluate(context.Background(), req, Policy{Mode: Strict, Allowlist: EmbeddedQualified()})
	assertDecision(t, dec, err, OutcomeQualified, "allowlisted", false)
	if dec.BuildID != build || dec.BinarySHA256 != digest {
		t.Fatalf("decision=%+v", dec)
	}
	dec, err = Evaluate(context.Background(), req, Policy{SensitiveEgress: true, Allowlist: EmbeddedQualified()})
	assertDecision(t, dec, err, OutcomeQualified, "allowlisted", false)
	// A different digest still refuses under strict and sensitive egress.
	mismatch, err := ParseQualified(qualifiedDoc(QualifiedSchema, qualifiedEntry(t, strings.Repeat("c", 64), runtime.GOOS, runtime.GOARCH)))
	if err != nil {
		t.Fatal(err)
	}
	embeddedQualifiedBuilds = mismatch
	dec, err = Evaluate(context.Background(), req, Policy{Mode: Strict, Allowlist: EmbeddedQualified()})
	assertDecision(t, dec, err, OutcomeRefused, "strict_miss", false)
	dec, err = Evaluate(context.Background(), req, Policy{SensitiveEgress: true, Allowlist: EmbeddedQualified()})
	assertDecision(t, dec, err, OutcomeRefused, "strict_miss", false)
	// A matching digest for another OS still refuses under strict and
	// sensitive egress.
	foreignOS, foreignArch := foreignPlatform(t)
	foreign, err := ParseQualified(qualifiedDoc(QualifiedSchema, qualifiedEntry(t, digest, foreignOS, runtime.GOARCH)))
	if err != nil {
		t.Fatal(err)
	}
	embeddedQualifiedBuilds = foreign
	dec, err = Evaluate(context.Background(), req, Policy{Mode: Strict, Allowlist: EmbeddedQualified()})
	assertDecision(t, dec, err, OutcomeRefused, "strict_miss", false)
	dec, err = Evaluate(context.Background(), req, Policy{SensitiveEgress: true, Allowlist: EmbeddedQualified()})
	assertDecision(t, dec, err, OutcomeRefused, "strict_miss", false)
	// A matching digest for another architecture still refuses under
	// strict and sensitive egress: a record never qualifies a build on
	// another platform, even with identical bytes.
	foreign, err = ParseQualified(qualifiedDoc(QualifiedSchema, qualifiedEntry(t, digest, runtime.GOOS, foreignArch)))
	if err != nil {
		t.Fatal(err)
	}
	embeddedQualifiedBuilds = foreign
	dec, err = Evaluate(context.Background(), req, Policy{Mode: Strict, Allowlist: EmbeddedQualified()})
	assertDecision(t, dec, err, OutcomeRefused, "strict_miss", false)
	dec, err = Evaluate(context.Background(), req, Policy{SensitiveEgress: true, Allowlist: EmbeddedQualified()})
	assertDecision(t, dec, err, OutcomeRefused, "strict_miss", false)
}

func TestEvaluateQualifiedDuplicateDigestAcrossPlatforms(t *testing.T) {
	req := claudeRequest()
	_, digest := digestOf(t, req)
	foreignOS, foreignArch := foreignPlatform(t)
	// A foreign-platform row for the same digest must not hide the
	// matching row: filtering precedes "first digest wins".
	fixture, err := ParseQualified(qualifiedDoc(QualifiedSchema,
		qualifiedEntry(t, digest, foreignOS, foreignArch),
		qualifiedEntry(t, digest, runtime.GOOS, runtime.GOARCH)))
	if err != nil {
		t.Fatal(err)
	}
	prev := embeddedQualifiedBuilds
	embeddedQualifiedBuilds = fixture
	defer func() { embeddedQualifiedBuilds = prev }()
	dec, err := Evaluate(context.Background(), req, Policy{Mode: Strict, Allowlist: EmbeddedQualified()})
	assertDecision(t, dec, err, OutcomeQualified, "allowlisted", false)
	dec, err = Evaluate(context.Background(), req, Policy{SensitiveEgress: true, Allowlist: EmbeddedQualified()})
	assertDecision(t, dec, err, OutcomeQualified, "allowlisted", false)
}
