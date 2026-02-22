# Layer 6: Liveness via ABC Fairness (Implementation Plan)

## The Discovery
While the official AIGER 1.9 format supports liveness properties via "justice" (`j0`), ABC's `read_aiger` engine currently rejects them with a hardcoded error. However, examining ABC's source code (`abc/src/proof/live/liveness.c`) reveals a hidden, undocumented port naming convention: if an AIGER output port's name starts with **`assert_fair`**, ABC intercepts it as a liveness Buchi acceptance condition (a fairness constraint).

This allows us to leverage ABC's highly optimized `l2s` (Live-to-Safe) transformation, which automatically injects Armin Biere's non-deterministic lasso-detection shadow registers. This converts the unbounded liveness problem into a standard safety problem that `pdr` can solve natively, completely avoiding the need to build a complex global state-saving automaton inside `mlir.go`.

## The Plan

### [x] Step 1: Update MLIR Emission State (`mlir.go`)
- Add a new accumulator to `MLIREmitter` (e.g., `livenessAsserts []string`) to track liveness properties separately from boolean safety (`__verif_bad`) and temporal safety (`!ltl.property`).

### [x] Step 2: Handle `PslEventuallyExpr` (`mlir.go`)
- **In BMC mode:** Continue emitting `ltl.eventually` (and wrapping it in `ltl.implication` and `ltl.clock`). `circt-bmc` will ignore the liveness aspect but the MLIR remains valid.
- **In IC3 mode:** 
  - Translate `left -> eventually! right` into the Buchi acceptance condition: we must *infinitely often* reach a state where the antecedent is false OR the consequent is true.
  - Emit the boolean logic: `NOT(left) OR right`.
  - Push the resulting SSA value into `livenessAsserts` instead of emitting `verif.assert`.

### [x] Step 3: Generate the `assert_fair` Port (`mlir.go`)
- Modify `EmitModule` to check if `livenessAsserts` is non-empty in IC3 mode.
- If true, generate an output port named `assert_fair: i1`.
- If there are multiple liveness properties, AND them together into a single `assert_fair` output (meaning *all* liveness conditions must be satisfied infinitely often).
- Ensure the port generation logic doesn't conflict with the `__verif_bad` safety output. A module can have both.

### [x] Step 4: Update the Compiler Pipeline (`compile.sh`)
- Add detection for the `assert_fair` string in the generated `_ic3.mlir` file (e.g., `HAS_LIVENESS=$(grep -c 'assert_fair' "$IC3_MLIR" 2>/dev/null || true)`).
- Alter the ABC command sequence in the IC3/PDR step based on what's found:
  - **Safety only (`__verif_bad` present, no `assert_fair`):** `read_aiger ${IC3_AIG}; pdr; quit`
  - **Liveness present (`assert_fair` present, with or without safety):** `read_aiger ${IC3_AIG}; l2s; pdr; quit`

### [x] Step 5: Testing and Verification
- Create a test file (e.g., `test_liveness.vhd`) with a known liveness property.
- Verify that `compile.sh` correctly invokes `l2s` and that ABC successfully proves or disproves the property.
- Update `copy_code.sh` to include the new test file.
- Update `verif-extensions.md` to mark Layer 6 as completed.
