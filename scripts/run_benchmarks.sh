#!/usr/bin/env bash
# Runs DigiPanel Go benchmarks and records raw results in docs/benchmark-results.txt.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_BIN="${GO_BIN:-go}"

if ! command -v "$GO_BIN" >/dev/null 2>&1; then
  TOOLCHAIN_GO="/home/kangpcode/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.3.linux-amd64/bin/go"
  if [[ -x "$TOOLCHAIN_GO" ]]; then
    GO_BIN="$TOOLCHAIN_GO"
  else
    echo "Error: go binary not found" >&2
    exit 1
  fi
fi

OUT_FILE="${ROOT_DIR}/docs/benchmark-results.txt"
{
  echo "# DigiPanel Benchmark Raw Output"
  echo "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "Host: $(uname -srm)"
  echo ""
  "$GO_BIN" test -v ./core -run TestAPILatencyP95UnderTarget
  echo ""
  "$GO_BIN" test -bench=. -benchmem -run='^$' ./core ./modules/billing
} | tee "$OUT_FILE"
