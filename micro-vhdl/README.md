# Micro-VHDL Formal Verification Suite

Micro-VHDL is a "physics engine" subset of VHDL designed specifically for formal verification. It eliminates the non-deterministic and simulation-only constructs of standard VHDL, providing a strict, mathematically sound path from RTL to formal proof engines.

## Key Features

- **Multi-Engine Verification**: Natively supports Bounded Model Checking (Z3), Unbounded Bit-Level PDR (ABC), and Word-Level PDR (BTOR2/Boolector).
- **Temporal PSL Support**: Full support for Property Specification Language (PSL) including `always`, `never`, `next[N]`, sequence concatenation `{a; b}`, repetition `{a[*N]}`, and overlapping/non-overlapping implications (`|->`, `|=>`).
- **Scalable Hierarchy**: Automated multi-file linking and hierarchical flattening. Modules can define `verif.contract` (Assume-Guarantee) blocks for compositional verification.
- **Unbounded Liveness**: Proves `eventually!` properties using ABC's `l2s` (Live-to-Safe) engine.
- **Visual Debugging**: Automatically generates `.vcd` waveforms from formal counterexamples.
- **Proof Quality Metrics**: Integrated Formal Mutation Testing (`formal_mutation.py`) to measure assertion coverage.

## Project Architecture

1.  **Go Frontend**: Parses VHDL using Tree-sitter and extracts a high-level AST.
2.  **MLIR Lowering**: Transforms AST into CIRCT dialects (`hw`, `comb`, `seq`, `verif`, `ltl`).
3.  **Formal Orchestrator (`compile.sh`)**:
    - **Step 1**: Compiles VHDL project to hierarchical MLIR.
    - **Step 2a**: Runs **BMC** via `circt-bmc` (up to 15 cycles).
    - **Step 2b**: Runs **Bit-Level IC3** via `yosys-abc` (unbounded).
    - **Step 2e**: Runs **Word-Level IC3** via `btormc` (unbounded, SMT-based).
    - **Step 3**: Lowers to standard **SystemVerilog** via `firtool`.

## Usage

### Compilation and Verification
```bash
./compile.sh top.vhd submodule1.vhd submodule2.vhd
```
This will automatically link the files, detect the top module, run all verification engines, and output `top.sv` and `top_trace.vcd` (if a bug is found).

### Mutation Testing
```bash
python3 formal_mutation.py build.mlir ../cirt/build/bin
```
This will systematically sabotage your design's logic to verify that your PSL assertions are strong enough to catch the injected bugs.

## Technical Documentation

- [Verification Extensions](verif-extensions.md): Detailed breakdown of supported PSL and verification layers.
- [LTL and Verif Guide](verif-ltl-guide.md): Examples of CIRCT dialect usage and lowering paths.
- [IC3 and k-Induction](k-induction-ic3.md): Deep dive into the underlying formal algorithms.
- [Liveness Plan](liveness-plan.md): Implementation details for unbounded liveness proofs.
- [Next-Level Ideas](next-level-verification-ideas.md): Future roadmap for the toolchain.

## Toolchain Dependencies

- **Go**: Frontend compiler.
- **LLVM/CIRCT**: MLIR infrastructure and SystemVerilog emission.
- **Z3**: SMT solver for bounded checking.
- **Yosys-ABC**: Formal solver for bit-level safety and liveness.
- **Boolector / BtorMC**: Word-level SMT solver for datapath verification.
