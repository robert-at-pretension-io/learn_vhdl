# Pushing Micro-VHDL Verification to the Next Level

Having successfully built a deterministic physics engine that compiles a subset of VHDL to MLIR and leverages both bounded (Z3) and unbounded (ABC) formal solvers, we have established a state-of-the-art foundation.

Based on current trends in the upstream LLVM/CIRCT ecosystem and the architectural gaps in our pipeline, here are the major ideas to push this toolchain to the absolute cutting edge.

---

## 1. Automated Logic Equivalence Checking (LEC) & Refinement

Currently, we verify that a *single* design meets its PSL properties. The next level is verifying that an *optimized* design is mathematically identical to a *reference* design.

*   **The Idea:** Add a CLI command like `./compile.sh --lec reference.vhd optimized.vhd`.
*   **The Implementation:** The Go compiler parses both files, generates symbolic inputs, feeds them to both modules, and asserts that their outputs are identical at every cycle. CIRCT already possesses the `verif.lec` and `verif.refines` operations specifically designed for this. 
*   **The Value:** You can write a brute-force combinational algorithm (reference), then write a highly complex pipelined version (optimized), and mathematically prove they behave identically without writing a single testbench.

## 2. Trace Extraction and Testbench Generation

When `circt-bmc` finds a bug, or when a `psl cover` property is reached, Z3 produces a mathematical model (a trace of signal values over time).

*   **The Idea:** Don't just print "Assertion can be violated!"—extract the exact cycle-by-cycle waveforms.
*   **The Implementation:** Write a Python or Go script that parses the Z3 output from `circt-bmc` and translates it into a standard `.vcd` (Value Change Dump) file, or automatically generates a SystemVerilog testbench that replays that exact failure scenario.
*   **The Value:** It bridges the gap between formal verification (math) and dynamic simulation (engineering). When a deep bug is found, the engineer gets a waveform they can open in GTKWave to debug visually.

## 3. Word-Level IC3 / PDR (Bypassing ABC)

Right now, to get unbounded proofs, we lower the design to AIGER (a gate-level format) and hand it to ABC. This "bit-blasts" everything. If you have a 32-bit multiplier, ABC sees thousands of individual AND/NOT gates, which destroys its performance.

