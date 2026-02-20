#!/bin/sh

# NDMS compatibility layer for TrustTunnel
# Detects NDMS version, firewall backend (iptables vs nftables),
# and kernel interface naming conventions.
# Source this file from other scripts: . /opt/trusttunnel_client/ndms-compat.sh

NDMS_VERSION_FILE="/tmp/ndm/version"
NDMS_COMPAT_CACHE="/opt/var/run/tt_ndms_compat"

# Send a CLI command to NDM via HTTP RCI API (POST to localhost:79).
# Falls back to ndmc binary if curl/RCI is unavailable.
# Usage: ndm_cmd "interface Proxy0" "interface Proxy0 proxy protocol socks5" ...
ndm_cmd() {
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

    # Fallback: run each command via ndmc
    for cmd in "$@"; do
        ndmc -c "$cmd" 2>/dev/null || true
    done
}

# Detect NDMS major version (3, 4 or 5)
ndms_detect_version() {
    local ver
    ver=$(cat "$NDMS_VERSION_FILE" 2>/dev/null | head -n1)

    # Fallback: RCI HTTP API
    if [ -z "$ver" ] && command -v curl > /dev/null 2>&1; then
        ver=$(curl -s "http://localhost:79/rci/show/version" 2>/dev/null | \
            sed -n 's/.*"release"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
    fi

    # Last resort: ndmc CLI
    if [ -z "$ver" ] && command -v ndmc > /dev/null 2>&1; then
        ver=$(ndmc -c "show version" 2>/dev/null | grep "^\s*release:" | head -n1 | sed 's/.*:\s*//')
    fi

    case "$ver" in
        5.*) echo "5" ;;
        4.*) echo "4" ;;
        3.*) echo "3" ;;
        *)   echo "4" ;; # conservative default
    esac
}

# Detect firewall backend: "iptables" or "nftables"
ndms_detect_fw_backend() {
    if command -v iptables > /dev/null 2>&1; then
        # Check if iptables is the nft-based wrapper
        if iptables --version 2>/dev/null | grep -q "nf_tables"; then
            echo "nftables"
        else
            echo "iptables"
        fi
    elif command -v nft > /dev/null 2>&1; then
        echo "nftables"
    else
        echo "iptables" # fallback
    fi
}

# Detect the kernel interface name prefix that NDM creates for OpkgTun interfaces.
# NDMS 4 uses "nwg{N}", NDMS 5 may use different naming.
ndms_detect_tun_kernel_name() {
    local idx="${1:-0}"
    local ndm_name="OpkgTun${idx}"

    # Try common kernel name patterns
    for prefix in "nwg" "tun" "wg" "utun"; do
        if ip link show "${prefix}${idx}" > /dev/null 2>&1; then
            echo "${prefix}${idx}"
            return 0
        fi
    done

    # If interface already renamed to our target name
    if ip link show "tun${idx}" > /dev/null 2>&1; then
        echo "tun${idx}"
        return 0
    fi

    # Default: NDMS 4 convention
    echo "nwg${idx}"
    return 1
}

# Initialize and cache compat info
ndms_init_compat() {
    NDMS_MAJOR=$(ndms_detect_version)
    NDMS_FW_BACKEND=$(ndms_detect_fw_backend)

    # Write cache for other scripts to use quickly
    cat > "$NDMS_COMPAT_CACHE" << EOF
NDMS_MAJOR="$NDMS_MAJOR"
NDMS_FW_BACKEND="$NDMS_FW_BACKEND"
EOF
}

# Load cached compat info or detect fresh
ndms_load_compat() {
    if [ -f "$NDMS_COMPAT_CACHE" ]; then
        . "$NDMS_COMPAT_CACHE"
    else
        ndms_init_compat
    fi
}

# --- Firewall abstraction layer ---

# Add iptables/nftables rule. Uses iptables syntax; wraps to nft if needed.
# For the iptables-nft wrapper case, iptables commands work as-is.
# For pure nftables, we provide nft equivalents for common operations.
fw_add_nat_masquerade() {
    local iface="$1"
    if [ "$NDMS_FW_BACKEND" = "nftables" ] && ! command -v iptables > /dev/null 2>&1; then
        nft add rule ip nat POSTROUTING oifname "$iface" counter masquerade 2>/dev/null
    else
        iptables -t nat -A POSTROUTING -o "$iface" -j MASQUERADE 2>/dev/null
    fi
}

