package indexer

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
)

func cacheEnabled(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if cfg.Analysis.Cache.Enabled == nil {
		return false
	}
	return *cfg.Analysis.Cache.Enabled
}

func resolveCacheDir(rootPath string, cfg *config.Config) string {
	baseDir := rootPath
	if info, err := os.Stat(rootPath); err == nil && !info.IsDir() {
		baseDir = filepath.Dir(rootPath)
	}
	cacheDir := cfg.Analysis.Cache.Dir
	if cacheDir == "" {
		cacheDir = ".vhdl_lint_cache"
	}
	if !filepath.IsAbs(cacheDir) {
		cacheDir = filepath.Join(baseDir, cacheDir)
	}
	return cacheDir
}

func computeCacheVersions(rootPath string) cacheVersions {
	// Prefer locating the repo root by walking up from this source file.
	repoRoot := findRepoRootForCache()
	if repoRoot == "" {
		repoRoot = rootPath
		if info, err := os.Stat(rootPath); err == nil && !info.IsDir() {
			repoRoot = filepath.Dir(rootPath)
		}
	}
	parserVersion := hashFileIfExists(filepath.Join(repoRoot, "tree-sitter-vhdl", "grammar.js"))
	extractorVersion := hashFileIfExists(filepath.Join(repoRoot, "internal", "extractor", "extractor.go"))

	if parserVersion == "" {
		parserVersion = "unknown"
	}
	if extractorVersion == "" {
		extractorVersion = "unknown"
	}

	return cacheVersions{parser: parserVersion, extractor: extractorVersion}
}

func findRepoRootForCache() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	dir := filepath.Dir(file)
	for {
		candidate := filepath.Join(dir, "tree-sitter-vhdl", "grammar.js")
		if _, err := os.Stat(candidate); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func hashFileIfExists(path string) string {
	if path == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	h, err := hashFile(path)
	if err != nil {
		return ""
	}
	return h
}
