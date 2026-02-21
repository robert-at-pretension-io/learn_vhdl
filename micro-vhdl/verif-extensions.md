# Micro-VHDL Verification Extensions

## Current State (as of February 2026)

### What is implemented

**Compile pipeline** (`compile.sh`):
```
[1/3]  VHDL → build.mlir + build_ic3.mlir   (Go compiler, two MLIR variants)
[2a/3] BMC via Z3 (circt-bmc, bound=15)      — finds shallow bugs, concrete traces
[2b/3] IC3/PDR via ABC pdr (unbounded)        — proves safety or finds deep bugs
[3/3]  MLIR → SystemVerilog (firtool)
```

**PSL operators supported**:
- `|=>` (next-cycle implication): delay register + boolean implication → `verif.assert : i1`
- `stable(x)`: delay register + `comb.icmp eq` → `verif.assert : i1`
- `eventually!`: stubbed as `hw.constant true` + TODO comment (liveness, skipped in BMC)

**MLIR emission modes**:
- **BMC mode** (default): emits `verif.assert`, `seq.initial` for zero-initialized registers
- **IC3 mode** (`_ic3.mlir`): emits `__verif_bad: i1` hw.output (negated conjunction), no `seq.initial` (PDR explores from unconstrained initial states)

**Toolchain**:
- `circt-bmc` + Z3: BMC, up to 15 cycles (configurable via `BOUND=N`)
- `circt-synth` → `circt-translate --export-aiger` → `yosys-abc pdr`: unbounded IC3/PDR
- `firtool`: SystemVerilog output

**Known gotchas**:
- `yosys-abc read_aiger` only accepts binary `.aig` format; ASCII `.aag` silently fails
- `circt-synth` cannot handle `seq.initial` — IC3 MLIR omits it by design
- IC3 step is skipped automatically when no PSL assertions are present

---

## Layer 1 — Low-Hanging Fruit (new keywords, trivial emission)

### `psl assume`

The single most impactful missing feature. Right now if you write `psl assert always stable(req)` and req is a free input, Z3 finds a counterexample because inputs are unconstrained. To make a property meaningful you first need to constrain the environment:

```
psl assume always (req = '1' -> not(req = '0'));  -- req never deasserts
psl assert always (req = '1' -> next(ack = '1'));
```

The AST node is structurally identical to `PslAssertion`. The only difference is the MLIR opcode: `verif.assume` instead of `verif.assert`. For the IC3 path, `verif.assume` would constrain the AIGER input space (encoded as an implication on the bad-state output: `bad = violation AND assumption_holds`).

Without `assume`, every property that touches an input port is suspect — the solver is free to choose adversarial inputs.

### `psl cover`

Maps to `verif.cover`. Tells the solver: find a reachable state where this is true. Used for coverage — proving that a specific scenario is reachable at all, not just that it never happens.

### `psl assert never`

Syntactic sugar for `always (not P)`. More readable for error conditions:

```
psl assert never (state = ERROR_STATE);
```

---

## Layer 2 — Sequence Operators (new AST nodes, clean LTL mapping)

Right now all PSL temporal expressions are single-level. There is no way to express patterns across multiple cycles except via `|=>`, hardcoded as exactly one cycle.

### `next[N]` / `next_a[M to N]`

`next[3](ack)` means "ack holds exactly 3 cycles from now." Maps to `ltl.delay %ack, 3`. `next_a[1 to 8](ack)` maps to `ltl.delay %ack, 1, 8`. Unlocks bounded response: "if req fires, ack must arrive within 8 cycles."

### Sequence concatenation `{a; b; c}`

PSL sequences: `{req; stable(data); ack}` means req is true now, data stable next cycle, ack fires the cycle after. Maps to `ltl.concat`. Foundation of protocol verification.

### Sequence repetition `{a[*N]}` / `{a[*M to N]}`

`{stable(data)[*4]}` means data stable for 4 consecutive cycles. Maps to `ltl.repeat`. Useful for bus hold-time requirements.

### `|->` (overlapping) vs current `|=>` (non-overlapping)

Currently `|=>` always creates a 1-cycle delay register. PSL's `|->` means the consequent starts at the same cycle as the antecedent match — no delay. Exposing both gives the full SVA vocabulary.

