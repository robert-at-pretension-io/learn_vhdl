#!/bin/bash
# Splits one or more files into numbered chunks and copies one chunk at a time.
#
# Usage:
#   ./slice_file.sh <file> [lines_per_chunk] [more_files...]
#   ./slice_file.sh -c <lines_per_chunk> <file1> [file2 ...]
#
# Controls:
#   Enter: copy next chunk
#   q: quit

set -euo pipefail

CHUNK=500
FILES=()

usage() {
  cat <<USAGE
Usage:
  $0 <file> [lines_per_chunk] [more_files...]
  $0 -c <lines_per_chunk> <file1> [file2 ...]

Default chunk size: 500 lines
USAGE
}

is_number() {
  [[ "$1" =~ ^[0-9]+$ ]]
}

# Backward-compatible mode:
#   script file 300 file2 file3
if [[ $# -ge 1 && -f "$1" ]]; then
  FILES+=("$1")
  shift
  if [[ $# -ge 1 ]] && is_number "$1"; then
    CHUNK="$1"
    shift
  fi
  while [[ $# -gt 0 ]]; do
    FILES+=("$1")
    shift
  done
else
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -c|--chunk)
        shift
        if [[ $# -eq 0 ]]; then
          echo "Error: missing value for --chunk"
          usage
          exit 1
        fi
        CHUNK="$1"
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        FILES+=("$1")
        ;;
    esac
    shift
  done
fi

if ! is_number "$CHUNK" || [[ "$CHUNK" -le 0 ]]; then
  echo "Error: lines_per_chunk must be a positive integer"
  exit 1
fi

if [[ ${#FILES[@]} -eq 0 ]]; then
  usage
  exit 1
fi

for file in "${FILES[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "Error: file not found: $file"
    exit 1
  fi
done

copy_to_clipboard() {
  if command -v xclip >/dev/null 2>&1; then
    xclip -selection clipboard
  elif command -v wl-copy >/dev/null 2>&1; then
    wl-copy
  elif command -v pbcopy >/dev/null 2>&1; then
    pbcopy
  else
    echo "Error: clipboard tool not found (need xclip, wl-copy, or pbcopy)." >&2
    exit 1
  fi
}

TOTAL_CHUNKS_ALL=0
for file in "${FILES[@]}"; do
  total_lines=$(wc -l < "$file")
  chunks=$(( (total_lines + CHUNK - 1) / CHUNK ))
  TOTAL_CHUNKS_ALL=$((TOTAL_CHUNKS_ALL + chunks))
done

echo "Files: ${#FILES[@]} | Chunk size: $CHUNK lines | Total chunks: $TOTAL_CHUNKS_ALL"
echo

GLOBAL_INDEX=0

for file in "${FILES[@]}"; do
  total_lines=$(wc -l < "$file")
  chunks=$(( (total_lines + CHUNK - 1) / CHUNK ))
  name=$(basename "$file")

  echo "File: $name ($total_lines lines, $chunks chunks)"

  for ((i=1; i<=chunks; i++)); do
    GLOBAL_INDEX=$((GLOBAL_INDEX + 1))
    start=$(( (i - 1) * CHUNK + 1 ))
    end=$(( i * CHUNK ))
    if [[ "$end" -gt "$total_lines" ]]; then
      end=$total_lines
    fi

    header="=== $name chunk $i/$chunks (global $GLOBAL_INDEX/$TOTAL_CHUNKS_ALL, lines $start-$end) ==="

    printf "[%d/%d] Press Enter to copy %s lines %d-%d (or q to quit): " "$GLOBAL_INDEX" "$TOTAL_CHUNKS_ALL" "$name" "$start" "$end"
    read -r reply
    if [[ "$reply" == "q" ]]; then
      exit 0
    fi

    {
      echo "$header"
      sed -n "${start},${end}p" "$file"
      echo
      echo "=== end chunk $i/$chunks ==="
    } | copy_to_clipboard

    echo "  Copied to clipboard."
  done

  echo
 done

echo "Done: all $TOTAL_CHUNKS_ALL chunks sent."
