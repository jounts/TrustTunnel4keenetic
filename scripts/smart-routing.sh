#!/bin/sh

# TrustTunnel Smart Routing — GeoIP-based routing for Keenetic (NDMS 4 & 5)
# Requires: ipset (or nft sets), dnsmasq-full, ip (iproute2)
#
# This script is sourced by S99trusttunnel and provides functions for:
#   - Managing ipsets (tt_domestic, tt_tunnel)
#   - Running a custom dnsmasq instance for DNS-based routing
#   - Setting up policy routing to direct domestic traffic directly
#   - Mangle/iptables rules (or nftables via ndms-compat.sh)

TT_DIR="/opt/trusttunnel_client"
SR_DIR="$TT_DIR/routing"
SR_DOMAINS_FILE="$SR_DIR/domains.txt"
SR_NETS_FILE="$SR_DIR/domestic_nets.txt"
SR_DNSMASQ_CONF="$SR_DIR/dnsmasq-sr.conf"
SR_DNSMASQ_PID="/opt/var/run/dnsmasq-sr.pid"
SR_DNSMASQ_RESOLVED="/opt/var/run/dnsmasq-sr-resolved.conf"
SR_ORIG_GW_FILE="/opt/var/run/tt_orig_gateway"
LOG_FILE="/opt/var/log/trusttunnel.log"

SR_IPSET_DOMESTIC="tt_domestic"
SR_IPSET_TUNNEL="tt_tunnel"
SR_FWMARK="0x100"
SR_TABLE=100

# Default values (overridden by mode.conf)
SR_HOME_COUNTRY="${SR_HOME_COUNTRY:-RU}"
SR_DNS_PORT="${SR_DNS_PORT:-5354}"
SR_DNS_UPSTREAM="${SR_DNS_UPSTREAM:-1.1.1.1}"

# Load NDMS compat layer
COMPAT_SH="$TT_DIR/ndms-compat.sh"
if [ -f "$COMPAT_SH" ]; then
    . "$COMPAT_SH"
fi

sr_log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [smart-routing] $*" >> "$LOG_FILE"
}

sr_check_deps() {
    local missing=""

    if [ "$NDMS_FW_BACKEND" = "nftables" ] && ! command -v ipset > /dev/null 2>&1; then
        # Pure nftables, ipset not required
        if ! command -v nft > /dev/null 2>&1; then
            missing="$missing nft"
        fi
    else
        if ! command -v ipset > /dev/null 2>&1; then
            missing="$missing ipset"
        fi
    fi

    if ! command -v dnsmasq > /dev/null 2>&1; then
        missing="$missing dnsmasq-full"
    fi

    if [ -n "$missing" ]; then
        sr_log "ERROR: Missing dependencies:$missing. Install via: opkg install$missing"
        return 1
    fi
    return 0
}

sr_save_orig_gateway() {
    local gw
    gw=$(ip route show default 2>/dev/null | grep -v "tun" | head -n1 | awk '{print $3}')
    local dev
    dev=$(ip route show default 2>/dev/null | grep -v "tun" | head -n1 | awk '{print $5}')
    if [ -n "$gw" ] && [ -n "$dev" ]; then
        echo "GW=$gw" > "$SR_ORIG_GW_FILE"
        echo "DEV=$dev" >> "$SR_ORIG_GW_FILE"
        sr_log "Saved original gateway: $gw via $dev"
    else
        sr_log "WARNING: Could not detect original gateway"
    fi
}

sr_create_ipsets() {
    fw_create_set "$SR_IPSET_DOMESTIC" "hash:net" 65536
    fw_create_set "$SR_IPSET_TUNNEL" "hash:ip" 4096
    sr_log "Ipsets created (backend: ${NDMS_FW_BACKEND:-iptables})"
}

sr_load_domestic_nets() {
    if [ ! -f "$SR_NETS_FILE" ]; then
        sr_log "No domestic nets file found at $SR_NETS_FILE"
        return 1
    fi
    fw_flush_set "$SR_IPSET_DOMESTIC"
    fw_restore_set "$SR_IPSET_DOMESTIC" "$SR_NETS_FILE"
    local count
    count=$(fw_set_count "$SR_IPSET_DOMESTIC")
    sr_log "Loaded $count domestic CIDRs for $SR_HOME_COUNTRY"
}

