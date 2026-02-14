package lsp

import (
	"bytes"
	"context"
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
	return r.RunWithContext(context.Background(), workspaceRoot, symbolsJSON)
}

// RunTarget executes vhdl-lint for a specific file or directory target.
// workingDir controls command execution context for config/path resolution.
func (r *Runner) RunTarget(targetPath, workingDir string, symbolsJSON bool) (*LintResult, error) {
	return r.RunTargetWithContext(context.Background(), targetPath, workingDir, symbolsJSON)
}

// RunWithContext executes vhdl-lint against the workspace root with cancellation support.
func (r *Runner) RunWithContext(ctx context.Context, workspaceRoot string, symbolsJSON bool) (*LintResult, error) {
	return r.run(ctx, workspaceRoot, workspaceRoot, symbolsJSON)
}

// RunTargetWithContext executes vhdl-lint for a specific file/directory with cancellation support.
func (r *Runner) RunTargetWithContext(ctx context.Context, targetPath, workingDir string, symbolsJSON bool) (*LintResult, error) {
	if workingDir == "" {
		workingDir = filepath.Dir(targetPath)
	}
	return r.run(ctx, targetPath, workingDir, symbolsJSON)
}

func (r *Runner) run(ctx context.Context, targetPath, workingDir string, symbolsJSON bool) (*LintResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	args := make([]string, 0, 4)
	if cfgPath := os.Getenv("VHDL_LINT_CONFIG"); cfgPath != "" {
		args = append(args, "-c", cfgPath)
	}

	flag := "-j"
	if symbolsJSON {
		flag = "--symbols-json"
	}
	args = append(args, flag, targetPath)

	cmd := exec.CommandContext(ctx, r.binaryPath, args...)
	cmd.Dir = workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return nil, fmt.Errorf("vhdl-lint exec failed: %w: %s", err, stderr.String())
		}
		// vhdl-lint can return non-zero while still producing JSON output.
		// If there's no JSON payload, treat this as a hard execution failure.
		if stdout.Len() == 0 {
			return nil, fmt.Errorf("vhdl-lint exited with code %d: %s", exitErr.ExitCode(), stderr.String())
		}
	}

	if stdout.Len() == 0 {
		return &LintResult{}, nil
	}

	var result LintResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("parse vhdl-lint output: %w (stderr: %s)", err, stderr.String())
		}
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
