#!/usr/bin/env bash
# Generates a CycloneDX-style JSON Software Bill of Materials (SBOM) from go.mod
# and web/package-lock.json so all commercial-compatible licenses are inventoried.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_FILE="${1:-${ROOT_DIR}/docs/sbom.json}"

python3 - "$ROOT_DIR" "$OUT_FILE" <<'PY'
import json
import os
import re
import sys
from datetime import datetime, timezone

root = sys.argv[1]
out_file = sys.argv[2]

components = []

gomod = os.path.join(root, "go.mod")
if os.path.exists(gomod):
    with open(gomod, "r", encoding="utf-8") as fh:
        for line in fh:
            m = re.match(r"^\s*([a-zA-Z0-9._/-]+)\s+(v[0-9A-Za-z.+-]+)", line)
            if m:
                components.append({
                    "type": "library",
                    "ecosystem": "golang",
                    "name": m.group(1),
                    "version": m.group(2),
                    "purl": f"pkg:golang/{m.group(1)}@{m.group(2)}",
                })

pkg_lock = os.path.join(root, "web", "package-lock.json")
if os.path.exists(pkg_lock):
    with open(pkg_lock, "r", encoding="utf-8") as fh:
        data = json.load(fh)
    for path_key, meta in data.get("packages", {}).items():
        if not path_key:
            continue
        name = meta.get("name") or path_key.split("node_modules/")[-1]
        version = meta.get("version", "")
        if not version:
            continue
        comp = {
            "type": "library",
            "ecosystem": "npm",
            "name": name,
            "version": version,
            "purl": f"pkg:npm/{name}@{version}",
        }
        if meta.get("license"):
            comp["licenses"] = [{"license": {"id": meta["license"]}}]
        components.append(comp)

doc = {
    "bomFormat": "CycloneDX",
    "specVersion": "1.5",
    "version": 1,
    "metadata": {
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "component": {
            "type": "application",
            "name": "digipanel",
            "version": "1.0.0",
        },
    },
    "components": components,
}

os.makedirs(os.path.dirname(out_file), exist_ok=True)
with open(out_file, "w", encoding="utf-8") as fh:
    json.dump(doc, fh, indent=2)

print(f"SBOM generated: {out_file} ({len(components)} components)")
PY