*   **The Idea:** Perform IC3 at the *word-level* (keeping 32-bit integers intact).
*   **The Implementation:** CIRCT has an upstream `smt` dialect. Instead of exporting to AIGER, we could write a pass that translates the `hw` and `seq` dialects directly into `smt` dialect operations (using the SMT-LIB bitvector theory), and feed it to a word-level model checker like **Pono** or Z3's internal **Spacer** (its own IC3 implementation).
*   *(Note: Upstream CIRCT currently has open discussions/PRs regarding `FSM` to `SMT` lowering (#9379) and `AIG` dialect evolution (#7717), indicating heavy active development in this exact space).*
*   **The Value:** It would allow you to do unbounded proofs on complex datapaths (ALUs, timers, memory addresses) that currently cause ABC to choke or run out of memory.

## 4. True Compositional "Assume-Guarantee" Reasoning

We have `verif.contract` parsing and extracting, but the pipeline still flattens the design for the top-level proof.

*   **The Idea:** If `Module A` instantiates `Module B`, the proof for `Module A` should *not* look inside `Module B`.
*   **The Implementation:** When compiling `Module A`, the MLIR emitter replaces the `hw.instance` of `Module B` with a black box. It then automatically maps `Module B`'s `require` contracts to `verif.assert` (A must provide valid inputs) and its `ensure` contracts to `verif.assume` (A can trust B's outputs).
*   **The Value:** This changes verification complexity from exponential to linear. You can prove a billion-gate SoC by proving each leaf node independently, and then proving their connections.

## 5. Formal Mutation Testing (Coverage Analysis)

How do you know if you wrote *enough* PSL assertions? If your assertions pass, maybe your design is perfect, or maybe your assertions are just weak.

*   **The Idea:** Prove that the verification suite itself is complete.
*   **The Implementation:** Automate a script that injects tiny faults into the MLIR (e.g., changes a `comb.and` to a `comb.or`, or shifts an index by 1). Re-run the proofs. If *all* PSL assertions still pass despite the injected fault, the verification suite is incomplete!
*   **The Value:** This is the formal equivalent of line-coverage in software. It provides a mathematical metric for verification quality.

---

## 6. Upstream CIRCT Alignment & Required Maintenance

Based on a review of active Pull Requests in the `llvm/circt` repository, the Micro-VHDL verification pipeline is heavily aligned with the bleeding edge of the ecosystem, but will require proactive maintenance as dialects evolve.

*   **PR #9722 (`[circt-bmc] Lower LTL delay/clock patterns for BMC`)**: This is your own active PR! It directly validates the architecture we've built. The ability for `circt-bmc` to natively lower `ltl.delay` into core combinational and sequential logic is what makes Micro-VHDL's bounded sequential assertions (like `next[N]`) possible.
*   **PR #9392 (`[LTL] Make ltl.delay clocking explicit (add %clock and <edge> operands)`)**: This is a critical breaking change on the horizon. Currently, `mlir.go` emits `ltl.delay %val, 1, 0 : i1` without a clock, and wraps the final sequence in an `ltl.clock` block. When this PR merges, the `ltl.delay` operation signature will change, and we will need to update `mlir.go` to explicitly pass the `%clk` SSA value to every `ltl.delay` operation we emit.
*   **PR #9379 (`[FSM] Convert FSM to SMT`)** & **PR #7717 (`RFC: [AIG] Add an AIG dialect and circt-synth`)**: These PRs show that the upstream community is actively building the infrastructure required for Idea #3 (Word-Level IC3/PDR). The push to natively lower constructs directly to the `smt` dialect or a native `aig` dialect implies that in the near future, we may be able to run unbounded proofs natively within the CIRCT ecosystem, completely bypassing the external Yosys-ABC AIGER export step.

---

## 7. Plan of Attack: Idea #3 (Word-Level IC3 / PDR)

Based on an architectural investigation of the vendored `cirt` codebase, we can achieve Word-Level unbounded verification immediately by pivoting our pipeline from AIGER to the BTOR2 format.

AIGER forces the design into an And-Inverter Graph (bit-blasting all 32-bit registers into 32 individual boolean latches). BTOR2, however, natively preserves word-level semantics (`sort bitvec 32`, `add`, `mul`), which radically reduces the state-space complexity for SMT-based IC3/PDR engines.

### Phase 1: BTOR2 Export via CIRCT (Available Now)
The current CIRCT framework already possesses the exact passes we need to bypass ABC. 

1.  **Lower Temporal Protocols to Core Logic:** 
    BTOR2 natively supports `verif.assert` and `verif.assume` but does not understand `ltl` temporal protocols. We first run the existing pass `circt-opt --lower-ltl-to-core` (or `--lower-ltl-to-bmc`), which mathematically translates all sequence delays and implications into an explicit shift-register state machine composed entirely of `seq.compreg` and `comb` operations.
2.  **Export to BTOR2:**
    We then run the undocumented `circt-opt --convert-hw-to-btor2` pass. This pass natively ingests `hw`, `seq`, `comb`, `verif.assert`, and `verif.assume` operations and directly emits a textual BTOR2 transition system to standard output.
    *Example Pipeline:* `circt-opt input.mlir --lower-ltl-to-core --convert-hw-to-btor2 > output.btor2`
3.  **External Word-Level Solver (Pono/BtorMC):**
    With the `output.btor2` file generated, we execute an external, BTOR2-compatible word-level model checker such as **Pono** or **BtorMC (Boolector)**. This completely bypasses `yosys-abc` and enables unbounded IC3/PDR verification on massive datapaths that would otherwise cause an AIGER bit-blast to OOM.

### Phase 2: Native SMT Dialect Integration (Future Work)
To eliminate external solver binaries entirely (like we currently do with Z3 in `circt-bmc`), we need to bridge the gap between `hw` and the upstream `smt` dialect.

1.  **The Missing Link (`HWToSMT`):** We must build (or adopt from upstream PRs) a lowering pass that translates `hw.module`, `seq.compreg`, and `comb` operations directly into an `smt` Transition System (using `smt.declare_fun`, `smt.eq`, and `smt.forall`).
2.  **Z3 Spacer Integration:** Expand `circt-bmc` to support an unbounded mode flag (e.g., `--engine=spacer`). Z3 Spacer is a specialized IC3/PDR solver engine for SMT Horn clauses. By feeding the lowered `smt` Transition System directly into Spacer via LLVM JIT, we achieve lightning-fast, native, word-level unbounded verification entirely inside the MLIR ecosystem.
