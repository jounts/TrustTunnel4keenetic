#!/bin/sh

# NDM netfilter hook for TrustTunnel
# Called when iptables/netfilter rules are rebuilt by NDM.
# Use to restore custom firewall rules.
# Environment variables from NDM:
#   $type   - event type
#   $table  - iptables table name (filter, nat, mangle)

TT_DIR="/opt/trusttunnel_client"
MODE_CONF="$TT_DIR/mode.conf"
LOG_FILE="/opt/var/log/trusttunnel.log"
COMPAT_SH="$TT_DIR/ndms-compat.sh"

log_msg() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [netfilter.d] $*" >> "$LOG_FILE"
}

[ ! -f "$MODE_CONF" ] && exit 0

. "$MODE_CONF"

# Load NDMS compatibility layer
if [ -f "$COMPAT_SH" ]; then
    . "$COMPAT_SH"
fi

if [ "$TT_MODE" = "tun" ] || [ "$TT_MODE" = "socks5-tun" ]; then
    TUN_IF="tun${TUN_IDX:-0}"

    if ip link show "$TUN_IF" > /dev/null 2>&1; then
        log_msg "Restoring firewall rules for $TUN_IF (table=$table, backend=${NDMS_FW_BACKEND:-iptables})"

        fw_add_nat_masquerade "$TUN_IF"
        fw_add_forward_accept "$TUN_IF"

        # Restore MSS clamping rules (PPPoE compatibility)
        if type fw_setup_mss_clamp > /dev/null 2>&1; then
            fw_setup_mss_clamp "$TUN_IF"
            log_msg "MSS clamping restored on $TUN_IF"
        fi

        # Restore smart routing mangle rules
        SMART_ROUTING_SH="$TT_DIR/smart-routing.sh"
        if [ "$SR_ENABLED" = "yes" ] && [ -f "$SMART_ROUTING_SH" ]; then
            . "$SMART_ROUTING_SH"
            sr_restore_iptables
        fi
    fi
fi

exit 0
