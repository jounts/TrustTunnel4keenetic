#!/bin/sh

# NDM WAN hook for TrustTunnel
# Environment variables from NDM:
#   $interface - WAN interface name (e.g., "ISP")
#   $address   - IP address assigned
#   $mask      - subnet mask
#   $gateway   - default gateway
#   $dns1, $dns2, $dns3 - DNS servers
#   $domain    - domain name
# Called on: WAN IP address change (up/down)

TT_DIR="/opt/trusttunnel_client"
TT_INIT="/opt/etc/init.d/S99trusttunnel"
LOG_FILE="/opt/var/log/trusttunnel.log"
PID_FILE="/opt/var/run/trusttunnel.pid"

log_msg() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [wan.d] $*" >> "$LOG_FILE"
}

[ ! -f "$TT_INIT" ] && exit 0

if [ -n "$address" ] && [ "$address" != "0.0.0.0" ]; then
    log_msg "WAN up: interface=$interface address=$address gateway=$gateway"

    if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE" 2>/dev/null)" 2>/dev/null; then
        log_msg "TrustTunnel already running, checking health after WAN change"
        sleep 5
        "$TT_INIT" check
    else
        log_msg "Starting TrustTunnel after WAN up"
        "$TT_INIT" start
    fi
else
    log_msg "WAN down: interface=$interface"
fi

exit 0
