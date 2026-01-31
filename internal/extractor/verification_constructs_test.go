package extractor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerificationFSMFixtureFacts(t *testing.T) {
	facts := extractVerificationFixture(t, "construct_fsm.vhd")
	if !hasCaseExpression(facts.CaseStatements, "mode") {
		t.Fatalf("expected case statement on mode")
	}
	if !hasSequentialProcess(facts.Processes, "p_seq") {
		t.Fatalf("expected sequential process p_seq")
	}
}

func TestVerificationReadyValidFixtureFacts(t *testing.T) {
	facts := extractVerificationFixture(t, "construct_ready_valid.vhd")
	if !hasConcurrentReadSignals(facts.ConcurrentAssignments, "xfer", []string{"src_ok", "sink_ok"}) {
		t.Fatalf("expected concurrent assignment to xfer with src_ok and sink_ok reads")
	}
}

func TestVerificationFIFOFixtureFacts(t *testing.T) {
	facts := extractVerificationFixture(t, "construct_fifo.vhd")
	memType := mustFindType(t, facts.Types, "mem_t")
	if memType.Kind != "array" {
		t.Fatalf("expected mem_t to be array type, got %q", memType.Kind)
	}
	if !hasSignal(facts.Signals, "mem") {
		t.Fatalf("expected mem signal")
	}
	if !hasSignalDepTarget(facts.SignalDeps, "mem", "write_p") {
		t.Fatalf("expected write_p to target mem")
	}
	if !hasSignalDepSource(facts.SignalDeps, "mem", "read_p") {
		t.Fatalf("expected read_p to read mem")
	}
}

func extractVerificationFixture(t *testing.T, name string) FileFacts {
	t.Helper()
	repoRoot := findVerificationRepoRoot(t)
	path := filepath.Join(repoRoot, "testdata", "verification", name)
	ext := New()
	facts, err := ext.Extract(path)
	if err != nil {
		t.Fatalf("extract fixture %s: %v", name, err)
	}
	return facts
}

func findVerificationRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		candidate := filepath.Join(dir, "testdata", "verification")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repo root not found from %s", dir)
		}
		dir = parent
	}
}

func hasCaseExpression(cases []CaseStatement, expr string) bool {
	for _, cs := range cases {
		if strings.EqualFold(cs.Expression, expr) {
			return true
		}
	}
	return false
}

func hasSequentialProcess(processes []Process, label string) bool {
	for _, proc := range processes {
		if strings.EqualFold(proc.Label, label) && proc.IsSequential {
			return true
		}
	}
	return false
}

func hasConcurrentReadSignals(
	assignments []ConcurrentAssignment,
	target string,
	readSignals []string,
) bool {
	for _, ca := range assignments {
		if !strings.EqualFold(ca.Target, target) {
			continue
		}
		if containsAllSignals(ca.ReadSignals, readSignals) {
			return true
		}
	}
	return false
}

func containsAllSignals(haystack []string, needles []string) bool {
	for _, needle := range needles {
		found := false
		for _, item := range haystack {
			if strings.EqualFold(item, needle) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func hasSignal(signals []Signal, name string) bool {
	for _, sig := range signals {
		if strings.EqualFold(sig.Name, name) {
			return true
		}
	}
	return false
}

func hasSignalDepTarget(deps []SignalDep, target, process string) bool {
	for _, dep := range deps {
		if strings.EqualFold(dep.Target, target) && strings.EqualFold(dep.InProcess, process) {
			return true
		}
	}
	return false
}

func hasSignalDepSource(deps []SignalDep, source, process string) bool {
	for _, dep := range deps {
		if strings.EqualFold(dep.Source, source) && strings.EqualFold(dep.InProcess, process) {
			return true
		}
	}
	return false
}
