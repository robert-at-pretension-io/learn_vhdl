# VHDL Compliance Compiler

A learning-first VHDL linter that behaves like a compiler: resilient parsing, rich fact extraction, and explicit policy checks. The goal is to understand VHDL by building tooling that understands it.

## Purpose

- Learn VHDL by implementing a real parser, extractor, and rule engine.
- Learn compiler construction by building an end-to-end pipeline that never silently fails.
- Turn VHDL text into a queryable fact model so higher‑level checks become possible.

## Ethos

- **Panic is failure**: user input should not crash the tool. If we can’t handle something, we report that clearly.
- **Grammar is the source of truth**: fix parsing errors at the grammar, not in downstream workarounds.
- **No silent failures**: schema drift or missing fields must fail loudly (CUE validation).
- **Production code is correct**: false positives are tool bugs until proven otherwise.

## Technology Highlights

- **Tree-sitter grammar** for error‑tolerant VHDL parsing (`tree-sitter-vhdl/grammar.js`).
- **Go extractor + indexer** for semantic facts and cross‑file linking (`internal/extractor`, `internal/indexer`).
- **CUE schemas** to enforce contracts between stages (`schema/ir.cue`).
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

## Capabilities (Current)

- Entities, architectures, packages, signals, ports
- Processes with sensitivity, clock/reset detection, read/write analysis
- Component instances with port/generic mappings
- Cross‑file symbol resolution and dependencies
- Rule evaluation with configurable severities

## Project Structure (Where Things Live)

- Grammar: `tree-sitter-vhdl/grammar.js`
- Extractor: `internal/extractor`
- Indexer: `internal/indexer`
- CUE schemas: `schema/`
- Policy rules: `src/policy`
- Test fixtures: `testdata/`

## How to Think About Changes

- Fix parsing issues in the grammar first.
- Only then update extraction or policy logic.
- Always add fixtures for rules (positive + negative).
- Treat new false positives as regression bugs.

See `AGENTS.md` for the detailed workflow, operational checklists, and improvement loops.
