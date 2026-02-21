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

echo "[2a/3] Bounded Model Checking via Z3 SMT Solver (module=${MODULE}, bound=${BOUND} cycles)..."
# circt-bmc unrolls the circuit for BOUND clock cycles and hands the resulting
# SMT formula to Z3.  Any reachable state that violates a verif.assert will be
# reported as "Assertion can be violated!" with a counterexample trace.
# Override the bound: BOUND=30 ./compile.sh file.vhd
"${CIRCT_BIN}/circt-bmc" \
  "$MLIR" \
  --module="${MODULE}" \
  -b "${BOUND}" \
  --run \
  --shared-libs=/usr/lib/x86_64-linux-gnu/libz3.so

# Only run IC3 when the IC3 MLIR contains a __verif_bad output
# (i.e. the design has PSL assertions worth proving unboundedly).
if grep -q '__verif_bad' "$IC3_MLIR" 2>/dev/null; then
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

echo "[3/3] Lowering to SystemVerilog..."
"${CIRCT_BIN}/circt-opt" "$MLIR" --lower-seq-to-sv \
  | "${CIRCT_BIN}/firtool" --format=mlir --verilog \
  > "$SV"

echo "Done: ${SV}"
