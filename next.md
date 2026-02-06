# Audit Round 2: Recommended Improvements (Ordered)

This is a fresh audit-driven list of **new, actionable changes** to strengthen the codebase. It focuses on correctness gaps discovered in the current implementation (indexer, resolver, policy, schemas, and extractor). Each step includes a concrete verification task so progress is measurable.

Scope note: this plan is **linter‑only**. No agent work is included.

---

## 1) Eliminate Schema Drift (single CUE source of truth)

**Reasoning:** There are two CUE schemas (`internal/validator/schema.cue` and `schema/ir.cue`) with different field coverage. This creates silent contract drift between docs and enforcement.

**Task:**
- Decide the canonical schema (recommend: `internal/validator/schema.cue`).
- Remove or auto‑generate the secondary schema from the canonical source.
- Add a CI/test guard that fails if a drifted copy reappears.

**Verification task (Go):**
- Add a test that loads both schemas (if a copy remains) and asserts they are byte‑identical.
- If removing `schema/ir.cue`, add a doc test that fails if the file exists.

**Notes (2026-02-02):**
- Added `TestNoSecondaryIRSchema` in `internal/validator/validator_test.go` to fail if `schema/ir.cue` reappears.
- Verified with `go test ./internal/validator -run TestNoSecondaryIRSchema`.

---

## 2) Surface Unresolved Non‑Instantiation Dependencies

**Reasoning:** The indexer computes missing imports for `use/library/context` dependencies but does not emit policy violations for them. That leaves cross‑file visibility errors silent.

**Task:**
- Add a rule (e.g., `unresolved_import`) that reports unresolved **package/context** dependencies, not just instantiations.
- Use `Dependency.Kind` to distinguish `use`, `library`, `context`, `package_instantiation`, etc.

**Verification task (Rust):**
- Fixture: file B uses `work.pkg.all` with no `pkg` defined in any file → expect `unresolved_import`.
- Fixture: same with `pkg` present in another file → no violation.

**Notes (2026-02-02):**
- Verified unresolved import coverage via `cargo test -q unresolved_import` (passes).
- Rule emits `unresolved_library` / `unresolved_package` under the `unresolved_import` family.

---

## 3) Make Resolution Library‑Aware (avoid cross‑library collisions)

**Reasoning:** Subprogram/package resolution currently ignores the library portion (e.g., `lib.pkg.fn`) and scope IDs are file‑based. This can incorrectly resolve symbols across libraries that share names.

**Task:**
- Include library name in scope IDs or in symbol definition keys.
- Update qualified name parsing to respect `lib.pkg.fn` and `work.pkg.fn` explicitly.
- Use file → library mapping from the indexer to resolve `work` correctly.

**Verification task (Rust):**
- Fixture: two libraries define `pkg.fn`; a file `use`s only one library. Ensure the call resolves to the correct one and the other is not considered visible.

**Notes (2026-02-02):**
- Verified `subprogram_resolution_library` fixture via `VHDL_CROSS_FILE_FILTER=subprogram_resolution_library go test ./internal/policy -run TestCrossFileSemanticFixtures`.
- Library-aware name resolution uses file→library mapping for `work` and qualified `lib.pkg.fn` paths.

---

## 4) Unqualified Subprogram Resolution with Visibility

**Reasoning:** Only qualified calls are checked. Unqualified calls should resolve using `use/context` visibility and overload matching; otherwise unresolved calls escape detection.

**Task:**
- Use the visibility set (Step 2/3) to resolve unqualified function/procedure calls.
- Emit `unresolved_unqualified_call` and `ambiguous_unqualified_call` as needed.

**Verification task (Rust):**
- Fixture: `use work.pkg.all;` and call `foo(...)` resolves; without `use`, it fails.
- Fixture: two visible overloads with same arity → ambiguous.

---

## 5) Strengthen Port/Generic Type Compatibility (beyond width)

**Reasoning:** Port width mismatches are caught, but type compatibility across files (record/array/enums, signed/unsigned, subtypes) is not enforced. This is a key semantic correctness gap.

**Task:**
- Implement a minimal type resolver that:
  - resolves package‑defined types and subtypes
  - compares base types (enum/record/array/physical) with constraints
- Use it to validate:
  - port map actuals vs formals
  - generic map actuals vs generic declarations

**Verification task (Rust):**
- Fixture: record type `pkt_t` defined in package A, used as a port in entity B, connected to a mismatched type in C → expect `port_type_mismatch`.
- Fixture: generic `G_WIDTH: natural` bound to a string literal → expect `generic_type_mismatch`.

---

## 6) Configuration Binding Correctness (with architecture resolution)

**Reasoning:** Configurations are parsed, but binding correctness is only shallow. Cross‑file correctness requires verifying that configuration specifications bind to real architectures and are applied to instances.

**Task:**
- Validate configuration declarations against available architectures.
- If a configuration targets a specific architecture, ensure instances of that entity respect the binding.

**Verification task (Rust):**
- Fixture: configuration binds missing architecture → error.
- Fixture: configuration binds architecture `rtl`, instance uses `work.ent(rtl)` → no error; binding mismatch → error.

---

## 7) Improve Verification‑Tag Robustness (stray tags + scope drift)

**Reasoning:** Tags are only parsed inside `verification` blocks; tags outside are silently ignored. This can hide real tags and create false negatives.

**Task:**
- Emit `stray_verification_tag` when a `--@check` appears outside a verification block.
- Add a rule to warn if a tag’s `scope=` does not match its enclosing architecture.

**Verification task (Go + Rust):**
- Fixture with `--@check` outside a verification block → expect `stray_verification_tag`.
- Fixture with `scope=arch:rtl` inside `arch:gate` → expect `scope_mismatch` warning.

---

## 8) Turn Output Schema into a Real Gate

**Reasoning:** There is an output schema (`output_schema.cue`) but it is not enforced anywhere. This is a perfect place to prevent regression in structured outputs.

**Task:**
- Validate `LintResult` against `output_schema.cue` in tests (or in `--json` mode when `VHDL_VALIDATE_OUTPUT=1`).

**Verification task (Go):**
- Add a test that runs the linter on a tiny fixture, validates JSON output against the output schema, and fails on mismatch.

---

# Priority Order Summary

1. Schema drift removal
2. Unresolved imports (non‑instantiation dependencies)
3. Library‑aware symbol resolution
4. Unqualified subprogram resolution
5. Type compatibility for ports/generics
6. Configuration binding correctness
7. Verification‑tag robustness
8. Output schema validation gate

---

If you want, I can draft the fixtures for steps 2–6 and list the exact rule IDs to use, but I won’t modify code unless explicitly asked.
