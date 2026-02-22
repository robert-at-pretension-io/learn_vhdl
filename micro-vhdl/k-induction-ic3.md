# k-Induction and IC3/PDR — Formal Verification Algorithms

## Bounded Model Checking Recap

BMC unrolls the circuit N times and asks: "does there exist an assignment of inputs and initial states that violates the property within N cycles?" It finds bugs but cannot prove absence of bugs — "bound reached" just means no bug in N steps, not no bug ever.

Both k-induction and IC3 are answers to the question: how do you get from "no bug in N steps" to "no bug ever"?

---

## k-Induction

Mathematical induction applied to hardware. Standard induction says: if P holds at step 0, and P(s) implies P(s+1) for any state s, then P holds for all time.

The hardware version has a problem immediately: the inductive step says "assume P holds in some state s, prove it holds in state s+1." But that arbitrary state s might be unreachable. The solver is free to start from a garbage state that satisfies P but transitions to a state that violates P — even if that garbage state is never reachable from the real initial state. So induction fails even when the property is true.

**k-induction** fixes this by widening the window. Instead of assuming P holds in one arbitrary state, assume P holds in k consecutive states, then prove it holds in state k+1. With a larger k, the k-step trace is harder to satisfy with unreachable states. Eventually — for a large enough k — the only k-step traces that satisfy all k copies of P are genuine reachable traces, and then the inductive step goes through.

Concretely, for k=3:

```
Base case:    prove P holds at cycles 0, 1, 2  (standard BMC)
Inductive:    assume P holds at cycles i, i+1, i+2 for arbitrary i
              prove P holds at cycle i+3
```

If both hold, P holds for all cycles. No bound on depth.

The practical issue: you may need a very large k before the inductive step stops failing, and there is no general way to know in advance what k is sufficient. For some designs k=1 works (the invariant is "1-inductive"). For others you need auxiliary lemmas — extra invariants added to strengthen the hypothesis — to make the inductive step go through at a smaller k.

k-induction is what most industrial tools used before IC3 arrived. It works well for simple invariants on well-structured pipelines but struggles with complex state machines where the reachable state space is hard to characterize with a short trace.

---

## IC3 / PDR

Developed by Aaron Bradley in 2011. IC3 stands for "Incremental Construction of Inductive Clauses for Indubitable Correctness." PDR (Property Directed Reachability) is the same algorithm under a different name that emphasizes what it actually does.

The core insight is different from k-induction. Instead of asking "does a long enough trace prove the property?", IC3 asks: "what is the strongest invariant I can discover that is both true and implies the property?"

### The Frame Sequence

IC3 maintains a sequence of sets of states F_0, F_1, F_2, ..., F_k where:

- F_0 = the initial states of the design
- F_i is an over-approximation of all states reachable in at most i steps
- Each F_i is represented as a conjunction of clauses (CNF), discovered incrementally

The invariant maintained: F_0 ⊆ F_1 ⊆ ... ⊆ F_k, and each F_i only contains states that satisfy property P.

### The Algorithm Loop

1. **Check if the bad state is reachable in one step from F_k.** If yes, try to block it.
2. **Blocking**: when a bad state B is reachable from some state in F_i, find a clause (a constraint on the predecessor) that rules out reaching B. Push that clause backwards through frames: if it blocks B from F_i, check if it also blocks it from F_{i-1}, and so on.
3. **Propagation**: push learned clauses forward through frames. If a clause holds in F_i and is inductive relative to F_i, it propagates to F_{i+1}.
4. **Termination**:
   - If F_i = F_{i+1} (the frame stabilizes), a fixed-point inductive invariant has been found. Property is proved.
   - If a chain of counterexample states reaches F_0, a genuine counterexample trace exists.

### Why It's Powerful

IC3 discovers the right invariants automatically, guided by the property it's trying to prove. Each "blocking clause" it learns is a piece of the inductive invariant. It never needs to know k in advance. It doesn't blow up exponentially with depth because it works in clause space (compact logical formulas) rather than unrolled circuit copies.

In practice IC3 can prove properties that require a conceptually infinite inductive argument — things like "a FIFO never loses data" — where k-induction would need an astronomically large k or a human-supplied auxiliary invariant.

