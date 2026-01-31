package indexer

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/policy"
)

func TestPolicyCacheRoundTripAndValidity(t *testing.T) {
	dir := t.TempDir()
	input := policy.Input{
		Standard: "2008",
		LintConfig: policy.LintRuleConfig{
			Rules: map[string]string{
				"entity_has_ports": "warning",
			},
		},
		ThirdPartyFiles: []string{"third_party.vhd"},
	}
	hash, err := policyConfigHash(input)
	if err != nil {
		t.Fatalf("policyConfigHash error: %v", err)
	}

	entry := policyCacheEntry{
		Version:    policyCacheVersion,
		ConfigHash: hash,
		Files:      []string{"a.vhd"},
		Result: policy.Result{
			Violations: []policy.Violation{
				{
					Rule:     "entity_has_ports",
					Severity: "warning",
					File:     "a.vhd",
					Line:     1,
					Message:  "Entity 'a' has no ports defined",
				},
			},
			Summary: policy.Summary{
				TotalViolations: 1,
				Warnings:        1,
			},
		},
	}

	if err := savePolicyCache(dir, entry); err != nil {
		t.Fatalf("savePolicyCache error: %v", err)
	}
	loaded, err := loadPolicyCache(dir)
	if err != nil {
		t.Fatalf("loadPolicyCache error: %v", err)
	}
	if !reflect.DeepEqual(entry, *loaded) {
		t.Fatalf("policy cache mismatch: expected %#v got %#v", entry, loaded)
	}
	ok, err := policyCacheValid(loaded, input, []string{"a.vhd"})
	if err != nil {
		t.Fatalf("policyCacheValid error: %v", err)
	}
	if !ok {
		t.Fatalf("expected cache to be valid")
	}

	input.Standard = "1993"
	ok, err = policyCacheValid(loaded, input, []string{"a.vhd"})
	if err != nil {
		t.Fatalf("policyCacheValid error: %v", err)
	}
	if ok {
		t.Fatalf("expected cache to be invalid after config change")
	}
}

func TestClearPolicyCache(t *testing.T) {
	dir := t.TempDir()
	entry := policyCacheEntry{
		Version:    policyCacheVersion,
		ConfigHash: "hash",
		Files:      []string{"a.vhd"},
		Result:     policy.Result{},
	}
	if err := savePolicyCache(dir, entry); err != nil {
		t.Fatalf("savePolicyCache error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "policy_cache.json")); err != nil {
		t.Fatalf("expected cache file to exist: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Analysis.Cache.Dir = dir
	if _, err := ClearPolicyCache(dir, cfg); err != nil {
		t.Fatalf("ClearPolicyCache error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "policy_cache.json")); !os.IsNotExist(err) {
		t.Fatalf("expected cache file to be removed, got err: %v", err)
	}
}