sr_update_nets() {
    local country
    country=$(echo "$SR_HOME_COUNTRY" | tr '[:upper:]' '[:lower:]')
    local url="https://raw.githubusercontent.com/herrbischoff/country-ip-blocks/master/ipv4/${country}.cidr"

    sr_log "Downloading CIDR list for $SR_HOME_COUNTRY from $url"

    local tmpfile="/tmp/tt_domestic_nets.tmp"
    if curl -fsSL "$url" -o "$tmpfile" 2>/dev/null; then
        local count
        count=$(wc -l < "$tmpfile")
        if [ "$count" -gt 10 ]; then
            mv "$tmpfile" "$SR_NETS_FILE"
            echo "$(date +%s)" > "$SR_DIR/nets_updated_ts"
            sr_log "Downloaded $count CIDRs for $SR_HOME_COUNTRY"
            return 0
        else
            sr_log "ERROR: Downloaded CIDR list too small ($count lines)"
            rm -f "$tmpfile"
            return 1
        fi
    else
        sr_log "ERROR: Failed to download CIDR list"
        rm -f "$tmpfile"
        return 1
    fi
}

sr_generate_dnsmasq_conf() {
    mkdir -p "$SR_DIR" /opt/etc/dnsmasq.d

    cat > "$SR_DNSMASQ_CONF" << EOF
port=$SR_DNS_PORT
no-resolv
no-hosts
server=$SR_DNS_UPSTREAM
cache-size=1500
min-cache-ttl=300
log-facility=$LOG_FILE
EOF

    # Add ipset/nftset directives for domain list
    if [ -f "$SR_DOMAINS_FILE" ]; then
        > "$SR_DNSMASQ_RESOLVED"
        while IFS= read -r domain; do
            domain=$(echo "$domain" | tr -d '[:space:]')
            [ -z "$domain" ] && continue
            echo "$domain" | grep -q '^#' && continue

            if [ "$NDMS_FW_BACKEND" = "nftables" ] && ! command -v ipset > /dev/null 2>&1; then
                echo "nftset=/$domain/4#ip#trusttunnel#$SR_IPSET_TUNNEL" >> "$SR_DNSMASQ_RESOLVED"
            else
                echo "ipset=/$domain/$SR_IPSET_TUNNEL" >> "$SR_DNSMASQ_RESOLVED"
            fi
        done < "$SR_DOMAINS_FILE"
        echo "conf-file=$SR_DNSMASQ_RESOLVED" >> "$SR_DNSMASQ_CONF"
    fi

    sr_log "Dnsmasq config generated (backend: ${NDMS_FW_BACKEND:-iptables})"
}

sr_start_dnsmasq() {
    sr_stop_dnsmasq
    sr_generate_dnsmasq_conf

    dnsmasq --conf-file="$SR_DNSMASQ_CONF" --pid-file="$SR_DNSMASQ_PID" 2>>"$LOG_FILE"
    if [ $? -eq 0 ]; then
        sr_log "Dnsmasq started on port $SR_DNS_PORT (pid: $(cat "$SR_DNSMASQ_PID" 2>/dev/null))"
    else
        sr_log "ERROR: Failed to start dnsmasq"
        return 1
    fi
}

sr_stop_dnsmasq() {
    if [ -f "$SR_DNSMASQ_PID" ]; then
        local pid
        pid=$(cat "$SR_DNSMASQ_PID" 2>/dev/null)
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null
            sr_log "Dnsmasq stopped (pid: $pid)"
        fi
        rm -f "$SR_DNSMASQ_PID"
    fi
}

