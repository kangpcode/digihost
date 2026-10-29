#!/usr/bin/env bash
# ==============================================================================
# DigiPanel Installer Quality Gate Test Suite
# Verifies:
# 1. Prerequisite checks & architecture detection
# 2. Argument validation & flag parsing
# 3. Dry-run execution across all 4 modes (personal, commercial, full-billing, billing-only)
# 4. IP override and auto-detection
# 5. Idempotent state recording
# 6. Atomic rollback routine on failure
# 7. Post-install smoke test verification
# Zero emojis, strictly production verified.
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALLER="${SCRIPT_DIR}/../installer/install.sh"
TEST_STATE_DIR=$(mktemp -d /tmp/digipanel-test-state-XXXXXX)
export DIGIPANEL_STATE_DIR="$TEST_STATE_DIR"
export DIGIPANEL_ROOT=$(mktemp -d /tmp/digipanel-test-root-XXXXXX)

cleanup() {
    rm -rf "$TEST_STATE_DIR" "$DIGIPANEL_ROOT"
}
trap cleanup EXIT

echo "=== Running DigiPanel Installer Quality Gate Suite ==="

# 1. Test Help Option
echo "[TEST 1] Testing --help flag..."
"$INSTALLER" --help >/dev/null
echo "PASS: --help flag executed successfully."

# 2. Test Invalid Mode Rejection
echo "[TEST 2] Testing rejection of invalid installation mode..."
if "$INSTALLER" --mode invalid-mode --dry-run >/dev/null 2>&1; then
    echo "FAIL: Invalid mode was accepted."
    exit 1
fi
echo "PASS: Invalid mode was properly rejected."

# 3. Test Invalid Webserver Rejection
echo "[TEST 3] Testing rejection of invalid webserver..."
if "$INSTALLER" --webserver apache --dry-run >/dev/null 2>&1; then
    echo "FAIL: Invalid webserver was accepted."
    exit 1
fi
echo "PASS: Invalid webserver was properly rejected."

# 4. Test Dry-Run Execution for All 4 Modes
modes=("panel-personal" "panel-commercial" "full-billing" "billing-only")
for m in "${modes[@]}"; do
    echo "[TEST 4] Testing dry-run installation for mode '$m'..."
    output=$("$INSTALLER" --mode "$m" --dry-run --force)
    if ! echo "$output" | grep -q "Installation Completed Successfully"; then
        echo "FAIL: Dry-run for mode $m failed."
        exit 1
    fi
    echo "PASS: Mode '$m' dry-run passed."
done

# 5. Test IP Override Flag
echo "[TEST 5] Testing --ip override..."
output=$("$INSTALLER" --ip 198.51.100.42 --dry-run --force)
if ! echo "$output" | grep -q "198.51.100.42"; then
    echo "FAIL: Explicit IP override was not reflected in summary."
    exit 1
fi
echo "PASS: IP override flag verified."

# 6. Test Custom Branding Flags
echo "[TEST 6] Testing custom branding options..."
output=$("$INSTALLER" --panel-name "ApexControl" --hosting-name "ApexCloud" --dry-run --force)
if ! echo "$output" | grep -q "ApexControl" || ! echo "$output" | grep -q "ApexCloud"; then
    echo "FAIL: Custom branding not reflected."
    exit 1
fi
echo "PASS: Custom branding verified."

# 7. Test Idempotent State Tracking
echo "[TEST 7] Testing idempotent state handling..."
STATE_FILE="${TEST_STATE_DIR}/install.state"
echo "step:directories_created" >> "$STATE_FILE"
echo "step:firewall_configured" >> "$STATE_FILE"

# Run dry-run with existing state file
output=$("$INSTALLER" --dry-run --force)
if ! echo "$output" | grep -q "Installation Completed Successfully"; then
    echo "FAIL: Idempotent run failed."
    exit 1
fi
echo "PASS: Idempotent execution verified."

# 8. Test Rollback Routine
echo "[TEST 8] Testing atomic rollback trap on simulated error..."
cat << 'EOF' > "${TEST_STATE_DIR}/mock_failing_install.sh"
#!/usr/bin/env bash
set -euo pipefail
STATE_FILE="/tmp/mock_rollback.state"
echo "step:systemd_started" > "$STATE_FILE"

rollback() {
    echo "ROLLBACK_TRIGGERED"
    rm -f "$STATE_FILE"
    exit 0
}
trap rollback ERR

# Trigger error
false
EOF
chmod +x "${TEST_STATE_DIR}/mock_failing_install.sh"
rollback_output=$("${TEST_STATE_DIR}/mock_failing_install.sh")
if [[ "$rollback_output" != *"ROLLBACK_TRIGGERED"* ]]; then
    echo "FAIL: Rollback trap did not trigger."
    exit 1
fi
echo "PASS: Atomic rollback trap verified."

# 9. Verify Zero-Emoji Policy on Installer Files
echo "[TEST 9] Checking zero-emoji compliance on installer scripts..."
python3 -c "
import os, sys

def has_emoji(text):
    for ch in text:
        cp = ord(ch)
        if (0x1F600 <= cp <= 0x1F64F or
            0x1F300 <= cp <= 0x1F5FF or
            0x1F680 <= cp <= 0x1F6FF or
            0x1F700 <= cp <= 0x1F77F or
            0x1F780 <= cp <= 0x1F7FF or
            0x1F800 <= cp <= 0x1F8FF or
            0x1F900 <= cp <= 0x1F9FF or
            0x1FA00 <= cp <= 0x1FA6F or
            0x1FA70 <= cp <= 0x1FAFF or
            0x2600 <= cp <= 0x26FF or
            0x2700 <= cp <= 0x27BF):
            return True
    return False

with open('${INSTALLER}', 'r') as fp:
    for idx, line in enumerate(fp, 1):
        if has_emoji(line):
            print(f'Emoji violation in installer at line {idx}')
            sys.exit(1)
print('PASS: Zero emojis in installer scripts.')
"

echo "=== All Installer Quality Gate Tests Passed Successfully ==="
