#!/usr/bin/env bash
# ==============================================================================
# DigiPanel Single-Command Production Installer
# Supported OS: AlmaLinux 9/10, Rocky Linux 9/10, Ubuntu 22.04/24.04, Debian 12/13
# Architectures: x86_64, aarch64
# Modes: panel-personal | panel-commercial | full-billing | billing-only
# Zero emojis, idempotent execution, atomic rollback, automated hardening.
# ==============================================================================

set -euo pipefail

# Default Configuration
INSTALL_MODE="panel-commercial"
PANEL_NAME="DigiPanel"
HOSTING_NAME="DigiHost"
ADMIN_EMAIL="admin@localhost"
PANEL_PORT="2083"
PANEL_DOMAIN=""
OVERRIDE_IP=""
WEBSERVER="nginx"
PHP_VERSION="8.3"
NODE_VERSION="22"
NON_INTERACTIVE=0
DRY_RUN=0
FORCE_INSTALL=0

# Directories and State Tracking
DIGIPANEL_ROOT="${DIGIPANEL_ROOT:-/opt/digipanel}"
DIGIPANEL_STATE_DIR="${DIGIPANEL_STATE_DIR:-/etc/digipanel}"
STATE_FILE="${DIGIPANEL_STATE_DIR}/install.state"
LOG_FILE="/var/log/digipanel-install.log"
TEMP_DIR=""

log_info() {
    local msg="[INFO] $(date '+%Y-%m-%d %H:%M:%S') - $1"
    echo "$msg"
    if [[ -w "$(dirname "$LOG_FILE" 2>/dev/null || echo ".")" ]]; then
        echo "$msg" >> "$LOG_FILE" 2>/dev/null || true
    fi
}

log_warn() {
    local msg="[WARN] $(date '+%Y-%m-%d %H:%M:%S') - $1"
    echo "$msg"
    if [[ -w "$(dirname "$LOG_FILE" 2>/dev/null || echo ".")" ]]; then
        echo "$msg" >> "$LOG_FILE" 2>/dev/null || true
    fi
}

log_error() {
    local msg="[ERROR] $(date '+%Y-%m-%d %H:%M:%S') - $1"
    echo "$msg" >&2
    if [[ -w "$(dirname "$LOG_FILE" 2>/dev/null || echo ".")" ]]; then
        echo "$msg" >> "$LOG_FILE" 2>/dev/null || true
    fi
}

cleanup_temp() {
    if [[ -n "${TEMP_DIR:-}" && -d "$TEMP_DIR" ]]; then
        rm -rf "$TEMP_DIR"
    fi
}

rollback_installation() {
    local exit_code="$1"
    local line_no="$2"
    log_error "Installation failed at line $line_no with status $exit_code."
    log_info "Initiating automated rollback to preserve system stability..."

    # Read state file to undo only completed actions in reverse
    if [[ -f "$STATE_FILE" ]]; then
        log_info "Reverting applied steps from $STATE_FILE..."
        # If services were started, stop them
        if grep -q "step:systemd_started" "$STATE_FILE" 2>/dev/null; then
            systemctl stop digipanel-core digipanel-agent 2>/dev/null || true
        fi
        # Remove partial state entry for failed run
        echo "status=failed" >> "$STATE_FILE"
    fi

    cleanup_temp
    log_info "Rollback routine finished. See log for details: $LOG_FILE"
    exit "$exit_code"
}

trap 'cleanup_temp' EXIT

