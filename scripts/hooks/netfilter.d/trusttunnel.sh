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

log_msg() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [netfilter.d] $*" >> "$LOG_FILE"
}

[ ! -f "$MODE_CONF" ] && exit 0

. "$MODE_CONF"

if [ "$TT_MODE" = "tun" ]; then
    TUN_IF="tun${TUN_IDX:-0}"

    if ip link show "$TUN_IF" > /dev/null 2>&1; then
        log_msg "Restoring iptables rules for $TUN_IF (table=$table)"

        iptables -t nat -A POSTROUTING -o "$TUN_IF" -j MASQUERADE 2>/dev/null
        iptables -A FORWARD -i "$TUN_IF" -m state --state RELATED,ESTABLISHED -j ACCEPT 2>/dev/null
        iptables -A FORWARD -o "$TUN_IF" -j ACCEPT 2>/dev/null

        # Restore smart routing mangle rules
        SMART_ROUTING_SH="$TT_DIR/smart-routing.sh"
        if [ "$SR_ENABLED" = "yes" ] && [ -f "$SMART_ROUTING_SH" ]; then
            . "$SMART_ROUTING_SH"
            sr_restore_iptables
        fi
    fi
fi

exit 0
