#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: tools/setup_vscode_vhdl_lsp.sh [options]

Builds vhdl-lsp/vhdl-lint, compiles the VS Code extension, installs it
into your local VS Code extensions directory, and configures workspace
settings for this repo.

Options:
  --skip-install        Build/compile/configure, but do not install extension
  --vscode-flavor <f>   One of: code, code-insiders, codium (default: auto-detect)
  --optional-severity <s>  Severity for all optional rules: warning|error|info|off (default: warning)
  --debounce-ms <n>     Set vhdlLsp.server.debounceMs in workspace settings (default: 100)
  -h, --help            Show this help message
USAGE
}

log() {
  printf '[setup-vscode] %s\n' "$*"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'Error: required command not found: %s\n' "$1" >&2
    exit 1
  fi
}

find_vscode_flavor() {
  if [[ -n "${VSCODE_FLAVOR_OVERRIDE}" ]]; then
    printf '%s' "$VSCODE_FLAVOR_OVERRIDE"
    return 0
  fi
  if command -v code >/dev/null 2>&1; then
    printf 'code'
    return 0
  fi
  if command -v code-insiders >/dev/null 2>&1; then
    printf 'code-insiders'
    return 0
  fi
  if command -v codium >/dev/null 2>&1; then
    printf 'codium'
    return 0
  fi
  printf 'code'
}

extensions_dir_for_flavor() {
  case "$1" in
    code)
      printf '%s' "${HOME}/.vscode/extensions"
      ;;
    code-insiders)
      printf '%s' "${HOME}/.vscode-insiders/extensions"
      ;;
    codium)
      printf '%s' "${HOME}/.vscode-oss/extensions"
      ;;
    *)
      printf 'Error: unsupported VS Code flavor: %s\n' "$1" >&2
      exit 1
      ;;
  esac
}

SKIP_INSTALL=0
VSCODE_FLAVOR_OVERRIDE=""
DEBOUNCE_MS=100
OPTIONAL_SEVERITY="warning"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-install)
      SKIP_INSTALL=1
      shift
      ;;
    --vscode-flavor)
      VSCODE_FLAVOR_OVERRIDE="${2:-}"
      if [[ -z "$VSCODE_FLAVOR_OVERRIDE" ]]; then
        printf 'Error: --vscode-flavor requires a value\n' >&2
        exit 1
      fi
      if [[ "$VSCODE_FLAVOR_OVERRIDE" != "code" && "$VSCODE_FLAVOR_OVERRIDE" != "code-insiders" && "$VSCODE_FLAVOR_OVERRIDE" != "codium" ]]; then
        printf 'Error: --vscode-flavor must be one of code, code-insiders, codium\n' >&2
        exit 1
      fi
      shift 2
      ;;
    --debounce-ms)
      DEBOUNCE_MS="${2:-}"
      if [[ -z "$DEBOUNCE_MS" || ! "$DEBOUNCE_MS" =~ ^[0-9]+$ || "$DEBOUNCE_MS" -lt 1 ]]; then
        printf 'Error: --debounce-ms must be a positive integer\n' >&2
        exit 1
      fi
      shift 2
      ;;
    --optional-severity)
      OPTIONAL_SEVERITY="${2:-}"
      if [[ "$OPTIONAL_SEVERITY" != "warning" && "$OPTIONAL_SEVERITY" != "error" && "$OPTIONAL_SEVERITY" != "info" && "$OPTIONAL_SEVERITY" != "off" ]]; then
        printf 'Error: --optional-severity must be one of warning, error, info, off\n' >&2
        exit 1
      fi
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'Error: unknown option: %s\n' "$1" >&2
      usage
      exit 1
      ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
EXT_DIR="$ROOT_DIR/editors/vscode-vhdl-lsp"
SETTINGS_PATH="$ROOT_DIR/.vscode/settings.json"
LSP_CONFIG_PATH="$ROOT_DIR/.vscode/vhdl_lsp_config.json"
EXT_VERSION="$(node -p "require('${EXT_DIR}/package.json').version")"
EXT_ID_DIR="local.vhdl-lsp-client-${EXT_VERSION}"

require_cmd go
require_cmd node
require_cmd npm

if [[ ! -d "$EXT_DIR" ]]; then
  printf 'Error: extension directory not found: %s\n' "$EXT_DIR" >&2
  exit 1
fi

log "Building vhdl-lsp binary"
go build -o "$ROOT_DIR/vhdl-lsp" ./cmd/vhdl-lsp

log "Building vhdl-lint binary"
go build -o "$ROOT_DIR/vhdl-lint" ./cmd/vhdl-lint

log "Enabling all optional lint rules (${OPTIONAL_SEVERITY}) in vhdl_lint.json"
go run ./tools/enable_optional_rules.go --config "$ROOT_DIR/vhdl_lint.json" --severity "$OPTIONAL_SEVERITY"

