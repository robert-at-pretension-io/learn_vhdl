package policy_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestStaleVerificationTagBinding(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "stale_binding_fixture.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"stale_verification_tag_binding": "warning",
	})

	if !hasRule(result, "stale_verification_tag_binding") {
		t.Fatalf("expected stale_verification_tag_binding violation, got rules: %v", collectRules(result))
	}

	found := false
	for _, v := range result.Violations {
		if v.Rule != "stale_verification_tag_binding" {
			continue
		}
		if strings.Contains(v.Message, "arch:rtl") &&
			strings.Contains(v.Message, "fsm.legal_state") &&
			strings.Contains(v.Message, "state=rx_state") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected stale binding message to include scope, id, and state=rx_state; got: %v", result.Violations)
	}
}