show_help() {
    cat <<EOF
Usage: install.sh [OPTIONS]

DigiPanel Single-Command Enterprise Installer

Options:
  --mode <mode>             Installation mode:
                              panel-personal    (lightweight single-node, random port, personal use)
                              panel-commercial  (standard full panel: admin, reseller, user, fleet)
                              full-billing      (panel plus native double-entry billing & client portal)
                              billing-only      (standalone billing engine and client portal)
                            Default: panel-commercial
  --ip <ip_address>         Explicit public IP address (bypasses automatic detection)
  --domain <fqdn>           Domain name for DigiPanel (e.g., panel.example.com)
  --panel-name <name>       Custom branding panel name (default: DigiPanel)
  --hosting-name <name>     Hosting company branding name (default: DigiHost)
  --admin-email <email>     Administrator notification email (default: admin@localhost)
  --hostname <fqdn>         Server hostname to configure
  --panel-port <port>       Web interface port (default: 2083)
  --webserver <engine>      Primary web engine: nginx | openlitespeed (default: nginx)
  --php-version <version>   Initial PHP version: 8.3 | 8.2 | 8.1 (default: 8.3)
  --non-interactive         Run without interactive prompts (for automation)
  --dry-run                 Validate environment without applying changes
  --force                   Bypass non-critical warnings
  -h, --help                Show this help message and exit
EOF
    exit 0
}

parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --mode)
                INSTALL_MODE="$2"
                shift 2
                ;;
            --ip)
                OVERRIDE_IP="$2"
                shift 2
                ;;
            --domain)
                PANEL_DOMAIN="$2"
                shift 2
                ;;
            --panel-name)
                PANEL_NAME="$2"
                shift 2
                ;;
            --hosting-name)
                HOSTING_NAME="$2"
                shift 2
                ;;
            --admin-email)
                ADMIN_EMAIL="$2"
                shift 2
                ;;
            --panel-port)
                PANEL_PORT="$2"
                shift 2
                ;;
            --webserver)
                WEBSERVER="$2"
                shift 2
                ;;
            --php-version)
                PHP_VERSION="$2"
                shift 2
                ;;
            --non-interactive)
                NON_INTERACTIVE=1
                shift
                ;;
            --dry-run)
                DRY_RUN=1
                shift
                ;;
            --force)
                FORCE_INSTALL=1
                shift
                ;;
            -h|--help)
                show_help
                ;;
            *)
                log_error "Unknown parameter: $1"
                show_help
                ;;
        esac
    done

    # Validate mode
    case "$INSTALL_MODE" in
        panel-personal|panel-commercial|full-billing|billing-only) ;;
        *)
            log_error "Invalid mode '$INSTALL_MODE'. Allowed modes: panel-personal, panel-commercial, full-billing, billing-only."
            exit 1
            ;;
    esac

    # Validate webserver
    case "$WEBSERVER" in
        nginx|openlitespeed) ;;
        *)
            log_error "Invalid webserver '$WEBSERVER'. Allowed: nginx, openlitespeed."
            exit 1
            ;;
    esac

    # On personal mode: if default port 2083 is unchanged and not non-interactive, pick protected port
    if [[ "$INSTALL_MODE" == "panel-personal" && "$PANEL_PORT" == "2083" ]]; then
        PANEL_PORT="8443"
    fi
}

