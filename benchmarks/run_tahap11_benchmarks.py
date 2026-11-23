#!/usr/bin/env python3
"""
DigiPanel Tahap 11 Benchmark Harness (benchmarks/run_tahap11_benchmarks.py)
Measures DigiPanel metrics using the Section 19 repeatable protocol:
- 5 independent launches of compiled digipanel-core + digipanel-agent for Idle RSS (VmRSS)
- 5 runs of 200 concurrent API requests (p95 latency)
- 5 runs of 100 sequential account provisions (median & p95)
- Outputs raw JSON to benchmarks/raw/digipanel-metrics.json and summary to docs/benchmark-report.md
"""

import json
import os
import platform
import re
import shutil
import socket
import statistics
import subprocess
import tempfile
import time
import urllib.request
from datetime import datetime, timezone

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
CORE_BIN = os.path.join(ROOT, "packages", "dist", "digipanel-1.0.0", "digipanel-core")
AGENT_BIN = os.path.join(ROOT, "packages", "dist", "digipanel-1.0.0", "digipanel-agent")
RAW_OUT = os.path.join(ROOT, "benchmarks", "raw", "digipanel-metrics.json")
DOC_OUT = os.path.join(ROOT, "docs", "benchmark-report.md")

GO_BIN = shutil.which("go") or "/home/kangpcode/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.3.linux-amd64/bin/go"


def free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def read_rss_kb(pid: int) -> int:
    with open(f"/proc/{pid}/status", "r", encoding="utf-8") as fh:
        for line in fh:
            if line.startswith("VmRSS:"):
                return int(line.split()[1])
    raise RuntimeError(f"VmRSS not found for pid {pid}")


def measure_idle_rss_runs(runs: int = 5) -> dict:
    samples = []
    for run in range(runs):
        tmp = tempfile.mkdtemp(prefix=f"dp-rss-{run}-")
        try:
            port = free_port()
            sock_path = os.path.join(tmp, "agent.sock")
            agent_env = os.environ.copy()
            agent_env["DIGIPANEL_AGENT_SOCKET"] = sock_path
            agent_proc = subprocess.Popen(
                [AGENT_BIN],
                env=agent_env,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )

            core_env = os.environ.copy()
            core_env["DIGIPANEL_STORAGE_DIR"] = os.path.join(tmp, "storage")
            core_env["DIGIPANEL_MODULES_DIR"] = os.path.join(ROOT, "modules")
            core_env["DIGIPANEL_PORT"] = str(port)
            core_env["DIGIPANEL_ADMIN_PASSWORD"] = "BenchSecurePass2026!"
            core_env["DIGIPANEL_AGENT_SOCKET"] = sock_path
            core_proc = subprocess.Popen(
                [CORE_BIN],
                env=core_env,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )

            try:
                deadline = time.time() + 10.0
                healthy = False
                while time.time() < deadline:
                    try:
                        with urllib.request.urlopen(f"http://127.0.0.1:{port}/api/v1/health", timeout=1.0) as resp:
                            if resp.status == 200:
                                healthy = True
                                break
                    except Exception:
                        time.sleep(0.1)
                if not healthy:
                    raise RuntimeError(f"digipanel-core failed health probe on run {run}")

                # Allow process RSS to settle briefly
                time.sleep(0.3)
                core_kb = read_rss_kb(core_proc.pid)
                agent_kb = read_rss_kb(agent_proc.pid)
                total_mb = round((core_kb + agent_kb) / 1024.0, 2)
                samples.append({
                    "run": run + 1,
                    "core_rss_mb": round(core_kb / 1024.0, 2),
                    "agent_rss_mb": round(agent_kb / 1024.0, 2),
                    "total_rss_mb": total_mb,
                })
            finally:
                core_proc.terminate()
                agent_proc.terminate()
                core_proc.wait(timeout=5)
                agent_proc.wait(timeout=5)
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    totals = [s["total_rss_mb"] for s in samples]
    return {
        "samples": samples,
        "median_mb": round(statistics.median(totals), 2),
        "min_mb": min(totals),
        "max_mb": max(totals),
        "target_mb": 200.0,
        "passed": statistics.median(totals) < 200.0,
    }


def run_go_benchmarks() -> dict:
    cmd = [
        GO_BIN, "test", "-v", "-count=1", "./core",
        "-run", "TestAPILatencyP95UnderTarget|TestBenchmark100SequentialAccounts5Runs|TestCompetitorCVERegressionSuite",
    ]
    res = subprocess.run(cmd, cwd=ROOT, capture_output=True, text=True, check=True)
    out = res.stdout + "\n" + res.stderr

    micro = subprocess.run(
        [GO_BIN, "test", "-bench=.", "-benchmem", "-run=^$", "./core", "./modules/billing"],
        cwd=ROOT, capture_output=True, text=True, check=True,
    )
    micro_out = micro.stdout + "\n" + micro.stderr

    api_match = re.search(r"API 200-concurrent p95 across 5 runs: runs=\[([^\]]+)\] median=([^\s]+) min=([^\s]+) max=([^\s]+)", out)
    acct_match = re.search(r"100-account provisioning .* totals=\[([^\]]+)\] median_per_account=([^\s]+) median_p95=([^\s]+)", out)
    cve_pass = "--- PASS: TestCompetitorCVERegressionSuite" in out

    return {
        "api_200_concurrent": {
            "runs": api_match.group(1) if api_match else "",
            "median_p95": api_match.group(2) if api_match else "",
            "min_p95": api_match.group(3) if api_match else "",
            "max_p95": api_match.group(4) if api_match else "",
            "target": "< 300ms",
            "passed": "--- PASS: TestAPILatencyP95UnderTarget" in out,
        },
        "sequential_100_accounts": {
            "totals_5_runs": acct_match.group(1) if acct_match else "",
            "median_per_account": acct_match.group(2) if acct_match else "",
            "median_p95_per_account": acct_match.group(3) if acct_match else "",
            "target": "median < 3s",
            "passed": "--- PASS: TestBenchmark100SequentialAccounts5Runs" in out,
        },
        "cve_regression_suite": {
            "passed": cve_pass,
            "target": "100% pass",
        },
        "raw_go_test_output": out.strip(),
        "raw_go_microbench_output": micro_out.strip(),
    }


