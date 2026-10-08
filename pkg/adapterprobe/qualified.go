// SPDX-License-Identifier: Apache-2.0
package adapterprobe

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"io"
	"net/url"
	"regexp"
	"runtime"

	"github.com/relux-works/curator-network-profiles/pkg/envpatch"
)

// QualifiedSchema versions the qualified-builds list document.
const QualifiedSchema = "relux-adapter-qualified-v1"

// embeddedQualifiedJSON is the shipped qualified-builds list. It is
// empty; releases add entries only for builds with reviewed central
// qualification evidence (see docs/harness-verification.md). The
// follow-up record for a qualified build is committed here.
//
//go:embed qualified.json
var embeddedQualifiedJSON []byte

// embeddedQualifiedBuilds is the parsed shipped list. A corrupt
// shipped document panics at load: the library cannot run with a list
// it cannot read, and silently dropping entries would hide the gap.
var embeddedQualifiedBuilds = mustParseQualified(embeddedQualifiedJSON)

var platformRE = regexp.MustCompile(`^[a-z0-9]+$`)

// QualifiedBuild is one centrally qualified build: the exact artifact
// digest plus the evidence provenance that justifies it. OS and Arch
// record the qualification runner's platform (GOOS/GOARCH spelling);
// EvidenceSHA256 is the SHA-256 of the reviewed evidence JSON and
// RunURL is the workflow run that produced it.
type QualifiedBuild struct {
	Harness        string `json:"harness"`
	Adapter        string `json:"adapter"`
	Entrypoint     string `json:"entrypoint"`
	Recipe         string `json:"recipe"`
	BinarySHA256   string `json:"binary_sha256"`
	OS             string `json:"os"`
	Arch           string `json:"arch"`
	EvidenceSHA256 string `json:"evidence_sha256"`
	RunURL         string `json:"run_url"`
}

// ParseQualified parses qualified-builds list bytes. The document must
// be a closed JSON object with exactly the members "schema" (string)
// and "builds" (array of closed entry objects); both are required.
// Each entry must carry exactly the members "harness", "adapter",
// "entrypoint", "recipe", "binary_sha256", "os", "arch",
// "evidence_sha256" and "run_url", all non-null strings. Member
// spellings are exact and case-sensitive, duplicates refuse, unknown
// members refuse, and trailing data refuses. Null, missing or
// wrong-typed members refuse as corrupt, never as an empty list. A
// single well-formed unsupported schema refuses as
// qualified_version_unsupported; any duplicate or ambiguous schema
// refuses as qualified_corrupt. Raw contents never enter refusals.
//
// Entries validate as qualified_corrupt unless the harness and
// entrypoint match the request identity syntax, the adapter is the
// implemented generic-env-v1, the recipe matches the revision syntax,
// both digests are 64 lowercase hex, os and arch use GOOS/GOARCH
// spelling, and the run URL is an absolute https URL without
// userinfo. An unknown schema with any invalid entry refuses as
// qualified_corrupt: the version reason needs a single well-formed
// document with every entry valid. JSON escapes that decode to the
// same member name meet the same duplicate check.
func ParseQualified(data []byte) ([]QualifiedBuild, error) {
	corrupt := &Unsupported{Reason: "qualified_corrupt"}
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, corrupt
	}
	seen := make(map[string]bool, 2)
	var schema string
	var builds []QualifiedBuild
	schemaSet, listSet := false, false
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, corrupt
		}
		key, ok := tok.(string)
		if !ok {
			return nil, corrupt
		}
		if seen[key] {
			return nil, corrupt
		}
		seen[key] = true
		switch key {
		case "schema":
			var s *string
			if err := dec.Decode(&s); err != nil || s == nil {
				return nil, corrupt
			}
			schema, schemaSet = *s, true
		case "builds":
			tok, err := dec.Token()
			if err != nil || tok != json.Delim('[') {
				return nil, corrupt
			}
			var list []QualifiedBuild
			for dec.More() {
				entry, ok := parseQualifiedEntry(dec)
				if !ok {
					return nil, corrupt
				}
				list = append(list, entry)
			}
			tok, err = dec.Token()
			if err != nil || tok != json.Delim(']') {
				return nil, corrupt
			}
			builds, listSet = list, true
		default:
			return nil, corrupt
		}
	}
	tok, err = dec.Token()
	if err != nil || tok != json.Delim('}') {
		return nil, corrupt
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, corrupt
	}
	if !schemaSet || !listSet {
		return nil, corrupt
	}
	if builds == nil {
		builds = []QualifiedBuild{}
	}
	for _, b := range builds {
		if !validQualifiedBuild(b) {
			return nil, corrupt
		}
	}
	if schema == "" {
		return nil, corrupt
	}
	if schema != QualifiedSchema {
		return nil, &Unsupported{Reason: "qualified_version_unsupported"}
	}
	return builds, nil
}

