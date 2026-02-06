package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyProjectOverridesUsesProjectLibraries(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "proj")
	rtlDir := filepath.Join(projectDir, "rtl")
	if err := os.MkdirAll(rtlDir, 0o755); err != nil {
		t.Fatalf("mkdir rtl: %v", err)
	}
	core := filepath.Join(rtlDir, "core.vhd")
	if err := os.WriteFile(core, []byte("-- core"), 0o644); err != nil {
		t.Fatalf("write core: %v", err)
	}

	cfg := DefaultConfig()
	cfg.configPath = filepath.Join(root, "vhdl_lint.json")
	cfg.Projects = map[string]ProjectConfig{
		"proj": {
			Libraries: map[string]LibraryConfig{
				"corelib": {Files: []string{"rtl/*.vhd"}},
			},
		},
	}

	projectKey, ok := cfg.ApplyProjectOverrides(projectDir)
	if !ok {
		t.Fatalf("expected project override to apply")
	}
	if projectKey != "proj" {
		t.Fatalf("expected project key proj, got %q", projectKey)
	}

	libs, err := cfg.ResolveLibraries(projectDir)
	if err != nil {
		t.Fatalf("ResolveLibraries: %v", err)
	}

	if len(libs) != 1 || libs[0].Name != "corelib" {
		t.Fatalf("expected corelib library only, got %+v", libs)
	}
	if !containsPath(libs[0].Files, core) {
		t.Fatalf("expected corelib to include %s, got %v", core, libs[0].Files)
	}
}

func TestApplyProjectOverridesChoosesLongestMatch(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig()
	cfg.configPath = filepath.Join(root, "vhdl_lint.json")
	cfg.Projects = map[string]ProjectConfig{
		"external_tests": {
			Libraries: map[string]LibraryConfig{
				"base": {Files: []string{"**/*.vhd"}},
			},
		},
		"external_tests/ghdl": {
			Libraries: map[string]LibraryConfig{
				"ghdl": {Files: []string{"**/*.vhd"}},
			},
		},
	}

	projectRoot := filepath.Join(root, "external_tests", "ghdl")
	if projectKey, ok := cfg.ApplyProjectOverrides(projectRoot); !ok {
		t.Fatalf("expected project override to apply")
	} else if projectKey != "external_tests/ghdl" {
		t.Fatalf("expected ghdl project override, got %q", projectKey)
	}

	if _, ok := cfg.Libraries["ghdl"]; !ok {
		t.Fatalf("expected ghdl library mapping to be active")
	}
	if _, ok := cfg.Libraries["base"]; ok {
		t.Fatalf("expected base library mapping to be overridden")
	}
}
