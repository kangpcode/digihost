# DigiPanel Tahap 11 Benchmark Report (Raw Data & Protocol Compliance)

Generated: 2026-11-23T04:21:00.787344+00:00
Host: Linux 7.0.0-43-generic (x86_64, 12 CPUs)
Protocol: Section 19 (5 repeated runs per metric, median and spread reported, zero fabricated competitor numbers).

## 1. DigiPanel Measured Results (5-Run Protocol)

| Metric | Target (Section 19) | Measured Median | Spread (Min - Max) | Status |
| --- | --- | --- | --- | --- |
| Idle RSS Memory (`digipanel-core` + `digipanel-agent`) | < 200 MB | 84.27 MB | 83.99 MB - 84.34 MB | PASS |
| API Latency (200 Concurrent Sessions p95) | < 300 ms | 130.074µs | 45.024µs - 311.296µs | PASS |
| 100 Sequential Accounts (User+VHost+DNS+Mail) | median < 3 s/account | 47.853698ms | p95 median: 129.849265ms | PASS |
| Competitor CVE Regression Suite | 100% pass | 100% pass | N/A | PASS |

### Idle RSS Per-Run Samples
| Run | `digipanel-core` RSS | `digipanel-agent` RSS | Combined RSS |
| --- | --- | --- | --- |
| 1 | 78.24 MB | 5.75 MB | 83.99 MB |
| 2 | 78.55 MB | 5.8 MB | 84.34 MB |
| 3 | 78.49 MB | 5.78 MB | 84.27 MB |
| 4 | 78.5 MB | 5.78 MB | 84.29 MB |
| 5 | 78.41 MB | 5.84 MB | 84.24 MB |

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
