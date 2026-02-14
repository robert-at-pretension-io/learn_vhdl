package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/policy"
)

func main() {
	configPath := flag.String("config", "vhdl_lint.json", "Path to vhdl-lint config JSON")
	severity := flag.String("severity", "warning", "Severity to assign to optional rules (warning|error|info|off)")
	flag.Parse()

	normSeverity := strings.ToLower(strings.TrimSpace(*severity))
	switch normSeverity {
	case "warning", "error", "info", "off":
	default:
		fmt.Fprintf(os.Stderr, "invalid severity %q\n", *severity)
		os.Exit(2)
	}

	cfg, err := loadOrDefault(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Lint.Rules == nil {
		cfg.Lint.Rules = make(map[string]string)
	}

	optional := policy.OptionalRuleIDs()
	updated := 0
	for _, rule := range optional {
		if current, ok := cfg.Lint.Rules[rule]; ok && strings.EqualFold(strings.TrimSpace(current), normSeverity) {
			continue
		}
		cfg.Lint.Rules[rule] = normSeverity
		updated++
	}

	if err := saveConfig(*configPath, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("optional rules configured: %d total, %d updated, severity=%s, file=%s\n", len(optional), updated, normSeverity, *configPath)
}

func loadOrDefault(path string) (*config.Config, error) {
	if _, err := os.Stat(path); err == nil {
		cfg, err := config.LoadFile(path)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}
	cfg := config.DefaultConfig()
	cfgPathAbs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	cfg.Libraries = map[string]config.LibraryConfig{
		"work": {
			Files: []string{"*.vhd", "*.vhdl", "**/*.vhd", "**/*.vhdl"},
		},
	}
	cfg.Standard = "2008"
	cfg.Analysis.UseProLibraries = true
	cfg.Analysis.Cache.Enabled = boolPtr(false)
	cfgPathDir := filepath.Dir(cfgPathAbs)
	if err := os.MkdirAll(cfgPathDir, 0o755); err != nil {
		return nil, err
	}
	return cfg, nil
}

func saveConfig(path string, cfg *config.Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return nil
}

func boolPtr(v bool) *bool {
	return &v
}
