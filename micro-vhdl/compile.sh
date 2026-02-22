#!/bin/bash
set -e
if [ "$#" -ne 1 ]; then
  echo "Usage: ./compile.sh <file.vhd>"
  exit 1
fi

CIRCT_BIN=../cirt/build/bin
ABC=../yosys/yosys-abc
BOUND=${BOUND:-15}
INPUT="$1"
DIR="$(dirname "$INPUT")"
BASE="$(basename "${INPUT%.*}")"
MLIR="${DIR}/${BASE}.mlir"
IC3_MLIR="${DIR}/${BASE}_ic3.mlir"
IC3_AIG="${DIR}/${BASE}_ic3.aig"
SV="${DIR}/${BASE}.sv"

echo "[1/3] Parsing and Extracting Micro-VHDL..."
go run . "$INPUT" "$MLIR" "$IC3_MLIR"

# Extract top-level module name from the generated MLIR (first hw.module @name)
MODULE=$(grep -oP '(?<=hw\.module @)\w+' "$MLIR" | head -1)

# Feature flags — computed once, used in multiple steps below.
HAS_BAD=$(grep -c '__verif_bad' "$IC3_MLIR" 2>/dev/null || true)
HAS_ASSUME=$(grep -c 'verif.assume' "$MLIR" 2>/dev/null || true)
HAS_AFTER_RESET=$(grep -c 'after_reset' "$INPUT" 2>/dev/null || true)
HAS_LTL=$(grep -c 'ltl' "$MLIR" 2>/dev/null || true)
HAS_CONTRACT=$(grep -c 'verif.contract' "$MLIR" 2>/dev/null || true)
HAS_ASSERT=$(grep -c 'verif.assert' "$MLIR" 2>/dev/null || true)

echo "[2a/3] Bounded Model Checking via Z3 SMT Solver (module=${MODULE}, bound=${BOUND} cycles)..."
# circt-bmc unrolls the circuit for BOUND clock cycles and hands the resulting
# SMT formula to Z3.  Any reachable state that violates a verif.assert will be
# reported as "Assertion can be violated!" with a counterexample trace.
# Override the bound: BOUND=30 ./compile.sh file.vhd
#
# circt-bmc cannot process verif.contract directly (ConvertHWToSMT fails on
# verif.symbolic_value emitted by the apply-mode lowering).  Strip contracts
# before BMC; they are verified separately in step [2c/3] below.
if [ "$HAS_CONTRACT" -gt 0 ] && [ "$HAS_ASSERT" -eq 0 ]; then
  echo "  (no direct PSL assertions — contract-only design, skipping BMC; contracts checked in step [2c/3])"
else
  BMC_MLIR="$MLIR"
  if [ "$HAS_CONTRACT" -gt 0 ]; then
    BMC_MLIR="${DIR}/${BASE}_bmc.mlir"
    "${CIRCT_BIN}/circt-opt" --strip-contracts "$MLIR" -o "$BMC_MLIR"
  fi
  "${CIRCT_BIN}/circt-bmc" \
    "$BMC_MLIR" \
    --module="${MODULE}" \
    -b "${BOUND}" \
    --run \
    --shared-libs=/usr/lib/x86_64-linux-gnu/libz3.so
fi

# Only run IC3 when the IC3 MLIR contains a __verif_bad output
# (i.e. the design has PSL assertions worth proving unboundedly).
# Skip IC3 when psl assume is present: circt-translate --export-aiger does not
# support the verif dialect, so assumptions cannot be encoded as AIGER constraints.
# The BMC step (Z3) handles constrained verification correctly via verif.assume.
if [ "$HAS_BAD" -gt 0 ] && [ "$HAS_LTL" -gt 0 ]; then
  echo "[2b/3] LTL properties present — IC3/PDR skipped (ltl lowering not supported in AIGER path; BMC covers this)."
elif [ "$HAS_BAD" -gt 0 ] && [ "$HAS_ASSUME" -gt 0 ]; then
  echo "[2b/3] PSL assumes present — IC3/PDR skipped (assumptions cannot be encoded as AIGER constraints; BMC covers this)."
