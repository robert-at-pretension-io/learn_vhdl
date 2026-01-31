package policy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/facts"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/validator"
)

// Daemon provides a streaming interface to the vhdl_policyd incremental engine.
type Daemon struct {
	cmd            *exec.Cmd
	stdin          io.WriteCloser
	stdout         *bufio.Reader
	validator      *validator.PolicyDaemonValidator
	factsValidator *validator.FactsValidator
}

type daemonCommand struct {
	Kind    string       `json:"kind"`
	Tables  facts.Tables `json:"tables,omitempty"`
	Added   facts.Tables `json:"added,omitempty"`
	Removed facts.Tables `json:"removed,omitempty"`
}

type daemonResponse struct {
	Kind                string               `json:"kind"`
	Summary             Summary              `json:"summary"`
	Violations          []Violation          `json:"violations"`
	MissingChecks       []MissingCheckTask   `json:"missing_checks,omitempty"`
	AmbiguousConstructs []AmbiguousConstruct `json:"ambiguous_constructs,omitempty"`
	Message             string               `json:"message"`
}

// NewDaemon starts the vhdl_policyd process and prepares it for commands.
func NewDaemon(policyDir string) (*Daemon, error) {
	bin, err := ensurePolicyDaemonBinary(policyDir)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(context.Background(), bin)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("daemon stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("daemon stdout: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start daemon: %w", err)
	}

	factsValidator, err := validator.NewFactsValidator()
	if err != nil {
		return nil, fmt.Errorf("init facts validator: %w", err)
	}
	daemonValidator, err := validator.NewPolicyDaemonValidator()
	if err != nil {
		return nil, fmt.Errorf("init daemon validator: %w", err)
	}

	return &Daemon{
		cmd:            cmd,
		stdin:          stdin,
		stdout:         bufio.NewReader(stdout),
		validator:      daemonValidator,
		factsValidator: factsValidator,
	}, nil
}

// Init loads a full snapshot into the daemon and returns the current violations.
func (d *Daemon) Init(tables facts.Tables) (*Result, error) {
	cmd := daemonCommand{
		Kind:   "init",
		Tables: tables,
	}
	return d.send(cmd)
}

// Delta applies an incremental update to the daemon and returns the updated violations.
func (d *Daemon) Delta(delta facts.Delta) (*Result, error) {
	cmd := daemonCommand{
		Kind:    "delta",
		Added:   delta.Added,
		Removed: delta.Removed,
	}
	return d.send(cmd)
}

// Snapshot asks the daemon to emit the current state without changes.
func (d *Daemon) Snapshot() (*Result, error) {
	cmd := daemonCommand{Kind: "snapshot"}
	return d.send(cmd)
}

// Close terminates the daemon process.
func (d *Daemon) Close() error {
	if d.stdin != nil {
		_ = d.stdin.Close()
	}
	if d.cmd != nil {
		return d.cmd.Wait()
	}
	return nil
}

func (d *Daemon) send(cmd daemonCommand) (*Result, error) {
	if d.factsValidator != nil {
		switch cmd.Kind {
		case "init":
			if err := d.factsValidator.Validate(cmd.Tables); err != nil {
				return nil, fmt.Errorf("daemon command facts invalid: %w", err)
			}
		case "delta":
			if err := d.factsValidator.Validate(cmd.Added); err != nil {
				return nil, fmt.Errorf("daemon delta added facts invalid: %w", err)
			}
			if err := d.factsValidator.Validate(cmd.Removed); err != nil {
				return nil, fmt.Errorf("daemon delta removed facts invalid: %w", err)
			}
		}
	}

	payload, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("marshal daemon command: %w", err)
	}
	if d.validator != nil {
		if err := d.validator.ValidateCommandJSON(payload); err != nil {
			return nil, fmt.Errorf("daemon command schema invalid: %w", err)
		}
	}
	if _, err := d.stdin.Write(append(payload, '\n')); err != nil {
		return nil, fmt.Errorf("write daemon command: %w", err)
	}

	line, err := d.stdout.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read daemon response: %w", err)
	}
	line = string(bytes.TrimSpace([]byte(line)))
	if d.validator != nil {
		if err := d.validator.ValidateResponseJSON([]byte(line)); err != nil {
			return nil, fmt.Errorf("daemon response schema invalid: %w", err)
		}
	}
	var resp daemonResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, fmt.Errorf("parse daemon response: %w", err)
	}
	if resp.Kind == "error" {
		return nil, fmt.Errorf("daemon error: %s", resp.Message)
	}

	return &Result{
		Violations:          resp.Violations,
		Summary:             resp.Summary,
		MissingChecks:       resp.MissingChecks,
		AmbiguousConstructs: resp.AmbiguousConstructs,
	}, nil
}

func ensurePolicyDaemonBinary(policyDir string) (string, error) {
	if env := os.Getenv("VHDL_POLICYD_BIN"); env != "" {
		if existsExecutable(env) {
			return env, nil
		}
		return "", fmt.Errorf("VHDL_POLICYD_BIN is set but not executable: %s", env)
	}

	base := filepath.Dir(policyDir)
	profile := os.Getenv("VHDL_POLICY_PROFILE")
	if profile == "" {
		profile = "release"
	}
	candidates := []string{filepath.Join(base, "vhdl_policyd")}
	if profile == "debug" {
		candidates = append([]string{
			filepath.Join(base, "target", "debug", "vhdl_policyd"),
			filepath.Join(base, "target", "release", "vhdl_policyd"),
		}, candidates...)
	} else {
		candidates = append([]string{
			filepath.Join(base, "target", "release", "vhdl_policyd"),
			filepath.Join(base, "target", "debug", "vhdl_policyd"),
		}, candidates...)
	}
	for _, candidate := range candidates {
		if existsExecutable(candidate) {
			return candidate, nil
		}
	}
	if path, err := exec.LookPath("vhdl_policyd"); err == nil {
		return path, nil
	}

	if err := buildPolicyDaemonBinary(base, profile); err != nil {
		return "", err
	}
	for _, candidate := range candidates {
		if existsExecutable(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("vhdl_policyd binary not found after build")
}

func buildPolicyDaemonBinary(base, profile string) error {
	args := []string{"build", "--quiet", "--bin", "vhdl_policyd"}
	if profile == "release" {
		args = append(args, "--release")
	}
	cmd := exec.Command("cargo", args...)
	cmd.Dir = base
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building vhdl_policyd: %w (%s)", err, stderr.String())
	}
	return nil
}