---

## Layer 3 — Reset-Conditional Properties

One of the most common sources of false positives: the tool reports a violation in cycle 0 because the register starts in an unconstrained state (especially true for IC3, which explores from all initial states). CIRCT has explicit infrastructure: `verif.has_been_reset`.

Adding a new PSL construct:

```
psl assert always after_reset(clk, rst) (invariant);
```

...would emit:

```mlir
%has_reset = verif.has_been_reset %clk_i1, sync %rst
%seq = ltl.clock %invariant, posedge %clk_i1 : i1
verif.ensure %seq if %has_reset : !ltl.sequence
```

This is especially valuable for the IC3 path where initial states are truly unconstrained.

---

## Layer 4 — `verif.contract` Blocks on Entities

The big architectural leap for scalability. Right now all `verif.assert` ops sit flat inside the module. For hierarchical designs the solver reasons about everything together, which blows up exponentially.

The idea: add a `contract` block to the micro-vhdl entity syntax:

```vhdl
entity Arbiter is
  port (req0, req1 : in std_logic; grant0, grant1 : out std_logic; clk : in std_logic);
  contract
    require: never(req0 = '1' and req1 = '1');  -- mutual exclusion on inputs
    ensure:  never(grant0 = '1' and grant1 = '1');  -- mutual exclusion on outputs
end entity;
```

This maps to `verif.contract` with `verif.require` and `verif.ensure`. When compiling the sub-module the tool proves the contract. When compiling the parent and instantiating Arbiter, it uses apply-mode — the `ensure` becomes an axiom without re-proving. Sub-module proof complexity does not grow with the top-level design.

---

## Layer 5 — `verif.formal` Standalone Blocks

Currently micro-vhdl always ties verification to a specific `hw.module`. A `verif.formal` block is an independent formal function that creates symbolic values, instantiates modules, and asserts properties without being part of any module.

This enables:
- **Cross-module properties**: instantiate two modules, constrain interaction, prove a joint property
- **Parameterized formal checks**: same property at different input widths
- **LEC via `verif.lec`**: two implementations, same symbolic inputs, assert output equality

---

## Layer 6 — Liveness

`psl assert always (req -> eventually! ack)` is a liveness property: ack must eventually arrive. It cannot be falsified in a finite number of cycles, so BMC cannot prove or disprove it.

**Current status**: emitted as `hw.constant true` + TODO comment in both BMC and IC3 MLIR. Effectively skipped.

**What's needed**: ABC has liveness checking via Buchi automaton encoding. The `ltl.eventually` op in CIRCT is designed for this. The missing piece is:
1. Emit `ltl.eventually` properly in the IC3 MLIR (not `hw.constant true`)
2. Use a different ABC command sequence: `read_aiger; ltl_properties; pdr` with fairness constraints
3. Encode the Buchi condition in the AIGER (a "justice" output rather than a "bad state" output)

This requires ABC's liveness mode and a different AIGER encoding convention — it's a separate effort from the safety IC3 path that is already working.

---

## Priority Summary

| Extension | Status | Effort | Value |
|---|---|---|---|
| IC3/PDR via ABC pdr | **Done** | — | Unbounded safety proofs |
| `psl assume` | Not started | Very low | Critical — constrains adversarial inputs |
| `psl never` | Not started | Very low | Readability |
| `psl cover` | Not started | Very low | Coverage completeness |
| Reset-conditional (`after_reset`) | Not started | Low | Eliminates cycle-0 false positives in IC3 |
| `next[N]` / delay | Not started | Low | Bounded response properties |
| `|->` (overlapping implication) | Not started | Low | Full SVA vocabulary |
| Sequence concat/repeat | Not started | Medium | Protocol verification |
| `verif.contract` on entities | Not started | High | Compositional, scalable verification |
| `verif.formal` blocks | Not started | High | Cross-module and LEC |
| Liveness via ABC fairness | Not started | High | Full temporal logic |

**Highest immediate value**: `psl assume` — without it, most input-touching assertions produce trivial counterexamples because the solver is free to choose worst-case inputs. This is the single change that makes formal verification of real protocols useful.