def main():
    if not os.path.exists(CORE_BIN) or not os.path.exists(AGENT_BIN):
        raise SystemExit(f"Compiled binaries missing; run scripts/build_release.sh first ({CORE_BIN})")

    idle_rss = measure_idle_rss_runs(5)
    go_bench = run_go_benchmarks()

    report = {
        "timestamp_utc": datetime.now(timezone.utc).isoformat(),
        "host": {
            "system": platform.system(),
            "release": platform.release(),
            "machine": platform.machine(),
            "cpu_count": os.cpu_count(),
        },
        "digipanel_version": "1.0.0",
        "idle_rss": idle_rss,
        "go_benchmarks": go_bench,
        "competitor_status": "Pending clean-VM execution and EULA review; no competitor numbers fabricated.",
    }

    os.makedirs(os.path.dirname(RAW_OUT), exist_ok=True)
    with open(RAW_OUT, "w", encoding="utf-8") as fh:
        json.dump(report, fh, indent=2)

    md = f"""# DigiPanel Tahap 11 Benchmark Report (Raw Data & Protocol Compliance)

Generated: {report['timestamp_utc']}
Host: {report['host']['system']} {report['host']['release']} ({report['host']['machine']}, {report['host']['cpu_count']} CPUs)
Protocol: Section 19 (5 repeated runs per metric, median and spread reported, zero fabricated competitor numbers).

## 1. DigiPanel Measured Results (5-Run Protocol)

| Metric | Target (Section 19) | Measured Median | Spread (Min - Max) | Status |
| --- | --- | --- | --- | --- |
| Idle RSS Memory (`digipanel-core` + `digipanel-agent`) | < 200 MB | {idle_rss['median_mb']} MB | {idle_rss['min_mb']} MB - {idle_rss['max_mb']} MB | {'PASS' if idle_rss['passed'] else 'FAIL'} |
| API Latency (200 Concurrent Sessions p95) | < 300 ms | {go_bench['api_200_concurrent']['median_p95']} | {go_bench['api_200_concurrent']['min_p95']} - {go_bench['api_200_concurrent']['max_p95']} | {'PASS' if go_bench['api_200_concurrent']['passed'] else 'FAIL'} |
| 100 Sequential Accounts (User+VHost+DNS+Mail) | median < 3 s/account | {go_bench['sequential_100_accounts']['median_per_account']} | p95 median: {go_bench['sequential_100_accounts']['median_p95_per_account']} | {'PASS' if go_bench['sequential_100_accounts']['passed'] else 'FAIL'} |
| Competitor CVE Regression Suite | 100% pass | 100% pass | N/A | {'PASS' if go_bench['cve_regression_suite']['passed'] else 'FAIL'} |

### Idle RSS Per-Run Samples
| Run | `digipanel-core` RSS | `digipanel-agent` RSS | Combined RSS |
| --- | --- | --- | --- |
"""
    for s in idle_rss["samples"]:
        md += f"| {s['run']} | {s['core_rss_mb']} MB | {s['agent_rss_mb']} MB | {s['total_rss_mb']} MB |\n"

    md += f"""
## 2. Unverified / Pending Clean-VM Metrics

The following Section 19 metrics cannot be validly measured inside this sandboxed workspace and remain pending clean-VM execution + legal EULA verification:
- **Full clean-VM installation duration** (target < 10 minutes across AlmaLinux/Rocky 9-10, Ubuntu 22.04/24.04, Debian 12-13).
- **Zero-downtime panel update & rollback duration** (target < 2 minutes).
- **Live PHP version switch per domain** (requires `php-fpm` binary installed on host).
- **First-load Lighthouse score** (requires headless Chromium & built `web/dist` served over simulated 4G).
- **Competitor control panel measurements** (cPanel/WHM, Plesk, DirectAdmin, HestiaCP, aaPanel, CyberPanel): not installed in this environment; EULA terms restricting benchmark publication must be reviewed prior to running `benchmarks/competitor_vm_harness.sh` on dedicated identical VMs.

## 3. Raw Artifact Location

- Raw JSON metrics: `benchmarks/raw/digipanel-metrics.json`
- Competitor clean-VM harness: `benchmarks/competitor_vm_harness.sh`
"""
    with open(DOC_OUT, "w", encoding="utf-8") as fh:
        fh.write(md)

    print(f"Saved raw metrics to {RAW_OUT} and report to {DOC_OUT}")
    print(f"Idle RSS median={idle_rss['median_mb']} MB (min={idle_rss['min_mb']}, max={idle_rss['max_mb']})")
    print(f"API 200-concurrent p95 median={go_bench['api_200_concurrent']['median_p95']}")
    print(f"100-account median_per_account={go_bench['sequential_100_accounts']['median_per_account']}")


if __name__ == "__main__":
    main()
