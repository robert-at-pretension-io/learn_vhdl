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
VHDL → Go compiler → build.mlir + build_ic3.mlir
                           |               |
                     circt-bmc (Z3)   circt-synth
                     BMC, 15 cycles   → AIG dialect
                           |               |
                    "Assertion can    circt-translate
                    be violated!"    --export-aiger
                    or "Bound        → binary .aig
                    reached"               |
                                     yosys-abc pdr
                                           |
                                    "Property proved"
                                    or counterexample
```

### Two MLIR Variants

The Go compiler emits two MLIR files from the same VHDL:

**BMC MLIR** (`foo.mlir`):
- `verif.assert %combined : i1` — circt-bmc consumes this directly
- `seq.initial` blocks — pins registers to zero at cycle 0 for deterministic BMC
- Used by both circt-bmc and firtool (SV output)

**IC3 MLIR** (`foo_ic3.mlir`):
- `out __verif_bad: i1` — negated conjunction as hw.output port
- No `seq.initial` — PDR must explore from all possible initial states, not just zero
- No `verif.assert` — only hw.output; circt-synth can lower this to AIG cleanly
- Used only for the AIGER/ABC path

### Key Design Decisions

**Why no `seq.initial` in IC3 MLIR**: `circt-synth` cannot lower `seq.initial` to AIG (it is not a comb or seq register op). More importantly, pinning initial register state to zero would cause IC3 to miss bugs only reachable from other starting states. PDR by design explores from all initial states.

**Why a separate MLIR file**: `verif.assert` cannot pass through `circt-synth`'s AIG lowering pipeline. Emitting a clean IC3 MLIR without `verif.assert` avoids needing a stripping pass.

**Why `__verif_bad = NOT(conjunction)`**: The AIGER model checking convention is that hw.outputs represent "bad state" signals. ABC's `pdr` command proves these outputs are unreachable. The negation converts "property holds" (which the assertion checks) to "property violated" (which PDR checks for reachability).

**Why binary AIGER only**: `yosys-abc read_aiger` rejects ASCII `.aag` files when symbol names contain bracket characters (e.g., `a[0]`). The binary `.aig` format is used throughout.

### Limitations

- IC3 counterexamples come back as generic `input_0`/`output_0` names, not the original VHDL signal names
- Liveness properties (`psl eventually!`) are still stubbed as `hw.constant true`
- Hierarchical designs are not yet flattened before AIGER export
- LTL properties (`ltl.delay` / `ltl.clock`) are BMC-only; IC3/PDR skips them in the AIGER path
