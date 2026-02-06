package policy_test

import (
	"path/filepath"
	"testing"
)

func TestVerificationWaiverSuppressesViolation(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "waived_cover_companion.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_cover_companion": "warning",
	})

	if hasRule(result, "missing_cover_companion") {
		t.Fatalf("expected missing_cover_companion to be waived, got rules: %v", collectRules(result))
	}

	if len(result.Waivers) == 0 {
		t.Fatalf("expected waiver records in output, got none")
	}

	found := false
	for _, waiver := range result.Waivers {
		if waiver.ID == "missing_cover_companion" && waiver.Scope == "arch:rtl" {
			found = true
			if waiver.File != fixture {
				t.Fatalf("expected waiver file %q, got %q", fixture, waiver.File)
			}
		}
	}
	if !found {
		t.Fatalf("expected waiver record for missing_cover_companion in arch:rtl")
	}
}