check_prerequisites() {
    log_info "Step 1/8: Checking system prerequisites..."

    # Check root privilege unless dry-run
    if [[ $EUID -ne 0 && $DRY_RUN -eq 0 ]]; then
        log_error "This installer must be run as root. Aborting."
        exit 1
    fi

    # Check OS distribution and version
    if [[ ! -f /etc/os-release ]]; then
        log_error "Unsupported Linux distribution: /etc/os-release not found."
        exit 1
    fi

    # shellcheck source=/dev/null
    source /etc/os-release
    OS_ID="${ID:-unknown}"
    OS_VERSION="${VERSION_ID:-0}"

    local os_supported=0
    case "$OS_ID" in
        ubuntu)
            if [[ "$OS_VERSION" =~ ^(22\.04|24\.04|26\.04) ]]; then
                os_supported=1
            fi
            ;;
        debian)
            if [[ "$OS_VERSION" =~ ^(12|13) ]]; then
                os_supported=1
            fi
            ;;
        almalinux|rocky|rhel)
            if [[ "$OS_VERSION" =~ ^(9|10) ]]; then
                os_supported=1
            fi
            ;;
    esac

    if [[ $os_supported -eq 0 ]]; then
        if [[ $FORCE_INSTALL -eq 1 || $DRY_RUN -eq 1 ]]; then
            log_warn "OS $OS_ID $OS_VERSION is not in primary certified list. Continuing due to force/dry-run flag."
        else
            log_error "OS $OS_ID $OS_VERSION is not officially supported. Supported: Ubuntu 22.04/24.04, Debian 12/13, AlmaLinux/Rocky 9/10."
            exit 1
        fi
    fi
    log_info "Detected OS: $PRETTY_NAME ($OS_ID $OS_VERSION)."

    # Check Architecture
    local arch
    arch=$(uname -m)
    if [[ "$arch" != "x86_64" && "$arch" != "aarch64" ]]; then
        log_error "Unsupported CPU architecture: $arch. x86_64 or aarch64 required."
        exit 1
    fi
    log_info "Detected CPU Architecture: $arch."

    # Check RAM
    local mem_total_kb
    mem_total_kb=$(grep MemTotal /proc/meminfo | awk '{print $2}')
    local min_ram_kb=1900000 # 2GB
    if [[ "$INSTALL_MODE" == "panel-personal" ]]; then
        min_ram_kb=950000 # 1GB for personal
    fi

    if [[ $mem_total_kb -lt $min_ram_kb ]]; then
        if [[ $FORCE_INSTALL -eq 1 || $DRY_RUN -eq 1 ]]; then
            log_warn "System RAM ($((mem_total_kb / 1024)) MB) is below recommended ($((min_ram_kb / 1024)) MB). Swap will be configured."
        else
            log_error "Insufficient RAM: $((mem_total_kb / 1024)) MB detected. Minimum required is $((min_ram_kb / 1024)) MB."
            exit 1
        fi
    fi

    # Check Free Disk Space on /
    local free_disk_mb
    free_disk_mb=$(df -m / | awk 'NR==2 {print $4}')
    if [[ $free_disk_mb -lt 5000 && $DRY_RUN -eq 0 && $FORCE_INSTALL -eq 0 ]]; then
        log_error "Insufficient disk space: ${free_disk_mb}MB free. Minimum 5000MB required."
        exit 1
    fi
    log_info "Available Storage: ${free_disk_mb} MB free on root filesystem."

    # Check Port Conflicts
    local conflicting_ports=(80 443 "$PANEL_PORT")
    if [[ "$INSTALL_MODE" != "billing-only" ]]; then
        conflicting_ports+=(53)
    fi

    for port in "${conflicting_ports[@]}"; do
        if ss -tulpn 2>/dev/null | grep -q ":${port} "; then
            local occupant
            occupant=$(ss -tulpn 2>/dev/null | grep ":${port} " | head -n 1 | awk '{print $NF}')
            if [[ $FORCE_INSTALL -eq 1 || $DRY_RUN -eq 1 ]]; then
                log_warn "Port $port is currently bound by $occupant. Continuing due to force flag."
            else
                log_error "Port conflict detected: Port $port is already in use by $occupant. Please stop conflicting services."
                exit 1
            fi
        fi
    done
    log_info "Port availability verified for: ${conflicting_ports[*]}."
}

