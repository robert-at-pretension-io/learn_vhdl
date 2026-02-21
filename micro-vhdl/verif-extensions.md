# Micro-VHDL Verification Extensions — Design Discussion

## Current Coverage

What micro-vhdl already emits:

- `verif.assert` (single, conjuncted from all PSL assertions)
- `seq.compreg` delay + `comb.or` for `|=>` implication
- `seq.compreg` + `comb.icmp eq` for `stable()`
- `hw.constant true` stub for `eventually!`

Everything else is either absent from the AST, absent from the grammar, or present but emitted as a no-op.

---

## Layer 1 — Low-Hanging Fruit (new keywords, trivial emission)

### `psl assume`

The single most impactful missing feature. Right now if you write `psl assert always stable(req)` and req is a free input, Z3 correctly finds a counterexample because inputs are unconstrained. To make a property meaningful you often need to first constrain the environment:

```
psl assume always (req = '1' -> not(req = '0'));  -- req never deasserts
psl assert always (req = '1' -> next(ack = '1'));
```

The AST node is structurally identical to `PslAssertion`. The only difference is the MLIR opcode: `verif.assume` instead of `verif.assert`. One new keyword, one new statement type, one-line change in the emitter.

Without `assume`, every property that touches an input port is suspect — Z3 is free to choose adversarial inputs that satisfy the antecedent but not the consequent.

### `psl cover`

Maps to `verif.cover`. Tells the solver: find a reachable state where this is true. Used for coverage — proving that a specific scenario is reachable at all, as opposed to proving it never happens. Useful during design to confirm the testbench can exercise interesting states.

### `psl assert never`

`never P` is syntactic sugar for `always (not P)`. Worth adding as a first-class keyword because it reads more naturally for error conditions:

```
psl assert never (state = ERROR_STATE);
```

---

## Layer 2 — Sequence Operators (new AST nodes, clean LTL mapping)

Right now all PSL temporal expressions are single-level. There is no way to express patterns across multiple cycles except via `|=>`, which is hardcoded as one cycle.

### `next[N]` / `next_a[M to N]`

`next[3](ack)` means "ack holds exactly 3 cycles from now." Maps directly to `ltl.delay %ack, 3`. `next_a[1 to 8](ack)` maps to `ltl.delay %ack, 1, 8`. This unlocks bounded response: "if req fires, ack must arrive within 8 cycles."

### Sequence concatenation `{a; b; c}`

PSL sequences use semicolon: `{req; stable(data); ack}` means req is true now, data is stable next cycle, ack fires the cycle after. Maps to `ltl.concat` with individual delay wrappers. This is the foundation of protocol verification.

### Sequence repetition `{a[*N]}` / `{a[*M to N]}`

`{stable(data)[*4]}` means data is stable for exactly 4 consecutive cycles. Maps to `ltl.repeat %stable_data, 4`. Useful for bus hold-time requirements.

### `|->` (overlapping) vs current `|=>` (non-overlapping)

Currently `|=>` is hardcoded as: antecedent fires, consequent must hold the *next* cycle. PSL's `|->` means the consequent starts at the *same* cycle as the antecedent match. In MLIR terms:

- `|->` is `ltl.implication` with no leading delay
- `|=>` is `ltl.implication` with `ltl.delay %consequent, 1` prepended

Right now the emitter conflates them by always creating a 1-cycle delay register. Exposing both as distinct operators gives the full SVA `|->` / `|=>` vocabulary.

---

## Layer 3 — Reset-Conditional Properties

One of the most common sources of false positives in formal verification: the tool reports a violation in the first cycle because the register hasn't been reset yet and starts in an unknown state. CIRCT has explicit infrastructure for this: `verif.has_been_reset`.

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

This is the difference between a property that fires spuriously on cycle 0 and one that only activates after the design reaches a known state. For any sequential design with registers this is almost always what you want.

---

## Layer 4 — `verif.contract` Blocks on Entities

This is the big architectural leap. Right now all `verif.assert` ops are emitted flat inside the module. If you have a hierarchy — a top-level module instantiating sub-modules — the solver has to reason about everything together, which blows up exponentially.

The idea: add a `contract` block to the micro-vhdl entity syntax:

```vhdl
entity Arbiter is
  port (req0, req1 : in std_logic; grant0, grant1 : out std_logic; clk : in std_logic);
  contract
    require: never(req0 = '1' and req1 = '1');  -- mutual exclusion on inputs
    ensure:  never(grant0 = '1' and grant1 = '1');  -- mutual exclusion on outputs
end entity;
```

This maps to `verif.contract` with `verif.require` and `verif.ensure`. When the tool compiles the sub-module it proves the contract. When the tool compiles the parent module and instantiates Arbiter, it uses the contract in apply-mode — treating the `ensure` as an axiom without re-proving it. This is compositional verification: sub-module proof complexity does not grow with the top-level design.

---

## Layer 5 — `verif.formal` Standalone Blocks

Currently micro-vhdl always ties verification to a specific `hw.module`. A `verif.formal` block is an independent formal function that can create symbolic values, instantiate modules, and assert properties without being part of the module itself.

This enables:

- **Cross-module properties**: instantiate two modules, constrain their interaction, prove a joint property
- **Parameterized formal checks**: check the same property for different input widths
- **LEC via `verif.lec`**: instantiate two different implementations, feed them the same symbolic inputs, assert their outputs are equal

---

## Layer 6 — Liveness (Requires a Different Backend)

`ltl.eventually` cannot be checked by BMC. The solver unrolls N cycles; if the property hasn't been violated in N steps it says "bound reached" — not "proved." For liveness you need either:

- **k-induction**: prove the property holds for 1 step assuming it held for the previous k steps. CIRCT's transform pipeline supports this.
- **IC3/PDR**: property directed reachability, which proves invariants without bounding the depth.

The micro-vhdl compiler could emit `ltl.eventually` properly if it invoked a different pipeline pass instead of `circt-bmc`. The grammar and AST change is trivial — it is the backend invocation that changes. Worth having a `--liveness` flag that switches from `circt-bmc` to a k-induction or IC3 pass.

---

## Priority Summary

| Extension | Effort | Value |
|---|---|---|
| `psl assume` | Very low | Critical — without it, most input-touching assertions are meaningless |
| `psl never` | Very low | Readability |
| `psl cover` | Very low | Coverage completeness |
| `next[N]` / delay | Low | Bounded response properties |
| `|->` (overlapping implication) | Low | Full SVA vocabulary |
| Reset-conditional (`after_reset`) | Medium | Eliminates spurious cycle-0 violations |
| Sequence concat/repeat | Medium | Protocol verification |
| `verif.contract` on entities | High | Compositional, scalable verification |
| `verif.formal` blocks | High | Cross-module and LEC |
| Liveness via k-induction | High | Full temporal logic |

The three that would make the biggest practical difference soonest: **`psl assume`**, **`next[N]`**, and **reset-conditional properties**. Together they let you write realistic, non-trivial properties about sequential designs without getting buried in false positives.
