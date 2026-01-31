package policy_test

import (
	"path/filepath"
	"testing"
)

func TestMissingCheckStructuredOutput(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "construct_fsm.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_verification_check": "warning",
	})

	if len(result.MissingChecks) == 0 {
		t.Fatalf("expected missing_checks output, got none")
	}

	found := false
	for _, task := range result.MissingChecks {
		if task.Scope != "arch:rtl" {
			continue
		}
		if !stringSliceContains(task.MissingIDs, "fsm.legal_state") {
			continue
		}
		if task.File != fixture {
			t.Fatalf("expected missing-check file %q, got %q", fixture, task.File)
		}
		if task.Bindings["state"] != "mode" {
			t.Fatalf("expected binding state=mode, got %v", task.Bindings)
		}
		if task.Anchor.LineStart < 1 || task.Anchor.LineEnd < task.Anchor.LineStart {
			t.Fatalf("expected anchor lines to be valid, got %+v", task.Anchor)
		}
		found = true
		break
	}

	if !found {
		t.Fatalf("expected missing-check task for fsm.legal_state in arch:rtl")
	}
}

func TestAmbiguousConstructWarning(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "ambiguous_ready_valid.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"ambiguous_construct": "warning",
	})

	if len(result.AmbiguousConstructs) == 0 {
		t.Fatalf("expected ambiguous_constructs output, got none")
	}
	if len(result.MissingChecks) != 0 {
		t.Fatalf("expected no missing checks for ambiguous ready/valid, got %v", result.MissingChecks)
	}
}

func stringSliceContains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
