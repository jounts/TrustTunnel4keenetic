#!/bin/sh

# NDM button hook for TrustTunnel
# Toggle TrustTunnel via FN button press.
# Environment variables from NDM:
#   $button  - button name (e.g., "FN1", "FN2")
#   $action  - action (e.g., "short_press", "long_press")

TT_INIT="/opt/etc/init.d/S99trusttunnel"
PID_FILE="/opt/var/run/trusttunnel.pid"
LOG_FILE="/opt/var/log/trusttunnel.log"

# Configure which button triggers the toggle
TT_BUTTON="${TT_BUTTON:-FN1}"
TT_ACTION="${TT_ACTION:-short_press}"

log_msg() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [button.d] $*" >> "$LOG_FILE"
}

[ ! -f "$TT_INIT" ] && exit 0

if [ "$button" = "$TT_BUTTON" ] && [ "$action" = "$TT_ACTION" ]; then
    if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE" 2>/dev/null)" 2>/dev/null; then
        log_msg "Button $button: stopping TrustTunnel"
        "$TT_INIT" stop
    else
        log_msg "Button $button: starting TrustTunnel"
        "$TT_INIT" start
    fi
fi

exit 0
