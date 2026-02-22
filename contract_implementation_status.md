# Micro-VHDL: `verif.contract` Implementation Status

## Goal
Implement support for Hierarchical Contracts (`verif.contract`) in the Micro-VHDL pipeline. This allows attaching preconditions (`require`) and postconditions (`ensure`) to `entity` definitions, enabling compositional formal verification.

## Progress So Far
We have successfully implemented the frontend parsing and extraction:
1. **Tree-Sitter Grammar:** Updated `grammar.js` to parse `contract` blocks inside `entity` declarations, supporting `require:` and `ensure:` statements with full PSL expression syntax. Recompiled the parser.
2. **AST & Extractor:** Added `Contract` struct to `ast.go` and implemented extraction logic in `extractor.go` to pull these expressions into the Go AST.
3. **MLIR Emission (`mlir.go`):** Updated the MLIR emitter to wrap module outputs in a `verif.contract` operation. The `require` blocks evaluate against the inputs, and the `ensure` blocks evaluate against the contract's outputs.
4. **Pipeline Execution (`compile.sh`):** Updated the compile script to skip unbounded IC3/PDR proofs when contracts are present. The AIGER format used by ABC PDR does not support symbolic values or assumptions (`verif.assume`), so contracts are currently restricted to the Bounded Model Checking (BMC) pipeline using Z3.

## The Blocker: MLIR Dominance Errors in CIRCT
We are currently stuck on how to correctly structure the MLIR for `verif.contract` so that `circt-bmc` will accept it without throwing an SSA dominance error.

Here is the generated MLIR for a simple contract:
```mlir
hw.module @Arbiter(in %req0: i1, in %req1: i1, out grant0: i1, out grant1: i1, in %clk: !seq.clock) {
  // ... internal logic ...

  // The contract wraps the internal signals (%grant0_internal, %grant1_internal)
  %10, %11 = verif.contract %grant0_internal, %grant1_internal : i1, i1 {
    // Require precondition against input
    %req_check = comb.icmp eq %req0, %req1 : i1
    verif.require %req_check : i1
    
    // Ensure postcondition against contract output
    %ens_check = comb.icmp eq %10, %11 : i1  // <--- ERROR HAPPENS HERE
    verif.ensure %ens_check : i1
  }
  hw.output %10, %11 : i1, i1
}
```

### The Problem
When we pass this MLIR to `circt-bmc --module=Arbiter -b 15 --run`, it fails with:
```
error: operand #0 does not dominate this use
    %ens_check = comb.icmp eq %10, %11 : i1
                              ^
note: operand defined here (op in a parent region)
  %10, %11 = verif.contract %grant0_internal, %grant1_internal : i1, i1 {
```

**Why this happens:** In MLIR, an operation's results (`%10`, `%11`) cannot be used inside the operation's own region. However, the conceptual design of `verif.contract` (and how it's documented in CIRCT) heavily implies that `ensure` conditions should be checking the *outputs* of the contract.

### What we've tried:
1. **Using the inputs instead:** If we change the `ensure` to check the inputs to the contract (e.g., `%grant0_internal`) instead of the results (`%10`), the dominance error disappears. However, `circt-bmc` then reports `warning: no property provided to check in module - will trivially find no violations.` because the contract is treated as a pass-through and is not actually evaluated or checked by the standard `circt-bmc` pipeline.
2. **Manual Lowering via `circt-opt`:** If we manually run `circt-opt --lower-contracts`, CIRCT extracts the contract into a new standalone test module: `verif.formal @Arbiter_CheckContract_0 {}`. But if we then try to run `circt-bmc` on *that* test module, it fails with errors regarding un-externalized registers (`no num_regs or initial_values attribute found`).
3. **Full Manual Pipeline:** Running `circt-opt --lower-contracts --verif-lower-tests --externalize-registers --lower-to-bmc="top-module=Arbiter_CheckContract_0"` works to generate the final lowered SMT/BMC logic, but doing this requires bypassing the standard `circt-bmc` tool's automated pass manager, which usually handles all of this seamlessly.

### Next Steps needed
We need to determine the correct way to express `verif.contract` in MLIR so that `circt-bmc` will automatically verify it, OR we need to update our `compile.sh` pipeline to run the necessary `circt-opt` lowering passes explicitly before invoking `circt-bmc` on the newly generated test wrappers.