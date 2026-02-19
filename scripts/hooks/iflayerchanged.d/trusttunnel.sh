#!/bin/sh

# NDM interface layer change hook for TrustTunnel
# Environment variables from NDM:
#   $id    - interface name (e.g., "Proxy0", "OpkgTun0")
#   $layer - layer that changed (e.g., "link", "ip")
#   $level - new state (e.g., "up", "down")

TT_DIR="/opt/trusttunnel_client"
TT_INIT="/opt/etc/init.d/S99trusttunnel"
LOG_FILE="/opt/var/log/trusttunnel.log"

log_msg() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [iflayerchanged.d] $*" >> "$LOG_FILE"
}

[ ! -f "$TT_INIT" ] && exit 0

case "$id" in
    Proxy*|OpkgTun*)
        log_msg "Interface $id: layer=$layer level=$level"

        if [ "$level" = "down" ]; then
            log_msg "TrustTunnel interface $id went down, scheduling restart"
            sleep 3
            "$TT_INIT" restart &
        fi
        ;;
esac

exit 0
