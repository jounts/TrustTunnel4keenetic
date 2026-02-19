#!/bin/sh

# Smart routing functions for TrustTunnel
# Sourced by S99trusttunnel and netfilter.d hook
# Provides GeoIP-based split tunneling: domestic IPs bypass tunnel, foreign IPs go through tunnel.
# DNS-based override via dnsmasq allows forcing specific domains through tunnel (CDN workaround).

SR_DIR="/opt/trusttunnel_client/routing"
SR_DOMAINS_FILE="$SR_DIR/domains.txt"
SR_NETS_FILE="$SR_DIR/domestic_nets.txt"
SR_DNSMASQ_CONF="/opt/etc/dnsmasq.d/trusttunnel.conf"
SR_DNSMASQ_PID="/opt/var/run/tt_dnsmasq.pid"
SR_ORIG_GW_FILE="/opt/var/run/tt_orig_gw"
SR_ORIG_IF_FILE="/opt/var/run/tt_orig_if"

SR_IPSET_DOMESTIC="tt_domestic"
SR_IPSET_TUNNEL="tt_tunnel"
SR_FWMARK="0x100"
SR_TABLE="100"
SR_CHAIN_SMART="TT_SMART"
SR_CHAIN_DIRECT="TT_DIRECT"

CIDR_BASE_URL="https://raw.githubusercontent.com/herrbischoff/country-ip-blocks/master/ipv4"

sr_log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [smart-routing] $*" >> "$LOG_FILE"
}

sr_check_deps() {
    local missing=""
    command -v ipset > /dev/null 2>&1 || missing="$missing ipset"
    command -v dnsmasq > /dev/null 2>&1 || missing="$missing dnsmasq-full"

    if [ -n "$missing" ]; then
        sr_log "Missing dependencies:$missing"
        if command -v opkg > /dev/null 2>&1; then
            sr_log "Attempting to install:$missing"
            opkg update > /dev/null 2>&1
            for pkg in $missing; do
                opkg install "$pkg" > /dev/null 2>&1
            done
        fi
    fi

    command -v ipset > /dev/null 2>&1 && command -v dnsmasq > /dev/null 2>&1
}

sr_save_orig_gateway() {
    local gw_line
    gw_line=$(ip route show default 2>/dev/null | head -n1)
    if [ -n "$gw_line" ]; then
        echo "$gw_line" | awk '{print $3}' > "$SR_ORIG_GW_FILE"
        echo "$gw_line" | awk '{print $5}' > "$SR_ORIG_IF_FILE"
        sr_log "Saved original gateway: $(cat "$SR_ORIG_GW_FILE") via $(cat "$SR_ORIG_IF_FILE")"
    else
        sr_log "WARNING: Could not detect original default gateway"
    fi
}

sr_create_ipsets() {
    ipset create "$SR_IPSET_DOMESTIC" hash:net hashsize 16384 maxelem 200000 -exist 2>/dev/null
    ipset create "$SR_IPSET_TUNNEL" hash:net hashsize 4096 maxelem 65536 -exist 2>/dev/null
    sr_log "ipsets created"
}

sr_load_domestic_nets() {
    if [ ! -f "$SR_NETS_FILE" ]; then
        sr_log "No domestic nets file, attempting download"
        sr_update_nets
    fi

    if [ -f "$SR_NETS_FILE" ]; then
        ipset flush "$SR_IPSET_DOMESTIC" 2>/dev/null

        local tmpfile="/tmp/tt_ipset_restore.txt"
        awk -v name="$SR_IPSET_DOMESTIC" '{print "add " name " " $0}' "$SR_NETS_FILE" > "$tmpfile"
        ipset restore -! < "$tmpfile" 2>/dev/null
        rm -f "$tmpfile"

        local count
        count=$(ipset list "$SR_IPSET_DOMESTIC" -t 2>/dev/null | grep "Number of entries" | awk '{print $NF}')
        sr_log "Loaded $count domestic CIDRs into $SR_IPSET_DOMESTIC"
    else
        sr_log "WARNING: No domestic nets available"
    fi
}

