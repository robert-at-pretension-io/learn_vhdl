package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// LintResult mirrors indexer.LintResult for JSON parsing.
// Re-declared here to avoid importing the indexer package (and its CGO deps).
//
// CONTRACT: This struct must stay in sync with indexer.LintResult.
// The TestLintResultContractSync test in contract_test.go validates this.
type LintResult struct {
	Violations          []Violation          `json:"violations"`
	MissingChecks       []MissingCheckTask   `json:"missing_checks,omitempty"`
	AmbiguousConstructs []AmbiguousConstruct `json:"ambiguous_constructs,omitempty"`
	Waivers             []Waiver             `json:"waivers,omitempty"`
	Summary             ResultSummary        `json:"summary"`
	ParseErrors         []ParseError         `json:"parse_errors,omitempty"`
	SymbolIndex         *SymbolIndex         `json:"symbol_index,omitempty"`
}

// Violation mirrors policy.Violation.
type Violation struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
}

// ResultSummary mirrors indexer.ResultSummary.
type ResultSummary struct {
	TotalViolations int `json:"total_violations"`
	Errors          int `json:"errors"`
	Warnings        int `json:"warnings"`
	Info            int `json:"info"`
}

// ParseError mirrors indexer.ParseError.
type ParseError struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

// MissingCheckTask mirrors policy.MissingCheckTask.
type MissingCheckTask struct {
	File       string            `json:"file"`
	Scope      string            `json:"scope"`
	MissingIDs []string          `json:"missing_ids"`
	Bindings   map[string]string `json:"bindings,omitempty"`
	Notes      []string          `json:"notes,omitempty"`
}

// AmbiguousConstruct mirrors policy.AmbiguousConstruct.
type AmbiguousConstruct struct {
	Kind  string `json:"kind"`
	Scope string `json:"scope"`
	File  string `json:"file"`
	Line  int    `json:"line"`
}

// Waiver mirrors policy.Waiver.
type Waiver struct {
	ID     string `json:"id"`
	Scope  string `json:"scope"`
	Reason string `json:"reason"`
	File   string `json:"file"`
	Line   int    `json:"line"`
}

// Runner manages subprocess invocations of vhdl-lint.
type Runner struct {
	mu         sync.Mutex
	binaryPath string
}

// NewRunner creates a runner that locates the vhdl-lint binary.
func NewRunner() (*Runner, error) {
	bin, err := findLintBinary()
	if err != nil {
		return nil, err
	}
	return &Runner{binaryPath: bin}, nil
}

// Run executes vhdl-lint against the workspace root and returns parsed results.
func (r *Runner) Run(workspaceRoot string, symbolsJSON bool) (*LintResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	flag := "-j"
	if symbolsJSON {
		flag = "--symbols-json"
	}

	cmd := exec.Command(r.binaryPath, flag, workspaceRoot)
	cmd.Dir = workspaceRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	// vhdl-lint may exit non-zero on lint errors; we still parse stdout
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, fmt.Errorf("vhdl-lint exec failed: %w: %s", err, stderr.String())
		}
	}

	if stdout.Len() == 0 {
		return &LintResult{}, nil
	}

	var result LintResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("parse vhdl-lint output: %w", err)
	}
	return &result, nil
}

// findLintBinary resolves the vhdl-lint binary path.
// Priority: VHDL_LINT_BIN env > co-located binary > PATH lookup.
func findLintBinary() (string, error) {
	if env := os.Getenv("VHDL_LINT_BIN"); env != "" {
		if isExecutable(env) {
			return env, nil
		}
		return "", fmt.Errorf("VHDL_LINT_BIN set but not executable: %s", env)
	}

	// Co-located: look next to the running binary
	self, err := os.Executable()
	if err == nil {
		colocated := filepath.Join(filepath.Dir(self), "vhdl-lint")
		if isExecutable(colocated) {
			return colocated, nil
		}
	}

	// PATH lookup
	if path, err := exec.LookPath("vhdl-lint"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("vhdl-lint binary not found: set VHDL_LINT_BIN or ensure vhdl-lint is in PATH")
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0111 != 0
}
