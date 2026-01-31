package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the top-level configuration for vhdl-lint
type Config struct {
	// Standard specifies the VHDL standard to use: "1993", "2002", "2008", "2019"
	Standard string `json:"standard,omitempty"`

	// Files is an explicit list of files with optional library/language overrides
	Files []FileEntry `json:"files,omitempty"`

	// Libraries maps library names to their configuration
	Libraries map[string]LibraryConfig `json:"libraries,omitempty"`

	// Lint contains linting rule configuration
	Lint LintConfig `json:"lint,omitempty"`

	// Analysis contains analysis options
	Analysis AnalysisConfig `json:"analysis,omitempty"`
}

// LibraryConfig defines a VHDL library's files and options
type LibraryConfig struct {
	// Files is a list of glob patterns for VHDL files in this library
	Files []string `json:"files"`

	// Exclude is a list of glob patterns to exclude from this library
	Exclude []string `json:"exclude,omitempty"`

	// IsThirdParty marks the library as third-party (suppress certain warnings)
	IsThirdParty bool `json:"isThirdParty,omitempty"`
}

// FileEntry is an explicit file entry with optional library and language metadata
type FileEntry struct {
	File         string `json:"file"`
	Library      string `json:"library,omitempty"`
	Language     string `json:"language,omitempty"`
	IsThirdParty bool   `json:"isThirdParty,omitempty"`
}

// LintConfig contains linting configuration
type LintConfig struct {
	// Rules maps rule names to severity: "off", "warning", "error"
	Rules map[string]string `json:"rules,omitempty"`

	// IgnorePatterns is a list of file patterns to skip linting entirely
	IgnorePatterns []string `json:"ignorePatterns,omitempty"`

	// IgnoreRegions enables -- vhdl_lint off/on comment support
	IgnoreRegions bool `json:"ignoreRegions,omitempty"`
}

// AnalysisConfig contains analysis options
// CacheConfig controls incremental indexing cache behavior
type CacheConfig struct {
	// Enabled turns on incremental cache usage
	Enabled *bool `json:"enabled,omitempty"`

	// Dir is the cache directory (relative to project root if not absolute)
	Dir string `json:"dir,omitempty"`
}

// AnalysisConfig contains analysis options
type AnalysisConfig struct {
	// MaxParallelFiles limits concurrent file processing (0 = auto)
	MaxParallelFiles int `json:"maxParallelFiles,omitempty"`

	// FollowLibraryUse resolves use clauses across libraries
	FollowLibraryUse bool `json:"followLibraryUse,omitempty"`

	// ResolveDefaultBinding computes component to entity default binding
	ResolveDefaultBinding bool `json:"resolveDefaultBinding,omitempty"`

	// Cache controls incremental indexing cache behavior
	Cache CacheConfig `json:"cache,omitempty"`
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		Standard: "2008",
		Libraries: map[string]LibraryConfig{
			"work": {
				Files:        []string{"*.vhd", "*.vhdl", "**/*.vhd", "**/*.vhdl"},
				Exclude:      []string{},
				IsThirdParty: false,
			},
		},
		Lint: LintConfig{
			Rules:          map[string]string{},
			IgnorePatterns: []string{},
			IgnoreRegions:  true,
		},
		Analysis: AnalysisConfig{
			MaxParallelFiles:      0, // auto
			FollowLibraryUse:      true,
			ResolveDefaultBinding: true,
			Cache: CacheConfig{
				Enabled: boolPtr(true),
				Dir:     ".vhdl_lint_cache",
			},
		},
	}
}

func boolPtr(v bool) *bool {
	return &v
}

// Load finds and loads the configuration file
// Search order:
//  1. ./vhdl_lint.json (current working directory)
//  2. ./.vhdl_lint.json (current working directory)
//  3. <rootPath>/vhdl_lint.json (if different from cwd)
//  4. ~/.config/vhdl_lint/config.json
//
// Returns DefaultConfig if no config file is found
func Load(rootPath string) (*Config, error) {
	// Get current working directory
	cwd, _ := os.Getwd()

	searchPaths := []string{
		// First check current working directory
		filepath.Join(cwd, "vhdl_lint.json"),
		filepath.Join(cwd, ".vhdl_lint.json"),
	}

	// If rootPath is a directory and different from cwd, also check there
	if info, err := os.Stat(rootPath); err == nil && info.IsDir() {
		absRoot, _ := filepath.Abs(rootPath)
		if absRoot != cwd {
			searchPaths = append(searchPaths,
				filepath.Join(rootPath, "vhdl_lint.json"),
				filepath.Join(rootPath, ".vhdl_lint.json"),
			)
		}
	}

	// Add user config path
	if home, err := os.UserHomeDir(); err == nil {
		searchPaths = append(searchPaths, filepath.Join(home, ".config", "vhdl_lint", "config.json"))
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return LoadFile(path)
		}
	}

	// No config found, return defaults
	return DefaultConfig(), nil
}

// LoadFile loads configuration from a specific file
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Apply defaults for missing fields
	cfg.applyDefaults()

	return &cfg, nil
}

// applyDefaults fills in missing configuration with defaults
func (c *Config) applyDefaults() {
	if c.Standard == "" {
		c.Standard = "2008"
	}

	if c.Libraries == nil {
		if len(c.Files) == 0 {
			c.Libraries = map[string]LibraryConfig{
				"work": {
					Files: []string{"*.vhd", "*.vhdl", "**/*.vhd", "**/*.vhdl"},
				},
			}
		} else {
			c.Libraries = map[string]LibraryConfig{}
		}
	}

	if c.Lint.Rules == nil {
		c.Lint.Rules = make(map[string]string)
	}

	if c.Analysis.Cache.Dir == "" {
		c.Analysis.Cache.Dir = ".vhdl_lint_cache"
	}
	if c.Analysis.Cache.Enabled == nil {
		c.Analysis.Cache.Enabled = boolPtr(true)
	}
}

// Save writes the configuration to a file
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// GetRuleSeverity returns the severity for a rule, or the default if not configured
func (c *Config) GetRuleSeverity(rule string, defaultSeverity string) string {
	if severity, ok := c.Lint.Rules[rule]; ok {
		return severity
	}
	return defaultSeverity
}

// IsRuleEnabled returns true if the rule is not set to "off"
func (c *Config) IsRuleEnabled(rule string) bool {
	if severity, ok := c.Lint.Rules[rule]; ok {
		return severity != "off"
	}
	return true // enabled by default
}

// IsThirdPartyFile checks if a file belongs to a third-party library
func (c *Config) IsThirdPartyFile(filePath string) bool {
	for _, entry := range c.Files {
		if entry.File == "" {
			continue
		}
		path := entry.File
		if matched, _ := filepath.Match(path, filePath); matched {
			return entry.IsThirdParty
		}
		if matched, _ := filepath.Match(path, filepath.Base(filePath)); matched {
			return entry.IsThirdParty
		}
	}
	for _, lib := range c.Libraries {
		if !lib.IsThirdParty {
			continue
		}
		for _, pattern := range lib.Files {
			if matched, _ := filepath.Match(pattern, filePath); matched {
				return true
			}
			// Also try matching against the base name
			if matched, _ := filepath.Match(pattern, filepath.Base(filePath)); matched {
				return true
			}
		}
	}
	return false
}

// ShouldIgnoreFile checks if a file should be skipped entirely
func (c *Config) ShouldIgnoreFile(filePath string) bool {
	for _, pattern := range c.Lint.IgnorePatterns {
		if matched, _ := filepath.Match(pattern, filePath); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, filepath.Base(filePath)); matched {
			return true
		}
	}
	return false
}
