# VS Code Extension: VHDL LSP (vhdl-lint)

This extension is a VS Code client for the `vhdl-lsp` server in this repository.

## What it provides

- Starts `vhdl-lsp` over stdio for `*.vhd` and `*.vhdl` files.
- Uses diagnostics, completion, hover, rename, symbols, semantic tokens, and code actions from the server.
- Exposes configuration for server path/args/env and lint binary path.

## Requirements

- VS Code 1.84+
- `vhdl-lsp` binary built and reachable
- `vhdl-lint` binary built and reachable (or set explicitly via setting)

## Build binaries

From repo root:

```bash
go build -o vhdl-lsp ./cmd/vhdl-lsp
go build -o vhdl-lint ./cmd/vhdl-lint
```

## Extension development setup

```bash
cd editors/vscode-vhdl-lsp
npm install
npm run compile
```

Then open `editors/vscode-vhdl-lsp` in VS Code and press `F5` to launch an Extension Development Host.

## Settings

- `vhdlLsp.server.path`: Path to `vhdl-lsp` (default: `vhdl-lsp`)
- `vhdlLsp.server.args`: Extra arguments for `vhdl-lsp`
- `vhdlLsp.server.env`: Extra env vars for `vhdl-lsp`
- `vhdlLsp.lint.path`: Optional path to `vhdl-lint` (sets `VHDL_LINT_BIN`)
- `vhdlLsp.lint.configPath`: Optional path to lint config (sets `VHDL_LINT_CONFIG`)
- `vhdlLsp.server.debounceMs`: Debounce delay (sets `VHDL_LSP_DEBOUNCE_MS`)
- `vhdlLsp.trace.server`: LSP trace level (`off`, `messages`, `verbose`)
- `vhdlLsp.ui.statusBar.enabled`: Show server state + diagnostics counts in status bar
- `vhdlLsp.ui.showStartupNotification`: Show notification when server starts
- `vhdlLsp.ui.autoRevealProblems`: Open Problems panel when diagnostics appear
- `vhdlLsp.ui.inlineDiagnostics.enabled`: Show inline line-end diagnostic text
- `vhdlLsp.ui.inlineDiagnostics.maxPerFile`: Cap inline diagnostics rendered per file
- `vhdlLsp.ui.inlineDiagnostics.maxMessagesPerLine`: Cap inline message preview count per line
- `vhdlLsp.ui.inlineDiagnostics.includeSeverities`: Choose severities shown inline
- `vhdlLsp.ui.richDiagnostics.enabled`: Show plain-language rule explanations in hover overlays
- `vhdlLsp.ui.richDiagnostics.maxInHover`: Cap diagnostics explained in one hover

Example workspace settings:

```json
{
  "vhdlLsp.server.path": "/home/elliot/Projects/vhdl_compiler/vhdl-lsp",
  "vhdlLsp.lint.path": "/home/elliot/Projects/vhdl_compiler/vhdl-lint",
  "vhdlLsp.lint.configPath": "/home/elliot/Projects/vhdl_compiler/.vscode/vhdl_lsp_config.json",
  "vhdlLsp.server.debounceMs": 100,
  "vhdlLsp.trace.server": "messages"
}
```

## Commands

- `VHDL LSP: Restart Server`
- `VHDL LSP: Show Output`

## Packaging (optional)

Install `@vscode/vsce` and package a `.vsix`:

```bash
npm install -D @vscode/vsce
npx vsce package
```
