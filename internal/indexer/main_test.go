package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	stubDir := ""
	if os.Getenv("VHDL_POLICY_BIN") == "" {
		dir, err := os.MkdirTemp("", "vhdl_policy_stub")
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to create policy stub dir:", err)
			os.Exit(1)
		}
		stubDir = dir
		stubPath := filepath.Join(dir, "vhdl_policy_stub")
		stub := "#!/bin/sh\ncat <<'EOF'\n{\"violations\":[],\"summary\":{\"total_violations\":0,\"errors\":0,\"warnings\":0,\"info\":0}}\nEOF\n"
		if err := os.WriteFile(stubPath, []byte(stub), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "failed to write policy stub:", err)
			_ = os.RemoveAll(dir)
			os.Exit(1)
		}
		if err := os.Setenv("VHDL_POLICY_BIN", stubPath); err != nil {
			fmt.Fprintln(os.Stderr, "failed to set VHDL_POLICY_BIN:", err)
			_ = os.RemoveAll(dir)
			os.Exit(1)
		}
	}

	code := m.Run()
	if stubDir != "" {
		_ = os.RemoveAll(stubDir)
	}
	os.Exit(code)
}
