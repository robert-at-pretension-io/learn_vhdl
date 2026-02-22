# Micro-VHDL Verification Extensions

## Current State (as of February 2026)

### What is implemented

**Compile pipeline** (`compile.sh`):
```
[1/3]  VHDL → build.mlir + build_ic3.mlir   (Go compiler, two MLIR variants)
[2a/3] BMC via Z3 (circt-bmc, bound=15)      — finds shallow bugs, concrete traces
                                               strips verif.contract first (--strip-contracts);
                                               skipped for contract-only designs
[2b/3] IC3/PDR via ABC pdr (unbounded)        — proves safety or finds deep bugs
[2c/3] Contract verification via circt-bmc    — present only when contract block exists;
                                               pipeline: --lower-contracts → extract verif.formal
                                               → circt-bmc (LowerTestsPass handles the rest)
[3/3]  MLIR → SystemVerilog (firtool)
```

**PSL operators supported**:
- `|=>` (next-cycle implication): delay register + boolean implication → `verif.assert : i1`
- `stable(x)`: delay register + `comb.icmp eq` → `verif.assert : i1`
- `assume always`: emits `verif.assume` in BMC MLIR; IC3 step is skipped (see below)
- `assert always after_reset(rst)`: sticky `rst_ever_high` register + guarded assertion; IC3 step skipped (see below)
- `next[N]` and `next_a[M to N]` on the RHS of `|=>`: emitted as `ltl.delay` + `ltl.implication` + `ltl.clock` and lowered for BMC via a dedicated `lower-ltl-to-bmc` pass
- `eventually!`: stubbed as `hw.constant true` + TODO comment (liveness, skipped in BMC)

**MLIR emission modes**:
- **BMC mode** (default): emits `verif.assert`, `verif.assume`, `seq.initial` for zero-initialized registers
- **IC3 mode** (`_ic3.mlir`): emits `__verif_bad: i1` hw.output (negated assertion conjunction), no `seq.initial` (PDR explores from unconstrained initial states); `psl assume` is omitted because `circt-translate --export-aiger` does not support the verif dialect

**Toolchain**:
- `circt-bmc` + Z3: BMC, up to 15 cycles (configurable via `BOUND=N`); respects `verif.assume`
- `circt-synth` → `circt-translate --export-aiger` → `yosys-abc pdr`: unbounded IC3/PDR; skipped when `psl assume` is present
- `firtool`: SystemVerilog output

**Known gotchas**:
- `yosys-abc read_aiger` only accepts binary `.aig` format; ASCII `.aag` silently fails
- `circt-synth` cannot handle `seq.initial` — IC3 MLIR omits it by design
- IC3 step is skipped automatically when no PSL assertions are present
- IC3 step is skipped when `psl assume` is present — `verif.assume` cannot pass through `circt-translate --export-aiger` (verif dialect not supported in AIGER lowering); BMC handles the constrained verification correctly
- IC3 step is skipped when `after_reset` is present — unconstrained initial states in IC3 would trivially violate reset-conditional properties; BMC correctly starts from zero-initialized registers and observes the reset sequence
- `circt-bmc` cannot process `verif.contract` directly — its `ConvertHWToSMT` pass converts all `hw.module` ops to `func.func`; the apply-mode `hw.module` emitted by `--lower-contracts` contains `verif.symbolic_value`, which is no longer inside a valid parent after conversion and fails dialect validation. The fix: strip contracts before BMC (`--strip-contracts`) and verify contracts separately via the `verif.formal` extraction pipeline (see step [2c/3])
- `--lower-contracts` emits two things: a `verif.formal @Mod_CheckContract_N` (the actual check) and an apply-mode `hw.module @Mod` (for use-site callers with symbolic outputs). Passing the full lowered file to `circt-bmc` fails because `ConvertHWToSMT` processes all modules. Only the `verif.formal` blocks should be passed to `circt-bmc` — it handles them via its internal `LowerTestsPass`
- The "operand does not dominate this use" error from `circt-bmc` on `verif.contract` MLIR is a toolchain pipeline issue, not an MLIR validity problem. `verif.contract` has `RegionKindInterface` (graph region) and no `IsolatedFromAbove` trait; results are legitimately referenceable inside the region. `circt-opt --lower-contracts` processes this correctly. `circt-bmc` fails only because it lacks `LowerContractsPass` in its pipeline

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

### `next[N]` / `next_a[M to N]` ✓ DONE (LTL lowering for BMC)

`next[3](ack)` means "ack holds exactly 3 cycles from now." `next_a[1 to 8](ack)` means "ack must hold at least once between 1 and 8 cycles from now." These are emitted as `ltl.delay` plus `ltl.implication` and then lowered into core logic in the `circt-bmc` pipeline. IC3 still skips LTL properties.

### Sequence concatenation `{a; b; c}`

PSL sequences: `{req; stable(data); ack}` means req is true now, data stable next cycle, ack fires the cycle after. Maps to `ltl.concat`. Foundation of protocol verification.

### Sequence repetition `{a[*N]}` / `{a[*M to N]}`

`{stable(data)[*4]}` means data stable for 4 consecutive cycles. Maps to `ltl.repeat`. Useful for bus hold-time requirements.

### `|->` (overlapping) vs current `|=>` (non-overlapping)

Currently `|=>` always creates a 1-cycle delay register. PSL's `|->` means the consequent starts at the same cycle as the antecedent match — no delay. Exposing both gives the full SVA vocabulary.

---

## Layer 3 — Reset-Conditional Properties ✓ DONE

One of the most common sources of false positives: the tool reports a violation in cycle 0 because the register starts in an unconstrained state (especially true for IC3, which explores from all initial states).

**Implemented** using a sticky `rst_ever_high` register encoding (not `verif.has_been_reset` — the CIRCT verif dialect form had toolchain compatibility issues):

```vhdl
psl assert always after_reset(rst) s_sig;
```

Emits:
```mlir
%next_rst_ever_high = comb.or %rst_ever_high, %rst : i1
%rst_ever_high = seq.compreg %next_rst_ever_high, %clk initial 0 : i1
-- effective = NOT(rst_ever_high) OR rst OR property
--   vacuous before any reset, vacuous during active reset, real check after
%not_rst_ever_high = comb.xor %rst_ever_high, -1 : i1
%partial = comb.or %not_rst_ever_high, %rst : i1
%effective = comb.or %partial, %s_sig : i1
verif.assert %effective : i1
```

IC3 step is skipped when `after_reset` is present (unconstrained initial states would trivially violate the property). BMC correctly starts from zero-initialized registers and observes the reset sequence.

See `examples/vhd/test_after_reset.vhd` for a demonstration.

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
| `psl assume` | **Done** | — | BMC-constrained verification; IC3 skips when assumes present |
| `psl never` | **Done** | — | Syntactic sugar for always (not P) |
| `psl cover` | **Done** | — | Checked via separate circt-bmc coverage pass; emitted to SV |
| Reset-conditional (`after_reset`) | **Done** | — | Eliminates cycle-0 false positives in IC3 |
| `next[N]` / delay | **Done** | — | Bounded response properties |
| `|->` (overlapping implication) | Not started | Low | Full SVA vocabulary |
| Sequence concat/repeat | Not started | Medium | Protocol verification |
| `verif.contract` on entities | Not started | High | Compositional, scalable verification |
| `verif.formal` blocks | Not started | High | Cross-module and LEC |
| Liveness via ABC fairness | Not started | High | Full temporal logic |

**Highest remaining value**: Sequence concat/repeat and overlapping implication `|->` — for full SVA vocabulary and protocol verification.