sr_update_nets() {
    local country
    country=$(echo "${SR_HOME_COUNTRY:-RU}" | tr 'A-Z' 'a-z')
    local url="${CIDR_BASE_URL}/${country}.cidr"

    sr_log "Downloading domestic CIDRs for $country from $url"
    mkdir -p "$SR_DIR"

    if curl -fsSL "$url" -o "${SR_NETS_FILE}.tmp" 2>/dev/null; then
        mv "${SR_NETS_FILE}.tmp" "$SR_NETS_FILE"
        local lines
        lines=$(wc -l < "$SR_NETS_FILE")
        sr_log "Downloaded $lines CIDRs for $country"
    else
        rm -f "${SR_NETS_FILE}.tmp"
        sr_log "ERROR: Failed to download CIDRs for $country"
        return 1
    fi
}

sr_generate_dnsmasq_conf() {
    mkdir -p "$(dirname "$SR_DNSMASQ_CONF")"
    local tun_if="tun${TUN_IDX:-0}"

    cat > "$SR_DNSMASQ_CONF" << EOF
port=${SR_DNS_PORT:-5354}
no-resolv
server=${SR_DNS_UPSTREAM:-1.1.1.1}
user=root
group=root
pid-file=${SR_DNSMASQ_PID}
log-facility=${LOG_FILE}
EOF

    if [ -f "$SR_DOMAINS_FILE" ]; then
        while IFS= read -r domain; do
            domain=$(echo "$domain" | sed 's/#.*//' | tr -d '[:space:]')
            [ -z "$domain" ] && continue
            echo "ipset=/${domain}/${SR_IPSET_TUNNEL}" >> "$SR_DNSMASQ_CONF"
        done < "$SR_DOMAINS_FILE"
    fi

    sr_log "dnsmasq config generated at $SR_DNSMASQ_CONF"
}

sr_start_dnsmasq() {
    sr_stop_dnsmasq

    if [ ! -f "$SR_DNSMASQ_CONF" ]; then
        sr_generate_dnsmasq_conf
    fi

    dnsmasq -C "$SR_DNSMASQ_CONF" 2>> "$LOG_FILE"
    if [ $? -eq 0 ]; then
        sr_log "dnsmasq started on port ${SR_DNS_PORT:-5354}"
    else
        sr_log "ERROR: dnsmasq failed to start"
        return 1
    fi
}

sr_stop_dnsmasq() {
    if [ -f "$SR_DNSMASQ_PID" ]; then
        local pid
        pid=$(cat "$SR_DNSMASQ_PID")
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null
            sr_log "dnsmasq stopped (PID $pid)"
        fi
        rm -f "$SR_DNSMASQ_PID"
    fi
}

sr_setup_routing() {
    local orig_gw orig_if
    [ -f "$SR_ORIG_GW_FILE" ] && orig_gw=$(cat "$SR_ORIG_GW_FILE")
    [ -f "$SR_ORIG_IF_FILE" ] && orig_if=$(cat "$SR_ORIG_IF_FILE")

    if [ -z "$orig_gw" ] || [ -z "$orig_if" ]; then
        sr_log "ERROR: Original gateway info missing, cannot set up policy routing"
        return 1
    fi

    ip route add default via "$orig_gw" dev "$orig_if" table "$SR_TABLE" 2>/dev/null
    ip rule add fwmark "$SR_FWMARK" table "$SR_TABLE" priority 100 2>/dev/null

    sr_log "Policy routing configured: fwmark $SR_FWMARK -> table $SR_TABLE via $orig_gw dev $orig_if"
}

