# CIRCT Dialects (in This Repo) -- A Plain-English Guide

This repository vendors a copy of the CIRCT project under `cirt/`. CIRCT is
built on MLIR, which is a compiler infrastructure where you represent a program
as a tree/graph of operations and then progressively transform ("lower") it into
forms that are easier to analyze, optimize, or finally emit as code.

In MLIR, a **dialect** is basically a "mini language" (a coherent set of ops,
types, and attributes) for a specific abstraction level. CIRCT uses many
dialects because hardware design flows naturally move through many levels of
abstraction: from "software-like" code, to pipelines and loops, to modules and
wires, to "printable SystemVerilog".

If you're not a compiler person, a helpful mental model is:

- A dialect is a **kind of blueprint** for hardware or verification.
- Lowering is **redrawing the blueprint** in a more concrete style.
- Different dialects exist because different questions are easier to answer at
  different stages (e.g., "is this a valid FSM?" vs "what does the final Verilog
  look like?").

## What Dialects Are Actually "In" This Repo?

The source of truth is the dialect registration helper:

- `cirt/include/circt/InitAllDialects.h` (function `circt::registerAllDialects`)

That file lists the dialects CIRCT registers into an `mlir::DialectRegistry`.
Everything below is derived from that list, plus the corresponding
documentation and TableGen (ODS) files in `cirt/`.

Note: one registered dialect (`mlir::smt::SMTDialect`) is **upstream MLIR**, not
a CIRCT dialect, but it is used in CIRCT workflows.

## How To Read Dialect Names

Dialects show up as prefixes on operations.

- `hw.module` is an op in the `hw` dialect.
- `comb.add` is an op in the `comb` dialect.
- `verif.assert` is an op in the `verif` dialect.

In text form, MLIR looks a bit like assembly, but the idea is simple:

- Operations consume values and produce values (SSA style).
- Regions inside ops are like "blocks of sub-IR" (bodies).
- Symbol names (like `@Top`) name things like modules/components.

## Dialect Inventory (Cheat Sheet)

Below is a practical cheat sheet for the dialects registered in
`cirt/include/circt/InitAllDialects.h`.

### Core "RTL-ish" Substrate (structural + logic + emission)

These are the ones you'll see most often if you're "just doing hardware".

- `hw`: structural modules/instances + common hardware types/attributes.
  This is the *substrate* other hardware dialects "mix into".
  Docs: `cirt/docs/Dialects/HW/RationaleHW.md`
- `comb`: combinational logic ops designed to be analyzable and transformable.
  Docs: `cirt/docs/Dialects/Comb/RationaleComb.md`
- `seq`: sequential/stateful constructs (e.g. abstract registers).
  Docs: `cirt/docs/Dialects/Seq/RationaleSeq.md`
- `sv`: SystemVerilog constructs aimed at *emission* (AST-like statements,
  behavioral blocks, verbatim escape hatches).
  Docs: `cirt/docs/Dialects/SV/RationaleSV.md`

Think of it like:

- `hw` answers "what modules exist and how are they connected?"
- `comb` answers "what pure logic happens between registers?"
- `seq` answers "where is the state and how does it update?"
- `sv` answers "what SystemVerilog do we want to print?"

### Frontend / High-Level / Scheduling-Oriented Dialects

These dialects are typically used "above" the core substrate, and are lowered
into it.

- `firrtl`: FIRRTL IR (from Chisel flows) + annotation handling.
  Docs: `cirt/docs/Dialects/FIRRTL/_index.md`, `cirt/docs/Dialects/FIRRTL/RationaleFIRRTL.md`
- `chirrtl`: "CHIRRTL flavor" of FIRRTL produced from Chisel.
  Source: `cirt/include/circt/Dialect/FIRRTL/CHIRRTLDialect.h`
- `moore`: ingestion dialect for elaborated SystemVerilog (from Slang AST),
  intended as a frontend-capture dialect.
  Docs: `cirt/docs/Dialects/Moore.md`
- `calyx`: Calyx accelerator IR (components, cells, wires, control schedule).
  Source: `cirt/include/circt/Dialect/Calyx/Calyx.td`
  Note: this repo's `cirt/docs/Dialects/` tree does not include a Calyx dialect
  page, even though the dialect is present in the source.
- `handshake`: dataflow/handshake IR for independent processes communicating
  through FIFO-like channels.
  Docs: `cirt/docs/Dialects/Handshake/RationaleHandshake.md`
- `dc`: dynamic control/dataflow *control-only* dialect that separates data from
  control (in contrast to handshake, which assigns control semantics broadly).
  Docs: `cirt/docs/Dialects/DC/RationaleDC.md`
- `pipeline`: pipeline IR with multiple phases (unscheduled -> scheduled ->
  registers materialized).
  Docs: `cirt/docs/Dialects/Pipeline/RationalePipeline.md`
- `loopschedule`: scheduled loop IR that preserves loop structure post-schedule
  (pipelined/sequential loops).
  Docs: `cirt/docs/Dialects/LoopSchedule/LoopSchedule.md`
- `hwarith`: bitwidth-extending arithmetic with explicit width-inference rules;
  intended as a *short-lived frontend target* lowered early to `comb`.
  Docs: `cirt/docs/Dialects/HWArith/RationaleHWArith.md`
- `fsm`: explicit FSM representation (states/transitions/variables) with
  conversions to HW (SV) and to software-like dialects.
  Docs: `cirt/docs/Dialects/FSM/RationaleFSM.md`
- `ssp`: "static scheduling problems" -- a storage/exchange dialect for
  scheduling problem instances (testing/benchmarking/prototyping).
  Docs: `cirt/docs/Dialects/SSP/RationaleSSP.md`

### System Construction / Metadata / Interop Dialects

These dialects aren't "gates and registers" themselves; they help build systems
or preserve intent across transforms.

- `esi`: "Elastic Silicon Interconnect" -- typed channels and "services" to build
  accelerator systems and software-facing communication.
  Docs: `cirt/docs/Dialects/ESI/_index.md`
  Note: the ESI docs explicitly warn parts are out of date.
- `emit`: output structuring/formatting dialect used by emission (e.g.
  ExportVerilog) to control files and collateral.
  Docs: `cirt/docs/Dialects/Emit/RationaleEmit.md`
- `debug`: first-class debug information ops that track source language
  constructs through compilation and optimization.
  Docs: `cirt/docs/Dialects/Debug.md`
- `om`: "object model" dialect for domain modeling tied to hardware designs
  (power, clocks, software bringup, physical hierarchy, etc.).
  Docs: `cirt/docs/Dialects/OM/RationaleOM.md`
- `kanagawa`: dialect focused on "containerization/tunneling/portref lowering"
  for relative references through instance hierarchies.
  Docs: `cirt/docs/Dialects/Kanagawa/RationaleKanagawa.md`
- `interop`: represents partially lowered interoperability layers and provides
  utilities to auto-generate interop between different mechanisms/backends.
  Docs: `cirt/docs/Dialects/Interop/RationaleInterop.md`
- `msft`: "umbrella" dialect for Microsoft-focused constructs. In this repo it
  includes:
  - Physical design / placement constraints (`msft.pd.*`, `msft.instance.*`)
  - Tcl export utilities for FPGA flows
  - Some higher-level constructs (e.g. systolic array modeling)
  Source entrypoint: `cirt/include/circt/Dialect/MSFT/MSFT.td`
  Physical-design ops: `cirt/include/circt/Dialect/MSFT/MSFTPDOps.td`

### Simulation / Execution / Alternate Backends

These dialects tend to show up when you want to *run* something or target a
non-Verilog backend.

- `llhd`: event-queue/time-based simulation IR, modeling how signals change over
  time.
  Docs: `cirt/docs/Dialects/LLHD.md`
- `arc`: simulation-oriented IR used by `arcilator`; flattens hierarchy into
  "arcs" (state transfer functions) for fast simulation.
  Docs: `cirt/docs/Dialects/Arc.md`
- `sim`: simulator-facing helper ops (e.g., plusargs wrappers) that are easier
  to analyze/transform than raw SV constructs.
  Docs: `cirt/docs/Dialects/Sim.md`
- `systemc`: SystemC emission target dialect (and supporting C++ modeling),
  intended to be emitted via an ExportSystemC flow.
  Docs: `cirt/docs/Dialects/SystemC/RationaleSystemC.md`

### Verification / Formal Methods / Test Generation

These dialects are about specifying and checking properties, not just building
hardware.

- `verif`: assertions/assumptions/contracts/formal entrypoints for verification
  workflows.
  Docs: `cirt/docs/Dialects/Verif.md`
- `ltl`: linear temporal logic sequences/properties, designed to capture the
  core semantics behind SystemVerilog Assertions (SVA).
  Docs: `cirt/docs/Dialects/LTL.md`
- `mlir::smt` (upstream): SMT problem modeling (SMT-LIB-like) used to interact
  with solvers or represent formal backends.
  Docs: `cirt/docs/Dialects/SMT.md`
- `rtg`: random test generation constructs and passes (randomization as IR).
  Docs: `cirt/docs/Dialects/RTG.md`

### Synthesis / Logic Optimization Building Blocks

These are about turning "high level logic" into "synthesis-friendly logic" and
specialized representations.

- `datapath`: arithmetic-circuit construction primitives (partial products,
  compressor trees, carry-save formats), used to decompose and optimize
  arithmetic before lowering back to gate-level `comb`.
  Docs: `cirt/docs/Dialects/Datapath/RationaleDatapath.md`
- `synth`: logic-synthesis-specific boolean representations like AIG/MIG,
  intended to enable scalable synthesis analyses/mappings.
  Docs: `cirt/docs/Dialects/Synth/RationaleSynth.md`

## How These Dialects Fit Together (The "Big Picture")

There's a diagram in this repo that captures intended relationships:

- `cirt/docs/dialects.dot`

In plain English, it encodes paths like:

- "Chisel/FIRRTL flows":
  - Chisel emits `.fir`
  - FIRRTL parsing/import produces `firrtl` (and `chirrtl`)
  - Lowering produces a mix of core dialects (`hw`, `comb`, `seq`) plus `sv`
  - Verilog emission prints SystemVerilog
- "SystemVerilog frontend flows":
  - SV/VHDL input goes through an external SV parser/elaborator (Slang)
  - The elaborated design is captured in `moore`
  - Then lowered to core dialects
- "HLS / scheduling-ish flows":
  - `pipeline` and `loopschedule` represent scheduling decisions explicitly
  - `ssp` can store scheduling problem instances for debugging/benchmarking
- "System construction":
  - `esi` provides typed channels and services for wiring bigger systems
  - `emit`/`debug`/`om` preserve intent and organize outputs
- "Simulation backends":
  - Core dialects can be lowered into `arc` (for arcilator) or `llhd` (event queue)
  - `systemc` provides a path to emit C++/SystemC simulators
- "Formal verification":
  - `verif` and `ltl` express properties
  - `smt` provides solver-facing encodings/backends

The key idea: **most flows converge on the core substrate (`hw` + `comb` + `seq`)
and then branch out into backends (`sv` emission, `systemc`, simulation dialects,
formal dialects).**

## Concrete Mini-Example (Why Multiple Dialects Help)

Here's a tiny "adder with a register" in the core substrate:

```mlir
hw.module @AddReg(in %a: i8, in %b: i8, in %clk: !seq.clock, out y: i8) {
  %sum = comb.add %a, %b : i8
  %q = seq.compreg %sum, %clk : i8
  hw.output %q : i8
}
```

Why this separation matters:

- If you want to optimize the pure logic, you focus on `comb.*`.
- If you want to change register implementation details, you focus on `seq.*`.
- If you want to pretty-print Verilog, you lower into / interact with `sv.*`.
- If you want to prove something, you add `verif.assert` or an `ltl` property.

## Where To Read More (In-Repo)

Most dialect docs live under `cirt/docs/Dialects/`. A good starting set:

- Core substrate rationales: `cirt/docs/Dialects/HW/RationaleHW.md`,
  `cirt/docs/Dialects/Comb/RationaleComb.md`, `cirt/docs/Dialects/Seq/RationaleSeq.md`,
  `cirt/docs/Dialects/SV/RationaleSV.md`
- Verification: `cirt/docs/Dialects/Verif.md`, `cirt/docs/Dialects/LTL.md`,
  `cirt/docs/Dialects/SMT.md`
- System-level: `cirt/docs/Dialects/ESI/_index.md`, `cirt/docs/Dialects/Interop/RationaleInterop.md`,
  `cirt/docs/Dialects/OM/RationaleOM.md`

If you need to know what's *actually built*, check:

- Dialect registration: `cirt/include/circt/InitAllDialects.h`
- Dialect definitions (ODS/TableGen): `cirt/include/circt/Dialect/*/*.td`

## Notes For This Repository (Micro-VHDL Context)

The `micro-vhdl/` directory in this repo already contains guides that focus on
verification-oriented dialects:

- `micro-vhdl/verif-ltl-guide.md`
- `micro-vhdl/verif-extensions.md`

If you are working on Micro-VHDL's compilation/verification pipeline, you'll
most commonly interact with:

- `hw`, `comb`, `seq` for the hardware core
- `verif`, `ltl` for properties
- `mlir::smt` / solver-driven tooling for BMC/SMT-backed checking

