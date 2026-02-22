# CIRCT `verif` and `ltl` Dialects — Use-Cases and Examples

## `verif` dialect

### 1. Inline assertions (`verif.assert` / `verif.assume` / `verif.cover`)

The simplest use. Placed directly inside an `hw.module` and evaluated during formal runs.

```mlir
hw.module @Adder(in %a: i8, in %b: i8, out z: i8) {
  %0 = comb.add %a, %b : i8
  %sum_geq_a = comb.icmp uge %0, %a : i8
  verif.assert %sum_geq_a : i1         // property to prove
  verif.cover  %sum_geq_a : i1         // track reachable states where this holds
  hw.output %0 : i8
}
```

`verif.assume` constrains the solver's input space — only consider inputs satisfying the assumption:

```mlir
hw.module @BoundedShift(in %a: i8, in %b: i8, out z: i8) {
  %c8 = hw.constant 8 : i8
  %b_lt_8 = comb.icmp ult %b, %c8 : i8
  verif.assume %b_lt_8 : i1            // tell Z3: only consider b < 8
  %0 = comb.shl %a, %b : i8
  %ok = comb.icmp uge %0, %a : i8
  verif.assert %ok : i1
  hw.output %0 : i8
}
```

**Rule of thumb**: `assume` = precondition handed to the solver; `assert` = postcondition the solver must prove cannot be violated.

---

### 2. `verif.formal` — Standalone Formal Function

An autonomous verification entry point. Constructs inputs symbolically and asserts properties without tying to a specific `hw.module`.

```mlir
verif.formal @CheckMultiplier {} {
  %a = verif.symbolic_value : i32    // unconstrained i32 input
  %b = verif.symbolic_value : i32

  %c9    = hw.constant 9 : i32
  %lhs   = comb.mul %a, %c9 : i32   // naive 9*a

  %c3      = hw.constant 3 : i32
  %shifted = comb.shl %a, %c3 : i32 // 8*a
  %rhs     = comb.add %a, %shifted : i32 // a + 8*a = 9*a

  %eq = comb.icmp eq %lhs, %rhs : i32
  verif.assert %eq : i1              // prove for ALL a: 9*a == a + 8*a
}
```

This is the cleanest pattern: no hardware module needed — just symbolic variables + logic + assertions.

---

### 3. `verif.contract` — Hierarchical Verification

Contracts attach proofs to module outputs inline. Two modes:

- **Check mode**: `require` → assume (constrain inputs), `ensure` → assert (prove outputs)
- **Apply mode**: `require` → assert (caller must satisfy), `ensure` → assume (treat as axiom, skip re-proving)

This is what makes compositional formal verification possible: prove a sub-module once, then use it as a trusted black box everywhere else.

**Simple arithmetic contract:**

```mlir
hw.module @Mul9(in %a: i42, out z: i42) {
  %c3     = hw.constant 3 : i42
  %c9     = hw.constant 9 : i42
  %shifted = comb.shl %a, %c3 : i42    // 8*a
  %result  = comb.add %a, %shifted : i42 // 9*a via shift+add
  %z = verif.contract %result : i42 {
    %ref = comb.mul %a, %c9 : i42      // reference: 9*a via multiply
    %eq  = comb.icmp eq %z, %ref : i42
    verif.ensure %eq : i1              // prove: shift+add == multiply
  }
  hw.output %z : i42
}
```

**Contract with precondition:**

```mlir
hw.module @ShiftLeft(in %a: i8, in %b: i8, out z: i8) {
  // ...mux tree implementing shift...
  %z = verif.contract %mux_result : i8 {
    %c8    = hw.constant 8 : i8
    %b_lt_8 = comb.icmp ult %b, %c8 : i8
    verif.require %b_lt_8 : i1         // precondition: b < 8

    %ref = comb.shl %a, %b : i8
    %eq  = comb.icmp eq %z, %ref : i8
    verif.ensure %eq : i1              // postcondition: mux == native shift
  }
  hw.output %z : i8
}
```

**Contract with LTL temporal postcondition — sequential module:**