// parseQualifiedEntry parses one closed entry object. Every member is
// a required non-null string; duplicates and unknown members refuse.
func parseQualifiedEntry(dec *json.Decoder) (QualifiedBuild, bool) {
	var entry QualifiedBuild
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return entry, false
	}
	seen := make(map[string]bool, 9)
	set := make(map[string]bool, 9)
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return entry, false
		}
		key, ok := tok.(string)
		if !ok {
			return entry, false
		}
		if seen[key] {
			return entry, false
		}
		seen[key] = true
		var s *string
		if err := dec.Decode(&s); err != nil || s == nil {
			return entry, false
		}
		set[key] = true
		switch key {
		case "harness":
			entry.Harness = *s
		case "adapter":
			entry.Adapter = *s
		case "entrypoint":
			entry.Entrypoint = *s
		case "recipe":
			entry.Recipe = *s
		case "binary_sha256":
			entry.BinarySHA256 = *s
		case "os":
			entry.OS = *s
		case "arch":
			entry.Arch = *s
		case "evidence_sha256":
			entry.EvidenceSHA256 = *s
		case "run_url":
			entry.RunURL = *s
		default:
			return entry, false
		}
	}
	tok, err = dec.Token()
	if err != nil || tok != json.Delim('}') {
		return entry, false
	}
	if len(set) != 9 {
		return entry, false
	}
	return entry, true
}

func validQualifiedBuild(b QualifiedBuild) bool {
	if !identityRE.MatchString(b.Harness) || b.Adapter != envpatch.AdapterGeneric || !identityRE.MatchString(b.Entrypoint) || !recipeRE.MatchString(b.Recipe) {
		return false
	}
	if !digestRE.MatchString(b.BinarySHA256) || !digestRE.MatchString(b.EvidenceSHA256) {
		return false
	}
	if !platformRE.MatchString(b.OS) || !platformRE.MatchString(b.Arch) {
		return false
	}
	u, err := url.Parse(b.RunURL)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil
}

func mustParseQualified(data []byte) []QualifiedBuild {
	builds, err := ParseQualified(data)
	if err != nil {
		panic("adapterprobe: embedded qualified list is unusable: " + err.Error())
	}
	return builds
}

// QualifiedPlatformEligible is the library-side eligibility predicate
// for a parsed qualified-build record: a record never qualifies a
// build on another platform. A record authorizes an evaluation only
// when its OS and architecture both match the current platform
// (GOOS/GOARCH spelling); identical bytes do not erase the
// architecture distinction, since one digest can execute different
// architecture slices. This predicate is necessary but not
// sufficient: a recorded hash is not a qualification until it is
// verified and eligible, which additionally requires reviewed
// pass-and-eligible evidence (see docs/harness-verification.md) and
// Evaluate's own guards — a corrupt list, known-bad membership and
// an active pin all refuse before any allowlist entry.
func QualifiedPlatformEligible(b QualifiedBuild) bool {
	return b.OS == runtime.GOOS && b.Arch == runtime.GOARCH
}

// EmbeddedQualified returns the shipped qualified builds as allowlist
// entries keyed by binary SHA-256: records are filtered by OS and
// architecture for the current platform before deduplication, and the
// first platform-matching entry wins when a digest repeats. Consumers
// merge the result into Policy.Allowlist; Evaluate then qualifies an
// exact tuple, recipe, platform and scope match under strict and
// sensitive-egress policies. Known-bad membership and an active pin
// still take precedence over every entry. The evidence digest and run
// URL stay in the parsed record for audit and do not enter the
// entry. The shipped list is currently empty.
func EmbeddedQualified() []AllowedBuild {
	return qualifiedAllowlist(embeddedQualifiedBuilds)
}

func qualifiedAllowlist(builds []QualifiedBuild) []AllowedBuild {
	var out []AllowedBuild
	seen := make(map[string]bool, len(builds))
	for _, b := range builds {
		// Platform confinement precedes "first digest wins": a
		// foreign-platform row must never hide a matching row for
		// the same digest, and must never contribute an entry here.
		if !QualifiedPlatformEligible(b) {
			continue
		}
		if seen[b.BinarySHA256] {
			continue
		}
		seen[b.BinarySHA256] = true
		build, err := BuildID(b.BinarySHA256)
		if err != nil {
			// Parsed entries always carry valid digests; anything
			// else fails closed by contributing no entry.
			continue
		}
		out = append(out, AllowedBuild{
			Adapter:  Identity{Adapter: b.Adapter, Harness: b.Harness, Entrypoint: b.Entrypoint},
			BuildID:  build,
			Recipe:   b.Recipe,
			Platform: b.OS,
			Scope:    TransportScope,
		})
	}
	return out
}
