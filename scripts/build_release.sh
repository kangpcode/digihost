#!/usr/bin/env bash
# DigiPanel Tahap 10 Signed Release Packaging Script
# Builds static core & agent binaries, refreshes SBOM, generates SHA256SUMS,
# and produces & verifies an ECDSA P-256 signature over the release manifest.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${DIGIPANEL_VERSION:-1.0.0}"
OUT_DIR="${ROOT_DIR}/packages/dist/digipanel-${VERSION}"
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

mkdir -p "$OUT_DIR"

echo "Building static DigiPanel binaries (${VERSION})..."
CGO_ENABLED=0 "$GO_BIN" build -trimpath -ldflags="-s -w" -o "${OUT_DIR}/digipanel-core" ./core
CGO_ENABLED=0 "$GO_BIN" build -trimpath -ldflags="-s -w" -o "${OUT_DIR}/digipanel-agent" ./agent

echo "Refreshing SBOM..."
"${ROOT_DIR}/scripts/generate_sbom.sh" "${OUT_DIR}/sbom.json"

echo "Generating SHA256SUMS..."
(
  cd "$OUT_DIR"
  sha256sum digipanel-core digipanel-agent sbom.json > SHA256SUMS
)

SIGNING_KEY="${RELEASE_SIGNING_KEY:-}"
PUB_KEY="${RELEASE_PUBLIC_KEY:-}"
TEMP_KEY_DIR=""

if [[ -z "$SIGNING_KEY" ]]; then
  TEMP_KEY_DIR="$(mktemp -d /tmp/digipanel-release-sign-XXXXXX)"
  trap 'rm -rf "$TEMP_KEY_DIR"' EXIT
  SIGNING_KEY="${TEMP_KEY_DIR}/release-ecdsa-priv.pem"
  PUB_KEY="${OUT_DIR}/release-signing-pub.pem"
  openssl ecparam -name prime256v1 -genkey -noout -out "$SIGNING_KEY" 2>/dev/null
  openssl ec -in "$SIGNING_KEY" -pubout -out "$PUB_KEY" 2>/dev/null
  echo "Using ephemeral ECDSA P-256 signing keypair for verification (no private key stored in repo)."
elif [[ -z "$PUB_KEY" ]]; then
  PUB_KEY="${OUT_DIR}/release-signing-pub.pem"
  openssl ec -in "$SIGNING_KEY" -pubout -out "$PUB_KEY" 2>/dev/null
fi

openssl dgst -sha256 -sign "$SIGNING_KEY" -out "${OUT_DIR}/SHA256SUMS.sig" "${OUT_DIR}/SHA256SUMS"
openssl dgst -sha256 -verify "$PUB_KEY" -signature "${OUT_DIR}/SHA256SUMS.sig" "${OUT_DIR}/SHA256SUMS"

(
  cd "$OUT_DIR"
  sha256sum -c SHA256SUMS
)

echo "Release package verified at: ${OUT_DIR}"