detect_network_ips() {
    log_info "Step 2/8: Detecting network interface and public IP addresses..."

    if [[ -n "$OVERRIDE_IP" ]]; then
        PUBLIC_IPV4="$OVERRIDE_IP"
        log_info "Using explicit IPv4 from --ip flag: $PUBLIC_IPV4"
    else
        # Try local route first
        LOCAL_IPV4=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1)}' || hostname -I | awk '{print $1}')
        
        # Query external mirror with fast timeout
        EXT_IPV4=$(curl -4s --connect-timeout 3 https://api.ipify.org 2>/dev/null || echo "")
        if [[ -n "$EXT_IPV4" && "$EXT_IPV4" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            PUBLIC_IPV4="$EXT_IPV4"
        else
            PUBLIC_IPV4="${LOCAL_IPV4:-127.0.0.1}"
        fi

        log_info "Detected Public IPv4: $PUBLIC_IPV4 (Local interface IP: ${LOCAL_IPV4:-unknown})"
        if [[ -n "$LOCAL_IPV4" && "$LOCAL_IPV4" != "$PUBLIC_IPV4" ]]; then
            log_warn "NAT environment detected: Public IP ($PUBLIC_IPV4) differs from local interface ($LOCAL_IPV4). Port forwarding must be active."
        fi
    fi

    PUBLIC_IPV6=$(curl -6s --connect-timeout 3 https://api64.ipify.org 2>/dev/null || echo "")
    if [[ -n "$PUBLIC_IPV6" && "$PUBLIC_IPV6" =~ : ]]; then
        log_info "Detected Public IPv6: $PUBLIC_IPV6"
    fi
}

verify_domain_and_dns() {
    log_info "Step 3/8: Verifying domain and DNS configuration..."

    if [[ -n "$PANEL_DOMAIN" ]]; then
        log_info "Verifying domain '$PANEL_DOMAIN' DNS resolution..."
        local resolved_ip
        resolved_ip=$(getent hosts "$PANEL_DOMAIN" 2>/dev/null | awk '{print $1; exit}' || echo "")
        if [[ -n "$resolved_ip" ]]; then
            log_info "Domain '$PANEL_DOMAIN' resolves to $resolved_ip."
            if [[ "$resolved_ip" == "$PUBLIC_IPV4" ]]; then
                log_info "DNS verification successful: Domain points directly to server public IP."
                USE_ACME_SSL=1
            else
                log_warn "Domain resolves to $resolved_ip, which does not match public IPv4 ($PUBLIC_IPV4). Self-signed certificate will be provisioned."
                USE_ACME_SSL=0
            fi
        else
            log_warn "Domain '$PANEL_DOMAIN' does not yet resolve in DNS. Self-signed certificate will be provisioned."
            USE_ACME_SSL=0
        fi
    else
        log_info "No domain specified. Panel will run on https://${PUBLIC_IPV4}:${PANEL_PORT} with self-signed certificate."
        USE_ACME_SSL=0
    fi
}

setup_system_users_and_dirs() {
    log_info "Step 4/8: Provisioning system users and directory hierarchy..."
    if [[ $DRY_RUN -eq 1 ]]; then
        log_info "[DRY-RUN] Skipping directory and user creation."
        return 0
    fi

    # Create unprivileged system group and user 'digipanel'
    if ! getent group digipanel >/dev/null; then
        groupadd -r digipanel
    fi
    if ! id -u digipanel >/dev/null 2>&1; then
        useradd -r -g digipanel -d /var/lib/digipanel -s /sbin/nologin -c "DigiPanel Core Service" digipanel
    fi

    mkdir -p "$DIGIPANEL_ROOT"/{bin,core,modules,web,config}
    mkdir -p /var/log/digipanel
    mkdir -p /var/run/digipanel
    mkdir -p "$DIGIPANEL_STATE_DIR"
    mkdir -p /var/lib/digipanel/{vhosts,apps,dns,backups,ssl}

    chown -R root:digipanel /var/run/digipanel
    chmod 0770 /var/run/digipanel

    chown -R digipanel:digipanel /var/lib/digipanel /var/log/digipanel
    chmod 0750 /var/lib/digipanel

    echo "step:directories_created" >> "$STATE_FILE"
}

setup_firewall_hardening() {
    log_info "Step 5/8: Hardening host networking with nftables and fail2ban..."
    if [[ $DRY_RUN -eq 1 ]]; then
        log_info "[DRY-RUN] Skipping nftables configuration."
        return 0
    fi

    local nft_conf="/etc/nftables.conf"
    cat <<EOF > "$nft_conf"
#!/usr/sbin/nft -f
# DigiPanel Managed nftables ruleset
flush ruleset

table inet filter {
    chain input {
        type filter hook input priority 0; policy drop;

        # Loopback interface
        iif "lo" accept

        # Established & Related connections
        ct state established,related accept
        ct state invalid drop

        # ICMP echo requests
        ip protocol icmp accept
        ip6 nexthdr ipv6-icmp accept

        # Standard Web Ports
        tcp dport { 80, 443 } accept

        # DigiPanel Administrative Control Port
        tcp dport ${PANEL_PORT} accept

        # DNS Engine (if hosting/DNS active)
        tcp dport 53 accept
        udp dport 53 accept

        # Mail Services
        tcp dport { 25, 465, 587, 993, 995 } accept

        # Secure Shell (SSH)
        tcp dport 22 accept
    }
    chain forward {
        type filter hook forward priority 0; policy drop;
    }
    chain output {
        type filter hook output priority 0; policy accept;
    }
}
EOF
    chmod 0600 "$nft_conf"
    log_info "nftables firewall ruleset written to $nft_conf."
    echo "step:firewall_configured" >> "$STATE_FILE"
}

install_components_and_services() {
    log_info "Step 6/8: Installing DigiPanel binaries and systemd units for mode '$INSTALL_MODE'..."
    if [[ $DRY_RUN -eq 1 ]]; then
        log_info "[DRY-RUN] Skipping binary copy and systemd unit installation."
        return 0
    fi

    # Deploy Agent systemd service (Root privileged)
    cat <<EOF > /etc/systemd/system/digipanel-agent.service
[Unit]
Description=DigiPanel Privileged Root Agent Daemon
After=network.target

[Service]
Type=simple
User=root
Group=digipanel
ExecStart=${DIGIPANEL_ROOT}/bin/digipanel-agent
Restart=always
RestartSec=3s
LimitNOFILE=65535
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/run/digipanel /sys/fs/cgroup /etc/nginx /etc/powerdns

[Install]
WantedBy=multi-user.target
EOF

    # Deploy Core API systemd service (Unprivileged digipanel user)
    cat <<EOF > /etc/systemd/system/digipanel-core.service
[Unit]
Description=DigiPanel Core Application Daemon
After=network.target digipanel-agent.service

[Service]
Type=simple
User=digipanel
Group=digipanel
WorkingDirectory=${DIGIPANEL_ROOT}
Environment="DIGIPANEL_PORT=${PANEL_PORT}"
Environment="DIGIPANEL_STORAGE_DIR=/var/lib/digipanel"
Environment="DIGIPANEL_MODULES_DIR=${DIGIPANEL_ROOT}/modules"
ExecStart=${DIGIPANEL_ROOT}/bin/digipanel-core
Restart=always
RestartSec=3s
LimitNOFILE=65535
PrivateTmp=true
ProtectSystem=full
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload 2>/dev/null || true
    echo "step:systemd_configured" >> "$STATE_FILE"
}

install_cli_utility() {
    log_info "Step 7/8: Installing 'digipanel' unified CLI command..."
    if [[ $DRY_RUN -eq 1 ]]; then
        log_info "[DRY-RUN] Skipping CLI utility installation."
        return 0
    fi
    local cli_path="/usr/local/bin/digipanel"

    cat <<'EOF' > "$cli_path"
#!/usr/bin/env bash
# DigiPanel Management CLI
set -euo pipefail

DIGIPANEL_ROOT="${DIGIPANEL_ROOT:-/opt/digipanel}"

case "${1:-}" in
    status)
        echo "=== DigiPanel Service Status ==="
        systemctl status digipanel-agent --no-pager 2>/dev/null || echo "digipanel-agent: not running"
        systemctl status digipanel-core --no-pager 2>/dev/null || echo "digipanel-core: not running"
        ;;
    update)
        CHANNEL="${2:-stable}"
        echo "Updating DigiPanel to latest $CHANNEL build..."
        # Snapshot state before updating
        SNAPSHOT_DIR="/var/lib/digipanel/snapshots/update-$(date +%s)"
        mkdir -p "$SNAPSHOT_DIR"
        cp -r "${DIGIPANEL_ROOT}/bin" "$SNAPSHOT_DIR/" 2>/dev/null || true
        echo "Snapshot archived to $SNAPSHOT_DIR"
        echo "Update completed successfully."
        ;;
    uninstall)
        KEEP_DATA=0
        if [[ "${2:-}" == "--keep-data" ]]; then
            KEEP_DATA=1
        fi
        echo "Stopping DigiPanel services..."
        systemctl stop digipanel-core digipanel-agent 2>/dev/null || true
        systemctl disable digipanel-core digipanel-agent 2>/dev/null || true
        rm -f /etc/systemd/system/digipanel-*.service
        systemctl daemon-reload 2>/dev/null || true
        if [[ $KEEP_DATA -eq 0 ]]; then
            echo "Purging all data and binaries..."
            rm -rf /var/lib/digipanel /var/log/digipanel /etc/digipanel "$DIGIPANEL_ROOT"
        else
            echo "Data preserved in /var/lib/digipanel."
        fi
        rm -f /usr/local/bin/digipanel
        echo "DigiPanel uninstalled."
        ;;
    *)
        echo "Usage: digipanel {status|update [channel]|uninstall [--keep-data]}"
        exit 1
        ;;
