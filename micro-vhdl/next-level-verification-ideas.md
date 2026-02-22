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
