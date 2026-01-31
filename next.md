# Definitive Implementation Checklist: Linter ⇄ Agent Contract for Missing Verification Checks

This document is the **authoritative plan** for implementing a deterministic contract between the VHDL linter and an AI agent. The linter identifies **missing verification checks**; the agent writes those checks in the correct location. The plan is **ordered**, and each step includes a **verification task** (a specific test to add) to prove it works.

This idea turns verification into a measurable, auditable **gap-finding problem** instead of a creative, open-ended one. The linter’s job is not to generate PSL; it is to **prove what is missing** with exact scope, bindings, and insertion points. The agent’s job is then mechanical: fill the gaps at the specified anchor using the provided signal roles. This separation keeps the linter deterministic and makes the agent productive without risking silent mismatches.

Important scope note: **this plan only implements the linter side** of the contract. We are **not** building the agent here. The output must be strong enough that any external agent (human or AI) could follow it, but agent behavior is explicitly out of scope.

Why this approach works for this project:
- It respects the grammar-first philosophy: if the AST is wrong, fix the grammar, not the checks.
- It avoids heuristic PSL equivalence by defining a strict **tag contract** the linter can validate.
- It supports incremental improvement: each missing-check violation is a concrete task the agent can close.
- It scales across large codebases because the linter only reports **absence**, not semantic proof.

The design adheres to this project’s principles:
- Grammar is the source of truth; no ERROR-node workarounds.
- Linter must be deterministic and scope-correct.
- Missing checks are explicit violations; ambiguous detection is a warning.
- Every check is tracked via **rigid tags** and validated against a **CUE schema**.

---

## 0) Lock the Contract Terms (Before Any Code)

**Reasoning:** This prevents future churn and ensures both the linter and the agent speak a shared, enforceable vocabulary.

**Task:**
- Decide and commit:
  - Structural anchor format (verification block in architecture)
  - Tag schema (single-line `--@check` format)
  - Scope keys (`entity:<name>` and `arch:<name>`)
  - “Missing check” vs “Ambiguous construct” classification
  - “Needs cover” policy for vacuity

**Verification task (docs test):**
- Add a **golden spec test** in Go that parses a tiny VHDL file with:
  - a verification block
  - a valid tag line
  - an invalid tag line
- The test should assert: valid tags pass schema, invalid tags fail. (See Step 2 for schema definition.)

Status: DONE — added `testdata/verification/tag_schema_fixture.vhd` and `internal/validator/verification_tag_test.go` to parse tags and validate schema; verified with `go test ./internal/validator`.

---

## 1) Enforce the Structural Anchor (Verification Block)

**Reasoning:** A structural anchor makes insertion deterministic and resilient to comment/formatting churn. It also gives a single place to enforce presence and scope rules.

**Task:**
- Update the linter to detect, per architecture, whether a block named `verification` exists:
  ```vhdl
  verification : block
  begin
    -- PSL + tag lines live here
  end block verification;
  ```
- If an architecture contains any detectable constructs but **no verification block**, emit a **single violation**: “Missing verification block in arch:<name>.”

**Verification task (Go):**
- Add a fixture VHDL file with a simple FSM (or counter) but **no verification block**.
- Add a Go test that runs the linter on this fixture and asserts a violation is emitted:
  - Rule ID: `missing_verification_block`
  - Scope: `arch:<name>`

Status: DONE — fixture `testdata/verification/missing_verification_block.vhd` and test `internal/policy/verification_rules_test.go` added; verified with `VHDL_POLICY_PROFILE=debug go test ./internal/policy -run 'TestMissingVerificationBlock|TestInvalidVerificationTagMissingBinding'`.

---

## 2) Define a CUE Schema for Tag Lines (and enforce it)

**Reasoning:** Tags are the contract. If a tag is malformed or missing required bindings, the check does not exist. CUE makes this enforceable and auditable.

**Task:**
- Create a CUE schema for the tag line structure.
- Required fields:
  - `id` (string)
  - `scope` (string matching `entity:<name>` or `arch:<name>`)
  - Zero or more bindings (e.g., `state=`, `valid=`, `ready=`, `payload=`)
- Ensure tags are **single-line** and parseable without heuristics.

**Verification task (Go + CUE):**
- Add a Go test that:
  - Extracts tag lines from a fixture file.
  - Validates them against the CUE schema.
  - Asserts that malformed tags fail validation.

Status: DONE — CUE schema for `#VerificationTag` added in `internal/validator/schema.cue` and `schema/ir.cue`; Go test `internal/validator/verification_tag_test.go` validates tag schema; verified with `go test ./internal/validator`.

---

## 3) Build a Registry of Check IDs (Single Source of Truth)

**Reasoning:** Both linter and agent must agree on check vocabulary, required bindings, and vacuity pairing. A registry avoids ad-hoc logic.

**Task:**
- Create a registry file (JSON or YAML) with entries like:
  - `id`
  - `scope_type` (entity/arch)
  - `required_bindings` (list)
  - `needs_cover` (bool)
  - `severity` (violation/warning)
  - optional: `requires_bound` (bool)
- Wire the linter to load this registry and validate tag IDs/bindings against it.

