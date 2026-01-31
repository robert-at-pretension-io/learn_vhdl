package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLibrariesWithExplicitFiles(t *testing.T) {
	root := t.TempDir()
	rtlDir := filepath.Join(root, "rtl")
	simDir := filepath.Join(root, "sim")
	if err := os.MkdirAll(rtlDir, 0o755); err != nil {
		t.Fatalf("mkdir rtl: %v", err)
	}
	if err := os.MkdirAll(simDir, 0o755); err != nil {
		t.Fatalf("mkdir sim: %v", err)
	}

	core := filepath.Join(rtlDir, "core.vhd")
	tb := filepath.Join(simDir, "tb_core.vhd")
	if err := os.WriteFile(core, []byte("-- core"), 0o644); err != nil {
		t.Fatalf("write core: %v", err)
	}
	if err := os.WriteFile(tb, []byte("-- tb"), 0o644); err != nil {
		t.Fatalf("write tb: %v", err)
	}

	cfg := Config{
		Libraries: map[string]LibraryConfig{
			"work": {Files: []string{"rtl/*.vhd"}},
		},
		Files: []FileEntry{
			{File: "sim/tb_core.vhd", Library: "sim", Language: "vhdl"},
			{File: "sim/skip.sv", Library: "sim", Language: "verilog"},
		},
	}

	libs, err := cfg.ResolveLibraries(root)
	if err != nil {
		t.Fatalf("ResolveLibraries: %v", err)
	}

	workFiles := findLibFiles(t, libs, "work")
	if !containsPath(workFiles, core) {
		t.Fatalf("expected work lib to include %s, got %v", core, workFiles)
	}

	simFiles := findLibFiles(t, libs, "sim")
	if !containsPath(simFiles, tb) {
		t.Fatalf("expected sim lib to include %s, got %v", tb, simFiles)
	}
}

func TestGetFileLibraryWithExplicitFiles(t *testing.T) {
	root := t.TempDir()
	simDir := filepath.Join(root, "sim")
	if err := os.MkdirAll(simDir, 0o755); err != nil {
		t.Fatalf("mkdir sim: %v", err)
	}
	tb := filepath.Join(simDir, "tb_core.vhd")
	if err := os.WriteFile(tb, []byte("-- tb"), 0o644); err != nil {
		t.Fatalf("write tb: %v", err)
	}

	cfg := Config{
		Files: []FileEntry{
			{File: "sim/tb_core.vhd", Library: "sim", Language: "vhdl", IsThirdParty: true},
		},
	}

	info := cfg.GetFileLibrary(tb, root)
	if info.LibraryName != "sim" {
		t.Fatalf("expected library sim, got %q", info.LibraryName)
	}
	if !info.IsThirdParty {
		t.Fatalf("expected IsThirdParty true")
	}
}

func findLibFiles(t *testing.T, libs []ResolvedLibrary, name string) []string {
	t.Helper()
	for _, lib := range libs {
		if lib.Name == name {
			return lib.Files
		}
	}
	t.Fatalf("library %s not found", name)
	return nil
}

func containsPath(files []string, target string) bool {
	for _, f := range files {
		if filepath.Clean(f) == filepath.Clean(target) {
			return true
		}
	}
	return false
}
