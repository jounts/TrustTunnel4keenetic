#!/bin/sh

# NDM schedule hook for TrustTunnel
# Allows start/stop via NDMS scheduler.
# Environment variables from NDM:
#   $schedule - schedule name

TT_INIT="/opt/etc/init.d/S99trusttunnel"
LOG_FILE="/opt/var/log/trusttunnel.log"

log_msg() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [schedule.d] $*" >> "$LOG_FILE"
}

[ ! -f "$TT_INIT" ] && exit 0

case "$schedule" in
    TrustTunnel_Start|trusttunnel_start)
        log_msg "Scheduled start"
        "$TT_INIT" start
        ;;
    TrustTunnel_Stop|trusttunnel_stop)
        log_msg "Scheduled stop"
        "$TT_INIT" stop
        ;;
    TrustTunnel_Restart|trusttunnel_restart)
        log_msg "Scheduled restart"
        "$TT_INIT" restart
        ;;
esac

exit 0