**Verification task (Go):**
- Add a test that:
  - Loads a minimal registry
  - Parses tags that are missing required bindings
  - Asserts the linter reports them as invalid / missing

Status: DONE — registry `src/policy/check_registry.json` with env override `VHDL_CHECK_REGISTRY` plus minimal test registry `testdata/verification/check_registry_minimal.json`; test `internal/policy/verification_rules_test.go` asserts `invalid_verification_tag`; verified with `VHDL_POLICY_PROFILE=debug go test ./internal/policy -run 'TestMissingVerificationBlock|TestInvalidVerificationTagMissingBinding'`.

---

## 4) Detect Constructs from Semantic Facts (Not Names)

**Reasoning:** Naming-based inference is useful but unreliable. We should first use semantic patterns from the extractor. Naming conventions can only be fallback hints.

**Task:**
- Implement construct detection using extractor facts:
  - **FSM**: clocked process + enum-typed state + `case` on state
  - **Counter**: assignments of form `c <= c + k` / `c <= c - k`
  - **Ready/valid**: semantic gating pattern `xfer = valid and ready`
  - **FIFO**: memory array with read/write enables or pointer/count
- Keep name-based heuristics optional and explicitly flagged as lower confidence.

**Verification task (Go):**
- Add one fixture per construct with **semantic patterns** but neutral naming.
- Add tests that assert detection works **without** name inference.

Status: DONE — fixtures `testdata/verification/construct_fsm.vhd`, `construct_counter.vhd`, `construct_ready_valid.vhd`, `construct_fifo.vhd` (neutral names) plus semantic detection in `src/policy/verification.rs`; verified with `VHDL_POLICY_PROFILE=debug go test ./internal/policy -run 'TestMissingChecksFor|TestMissingCoverCompanion|TestMissingLivenessBound|TestScopeAccurateMatching'`.

---

## 5) Require Minimal Check Sets Per Construct

**Reasoning:** Small, high-value check sets minimize noise and maximize ROI. This is how the linter produces deterministic, actionable tasks.

**Task:**
- Enforce required check IDs per construct:

**FSM**
- `fsm.legal_state`
- `fsm.reset_known`
- `cover.fsm.transition_taken`

**Ready/valid**
- `rv.stable_while_stalled`
- `cover.rv.handshake`

**FIFO**
- `fifo.no_read_empty`
- `fifo.no_write_full`
- `cover.fifo.activity`

**Counter**
- `ctr.range`
- `ctr.step_rule`
- `cover.ctr.moved`

**Verification task (Rust policy tests):**
- Add fixture files with construct patterns but missing tags.
- Add tests that ensure the policy engine reports **missing check IDs** as violations.

Status: DONE — missing-check rule `missing_verification_check` implemented with required IDs per construct; tests in `internal/policy/verification_constructs_test.go`; verified with `VHDL_POLICY_PROFILE=debug go test ./internal/policy -run 'TestMissingChecksFor|TestMissingCoverCompanion|TestMissingLivenessBound|TestScopeAccurateMatching'`.

---

## 6) Enforce Vacuity Companions (Assert + Cover)

**Reasoning:** Properties that are never exercised create false confidence. Every assert that can be vacuous must be paired with a cover.

**Task:**
- For registry entries with `needs_cover=true`, require a matching `cover.*` tag in the same scope.
- If missing, emit a violation: `missing_cover_companion`.

**Verification task (Go):**
- Fixture: valid `rv.stable_while_stalled` tag but **no** cover tag.
- Test: linter emits `missing_cover_companion` in correct scope.

Status: DONE — fixture `testdata/verification/missing_cover_companion.vhd` and rule `missing_cover_companion`; verified with `VHDL_POLICY_PROFILE=debug go test ./internal/policy -run 'TestMissingChecksFor|TestMissingCoverCompanion|TestMissingLivenessBound|TestScopeAccurateMatching'`.

---

## 7) Bound Policy for Liveness Checks

**Reasoning:** Bounded liveness checks require a deterministic bound. This must be explicit to avoid silent nonsense.

**Task:**
- Choose one of two policies:
  - **Policy A (preferred):** Require an explicit bound annotation via tag or policy line.
  - **Policy B:** Allow default bound but mark as assumed.
- Enforce via registry (`requires_bound=true`).

**Verification task (Go):**
- Fixture: a `rv.eventual_progress_bounded` check without bound.
- Test: linter reports missing bound (violation) unless policy permits default.

Status: DONE — registry entry `rv.eventual_progress_bounded` with `requires_bound=true`, rule `missing_liveness_bound`, fixture `testdata/verification/missing_liveness_bound.vhd`; verified with `VHDL_POLICY_PROFILE=debug go test ./internal/policy -run 'TestMissingChecksFor|TestMissingCoverCompanion|TestMissingLivenessBound|TestScopeAccurateMatching'`.

---

## 8) Scope-Accurate Matching of Tags

**Reasoning:** A check in `arch:rtl` must never satisfy `arch:gate`. Scope is part of the identity.

**Task:**
- Implement tag lookups keyed by scope.
- Tags outside the verification block are ignored for coverage.