sr_setup_routing() {
    if [ ! -f "$SR_ORIG_GW_FILE" ]; then
        sr_log "ERROR: No original gateway saved, cannot setup policy routing"
        return 1
    fi
    . "$SR_ORIG_GW_FILE"

    # Check for conflicts with existing policy routing rules
    if ip rule show 2>/dev/null | grep -q "fwmark $SR_FWMARK"; then
        sr_log "WARNING: fwmark $SR_FWMARK already in use by another rule, possible VPN conflict"
    fi
    if ip route show table "$SR_TABLE" 2>/dev/null | grep -q "default"; then
        sr_log "WARNING: routing table $SR_TABLE already has a default route, possible VPN conflict"
    fi

    ip route replace default via "$GW" dev "$DEV" table "$SR_TABLE" 2>/dev/null
    ip rule add fwmark "$SR_FWMARK" table "$SR_TABLE" priority 100 2>/dev/null

    sr_log "Policy routing configured: mark $SR_FWMARK -> table $SR_TABLE (gw=$GW dev=$DEV)"
}

sr_cleanup_routing() {
    ip rule del fwmark "$SR_FWMARK" table "$SR_TABLE" 2>/dev/null
    ip route flush table "$SR_TABLE" 2>/dev/null
    sr_log "Policy routing cleaned up"
}

sr_setup_iptables() {
    fw_setup_mangle_smart "$SR_IPSET_TUNNEL" "$SR_IPSET_DOMESTIC" "$SR_FWMARK"
    sr_log "Firewall mangle rules configured (backend: ${NDMS_FW_BACKEND:-iptables})"
}

sr_cleanup_iptables() {
    fw_cleanup_mangle_smart
    sr_log "Firewall mangle rules cleaned up"
}

sr_restore_iptables() {
    sr_log "Restoring smart routing firewall rules (backend: ${NDMS_FW_BACKEND:-iptables})"
    fw_setup_mangle_smart "$SR_IPSET_TUNNEL" "$SR_IPSET_DOMESTIC" "$SR_FWMARK"
}

sr_destroy_ipsets() {
    fw_destroy_set "$SR_IPSET_DOMESTIC"
    fw_destroy_set "$SR_IPSET_TUNNEL"
    sr_log "Ipsets destroyed"
}

sr_reload_dnsmasq() {
    sr_generate_dnsmasq_conf
    if [ -f "$SR_DNSMASQ_PID" ]; then
        local pid
        pid=$(cat "$SR_DNSMASQ_PID" 2>/dev/null)
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            kill -HUP "$pid" 2>/dev/null
            sr_log "Dnsmasq reloaded (pid: $pid)"
            return 0
        fi
    fi
    sr_start_dnsmasq
}

sr_reload_nets() {
    sr_load_domestic_nets
}

sr_start() {
    sr_log "Starting smart routing (NDMS ${NDMS_MAJOR:-?}, FW backend: ${NDMS_FW_BACKEND:-iptables})"

    if ! sr_check_deps; then
        return 1
    fi

    mkdir -p "$SR_DIR"

    # Create default domains file if missing
    if [ ! -f "$SR_DOMAINS_FILE" ]; then
        cat > "$SR_DOMAINS_FILE" << 'EOF'
# Domains to route through tunnel (one per line)
# IPs resolved from these domains go to tt_tunnel ipset,
# overriding the domestic CIDR list.
# Example:
# netflix.com
# youtube.com
EOF
    fi

    sr_create_ipsets

    # Download domestic nets if absent or older than 7 days
    if [ ! -f "$SR_NETS_FILE" ]; then
        sr_update_nets
    else
        local ts_file="$SR_DIR/nets_updated_ts"
        if [ -f "$ts_file" ]; then
            local ts
            ts=$(cat "$ts_file")
            local now
            now=$(date +%s)
            local age=$((now - ts))
            if [ "$age" -gt 604800 ]; then
                sr_update_nets
            fi
        fi
    fi

    sr_load_domestic_nets
    sr_start_dnsmasq
    sr_setup_routing
    sr_setup_iptables

    sr_log "Smart routing started successfully"
}

sr_stop() {
    sr_log "Stopping smart routing"

    sr_cleanup_iptables
    sr_cleanup_routing
    sr_stop_dnsmasq
    sr_destroy_ipsets

    sr_log "Smart routing stopped"
}