log "Writing workspace-focused LSP lint config: $LSP_CONFIG_PATH"
mkdir -p "$(dirname "$LSP_CONFIG_PATH")"
cat > "$LSP_CONFIG_PATH" <<'JSON'
{
  "standard": "2008",
  "libraries": {
    "work": {
      "files": [
        "*.vhd",
        "*.vhdl",
        "src/**/*.vhd",
        "src/**/*.vhdl",
        "rtl/**/*.vhd",
        "rtl/**/*.vhdl",
        "hdl/**/*.vhd",
        "hdl/**/*.vhdl",
        "tb/**/*.vhd",
        "tb/**/*.vhdl",
        "sim/**/*.vhd",
        "sim/**/*.vhdl",
        "test/**/*.vhd",
        "test/**/*.vhdl",
        "tests/**/*.vhd",
        "tests/**/*.vhdl",
        "testdata/**/*.vhd",
        "testdata/**/*.vhdl"
      ]
    }
  },
  "lint": {
    "rules": {},
    "ignorePatterns": [],
    "ignoreRegions": true
  },
  "analysis": {
    "maxParallelFiles": 0,
    "followLibraryUse": true,
    "resolveDefaultBinding": true,
    "requireLibraryMapping": true,
    "cache": {
      "enabled": true,
      "dir": ".vhdl_lint_cache"
    }
  }
}
JSON

log "Enabling all optional lint rules (${OPTIONAL_SEVERITY}) in workspace LSP config"
go run ./tools/enable_optional_rules.go --config "$LSP_CONFIG_PATH" --severity "$OPTIONAL_SEVERITY"

pushd "$EXT_DIR" >/dev/null
if [[ -f package-lock.json ]]; then
  log "Installing extension dependencies (npm ci)"
  npm ci
else
  log "Installing extension dependencies (npm install)"
  npm install
fi

log "Compiling extension"
npm run compile
popd >/dev/null

if [[ "$SKIP_INSTALL" -eq 0 ]]; then
  VSCODE_FLAVOR="$(find_vscode_flavor)"
  EXTENSIONS_DIR="$(extensions_dir_for_flavor "$VSCODE_FLAVOR")"
  DEST_DIR="${EXTENSIONS_DIR}/${EXT_ID_DIR}"
  log "Installing extension into ${DEST_DIR}"
  mkdir -p "$EXTENSIONS_DIR"
  rm -rf "$DEST_DIR"
  cp -a "$EXT_DIR" "$DEST_DIR"
  rm -rf "${DEST_DIR}/src" "${DEST_DIR}/.gitignore"
else
  log "Skipping extension install (--skip-install)"
fi

log "Writing workspace settings: $SETTINGS_PATH"
mkdir -p "$(dirname "$SETTINGS_PATH")"

ROOT_DIR="$ROOT_DIR" SETTINGS_PATH="$SETTINGS_PATH" DEBOUNCE_MS="$DEBOUNCE_MS" node <<'NODE'
const fs = require('fs');
const path = require('path');

const root = process.env.ROOT_DIR;
const settingsPath = process.env.SETTINGS_PATH;
const debounceMs = Number(process.env.DEBOUNCE_MS || '100');

let settings = {};
if (fs.existsSync(settingsPath)) {
  const raw = fs.readFileSync(settingsPath, 'utf8').trim();
  if (raw) {
    settings = JSON.parse(raw);
  }
}

if (!settings['files.associations'] || typeof settings['files.associations'] !== 'object') {
  settings['files.associations'] = {};
}
settings['files.associations']['*.vhd'] = 'vhdl';
settings['files.associations']['*.vhdl'] = 'vhdl';

settings['vhdlLsp.server.path'] = path.join(root, 'vhdl-lsp');
settings['vhdlLsp.lint.path'] = path.join(root, 'vhdl-lint');
settings['vhdlLsp.lint.configPath'] = path.join(root, '.vscode', 'vhdl_lsp_config.json');
settings['vhdlLsp.server.debounceMs'] = debounceMs;
settings['vhdlLsp.trace.server'] = 'messages';
settings['vhdlLsp.ui.statusBar.enabled'] = true;
settings['vhdlLsp.ui.showStartupNotification'] = true;
settings['vhdlLsp.ui.autoRevealProblems'] = true;
settings['vhdlLsp.ui.inlineDiagnostics.enabled'] = true;
settings['vhdlLsp.ui.inlineDiagnostics.maxPerFile'] = 120;
settings['vhdlLsp.ui.inlineDiagnostics.maxMessagesPerLine'] = 2;
settings['vhdlLsp.ui.inlineDiagnostics.includeSeverities'] = ['error', 'warning', 'information'];
settings['vhdlLsp.ui.richDiagnostics.enabled'] = true;
settings['vhdlLsp.ui.richDiagnostics.maxInHover'] = 8;
settings['editor.codeLens'] = true;
settings['editor.renderValidationDecorations'] = 'on';
settings['problems.showCurrentInStatus'] = true;
settings['problems.autoReveal'] = true;

const text = JSON.stringify(settings, null, 2) + '\n';
fs.writeFileSync(settingsPath, text, 'utf8');
NODE

log "Setup complete"
if [[ "$SKIP_INSTALL" -eq 0 ]]; then
  log "Installed extension id dir: $EXT_ID_DIR"
fi