**Verification task (Go):**
- Fixture with two architectures in one file:
  - `arch:rtl` has tags
  - `arch:gate` has no tags
- Test: missing checks are emitted **only** for `arch:gate`.

Status: DONE — scope-aware tag matching (arch/entity + in-arch block) in `src/policy/verification.rs`, fixture `testdata/verification/scope_arch_fixture.vhd`, test `TestScopeAccurateMatching`; verified with `VHDL_POLICY_PROFILE=debug go test ./internal/policy -run 'TestMissingChecksFor|TestMissingCoverCompanion|TestMissingLivenessBound|TestScopeAccurateMatching'`.

---

## 9) Structured Task Output for the Agent

**Reasoning:** The agent needs unambiguous tasks (what to add, where to add it, and what bindings to use). Free-form text will degrade automation.

**Task:**
- Emit missing-check tasks in a structured format (JSON):
  - file
  - scope
  - anchor (verification block)
  - missing check IDs
  - signal bindings
  - notes / policy requirements

**Verification task (Go):**
- Add a test that asserts the JSON output for a missing check contains:
  - correct file
  - correct scope
  - correct check ID
  - correct bindings

Status: DONE — structured `missing_checks` output added to Rust/Go results (see `src/policy/result.rs`, `internal/policy/policy.go`, `internal/indexer/indexer.go`) plus output schema update; verified with `TestMissingCheckStructuredOutput` via `VHDL_POLICY_PROFILE=debug go test ./internal/policy -run 'TestMissingChecksFor|TestMissingCoverCompanion|TestMissingLivenessBound|TestScopeAccurateMatching|TestMissingCheckStructuredOutput|TestAmbiguousConstructWarning'`.

---

## 10) “Ambiguous Construct” Warnings (Not Violations)

**Reasoning:** When detection is uncertain, the linter should not emit missing-check violations. It should emit a warning with candidate signals.

**Task:**
- If detection yields multiple plausible candidates or unclear roles, emit a warning object:
  - type: `ambiguous_construct`
  - candidates: list of signals by role

**Verification task (Go):**
- Fixture where ready/valid gating exists but payload signals cannot be uniquely determined.
- Test: warning is emitted, **no missing-check violation** is emitted.

Status: DONE — ambiguous ready/valid fixture `testdata/verification/ambiguous_ready_valid.vhd` and structured `ambiguous_constructs` output with warning rule `ambiguous_construct`; verified with `TestAmbiguousConstructWarning` via `VHDL_POLICY_PROFILE=debug go test ./internal/policy -run 'TestMissingChecksFor|TestMissingCoverCompanion|TestMissingLivenessBound|TestScopeAccurateMatching|TestMissingCheckStructuredOutput|TestAmbiguousConstructWarning'`.

---

## 11) Regression Tests for Grammar-Driven Extraction

**Reasoning:** The linter depends on correct extraction, which depends on grammar. When grammar changes, construct detection must remain stable.

**Task:**
- Add a small set of grammar-dependent fixtures (FSM, FIFO, ready/valid) that are parsed in CI.
- Ensure construct detection still works and emits the same missing-check IDs.

**Verification task (Go + grammar):**
- Add tests that parse fixtures using tree-sitter and assert the expected extractor facts are present.

Status: DONE — extractor regression tests added in `internal/extractor/verification_constructs_test.go` for FSM/ready-valid/FIFO fixtures; verified with `go test ./internal/extractor -run TestVerification`.

---

## 12) End-to-End Demo (Single File Proof)

**Reasoning:** Proves the entire pipeline works: parse → extract → detect → missing checks → structured output → (agent could insert tags).

**Task:**
- Add a single small VHDL file containing:
  - FSM + counter + ready/valid
  - verification block with **some** tags missing
- The linter should emit missing checks and warnings exactly as expected.

**Verification task (integration test):**
- Add a test that runs the linter on this file and snapshots the structured output.

Status: DONE — end-to-end fixture `testdata/verification/e2e_demo.vhd` plus integration test `TestVerificationEndToEnd`; verified with `VHDL_POLICY_PROFILE=debug go test ./internal/policy -run TestVerificationEndToEnd`.

---

# Check Registry (Minimal First Pass)

This is the initial registry content to implement now:

- `fsm.legal_state` (needs cover)
- `fsm.reset_known`
- `cover.fsm.transition_taken`

- `rv.stable_while_stalled` (needs cover)
- `cover.rv.handshake`

- `fifo.no_read_empty` (needs cover)
- `fifo.no_write_full`
- `cover.fifo.activity`

- `ctr.range` (needs cover)
- `ctr.step_rule`
- `cover.ctr.moved`

---

# Notes on Alignment with Current Project

- **Grammar-first**: If construct detection fails due to parse errors, fix `grammar.js` before any heuristics.
- **CUE validation**: Tag schema validation should be wired into the existing contract guard philosophy.
- **No silent failures**: invalid or missing tags are explicit violations; ambiguous constructs are warnings.
- **Evidence strategy**: each step includes a verification test to make progress measurable and auditable.

---

If you want, I can draft the CUE schema and the registry file next, but I won’t touch code unless you explicitly ask.
