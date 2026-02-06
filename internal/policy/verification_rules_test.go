package policy_test

import (
	"path/filepath"
	"testing"
)

func TestMissingVerificationBlock(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "missing_verification_block.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_verification_block": "warning",
	})

	if !hasRule(result, "missing_verification_block") {
		t.Fatalf("expected missing_verification_block violation, got rules: %v", collectRules(result))
	}
}

func TestInvalidVerificationTagMissingBinding(t *testing.T) {
	repoRoot := findRepoRoot(t)
	registry := filepath.Join(repoRoot, "testdata", "verification", "check_registry_minimal.json")
	t.Setenv("VHDL_CHECK_REGISTRY", registry)

	fixture := filepath.Join(repoRoot, "testdata", "verification", "invalid_tag_missing_binding.vhd")
	result := lintFile(t, repoRoot, fixture, map[string]string{
		"invalid_verification_tag": "warning",
	})

	if !hasRule(result, "invalid_verification_tag") {
		t.Fatalf("expected invalid_verification_tag violation, got rules: %v", collectRules(result))
	}
}

func TestStrayVerificationTag(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "stray_verification_tag.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"stray_verification_tag": "warning",
	})

	if !hasRule(result, "stray_verification_tag") {
		t.Fatalf("expected stray_verification_tag violation, got rules: %v", collectRules(result))
	}
}

func TestVerificationScopeMismatch(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "scope_mismatch.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"scope_mismatch": "warning",
	})

	if !hasRule(result, "scope_mismatch") {
		t.Fatalf("expected scope_mismatch violation, got rules: %v", collectRules(result))
	}
}