### The Intuition

Think of IC3 as building a wall between the initial states and the bad states, one brick at a time. Each brick is a clause it discovers. The wall is an inductive invariant. The algorithm is "property directed" because it only places bricks where they're needed to block paths to the bad state.

---

## Liveness: The Harder Problem

Both k-induction and IC3 as described handle **safety** properties: "bad thing never happens." Liveness — "good thing eventually happens" — is strictly harder.

The standard reduction is **liveness-to-safety**: convert `eventually P` into a safety property by introducing a cycle counter. If P hasn't fired within N cycles, that's a safety violation. But this re-introduces a bound, which is exactly what you were trying to avoid.

The proper treatment uses **Buchi automata**. A liveness property `always eventually P` is encoded as: there is no infinite path where P is never true. This becomes: the system has no reachable "lasso" — a cycle the system can loop in forever while P stays false. IC3 with fairness constraints (IC3F) can check this. It looks for lassos in the state graph rather than paths to bad states.

CIRCT's `ltl.eventually` is designed to feed into this kind of analysis, but `circt-bmc` does not implement it. The `--lower-ltl-to-core` pass handles only safety-reducible LTL fragments.

---

## Comparison

| | BMC | k-Induction | IC3/PDR |
|---|---|---|---|
| Can find bugs | Yes (within bound) | Yes (within bound) | Yes (any depth) |
| Can prove safety | No | Yes (if k found) | Yes |
| Can prove liveness | No | No (without reduction) | Yes (with fairness) |
| Needs bound parameter | Yes | Yes (k) | No |
| Needs auxiliary invariants | No | Sometimes | No (discovers them) |
| Scales with state space | Poorly | Poorly | Better |
| Implementation complexity | Low | Medium | High |

---

## Implementation in Micro-VHDL (Current State)

Both BMC and IC3/PDR are now integrated and run automatically in sequence.

### Pipeline

```
VHDL Files → Go compiler → Linking/Detection → build.mlir + build_ic3.mlir
                               |                      |
                        circt-bmc (Z3)          circt-opt (Flattening)
                        BMC, 15 cycles                |
                               |                ---------------------
                        "Assertion can          |                   |
                        be violated!"     circt-translate     circt-opt (BTOR2)
                        + VCD Trace       → AIGER (.aig)      → BTOR2 (.btor2)
                               |                |                   |
                                          yosys-abc pdr       btormc (Word-Level)
                                                |                   |
                                         "Property proved"   "Property proved"
                                         or counterexample   or counterexample
```

### Two MLIR Variants

The Go compiler emits two MLIR files from the same VHDL project:

**BMC MLIR** (`foo.mlir`):
- Hierarchical `hw.instance` nodes
- `verif.assert %combined : i1`
- `seq.initial` blocks — pins registers to zero for deterministic BMC
- Used by `circt-bmc`, `btormc` (via BTOR2 export), and `firtool`

**IC3 MLIR** (`foo_ic3.mlir`):
- `out __verif_bad: i1` port for bit-level PDR
- Sub-modules marked `private` to enable automated flattening
- No `seq.initial` — PDR must explore from all possible initial states
- Used only for the AIGER/ABC path

### Key Design Decisions

**Word-Level verification (BTOR2)**: Bit-level engines (AIGER) struggle with wide datapaths (e.g. 32-bit adders) because they see thousands of individual gates. BTOR2 preserves the "Word" semantics (`add i32`), allowing SMT-based solvers like `btormc` to solve complex math instantly.

**Automated Hierarchical Flattening**: `compile.sh` automatically runs `--hw-flatten-modules` and `--symbol-dce`. This collapses the entire VHDL project into a single top-level module, ensuring that sub-module logic is fully visible to the formal engines without needing a manual elaboration step.

**Trace Extraction**: Failing properties in the BTOR2 path automatically generate a `_trace.vcd` file via `btor2vcd.py`, enabling visual debugging.

### Limitations

- IC3 (ABC) counterexamples still come back as generic `input_0`/`output_0` names (use BTOR2 for named signals)
- LTL properties (`ltl.delay` / `ltl.clock`) are flattened to core logic for the BTOR2 path