```mlir
hw.module @Counter(in %in: i2, out out: i2, in %clock: !seq.clock, in %reset: i1) {
  %zero = hw.constant 0 : i2
  %one  = hw.constant 1 : i2
  %max  = hw.constant -2 : i2         // 0b10 = 2 in i2

  %reg  = seq.firreg %next clock %clock reset sync %reset, %zero : i2
  %eq   = comb.icmp eq %reg, %max : i2
  %incr = comb.add %reg, %one : i2
  %next = comb.mux %eq, %zero, %incr : i2

  %out = verif.contract %reg : i2 {
    // After reset has occurred, the counter never reaches 0b11
    %never_ff  = hw.constant -1 : i2
    %ne        = comb.icmp ne %out, %never_ff : i2
    %clk_i1    = seq.from_clock %clock
    %has_reset = verif.has_been_reset %clk_i1, sync %reset
    %seq       = ltl.clock %ne, posedge %clk_i1 : i1
    verif.ensure %seq if %has_reset : !ltl.sequence  // only enforced after reset
  }
  hw.output %out : i2
}
```

`verif.has_been_reset` returns `i1` that becomes true once a reset pulse has been observed. This prevents the solver from requiring invariants to hold in the arbitrary pre-reset state.

---

### 4. `verif.lec` — Logic Equivalence Checking

Prove that two circuit implementations are input-output identical. Classic use: verify that an optimized implementation matches a reference.

```mlir
func.func @CheckEquivalence() {
  verif.lec
  first {
    ^bb0(%a: i32, %b: i32):
      %r = comb.mul %a, %b : i32      // naive multiply
      verif.yield %r : i32
  }
  second {
    ^bb0(%a: i32, %b: i32):
      %c3      = hw.constant 3 : i32
      %shifted = comb.shl %b, %c3 : i32  // 8*b (shift-and-add approximation)
      %r       = comb.add %b, %shifted : i32
      verif.yield %r : i32
  }
}
```

The tool creates symbolic inputs, evaluates both bodies, then asserts `first_output == second_output`. UNSAT → equivalent. SAT → counterexample input returned.

`verif.refines` is the asymmetric variant: proves that every output of `first` is also producible by `second` (refinement relation), without requiring full equivalence.

---

## `ltl` dialect: Temporal Properties

LTL encodes time-indexed properties across clock cycles. Three types:

- `i1` — a Boolean signal at a single instant
- `!ltl.sequence` — a Boolean pattern across a range of cycles
- `!ltl.property` — a temporal claim asserted globally

### Building Blocks

#### `ltl.delay` — exact or bounded cycle gap

```mlir
%s1 = ltl.delay %a, 2     : i1   // next[2](a)  (exactly 2 cycles later)
%s2 = ltl.delay %a, 1, 4  : i1   // next_a[1 to 4](a)  (1 to 4 cycles later)
```

**Note for Micro‑VHDL**: `circt-bmc` in this repo now registers the `ltl` dialect and lowers `ltl.delay`/`ltl.implication`/`ltl.clock` into core logic via a dedicated `lower-ltl-to-bmc` pass. LTL properties are still **BMC-only**; IC3/PDR skips them in the AIGER path. Standalone temporal sequences like `next[N](a)` are automatically wrapped in an `ltl.clock` block during MLIR emission so they evaluate correctly.

#### `ltl.concat` — consecutive sequences

```mlir
// req high for 1 cycle, immediately followed by ack
%req_then_ack = ltl.concat %req, %ack : !ltl.sequence, !ltl.sequence

// req, then exactly 2 cycles gap, then ack
%handshake = ltl.concat %req, %gap2, %ack
              : !ltl.sequence, !ltl.sequence, !ltl.sequence
```
*(In Micro-VHDL, you can write this directly as `{req; ack}`)*

#### `ltl.repeat` — repetition

```mlir
%stable3 = ltl.repeat %bus_stable, 3    : !ltl.sequence  // [*3]
%burst   = ltl.repeat %valid, 0, 5     : !ltl.sequence  // [*0:5]
```
*(In Micro-VHDL, you can write this as `{bus_stable[*3]}` or `{valid[*0 to 5]}`)*

---

### Temporal Properties

#### `ltl.implication` — trigger |-> and |=> response

"Whenever the antecedent sequence matches, the consequent property must hold starting at the match endpoint."

```mlir
// req |-> next_a[1 to 3](ack)  (Overlapping: response starts same cycle)
%ack_within_3 = ltl.delay %ack, 1, 3 : i1
%prop = ltl.implication %req, %ack_within_3 : i1, !ltl.sequence
%clocked = ltl.clock %prop, posedge %clk_i1 : !ltl.property
verif.assert %clocked : !ltl.property
```

*(In Micro-VHDL, `|=>` is non-overlapping (starts next cycle) and `|->` is overlapping (starts same cycle). Both are now fully supported for boolean and sequence operands.)*

