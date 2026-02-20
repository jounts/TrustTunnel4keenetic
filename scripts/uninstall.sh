#!/bin/sh
set -e

INSTALL_DIR="/opt/trusttunnel_client"
HOOKS_DIR="/opt/etc/ndm"
INIT_DIR="/opt/etc/init.d"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { printf "${GREEN}[INFO]${NC} %s\n" "$1"; }
warn()  { printf "${YELLOW}[WARN]${NC} %s\n" "$1"; }
error() { printf "${RED}[ERROR]${NC} %s\n" "$1"; exit 1; }

ndm_cmd_uninstall() {
    local json="["
    local first=1
    for cmd in "$@"; do
        [ "$first" = "1" ] && first=0 || json="${json},"
        json="${json}{\"parse\":\"${cmd}\"}"
    done
    json="${json}]"

    if command -v curl > /dev/null 2>&1; then
        local resp
        resp=$(curl -s -o /dev/null -w "%{http_code}" \
            -X POST -H "Content-Type: application/json" \
            -d "$json" "http://localhost:79/rci/" 2>/dev/null)
        if [ "$resp" = "200" ]; then
            return 0
        fi
    fi

    for cmd in "$@"; do
        ndmc -c "$cmd" 2>/dev/null || true
    done
}

remove_ndm_interfaces() {
    info "Removing NDMS interfaces..."
    for idx in 0 1 2 3; do
        ndm_cmd_uninstall \
            "no interface Proxy${idx}" \
            2>/dev/null && info "  Removed Proxy${idx}" || true
        ndm_cmd_uninstall \
            "no interface OpkgTun${idx}" \
            "no ip route default 172.16.219.2 OpkgTun${idx}" \
            2>/dev/null && info "  Removed OpkgTun${idx}" || true
    done
    ndm_cmd_uninstall "system configuration save" 2>/dev/null || true
}

main() {
    info "TrustTunnel for Keenetic uninstaller"
    echo ""

    # 1. Stop services (also cleans up iptables rules via stop_client)
    if [ -x "$INIT_DIR/S99trusttunnel" ]; then
        info "Stopping trusttunnel client..."
        "$INIT_DIR/S99trusttunnel" stop 2>/dev/null || warn "Client was not running"
    fi

    if [ -x "$INIT_DIR/S98trusttunnel-manager" ]; then
        info "Stopping trusttunnel manager..."
        "$INIT_DIR/S98trusttunnel-manager" stop 2>/dev/null || warn "Manager was not running"
    fi

    # 2. Remove NDMS interfaces (Proxy/OpkgTun) to restore ISP connection display
    remove_ndm_interfaces

    # 3. Safety net: clean up any remaining iptables MSS clamp rules
    info "Cleaning up firewall rules..."
    for idx in 0 1 2 3; do
        iptables -t mangle -D FORWARD -o "tun${idx}" -p tcp --tcp-flags SYN,RST SYN \
            -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || true
        iptables -t mangle -D FORWARD -i "tun${idx}" -p tcp --tcp-flags SYN,RST SYN \
            -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || true
    done
    nft delete chain ip trusttunnel forward_mss 2>/dev/null || true
    nft delete table ip trusttunnel 2>/dev/null || true

    # 4. Remove init scripts
    info "Removing init scripts..."
    rm -f "$INIT_DIR/S99trusttunnel"
    rm -f "$INIT_DIR/S98trusttunnel-manager"

    # 5. Remove NDM hooks
    info "Removing NDM hooks..."
    rm -f "$HOOKS_DIR/wan.d/010-trusttunnel.sh"
    rm -f "$HOOKS_DIR/iflayerchanged.d/trusttunnel.sh"
    rm -f "$HOOKS_DIR/netfilter.d/trusttunnel.sh"
    rm -f "$HOOKS_DIR/schedule.d/trusttunnel.sh"
    rm -f "$HOOKS_DIR/button.d/trusttunnel.sh"

    # 6. Remove main directory (binaries, scripts, configs)
    if [ -d "$INSTALL_DIR" ]; then
        info "Removing $INSTALL_DIR..."
        rm -rf "$INSTALL_DIR"
    fi

    echo ""
    info "TrustTunnel has been completely removed."
    info "If the ISP connection is not visible, reboot the router."
}

main "$@"
