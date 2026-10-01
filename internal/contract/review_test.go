package contract_test

import (
	"strings"
	"testing"
)

func TestBuildAndDemoSafetyContract(t *testing.T) {
	for _, tc := range []struct {
		file     string
		required []string
	}{
		{"Makefile", []string{"go test -p 2 -count=1 -timeout 120s ./...", "go test -p 2 -race -count=1 -timeout 120s ./..."}},
		{".github/workflows/ci.yml", []string{"go test -p 2 -count=1 -timeout 120s ./...", "go test -p 2 -race -count=1 -timeout 120s ./..."}},
		{"scripts/demo-two-egresses.sh", []string{"XDG_CONFIG_HOME=", "XDG_CACHE_HOME=", "XDG_DATA_HOME=", "GOPROXY=off", "GOFLAGS=-mod=readonly", "GOCACHE", "GOMODCACHE", "make build", "parent-proxy-before", "parent-proxy-after", "cmp", "egress=A", "egress=B"}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			doc := repoFile(t, tc.file)
			for _, s := range tc.required {
				if !strings.Contains(doc, s) {
					t.Errorf("%s lacks %q", tc.file, s)
				}
			}
		})
	}
}

func TestErrorAndRecordDocumentation(t *testing.T) {
	appendix := repoFile(t, "spec/contract-appendix.md")
	for _, s := range []string{"relux-network-binding-record-v1", "absent `network` key", "cannot read or write", "curator-network-version-v1"} {
		if !strings.Contains(appendix, s) {
			t.Errorf("appendix lacks %q", s)
		}
	}
}
