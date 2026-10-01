package cli

import (
	"errors"
	"github.com/relux-works/curator-network-profiles/pkg/refusal"
	"testing"
)

func TestUnexpectedFailureUsesClosedSanitizedCode(t *testing.T) {
	body := bodyOf(errors.New("private path /operator/secret"))
	if !refusal.Valid(body.Code) || body.Message == "private path /operator/secret" {
		t.Fatalf("unclosed or unsanitized error: %+v", body)
	}
}

func TestJSONBodySanitizesDirectRefusalSubjects(t *testing.T) {
	body := bodyOf(&refusal.Refusal{Code: refusal.CodeProfileUnknown, Subject: "http://u:pw@h:1", Detail: "unknown"})
	if body.Subject != "<invalid name>" || body.Message != "network_profile_unknown: <invalid name>: unknown" {
		t.Error("direct refusal leaked into JSON body")
	}
}
