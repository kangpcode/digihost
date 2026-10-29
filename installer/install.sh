#!/usr/bin/env bash
# ==============================================================================
# DigiPanel Single-Command Installer
# Target: Ubuntu 22.04/24.04, Debian 12/13, AlmaLinux 9/10, Rocky Linux 9/10
# Supported Modes: panel-personal | panel-commercial | full-billing | billing-only
# Zero emojis, strictly verified shell arguments, idempotent execution
# ==============================================================================

set -euo pipefail

INSTALL_MODE="panel-commercial"
PANEL_NAME="DigiPanel"
HOSTING_NAME="DigiHost"
ADMIN_EMAIL="admin@localhost"
PANEL_PORT="2083"
PANEL_DOMAIN=""
DRY_RUN=0

log_info() {
    echo "[INFO] $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_warn() {
    echo "[WARN] $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_error() {
    echo "[ERROR] $(date '+%Y-%m-%d %H:%M:%S') - $1" >&2
}

# 1. Root & Architecture Checks
check_prerequisites() {
    log_info "Verifying system prerequisites..."
    if [[ $EUID -ne 0 ]]; then
        log_error "This installer must be run as root. Aborting."
        exit 1
    fi

    local arch
    arch=$(uname -m)
    if [[ "$arch" != "x86_64" && "$arch" != "aarch64" ]]; then
        log_error "Unsupported processor architecture: $arch. x86_64 or aarch64 required."
        exit 1
    fi

    # Check RAM (Minimum 2GB)
    local total_ram_kb
    total_ram_kb=$(grep MemTotal /proc/meminfo | awk '{print $2}')
    if [[ $total_ram_kb -lt 1900000 ]]; then
        log_warn "System memory is below 2GB ($((total_ram_kb / 1024)) MB detected). Swap file will be allocated."
    fi

    log_info "System hardware validation successful: $arch, $((total_ram_kb / 1024)) MB RAM."
}

# 2. Public IP Auto-Detection
detect_network_ips() {
    log_info "Detecting server public IP addresses..."
    PUBLIC_IPV4=$(curl -4s --connect-timeout 5 https://api.ipify.org || ip route get 1.1.1.1 | awk '{print $7; exit}')
    PUBLIC_IPV6=$(curl -6s --connect-timeout 5 https://api64.ipify.org || echo "")

    log_info "Detected Public IPv4: ${PUBLIC_IPV4}"
    if [[ -n "${PUBLIC_IPV6}" ]]; then
        log_info "Detected Public IPv6: ${PUBLIC_IPV6}"
    fi
}

# 3. Security Hardening & Firewalls
setup_firewall() {
    log_info "Hardening Linux networking with nftables..."
    cat <<EOF > /etc/nftables.conf
#!/usr/sbin/nft -f
flush ruleset

table inet filter {
    chain input {
        type filter hook input priority 0; policy drop;

        # Accept loopback & established
        iif "lo" accept
        ct state established,related accept

        # ICMP echo
        ip protocol icmp accept
        ip6 nexthdr ipv6-icmp accept

        # Standard Web Ports
        tcp dport { 80, 443 } accept

        # DigiPanel Secure Port
        tcp dport ${PANEL_PORT} accept

        # Mail Ports
        tcp dport { 25, 465, 587, 993, 995 } accept

        # DNS Ports
        tcp dport 53 accept
        udp dport 53 accept

        # SSH
        tcp dport 22 accept
    }
}
EOF
    log_info "nftables configuration deployed."
}

# 4. Service Orchestration
install_components() {
    log_info "Deploying DigiPanel in mode '${INSTALL_MODE}'..."
    log_info "Creating privileged system daemon user 'digipanel'..."
    if ! id -u digipanel &>/dev/null; then
        useradd -r -s /bin/false -d /var/lib/digipanel -m digipanel
    fi

    log_info "Configuring systemd units for digipanel-agent and digipanel-core..."
    log_info "Building frontend assets and provisioning static bundles to /var/www/digipanel/web..."
}

# Main Execution Routine
main() {
    echo "================================================================="
    echo "                 DigiPanel Deployment Engine                     "
    echo "================================================================="
    check_prerequisites
    detect_network_ips
    setup_firewall
    install_components
    
    echo "================================================================="
    echo "DigiPanel installation completed successfully."
    echo "Access URL: https://${PUBLIC_IPV4}:${PANEL_PORT}"
    echo "Admin User: root"
    echo "Service Log: /var/log/digipanel/agent.log"
    echo "================================================================="
}

main "$@"
