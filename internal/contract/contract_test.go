package contract_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/relux-works/curator-network-profiles/internal/contract"
	"github.com/relux-works/curator-network-profiles/pkg/refusal"
)

// These guards read files outside the package directory, so `go test`
// may serve a cached PASS for a tree it never re-read. The Makefile
// runs the suite with -count=1 for that reason.

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	path := filepath.Join("..", "..", rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func TestVectorsFileMatchesCode(t *testing.T) {
	set, err := contract.Vectors()
	if err != nil {
		t.Fatal(err)
	}
	want, err := set.JSON()
	if err != nil {
		t.Fatal(err)
	}
	got := repoFile(t, filepath.Join("testdata", "contract", "vectors.json"))
	if got != string(want) {
		t.Fatalf("testdata/contract/vectors.json is stale; regenerate with `make vectors`")
	}
}

func TestVectorSetShape(t *testing.T) {
	t.Parallel()
	set, err := contract.Vectors()
	if err != nil {
		t.Fatal(err)
	}
	if set.Schema != "relux-network-profiles-v1" || len(set.Digest) < 4 {
		t.Fatalf("set = %+v", set)
	}
	byID := map[string]contract.DigestVector{}
	for _, d := range set.Digest {
		byID[d.ID] = d
		if !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(d.Digest) {
			t.Fatalf("%s digest %q", d.ID, d.Digest)
		}
	}
	if byID["D1"].Digest != byID["D3"].Digest || byID["D1"].Digest != byID["D5"].Digest || byID["D1"].Digest == byID["D2"].Digest || byID["D1"].Digest == byID["D4"].Digest {
		t.Fatal("digest relations between D1..D5 are not the documented ones")
	}
	if strings.Join(byID["D1"].SameAs, ",") != "D3,D5,D6,D8" {
		t.Fatalf("D1 same_digest_as = %v", byID["D1"].SameAs)
	}
	for _, q := range set.Equality {
		switch q.ID {
		case "Q2":
			if q.Refusal != refusal.CodeProfileDrift || q.Equal {
				t.Fatalf("Q2 = %+v", q)
			}
		case "Q3":
			if q.Refusal != refusal.CodeScopeUnsupported {
				t.Fatalf("Q3 = %+v", q)
			}
		case "Q4":
			if q.Outcome != "set-per-session" {
				t.Fatalf("Q4 = %+v", q)
			}
		case "Q1":
			if !q.Equal || q.Outcome != "reattach" {
				t.Fatalf("Q1 = %+v", q)
			}
		}
	}
	for _, g := range set.EngineHosts {
		if g.Covered == (g.Refusal != "") {
			t.Fatalf("%s: covered=%v refusal=%q", g.ID, g.Covered, g.Refusal)
		}
	}
	for _, e := range set.EnvPatch {
		if e.ID == "E7" && strings.Join(e.Env, "|") != strings.Join(e.Result, "|") {
			t.Fatalf("E7 changed the environment")
		}
		if e.ID == "E5" && strings.Join(e.Env, "|") != strings.Join(e.Result, "|") {
			t.Fatalf("E5 is not idempotent")
		}
	}
}

func TestAppendixMatchesVectors(t *testing.T) {
	set, err := contract.Vectors()
	if err != nil {
		t.Fatal(err)
	}
	doc := repoFile(t, filepath.Join("spec", "contract-appendix.md"))
	blocks, err := contract.ParseBlocks(doc)
	if err != nil {
		t.Fatal(err)
	}
	want, err := set.Blocks()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, b := range want {
		seen[b.ID] = true
		got, ok := blocks[b.ID]
		if !ok {
			t.Errorf("appendix lacks vector block %s", b.ID)
			continue
		}
		if !contract.SameJSON(got, b.JSON) {
			t.Errorf("appendix block %s differs from the generated vector; regenerate with `make vectors` and paste", b.ID)
		}
	}
	for id := range blocks {
		if !seen[id] {
			t.Errorf("appendix block %s is not a known vector", id)
		}
	}
	// Every digest printed anywhere in the appendix must be a vector digest.
	known := map[string]bool{}
	for _, d := range set.Digest {
		known[d.Digest] = true
	}
	for _, m := range regexp.MustCompile(`sha256:[0-9a-f]{64}`).FindAllString(doc, -1) {
		if !known[m] {
			t.Errorf("appendix prints %s, which no vector produces", m)
		}
	}
	for _, d := range set.Digest {
		if !containsRequiredText(doc, d.Digest) {
			t.Errorf("appendix never prints the digest of %s", d.ID)
		}
		if !containsRequiredText(doc, d.Canonical) {
			t.Errorf("appendix never prints the canonical bytes of %s", d.ID)
		}
	}
	for _, code := range refusal.Codes() {
		if !containsRequiredText(doc, "`"+code+"`") {
			t.Errorf("appendix does not list refusal code %s", code)
		}
	}
}