#### `ltl.eventually` — liveness

"At some future point, X must hold." Cannot be falsified in finite time — requires IC3 with fairness, not BMC.

```mlir
%drain = ltl.eventually %not_full : i1
%clocked = ltl.clock %drain, posedge %clk_i1 : !ltl.property
verif.assert %clocked : !ltl.property
// circt-bmc cannot verify this; ABC pdr requires Buchi/fairness encoding
```

#### `ltl.until` — conditional liveness

"X holds until Y becomes true, and Y must eventually become true."

```mlir
%wait = ltl.until %in_wait_state, %grant_arrived : !ltl.property, !ltl.property
%clocked = ltl.clock %wait, posedge %clk_i1 : !ltl.property
verif.assert %clocked : !ltl.property
```

---

### Complete Handshake Protocol Example

```mlir
hw.module @HandshakeChecker(
    in %clk: !seq.clock, in %req: i1, in %ack: i1, in %data_valid: i1) {

  %clk_i1 = seq.from_clock %clk

  // Property 1: no spurious ack — ack only after req
  %no_spurious_ack = ltl.implication %ack, %req : i1, i1
  %p1 = ltl.clock %no_spurious_ack, posedge %clk_i1 : i1

  // Property 2: req must be acknowledged within 8 cycles
  %ack_within_8   = ltl.delay %ack, 1, 8 : i1
  %req_gets_ack   = ltl.implication %req, %ack_within_8 : i1, !ltl.sequence
  %p2 = ltl.clock %req_gets_ack, posedge %clk_i1 : !ltl.property

  // Property 3: data_valid stable for 2 cycles after ack
  %stable2    = ltl.repeat %data_valid, 2 : i1
  %data_ok    = ltl.implication %ack, %stable2 : i1, !ltl.sequence
  %p3 = ltl.clock %data_ok, posedge %clk_i1 : !ltl.property

  verif.clocked_assert %p1, posedge %clk_i1 : !ltl.property
  verif.assert %p2 : !ltl.property
  verif.assert %p3 : !ltl.property
}
```

---

## The Lowering Hierarchy

### BMC path (circt-bmc + Z3)

```
LTL property
    |
    v  (LTLToCore: converts i1-operand implications -> comb logic)
verif.assert %i1_prop : i1
    |
    v  (VerifToSMT)
smt.assert (smt.not (%p))    <- negation: look for counterexample
    |
    v  (SMTToZ3LLVM)
LLVM IR calling Z3 C API
    |
    v  (ExecutionEngine JIT)
Z3 says UNSAT -> "Bound reached with no violations!"
Z3 says SAT   -> "Assertion can be violated!" + counterexample trace
```

**Key insight**: VerifToSMT *negates* the assertion. It asks "can the property be false?" UNSAT means no violation in N cycles. SAT gives exact signal values at each unrolled cycle.

### IC3/PDR path (circt-synth + AIGER + ABC pdr)

```
__verif_bad = NOT(combined assertion) as hw.output
    |
    v  (circt-synth --until-before=all)
AIG dialect (synth.aig.and_inv, seq.compreg as latches)
    |
    v  (circt-translate --export-aiger)
binary AIGER (.aig) — M inputs, L latches, O outputs (bad states)
    |
    v  (yosys-abc pdr)
IC3 frame sequence construction
    |
    v
"Property proved."   <- fixed-point invariant found (all cycles)
or
"CEX found in frame N"  <- concrete counterexample
```

**Key insight**: AIGER treats `hw.output` as bad-state signals. ABC `pdr` proves those outputs are unreachable from any initial state across all cycles.

---

## What Each Solver Proves

| Property type | circt-bmc (BMC) | ABC pdr (IC3) |
|---|---|---|
| Safety — finds bugs | Yes (within bound) | Yes (any depth) |
| Safety — proves correct | No | Yes (unbounded) |
| Bitvector arithmetic | Native (Z3 BV theory) | Bit-exploded (slower for wide types) |
| Counterexample detail | Named MLIR signals | Generic input_N/output_N |
| `verif.assume` support | Native | Needs manual encoding |
| `ltl.eventually` (liveness) | No | Needs Buchi encoding (not yet wired) |
| LEC | Yes via `verif.lec` | Via AIGER equivalence checking |

The two solvers are complementary. BMC runs first (fast, finds shallow bugs, readable traces). IC3 runs second (proves the property holds for all cycles, or finds deep bugs BMC missed).