fw_add_forward_accept() {
    local iface="$1"
    if [ "$NDMS_FW_BACKEND" = "nftables" ] && ! command -v iptables > /dev/null 2>&1; then
        nft add rule ip filter FORWARD iifname "$iface" ct state related,established counter accept 2>/dev/null
        nft add rule ip filter FORWARD oifname "$iface" counter accept 2>/dev/null
    else
        iptables -A FORWARD -i "$iface" -m state --state RELATED,ESTABLISHED -j ACCEPT 2>/dev/null
        iptables -A FORWARD -o "$iface" -j ACCEPT 2>/dev/null
    fi
}

# Create ipset or nft set
fw_create_set() {
    local name="$1"
    local type="${2:-hash:net}"
    local maxelem="${3:-65536}"

    if [ "$NDMS_FW_BACKEND" = "nftables" ] && ! command -v ipset > /dev/null 2>&1; then
        nft add table ip trusttunnel 2>/dev/null
        nft add set ip trusttunnel "$name" "{ type ipv4_addr; flags interval; auto-merge; }" 2>/dev/null
    else
        ipset create "$name" "$type" hashsize 16384 maxelem "$maxelem" -exist 2>/dev/null
    fi
}

fw_flush_set() {
    local name="$1"
    if [ "$NDMS_FW_BACKEND" = "nftables" ] && ! command -v ipset > /dev/null 2>&1; then
        nft flush set ip trusttunnel "$name" 2>/dev/null
    else
        ipset flush "$name" 2>/dev/null
    fi
}

fw_destroy_set() {
    local name="$1"
    if [ "$NDMS_FW_BACKEND" = "nftables" ] && ! command -v ipset > /dev/null 2>&1; then
        nft delete set ip trusttunnel "$name" 2>/dev/null
    else
        ipset destroy "$name" 2>/dev/null
    fi
}

fw_set_count() {
    local name="$1"
    if [ "$NDMS_FW_BACKEND" = "nftables" ] && ! command -v ipset > /dev/null 2>&1; then
        nft list set ip trusttunnel "$name" 2>/dev/null | grep -c "/"
    else
        ipset list "$name" -t 2>/dev/null | grep "Number of entries" | awk '{print $NF}'
    fi
}

# Restore ipset entries from file (CIDR list)
fw_restore_set() {
    local name="$1"
    local file="$2"

    if [ "$NDMS_FW_BACKEND" = "nftables" ] && ! command -v ipset > /dev/null 2>&1; then
        local elements=""
        while IFS= read -r cidr; do
            cidr=$(echo "$cidr" | tr -d '[:space:]')
            [ -z "$cidr" ] && continue
            [ -n "$elements" ] && elements="${elements}, "
            elements="${elements}${cidr}"
        done < "$file"
        if [ -n "$elements" ]; then
            nft add element ip trusttunnel "$name" "{ $elements }" 2>/dev/null
        fi
    else
        local tmpfile="/tmp/tt_ipset_restore.txt"
        awk -v n="$name" '{print "add " n " " $0}' "$file" > "$tmpfile"
        ipset restore -! < "$tmpfile" 2>/dev/null
        rm -f "$tmpfile"
    fi
}

# --- Mangle / policy routing abstraction ---

