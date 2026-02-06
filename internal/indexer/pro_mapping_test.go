package indexer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
)

func TestProIncludeSelectsBuildPro(t *testing.T) {
	dir := t.TempDir()
	commonDir := filepath.Join(dir, "Common")
	if err := os.MkdirAll(commonDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(commonDir, "a.vhd"), "entity a is end entity;")
	writeFile(t, filepath.Join(commonDir, "build.pro"), "library lib1\nanalyze a.vhd\n")
	top := filepath.Join(dir, "top.pro")
	writeFile(t, top, "include ./Common\n")

	mapping, err := loadProLibraryMapping(top, nil)
	if err != nil {
		t.Fatalf("loadProLibraryMapping: %v", err)
	}
	path, _ := filepath.Abs(filepath.Join(commonDir, "a.vhd"))
	lib := mapping.fileLibraries[path]
	if lib != "lib1" {
		t.Fatalf("expected lib1 for %s, got %q", path, lib)
	}
}

func TestProIncludeSelectsDirPro(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "Mod")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(modDir, "b.vhd"), "entity b is end entity;")
	writeFile(t, filepath.Join(modDir, "Mod.pro"), "library lib2\nanalyze b.vhd\n")
	top := filepath.Join(dir, "top.pro")
	writeFile(t, top, "include ./Mod\n")

	mapping, err := loadProLibraryMapping(top, nil)
	if err != nil {
		t.Fatalf("loadProLibraryMapping: %v", err)
	}
	path, _ := filepath.Abs(filepath.Join(modDir, "b.vhd"))
	lib := mapping.fileLibraries[path]
	if lib != "lib2" {
		t.Fatalf("expected lib2 for %s, got %q", path, lib)
	}
}

func TestProIncludeDirWithoutProAddsVHDL(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(srcDir, "c.vhd"), "entity c is end entity;")
	top := filepath.Join(dir, "top.pro")
	writeFile(t, top, "library lib3\ninclude ./src\n")

	mapping, err := loadProLibraryMapping(top, nil)
	if err != nil {
		t.Fatalf("loadProLibraryMapping: %v", err)
	}
	path, _ := filepath.Abs(filepath.Join(srcDir, "c.vhd"))
	lib := mapping.fileLibraries[path]
	if lib != "lib3" {
		t.Fatalf("expected lib3 for %s, got %q", path, lib)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestOSVVMProMappingIncludesTestbenchLibrary(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "external_tests", "osvvm"))
	if _, err := os.Stat(filepath.Join(root, "OsvvmLibraries.pro")); err != nil {
		t.Skipf("osvvm not available: %v", err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	mapping, err := loadProLibraryMapping(root, cfg)
	if err != nil {
		t.Fatalf("loadProLibraryMapping: %v", err)
	}
	if mapping == nil {
		t.Fatalf("expected mapping for osvvm")
	}
	target := filepath.Join(root, "Common", "TbStream", "testbench", "TestCtrl_e.vhd")
	target, _ = filepath.Abs(target)
	lib, ok := mapping.fileLibraries[target]
	if !ok {
		t.Fatalf("expected mapping for %s", target)
	}
	if !strings.HasPrefix(strings.ToLower(lib), "osvvm_tb") {
		t.Fatalf("expected testbench library for %s, got %q", target, lib)
	}
}
