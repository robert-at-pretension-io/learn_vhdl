package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ApplyProjectOverrides applies per-project library/file mappings based on rootPath.
// Returns the matched project key when applied.
func (c *Config) ApplyProjectOverrides(rootPath string) (string, bool) {
	if len(c.Projects) == 0 {
		return "", false
	}

	rootDir := rootPath
	if info, err := os.Stat(rootPath); err == nil && !info.IsDir() {
		rootDir = filepath.Dir(rootPath)
	}
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		absRoot = filepath.Clean(rootDir)
	}

	baseDir := absRoot
	if c.configPath != "" {
		baseDir = filepath.Dir(c.configPath)
	}
	if absBase, err := filepath.Abs(baseDir); err == nil {
		baseDir = absBase
	}

	var bestKey string
	var bestPath string
	for key := range c.Projects {
		projPath := key
		if !filepath.IsAbs(projPath) {
			projPath = filepath.Join(baseDir, projPath)
		}
		if absProj, err := filepath.Abs(projPath); err == nil {
			projPath = absProj
		}
		projPath = filepath.Clean(projPath)
		if !pathWithin(absRoot, projPath) {
			continue
		}
		if len(projPath) > len(bestPath) {
			bestKey = key
			bestPath = projPath
		}
	}

	if bestKey == "" {
		return "", false
	}

	c.captureBaseMappings()
	c.Libraries = cloneLibraries(c.baseLibraries)
	c.Files = cloneFiles(c.baseFiles)

	project := c.Projects[bestKey]
	if project.Libraries != nil {
		c.Libraries = cloneLibraries(project.Libraries)
	} else if project.Files != nil {
		c.Libraries = map[string]LibraryConfig{}
	}
	if project.Files != nil {
		c.Files = cloneFiles(project.Files)
	}

	return bestKey, true
}

func (c *Config) captureBaseMappings() {
	if c.baseLibraries != nil || c.baseFiles != nil {
		return
	}
	c.baseLibraries = cloneLibraries(c.Libraries)
	c.baseFiles = cloneFiles(c.Files)
}

func cloneLibraries(input map[string]LibraryConfig) map[string]LibraryConfig {
	if input == nil {
		return nil
	}
	out := make(map[string]LibraryConfig, len(input))
	for name, lib := range input {
		files := append([]string{}, lib.Files...)
		exclude := append([]string{}, lib.Exclude...)
		out[name] = LibraryConfig{
			Files:        files,
			Exclude:      exclude,
			IsThirdParty: lib.IsThirdParty,
		}
	}
	return out
}

func cloneFiles(input []FileEntry) []FileEntry {
	if input == nil {
		return nil
	}
	out := make([]FileEntry, len(input))
	copy(out, input)
	return out
}

func pathWithin(path, base string) bool {
	if path == base {
		return true
	}
	if strings.HasPrefix(path, base+string(filepath.Separator)) {
		return true
	}
	return false
}
