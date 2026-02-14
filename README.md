# VHDL Compliance Compiler

A VHDL linter built for real RTL codebases. It combines error-tolerant parsing, rich semantic extraction, and a fast policy engine to catch hardware bugs, synthesis issues, and code quality problems before they reach simulation or synthesis.

Born as a learning project, now targeting production use on real FPGA and ASIC designs.

## Purpose

- Catch real bugs: CDC violations, unintended latches, combinational loops, missing resets, unreachable FSM states.
- Scale to large codebases: tested against ~18k VHDL files from open-source projects (GHDL, GRLIB, OSVVM, PoC).
- Stay out of the way: configurable severities, optional rules disabled by default, library-aware analysis that understands multi-library projects.
- Never lie: false positives are treated as tool bugs. Every rule has positive and negative test fixtures.

## Ethos

- **Panic is failure**: user input should not crash the tool. If we can’t handle something, we report that clearly.
- **Grammar is the source of truth**: fix parsing errors at the grammar, not in downstream workarounds.
- **No silent failures**: schema drift or missing fields must fail loudly (CUE validation).
- **Production code is correct**: false positives are tool bugs until proven otherwise.

## Technology Highlights

- **Tree-sitter grammar** for error‑tolerant VHDL parsing (`tree-sitter-vhdl/grammar.js`).
- **Go extractor + indexer** for semantic facts and cross‑file linking (`internal/extractor`, `internal/indexer`).
- **CUE schemas** to enforce contracts between stages (`internal/validator/schema.cue`).
- **Rust policy engine** for fast, declarative rule evaluation (`src/policy`).
- **Incremental + observable**: progress output, timing traces, and cache/daemon support.

## Architecture (Pipeline)

```
VHDL Files
  -> Tree-sitter Parser (grammar.js)
  -> Go Extractor (semantic facts)
  -> Go Indexer (cross-file resolution)
  -> CUE Validator (contract guard)
  -> Rust Policy Engine (rules)
  -> Violations / Reports
```

## Capabilities

- **170+ rules** across CDC, reset hygiene, FSM analysis, synthesis, latch detection, naming, code quality, and more
- Entities, architectures, packages, signals, ports, generics, type declarations
- Processes with sensitivity list analysis, clock/reset detection, read/write tracking
- Component instances with port/generic mappings and cross-entity validation
- Cross-file symbol resolution, dependency tracking, and library-aware analysis
- Configurable rule severities, optional rules, per-library third-party exclusions
- Incremental daemon mode with delta evaluation for fast re-checks
- JSON output, timing traces, and Chrome trace export for profiling

## Project Structure (Where Things Live)

- Grammar: `tree-sitter-vhdl/grammar.js`
- Extractor: `internal/extractor`
- Indexer: `internal/indexer`
- CUE schemas: `internal/validator/schema.cue` (input) + `internal/validator/output_schema.cue` (output)
- Policy rules: `src/policy`
- Test fixtures: `testdata/`
- VS Code extension client: `editors/vscode-vhdl-lsp`
- VS Code setup script: `tools/setup_vscode_vhdl_lsp.sh`

## How to Think About Changes

- Fix parsing issues in the grammar first.
- Only then update extraction or policy logic.
- Always add fixtures for rules (positive + negative).
- Treat new false positives as regression bugs.

See `AGENTS.md` for the detailed workflow, operational checklists, and improvement loops.