fw_setup_mangle_smart() {
    local ipset_tunnel="$1"
    local ipset_domestic="$2"
    local fwmark="$3"

    if [ "$NDMS_FW_BACKEND" = "nftables" ] && ! command -v iptables > /dev/null 2>&1; then
        nft add table ip trusttunnel 2>/dev/null
        nft add chain ip trusttunnel prerouting "{ type filter hook prerouting priority mangle; policy accept; }" 2>/dev/null
        nft add chain ip trusttunnel tt_smart 2>/dev/null

        # Packets to tunnel set -> return (go through tunnel via default route)
        nft add rule ip trusttunnel tt_smart ip daddr @"$ipset_tunnel" return 2>/dev/null
        # Packets to domestic set -> mark for direct routing
        nft add rule ip trusttunnel tt_smart ip daddr @"$ipset_domestic" mark set "$fwmark" 2>/dev/null
        nft add rule ip trusttunnel tt_smart ip daddr @"$ipset_domestic" ct mark set mark 2>/dev/null
        # Jump to our chain from prerouting
        nft add rule ip trusttunnel prerouting jump tt_smart 2>/dev/null
    else
        iptables -t mangle -N TT_DIRECT 2>/dev/null
        iptables -t mangle -F TT_DIRECT 2>/dev/null
        iptables -t mangle -A TT_DIRECT -j MARK --set-mark "$fwmark"
        iptables -t mangle -A TT_DIRECT -j CONNMARK --save-mark --nfmask 0xffffffff --ctmask 0xffffffff

        iptables -t mangle -N TT_SMART 2>/dev/null
        iptables -t mangle -F TT_SMART 2>/dev/null
        iptables -t mangle -A TT_SMART -m set --match-set "$ipset_tunnel" dst -j RETURN
        iptables -t mangle -A TT_SMART -m set --match-set "$ipset_domestic" dst -j TT_DIRECT

        iptables -t mangle -C PREROUTING -j TT_SMART 2>/dev/null || \
            iptables -t mangle -A PREROUTING -j TT_SMART
    fi
}

fw_cleanup_mangle_smart() {
    if [ "$NDMS_FW_BACKEND" = "nftables" ] && ! command -v iptables > /dev/null 2>&1; then
        nft delete chain ip trusttunnel tt_smart 2>/dev/null
        nft delete chain ip trusttunnel prerouting 2>/dev/null
    else
        iptables -t mangle -D PREROUTING -j TT_SMART 2>/dev/null
        iptables -t mangle -F TT_SMART 2>/dev/null
        iptables -t mangle -X TT_SMART 2>/dev/null
        iptables -t mangle -F TT_DIRECT 2>/dev/null
        iptables -t mangle -X TT_DIRECT 2>/dev/null
    fi
}

# --- MSS clamping (PPPoE compatibility) ---

fw_setup_mss_clamp() {
    local iface="$1"
    if [ "$NDMS_FW_BACKEND" = "nftables" ] && ! command -v iptables > /dev/null 2>&1; then
        nft add table ip trusttunnel 2>/dev/null
        nft add chain ip trusttunnel forward_mss \
            "{ type filter hook forward priority mangle; policy accept; }" 2>/dev/null
        nft add rule ip trusttunnel forward_mss \
            oifname "$iface" tcp flags syn tcp option maxseg size set rt mtu 2>/dev/null
        nft add rule ip trusttunnel forward_mss \
            iifname "$iface" tcp flags syn tcp option maxseg size set rt mtu 2>/dev/null
    else
        iptables -t mangle -C FORWARD -o "$iface" -p tcp --tcp-flags SYN,RST SYN \
            -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || \
        iptables -t mangle -A FORWARD -o "$iface" -p tcp --tcp-flags SYN,RST SYN \
            -j TCPMSS --clamp-mss-to-pmtu
        iptables -t mangle -C FORWARD -i "$iface" -p tcp --tcp-flags SYN,RST SYN \
            -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || \
        iptables -t mangle -A FORWARD -i "$iface" -p tcp --tcp-flags SYN,RST SYN \
            -j TCPMSS --clamp-mss-to-pmtu
    fi
}

fw_cleanup_mss_clamp() {
    local iface="$1"
    if [ "$NDMS_FW_BACKEND" = "nftables" ] && ! command -v iptables > /dev/null 2>&1; then
        nft delete chain ip trusttunnel forward_mss 2>/dev/null
    else
        iptables -t mangle -D FORWARD -o "$iface" -p tcp --tcp-flags SYN,RST SYN \
            -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null
        iptables -t mangle -D FORWARD -i "$iface" -p tcp --tcp-flags SYN,RST SYN \
            -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null
    fi
}

# Initialize compat on source
ndms_load_compat
