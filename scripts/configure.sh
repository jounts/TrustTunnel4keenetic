#!/bin/sh

INSTALL_DIR="/opt/trusttunnel_client"
TOML_CONF="$INSTALL_DIR/trusttunnel_client.toml"
MODE_CONF="$INSTALL_DIR/mode.conf"
MANAGER_CONF="$INSTALL_DIR/manager.conf"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { printf "${GREEN}[INFO]${NC} %s\n" "$1"; }
ask()  { printf "${YELLOW}?${NC} %s " "$1"; }

echo "=== TrustTunnel для Keenetic — настройка ==="
echo ""

# Mode selection
ask "Режим работы (socks5/tun) [socks5]:"
read -r mode
mode=${mode:-socks5}

if [ "$mode" != "socks5" ] && [ "$mode" != "tun" ]; then
    echo "Неверный режим. Используем socks5."
    mode="socks5"
fi

# TUN/Proxy index
if [ "$mode" = "tun" ]; then
    ask "TUN interface index [0]:"
    read -r tun_idx
    tun_idx=${tun_idx:-0}
    proxy_idx=0
else
    ask "Proxy interface index [0]:"
    read -r proxy_idx
    proxy_idx=${proxy_idx:-0}
    tun_idx=0
fi

# Health check
ask "Включить health check? (yes/no) [yes]:"
read -r hc_enabled
hc_enabled=${hc_enabled:-yes}

hc_interval=30
hc_threshold=3
if [ "$hc_enabled" = "yes" ]; then
    ask "Интервал health check (сек) [30]:"
    read -r hc_interval
    hc_interval=${hc_interval:-30}

    ask "Порог отказов [3]:"
    read -r hc_threshold
    hc_threshold=${hc_threshold:-3}
fi

# Smart routing
sr_enabled="no"
sr_country="RU"
sr_dns_port=5354
sr_dns_upstream="1.1.1.1"

if [ "$mode" = "tun" ]; then
    echo ""
    ask "Включить Smart Routing (GeoIP)? (yes/no) [no]:"
    read -r sr_enabled
    sr_enabled=${sr_enabled:-no}

    if [ "$sr_enabled" = "yes" ]; then
        ask "Код домашней страны (RU/UA/BY/KZ/...) [RU]:"
        read -r sr_country
        sr_country=${sr_country:-RU}

        ask "DNS upstream сервер [1.1.1.1]:"
        read -r sr_dns_upstream
        sr_dns_upstream=${sr_dns_upstream:-1.1.1.1}

        ask "DNS порт для dnsmasq [5354]:"
        read -r sr_dns_port
        sr_dns_port=${sr_dns_port:-5354}
    fi
fi

# Web panel
ask "Порт веб-панели [8080]:"
read -r web_port
web_port=${web_port:-8080}

# TrustTunnel client config
echo ""
ask "Файл конфигурации клиента (TOML)."
echo "  Если у вас уже есть конфигурация, пропустите этот шаг."

if [ ! -f "$TOML_CONF" ]; then
    ask "Создать шаблон конфигурации? (yes/no) [yes]:"
    read -r create_toml
    create_toml=${create_toml:-yes}

    if [ "$create_toml" = "yes" ]; then
        ask "Адрес сервера TrustTunnel:"
        read -r server_addr

        ask "Порт сервера [443]:"
        read -r server_port
        server_port=${server_port:-443}

        ask "Ваш токен/ключ:"
        read -r token

        cat > "$TOML_CONF" << EOF
[server]
address = "$server_addr"
port = $server_port

[auth]
token = "$token"

[client]
mode = "$mode"
socks5_port = 1080
log_level = "info"
EOF
        info "Конфигурация клиента создана: $TOML_CONF"
    fi
fi

# Smart Routing
sr_enabled="no"
sr_country="RU"
sr_dns_upstream="1.1.1.1"
sr_dns_port=5354

if [ "$mode" = "tun" ]; then
    echo ""
    ask "Включить Smart Routing (GeoIP)? (yes/no) [no]:"
    read -r sr_enabled
    sr_enabled=${sr_enabled:-no}

    if [ "$sr_enabled" = "yes" ]; then
        ask "Домашняя страна (RU/UA/BY/KZ/...) [RU]:"
        read -r sr_country
        sr_country=${sr_country:-RU}

        ask "DNS Upstream [1.1.1.1]:"
        read -r sr_dns_upstream
        sr_dns_upstream=${sr_dns_upstream:-1.1.1.1}

        ask "DNS порт [5354]:"
        read -r sr_dns_port
        sr_dns_port=${sr_dns_port:-5354}
    fi
fi

# Write mode.conf
cat > "$MODE_CONF" << EOF
TT_MODE="$mode"
TUN_IDX="$tun_idx"
PROXY_IDX="$proxy_idx"
HC_ENABLED="$hc_enabled"
HC_INTERVAL="$hc_interval"
HC_FAIL_THRESHOLD="$hc_threshold"
HC_GRACE_PERIOD="60"
HC_TARGET_URL="http://connectivitycheck.gstatic.com/generate_204"
HC_CURL_TIMEOUT="5"
HC_SOCKS5_PROXY="127.0.0.1:1080"
SR_ENABLED="$sr_enabled"
SR_HOME_COUNTRY="$sr_country"
SR_DNS_PORT="$sr_dns_port"
SR_DNS_UPSTREAM="$sr_dns_upstream"
EOF
info "Конфигурация режима сохранена: $MODE_CONF"

# Write manager.conf
cat > "$MANAGER_CONF" << EOF
LISTEN_ADDR=":$web_port"
AUTH_MODE="ndm"
EOF
info "Конфигурация менеджера сохранена: $MANAGER_CONF"

echo ""
info "Настройка завершена!"
echo ""
echo "Для запуска:"
echo "  /opt/etc/init.d/S98trusttunnel-manager start"
echo "  /opt/etc/init.d/S99trusttunnel start"
echo ""
echo "Веб-панель: http://<router-ip>:$web_port"
