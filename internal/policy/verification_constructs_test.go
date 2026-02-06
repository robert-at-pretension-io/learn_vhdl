package policy_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/indexer"
)

func TestMissingChecksForFSM(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "construct_fsm.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_verification_check": "warning",
	})

	assertMissingChecks(t, result, []string{
		"fsm.legal_state",
		"fsm.reset_known",
		"cover.fsm.transition_taken",
	})
}

func TestMissingChecksForCounter(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "construct_counter.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_verification_check": "warning",
	})

	assertMissingChecks(t, result, []string{
		"ctr.range",
		"ctr.step_rule",
		"cover.ctr.moved",
	})
}

func TestMissingChecksForReadyValid(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "construct_ready_valid.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_verification_check": "warning",
	})

	assertMissingChecks(t, result, []string{
		"rv.stable_while_stalled",
		"cover.rv.handshake",
	})
}

func TestMissingChecksForFIFO(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "construct_fifo.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_verification_check": "warning",
	})

	assertMissingChecks(t, result, []string{
		"fifo.no_read_empty",
		"fifo.no_write_full",
		"cover.fifo.activity",
	})
}

func TestReadyValidSequentialGateDetectionGap(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "construct_ready_valid_sequential.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_verification_check": "warning",
	})

	if hasMissingCheck(result, "rv.stable_while_stalled") || hasMissingCheck(result, "cover.rv.handshake") {
		t.Fatalf("did not expect ready/valid missing checks for sequential gating fixture")
	}
}

func TestReadyValidInternalSignalsDetectionGap(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "construct_ready_valid_internal.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_verification_check": "warning",
	})

	if hasMissingCheck(result, "rv.stable_while_stalled") || hasMissingCheck(result, "cover.rv.handshake") {
		t.Fatalf("did not expect ready/valid missing checks for internal-signal fixture")
	}
}

func TestMissingChecksForResetHygiene(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "construct_reset_hygiene.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_verification_check": "warning",
	})

	assertMissingChecks(t, result, []string{
		"reset.no_unknown_after_grace",
		"reset.asserted_implies_known_defaults",
	})
}

func TestCounterFalsePositiveFixture(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "construct_counter_false_positive.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_verification_check": "warning",
	})

	assertMissingChecks(t, result, []string{
		"ctr.range",
		"ctr.step_rule",
		"cover.ctr.moved",
	})
}

func TestMissingChecksForPulse(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "construct_pulse.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_verification_check": "warning",
	})

	assertMissingChecks(t, result, []string{
		"pulse.width_one_cycle",
		"cover.pulse.fired",
	})
}

func TestMissingChecksForArb(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "construct_arb.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_verification_check": "warning",
	})

	assertMissingChecks(t, result, []string{
		"arb.onehot0",
		"arb.no_grant_without_req",
		"cover.arb.grant_seen",
	})
}

func TestMissingCoverCompanion(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "missing_cover_companion.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_cover_companion": "warning",
	})

	if !hasRule(result, "missing_cover_companion") {
		t.Fatalf("expected missing_cover_companion violation, got rules: %v", collectRules(result))
	}
}

func TestMissingLivenessBound(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "missing_liveness_bound.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_liveness_bound": "warning",
	})

	if !hasRule(result, "missing_liveness_bound") {
		t.Fatalf("expected missing_liveness_bound violation, got rules: %v", collectRules(result))
	}
}

func TestScopeAccurateMatching(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "scope_arch_fixture.vhd")

	result := lintFile(t, repoRoot, fixture, map[string]string{
		"missing_verification_check": "warning",
	})

	if !hasMissingCheckScopeID(result, "arch:gate", "fsm.legal_state") {
		t.Fatalf("expected missing fsm checks for arch:gate, got violations: %v", collectRules(result))
	}
	if hasMissingCheckScopeID(result, "arch:rtl", "fsm.legal_state") {
		t.Fatalf("did not expect fsm missing checks for arch:rtl")
	}
}

func assertMissingChecks(t *testing.T, result indexer.LintResult, ids []string) {
	t.Helper()
	for _, id := range ids {
		if !hasMissingCheck(result, id) {
			t.Fatalf("expected missing check %q, got missing-check messages: %v", id, missingCheckMessages(result))
		}
	}
}

func hasMissingCheck(result indexer.LintResult, id string) bool {
	for _, v := range result.Violations {
		if v.Rule == "missing_verification_check" && strings.Contains(v.Message, id) {
			return true
		}
	}
	return false
}

func hasMissingCheckScope(result indexer.LintResult, scope string) bool {
	for _, v := range result.Violations {
		if v.Rule == "missing_verification_check" && strings.Contains(v.Message, scope) {
			return true
		}
	}
	return false
}

func hasMissingCheckScopeID(result indexer.LintResult, scope, id string) bool {
	for _, v := range result.Violations {
		if v.Rule == "missing_verification_check" &&
			strings.Contains(v.Message, scope) &&
			strings.Contains(v.Message, id) {
			return true
		}
	}
	return false
}

func missingCheckMessages(result indexer.LintResult) []string {
	var msgs []string
	for _, v := range result.Violations {
		if v.Rule == "missing_verification_check" {
			msgs = append(msgs, v.Message)
		}
	}
	return msgs
}
