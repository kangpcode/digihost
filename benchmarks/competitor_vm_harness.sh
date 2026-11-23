#!/usr/bin/env bash
# Reproducible Clean-VM Competitor HTTP Latency & RSS Probe Harness
# Usage (after reviewing target product EULA on identical VM hardware):
#   ./benchmarks/competitor_vm_harness.sh --name hestiacp --url https://10.0.0.5:8083/login/ --runs 5

set -euo pipefail

TARGET_NAME=""
TARGET_URL=""
RUNS=5
CONCURRENCY=200

while [[ $# -gt 0 ]]; do
  case "$1" in
    --name) TARGET_NAME="$2"; shift 2 ;;
    --url) TARGET_URL="$2"; shift 2 ;;
    --runs) RUNS="$2"; shift 2 ;;
    --concurrency) CONCURRENCY="$2"; shift 2 ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

if [[ -z "$TARGET_NAME" || -z "$TARGET_URL" ]]; then
  echo "Usage: $0 --name <panel> --url <endpoint> [--runs 5] [--concurrency 200]" >&2
  exit 1
fi

OUT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/raw"
mkdir -p "$OUT_DIR"
OUT_FILE="${OUT_DIR}/${TARGET_NAME}-http-probe.json"

python3 - "$TARGET_NAME" "$TARGET_URL" "$RUNS" "$CONCURRENCY" "$OUT_FILE" <<'PY'
import concurrent.futures
import json
import statistics
import sys
import time
import urllib.request
import ssl

name, url, runs, conc, out_file = sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4]), sys.argv[5]
ctx = ssl._create_unverified_context()

def one_req():
    t0 = time.perf_counter()
    req = urllib.request.Request(url, method="GET")
    with urllib.request.urlopen(req, context=ctx, timeout=10) as resp:
        resp.read()
    return (time.perf_counter() - t0) * 1000.0

p95s = []
for _ in range(runs):
    with concurrent.futures.ThreadPoolExecutor(max_workers=conc) as ex:
        times = sorted(ex.map(lambda _: one_req(), range(conc)))
    p95s.append(round(times[int(0.95 * len(times))], 3))

res = {
    "target": name,
    "url": url,
    "runs": runs,
    "concurrency": conc,
    "p95_ms_samples": p95s,
    "median_p95_ms": round(statistics.median(p95s), 3),
    "min_p95_ms": min(p95s),
    "max_p95_ms": max(p95s),
}
with open(out_file, "w", encoding="utf-8") as fh:
    json.dump(res, fh, indent=2)
print(json.dumps(res, indent=2))
PY
