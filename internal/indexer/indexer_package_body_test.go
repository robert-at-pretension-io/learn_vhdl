package indexer

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
)

func TestPolicyInputIncludesPackageBodies(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixtureDir := filepath.Join(repoRoot, "testdata", "cross_file_semantic", "package_body_without_declaration", "fail")

	cfg := config.DefaultConfig()
	require := false
	cfg.Analysis.RequireLibraryMapping = &require
	disabled := false
	cfg.Analysis.Cache.Enabled = &disabled
	cfg.Libraries = map[string]config.LibraryConfig{
		"work": {
			Files: []string{filepath.Join(fixtureDir, "*.vhd")},
		},
	}

	idx := NewWithConfig(cfg)
	idx.JSONOutput = true
	idx.Output = io.Discard
	idx.KeepFacts = true

	if err := idx.Run(repoRoot); err != nil {
		t.Fatalf("run indexer: %v", err)
	}

	input := idx.buildPolicyInput()
	if len(input.PackageBodies) != 1 {
		t.Fatalf("expected 1 package body, got %d: %#v", len(input.PackageBodies), input.PackageBodies)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repo root not found from %s", dir)
		}
		dir = parent
	}
}