elif [ "$HAS_BAD" -gt 0 ] && [ "$HAS_AFTER_RESET" -gt 0 ]; then
  echo "[2b/3] after_reset assertions present — IC3/PDR skipped (unconstrained initial states cause spurious CEX; BMC covers this)."
elif [ "$HAS_BAD" -gt 0 ] && [ "$HAS_CONTRACT" -gt 0 ]; then
  echo "[2b/3] Contracts present — IC3/PDR skipped (assumptions cannot be encoded as AIGER constraints; BMC covers this)."
elif [ "$HAS_BAD" -gt 0 ]; then
  echo "[2b/3] Unbounded IC3/PDR proof via ABC (module=${MODULE})..."
  # Lower to AIG, export to binary AIGER, run ABC pdr.
  # ABC treats each hw.output as a bad-state signal: pdr proves it unreachable.
  "${CIRCT_BIN}/circt-synth" \
    --until-before=all \
    "$IC3_MLIR" \
    | "${CIRCT_BIN}/circt-translate" \
        --export-aiger -o "$IC3_AIG"
  "$ABC" -c "read_aiger ${IC3_AIG}; pdr; quit" 2>&1
else
  echo "[2b/3] No PSL assertions found — skipping IC3/PDR."
fi

# Contract verification: lower verif.contract → verif.formal → circt-bmc.
#
# Why not pass the MLIR directly to circt-bmc?
#   --lower-contracts emits an apply-mode hw.module that contains verif.symbolic_value.
#   circt-bmc's ConvertHWToSMT pass converts all hw.modules to func.funcs, after which
#   verif.symbolic_value is no longer inside hw.module and the dialect verifier rejects it.
#   The fix: extract only the verif.formal blocks (the actual check modules) and pass
#   those to circt-bmc, which handles them via its own internal LowerTestsPass.
if [ "$HAS_CONTRACT" -gt 0 ]; then
  echo "[2c/3] Contract verification (module=${MODULE})..."
  CONTRACT_LOWERED="${DIR}/${BASE}_contract.mlir"
  FORMAL_ONLY="${DIR}/${BASE}_formal.mlir"

  # Step 1: lower verif.contract → verif.formal check blocks + apply-mode hw.module
  "${CIRCT_BIN}/circt-opt" --lower-contracts "$MLIR" -o "$CONTRACT_LOWERED"

  # Step 2: extract only verif.formal blocks — drop the apply-mode hw.module
  python3 - "$CONTRACT_LOWERED" "$FORMAL_ONLY" << 'PYEOF'
import sys
in_f, depth, out = False, 0, ["module {"]
for line in open(sys.argv[1]):
    s = line.rstrip()
    if not in_f and "verif.formal " in s:
        in_f = True
        depth = 0
    if in_f:
        out.append(s)
        depth += s.count("{") - s.count("}")
        if depth <= 0:
            in_f = False
out.append("}")
open(sys.argv[2], "w").write("\n".join(out) + "\n")
PYEOF

  # Step 3: verify each contract check module via circt-bmc's LowerTestsPass
  idx=0
  while true; do
    cmod="${MODULE}_CheckContract_${idx}"
    grep -q "@${cmod}" "$FORMAL_ONLY" 2>/dev/null || break
    echo "  Checking ${cmod}..."
    "${CIRCT_BIN}/circt-bmc" \
      "$FORMAL_ONLY" \
      --module="${cmod}" \
      -b "${BOUND}" \
      --run \
      --shared-libs=/usr/lib/x86_64-linux-gnu/libz3.so
    idx=$((idx + 1))
  done
  [ "$idx" -eq 0 ] && echo "  (no contract check modules generated — check contract syntax)"
fi

echo "[3/3] Lowering to SystemVerilog..."
"${CIRCT_BIN}/circt-opt" "$MLIR" --lower-seq-to-sv \
  | "${CIRCT_BIN}/firtool" --format=mlir --verilog \
  > "$SV"

echo "Done: ${SV}"