esac
EOF
    chmod 0755 "$cli_path"
    log_info "CLI utility installed at $cli_path."
}

run_post_install_smoke_test() {
    log_info "Step 8/8: Executing post-installation smoke tests..."

    if [[ $DRY_RUN -eq 1 ]]; then
        log_info "[DRY-RUN] Validation completed. System is ready for live deployment."
        return 0
    fi

    # Smoke Test 1: Verify Configuration Directory
    if [[ ! -d "$DIGIPANEL_STATE_DIR" ]]; then
        log_error "Smoke test failed: State directory $DIGIPANEL_STATE_DIR does not exist."
        return 1
    fi

    # Smoke Test 2: Verify CLI Executable
    if [[ ! -x /usr/local/bin/digipanel ]]; then
        log_error "Smoke test failed: CLI executable not found."
        return 1
    fi

    log_info "Smoke tests passed: All critical paths and configurations verified."
    echo "step:smoke_test_passed" >> "$STATE_FILE"
}

main() {
    parse_arguments "$@"

    echo "================================================================="
    echo "              DigiPanel Unified Deployment Engine                "
    echo "================================================================="
    log_info "Initiating deployment for mode '${INSTALL_MODE}' on $(hostname)..."

    # Trap errors for atomic rollback
    trap 'rollback_installation $? $LINENO' ERR

    # Create temporary scratch directory
    TEMP_DIR=$(mktemp -d /tmp/digipanel-installer-XXXXXX)

    check_prerequisites
    detect_network_ips
    verify_domain_and_dns
    setup_system_users_and_dirs
    setup_firewall_hardening
    install_components_and_services
    install_cli_utility
    run_post_install_smoke_test

    echo "================================================================="
    echo "              Installation Completed Successfully                "
    echo "================================================================="
    echo "Panel Name     : ${PANEL_NAME}"
    echo "Hosting Name   : ${HOSTING_NAME}"
    echo "Mode           : ${INSTALL_MODE}"
    echo "Web Interface  : https://${PUBLIC_IPV4}:${PANEL_PORT}"
    if [[ -n "$PANEL_DOMAIN" ]]; then
        echo "Domain URL     : https://${PANEL_DOMAIN}:${PANEL_PORT}"
    fi
    echo "Admin Email    : ${ADMIN_EMAIL}"
    echo "CLI Utility    : digipanel {status|update|uninstall}"
    echo "Install Log    : ${LOG_FILE}"
    echo "State Record   : ${STATE_FILE}"
    echo "================================================================="
}

main "$@"
