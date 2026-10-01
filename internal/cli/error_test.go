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