sr_setup_iptables() {
    iptables -t mangle -N "$SR_CHAIN_DIRECT" 2>/dev/null
    iptables -t mangle -F "$SR_CHAIN_DIRECT" 2>/dev/null
    iptables -t mangle -A "$SR_CHAIN_DIRECT" -j MARK --set-mark "$SR_FWMARK"
    iptables -t mangle -A "$SR_CHAIN_DIRECT" -j CONNMARK --save-mark --nfmask 0xffffffff --ctmask 0xffffffff

    iptables -t mangle -N "$SR_CHAIN_SMART" 2>/dev/null
    iptables -t mangle -F "$SR_CHAIN_SMART" 2>/dev/null
    iptables -t mangle -A "$SR_CHAIN_SMART" -m set --match-set "$SR_IPSET_TUNNEL" dst -j RETURN
    iptables -t mangle -A "$SR_CHAIN_SMART" -m set --match-set "$SR_IPSET_DOMESTIC" dst -j "$SR_CHAIN_DIRECT"

    iptables -t mangle -C PREROUTING -j "$SR_CHAIN_SMART" 2>/dev/null || \
        iptables -t mangle -A PREROUTING -j "$SR_CHAIN_SMART"

    # Also handle locally generated DNS traffic to upstream through tunnel
    local dns_upstream="${SR_DNS_UPSTREAM:-1.1.1.1}"
    iptables -t mangle -C OUTPUT -p udp -d "$dns_upstream" --dport 53 -m set --match-set "$SR_IPSET_DOMESTIC" dst -j RETURN 2>/dev/null || \
        iptables -t mangle -A OUTPUT -p udp -d "$dns_upstream" --dport 53 -m set --match-set "$SR_IPSET_DOMESTIC" dst -j RETURN 2>/dev/null

    sr_log "iptables mangle rules configured"
}

sr_cleanup_iptables() {
    iptables -t mangle -D PREROUTING -j "$SR_CHAIN_SMART" 2>/dev/null
    iptables -t mangle -F "$SR_CHAIN_SMART" 2>/dev/null
    iptables -t mangle -X "$SR_CHAIN_SMART" 2>/dev/null
    iptables -t mangle -F "$SR_CHAIN_DIRECT" 2>/dev/null
    iptables -t mangle -X "$SR_CHAIN_DIRECT" 2>/dev/null

    # Clean OUTPUT rules
    local dns_upstream="${SR_DNS_UPSTREAM:-1.1.1.1}"
    iptables -t mangle -D OUTPUT -p udp -d "$dns_upstream" --dport 53 -m set --match-set "$SR_IPSET_DOMESTIC" dst -j RETURN 2>/dev/null

    sr_log "iptables mangle rules cleaned up"
}

sr_cleanup_routing() {
    ip rule del fwmark "$SR_FWMARK" table "$SR_TABLE" 2>/dev/null
    ip route flush table "$SR_TABLE" 2>/dev/null
    sr_log "Policy routing cleaned up"
}

sr_destroy_ipsets() {
    ipset destroy "$SR_IPSET_TUNNEL" 2>/dev/null
    ipset destroy "$SR_IPSET_DOMESTIC" 2>/dev/null
    sr_log "ipsets destroyed"
}

sr_start() {
    if [ "$SR_ENABLED" != "yes" ]; then
        return 0
    fi

    if [ "$TT_MODE" != "tun" ]; then
        sr_log "Smart routing only supported in TUN mode (current: $TT_MODE)"
        return 0
    fi

    sr_log "Starting smart routing (country=$SR_HOME_COUNTRY)"

    if ! sr_check_deps; then
        sr_log "ERROR: Required packages not available"
        return 1
    fi

    mkdir -p "$SR_DIR"

    sr_create_ipsets
    sr_load_domestic_nets
    sr_generate_dnsmasq_conf
    sr_start_dnsmasq
    sr_setup_routing
    sr_setup_iptables

    sr_log "Smart routing started"
}

sr_stop() {
    sr_log "Stopping smart routing"
    sr_stop_dnsmasq
    sr_cleanup_iptables
    sr_cleanup_routing
    sr_destroy_ipsets
    rm -f "$SR_ORIG_GW_FILE" "$SR_ORIG_IF_FILE"
    sr_log "Smart routing stopped"
}

sr_restore_iptables() {
    if [ "$SR_ENABLED" != "yes" ] || [ "$TT_MODE" != "tun" ]; then
        return 0
    fi

    if ipset list "$SR_IPSET_DOMESTIC" -t > /dev/null 2>&1; then
        sr_log "Restoring smart routing iptables rules"
        sr_setup_iptables
    fi
}

sr_reload_dnsmasq() {
    sr_generate_dnsmasq_conf
    sr_stop_dnsmasq
    sr_start_dnsmasq
}

sr_reload_nets() {
    sr_update_nets
    sr_load_domestic_nets
}
