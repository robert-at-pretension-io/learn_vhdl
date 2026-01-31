package policy_test

import (
	"path/filepath"
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/policy"
)

func TestVerificationEndToEnd(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "e2e_demo.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_verification_check": "warning",
	})

	if len(result.MissingChecks) == 0 {
		t.Fatalf("expected missing_checks in end-to-end demo")
	}

	fsmTask := findTaskWithMissingID(result.MissingChecks, "fsm.reset_known")
	if fsmTask == nil {
		t.Fatalf("expected FSM missing checks")
	}
	if !stringSliceContains(fsmTask.MissingIDs, "cover.fsm.transition_taken") {
		t.Fatalf("expected FSM task to include cover.fsm.transition_taken, got %v", fsmTask.MissingIDs)
	}
	if fsmTask.Bindings["state"] != "mode" {
		t.Fatalf("expected FSM binding state=mode, got %v", fsmTask.Bindings)
	}

	counterTask := findTaskWithMissingID(result.MissingChecks, "ctr.range")
	if counterTask == nil {
		t.Fatalf("expected counter missing checks")
	}
	if !stringSliceContains(counterTask.MissingIDs, "ctr.step_rule") ||
		!stringSliceContains(counterTask.MissingIDs, "cover.ctr.moved") {
		t.Fatalf("expected counter task to include ctr.step_rule and cover.ctr.moved, got %v", counterTask.MissingIDs)
	}
	if counterTask.Bindings["counter"] != "idx" {
		t.Fatalf("expected counter binding counter=idx, got %v", counterTask.Bindings)
	}

	rvTask := findTaskWithMissingID(result.MissingChecks, "rv.stable_while_stalled")
	if rvTask == nil {
		t.Fatalf("expected ready/valid missing checks")
	}
	if rvTask.Bindings["valid"] != "v_out" || rvTask.Bindings["ready"] != "r_in" {
		t.Fatalf("expected ready/valid bindings valid=v_out ready=r_in, got %v", rvTask.Bindings)
	}
}

func findTaskWithMissingID(tasks []policy.MissingCheckTask, id string) *policy.MissingCheckTask {
	for i := range tasks {
		if stringSliceContains(tasks[i].MissingIDs, id) {
			return &tasks[i]
		}
	}
	return nil
}
