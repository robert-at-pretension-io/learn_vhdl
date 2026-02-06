#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FIXTURE_DIR="$ROOT_DIR/testdata/cross_file_semantic"

POLICY_BIN="$ROOT_DIR/target/release/vhdl_policy"
if [[ -x "$POLICY_BIN" ]]; then
  export VHDL_POLICY_BIN="$POLICY_BIN"
elif [[ -x "$ROOT_DIR/target/debug/vhdl_policy" ]]; then
  export VHDL_POLICY_BIN="$ROOT_DIR/target/debug/vhdl_policy"
else
  (cd "$ROOT_DIR" && cargo build --bin vhdl_policy)
  if [[ -x "$POLICY_BIN" ]]; then
    export VHDL_POLICY_BIN="$POLICY_BIN"
  fi
fi

LOG_DIR="${VHDL_CROSS_FILE_LOG_DIR:-$ROOT_DIR/tmp/cross_file_logs}"
mkdir -p "$LOG_DIR"

VERBOSE_FLAG=""
if [[ "${VHDL_CROSS_FILE_VERBOSE:-1}" != "0" ]]; then
  VERBOSE_FLAG="-v"
fi

TIMEOUT_CMD=()
if [[ -n "${VHDL_CROSS_FILE_TIMEOUT:-}" ]]; then
  if command -v timeout >/dev/null 2>&1; then
    TIMEOUT_CMD=(timeout "$VHDL_CROSS_FILE_TIMEOUT")
  fi
fi

FILTERS=()
if [[ $# -gt 0 ]]; then
  for arg in "$@"; do
    FILTERS+=("$arg")
  done
else
  while IFS= read -r -d '' dir; do
    FILTERS+=("$(basename "$dir")")
  done < <(find "$FIXTURE_DIR" -mindepth 1 -maxdepth 1 -type d -print0 | sort -z)
fi

for filter in "${FILTERS[@]}"; do
  start_ts=$(date +%s)
  log_file="$LOG_DIR/${filter}.log"
  echo "==> cross-file fixtures: $filter"
  echo "    log: $log_file"
  VHDL_CROSS_FILE_FILTER="$filter" VHDL_CROSS_FILE_TIMING=1 \
    "${TIMEOUT_CMD[@]}" go test $VERBOSE_FLAG ./internal/policy -run TestCrossFileSemanticFixtures -count=1 2>&1 | tee "$log_file"
  status=${PIPESTATUS[0]}
  end_ts=$(date +%s)
  echo "    duration: $((end_ts - start_ts))s"
  if [[ $status -ne 0 ]]; then
    echo "    status: FAILED"
    echo "    tail:"
    tail -n 80 "$log_file"
    exit $status
  fi
  echo "    status: OK"
  echo
done
