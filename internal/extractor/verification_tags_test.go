package extractor

import (
	"strings"
	"testing"
)

func TestVerificationTagParsingQuotedAndCommaList(t *testing.T) {
	facts := extractVerificationFixture(t, "tag_parsing_fixture.vhd")

	if len(facts.VerificationTagErrors) != 1 {
		t.Fatalf("expected 1 tag error, got %#v", facts.VerificationTagErrors)
	}
	if !strings.Contains(strings.ToLower(facts.VerificationTagErrors[0].Message), "outside verification block") {
		t.Fatalf("expected stray tag error, got %#v", facts.VerificationTagErrors[0])
	}

	tag := findTagByID(t, facts.VerificationTags, "rv.stable_while_stalled")
	if tag.Bindings["valid"] != "valid signal" {
		t.Fatalf("expected quoted valid binding, got %q", tag.Bindings["valid"])
	}
	if tag.Bindings["ready"] != "ready" {
		t.Fatalf("expected ready binding, got %q", tag.Bindings["ready"])
	}

	arb := findTagByID(t, facts.VerificationTags, "arb.onehot0")
	if arb.Bindings["grants"] != "gnt_a,gnt_b,gnt_c" {
		t.Fatalf("expected comma list grants binding, got %q", arb.Bindings["grants"])
	}

	fsm := findTagByID(t, facts.VerificationTags, "fsm.legal_state")
	if fsm.Bindings["state"] != "state" {
		t.Fatalf("expected state binding, got %q", fsm.Bindings["state"])
	}
	if fsm.Bindings["note"] != "comma, list" {
		t.Fatalf("expected quoted note binding, got %q", fsm.Bindings["note"])
	}

	if hasTagWithState(facts.VerificationTags, "outside") {
		t.Fatalf("expected tag outside verification block to be ignored")
	}
}

func findTagByID(t *testing.T, tags []VerificationTag, id string) VerificationTag {
	t.Helper()
	for _, tag := range tags {
		if strings.EqualFold(tag.ID, id) {
			return tag
		}
	}
	t.Fatalf("tag %q not found in %#v", id, tags)
	return VerificationTag{}
}

func hasTagWithState(tags []VerificationTag, state string) bool {
	for _, tag := range tags {
		if val, ok := tag.Bindings["state"]; ok && strings.EqualFold(val, state) {
			return true
		}
	}
	return false
}
