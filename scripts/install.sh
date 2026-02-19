#!/bin/sh
set -e

MANAGER_REPO="jounts/TrustTunnel4keenetic"
CLIENT_REPO="TrustTunnel/TrustTunnelClient"
GITHUB_API="https://api.github.com/repos"
INSTALL_DIR="/opt/trusttunnel_client"
HOOKS_DIR="/opt/etc/ndm"
INIT_DIR="/opt/etc/init.d"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { printf "${GREEN}[INFO]${NC} %s\n" "$1"; }
warn()  { printf "${YELLOW}[WARN]${NC} %s\n" "$1"; }
error() { printf "${RED}[ERROR]${NC} %s\n" "$1"; exit 1; }

detect_arch() {
    local machine
    machine=$(uname -m)
    case "$machine" in
        mips)
            if [ "$(echo -n I | od -to2 | head -n1 | awk '{print $2}')" = "0100111" ]; then
                echo "mipsel"
            else
                echo "mips"
            fi
            ;;
        mipsel|mipsle)
            echo "mipsel"
            ;;
        aarch64|arm64)
            echo "aarch64"
            ;;
        armv7*|armhf)
            echo "armv7"
            ;;
        x86_64|amd64)
            echo "x86_64"
            ;;
        *)
            error "Unsupported architecture: $machine"
            ;;
    esac
}

check_prerequisites() {
    for cmd in curl tar; do
        if ! command -v "$cmd" > /dev/null 2>&1; then
            error "Required command not found: $cmd. Install it via opkg: opkg install $cmd"
        fi
    done

    if [ ! -d "/opt" ]; then
        error "Entware (/opt) not found. Install Entware first: https://keenetic.link/entware"
    fi
}

get_latest_version() {
    local repo="$1"
    local version
    version=$(curl -s "$GITHUB_API/$repo/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
    echo "$version"
}

download_manager() {
    local arch="$1"
    local version
    version=$(get_latest_version "$MANAGER_REPO")

    if [ -z "$version" ]; then
        error "Failed to get latest manager version"
    fi

    info "Downloading trusttunnel-manager $version for $arch..."

    local go_arch=""
    case "$arch" in
        mipsel) go_arch="linux-mipsel" ;;
        mips)   go_arch="linux-mips" ;;
        aarch64) go_arch="linux-aarch64" ;;
        armv7)  go_arch="linux-armv7" ;;
        *)      error "No manager build for $arch" ;;
    esac

    local url="https://github.com/$MANAGER_REPO/releases/download/$version/trusttunnel-manager-${go_arch}"
    curl -fsSL "$url" -o "$INSTALL_DIR/trusttunnel-manager" || error "Download failed: $url"
    chmod 755 "$INSTALL_DIR/trusttunnel-manager"
    info "Manager $version installed"
}

download_client() {
    local arch="$1"

    info "Downloading TrustTunnel client..."

    local client_arch=""
    case "$arch" in
        mipsel) client_arch="mipsel" ;;
        mips)   client_arch="mips" ;;
        aarch64) client_arch="aarch64" ;;
        armv7)  client_arch="armv7" ;;
        *)      error "No client build for $arch" ;;
    esac

    local version
    version=$(get_latest_version "$CLIENT_REPO")
    if [ -z "$version" ]; then
        error "Failed to get latest client version"
    fi

    local url="https://github.com/$CLIENT_REPO/releases/download/$version/trusttunnel_client-linux-${client_arch}.tar.gz"
    local tmp="/tmp/tt_client.tar.gz"

    curl -fsSL "$url" -o "$tmp" || error "Client download failed"
    tar xzf "$tmp" -C "$INSTALL_DIR/" || error "Failed to extract client"
    rm -f "$tmp"
    chmod 755 "$INSTALL_DIR/trusttunnel_client"
    info "Client $version installed"
}

install_scripts() {
    info "Installing init scripts..."

    # Fetch scripts from GitHub release or use embedded
    local base_url="https://raw.githubusercontent.com/$MANAGER_REPO/master/scripts"

    curl -fsSL "$base_url/init.d/S99trusttunnel" -o "$INIT_DIR/S99trusttunnel"
    chmod 755 "$INIT_DIR/S99trusttunnel"

    curl -fsSL "$base_url/init.d/S98trusttunnel-manager" -o "$INIT_DIR/S98trusttunnel-manager"
    chmod 755 "$INIT_DIR/S98trusttunnel-manager"

    info "Installing NDM hooks..."

    local hooks="wan.d/010-trusttunnel.sh iflayerchanged.d/trusttunnel.sh netfilter.d/trusttunnel.sh schedule.d/trusttunnel.sh button.d/trusttunnel.sh"

    for hook in $hooks; do
        local dir=$(dirname "$hook")
        mkdir -p "$HOOKS_DIR/$dir"
        curl -fsSL "$base_url/hooks/$hook" -o "$HOOKS_DIR/$hook"
        chmod 755 "$HOOKS_DIR/$hook"
    done

    info "Installing smart routing script..."
    curl -fsSL "$base_url/smart-routing.sh" -o "$INSTALL_DIR/smart-routing.sh"
    chmod 755 "$INSTALL_DIR/smart-routing.sh"

    info "All scripts and hooks installed"
}

install_smart_routing_deps() {
    info "Installing smart routing dependencies (optional)..."
    if command -v opkg > /dev/null 2>&1; then
        opkg update > /dev/null 2>&1 || true
        opkg install dnsmasq-full ipset 2>/dev/null && info "dnsmasq-full and ipset installed" || \
            warn "Could not install dnsmasq-full/ipset. Smart routing will install them on first use."
    else
        warn "opkg not available, skipping smart routing deps"
    fi
}

create_default_config() {
    if [ ! -f "$INSTALL_DIR/mode.conf" ]; then
        cat > "$INSTALL_DIR/mode.conf" << 'MODECONF'
TT_MODE="socks5"
TUN_IDX="0"
PROXY_IDX="0"
HC_ENABLED="yes"
HC_INTERVAL="30"
HC_FAIL_THRESHOLD="3"
HC_GRACE_PERIOD="60"
HC_TARGET_URL="http://connectivitycheck.gstatic.com/generate_204"
HC_CURL_TIMEOUT="5"
HC_SOCKS5_PROXY="127.0.0.1:1080"
SR_ENABLED="no"
SR_HOME_COUNTRY="RU"
SR_DNS_PORT="5354"
SR_DNS_UPSTREAM="1.1.1.1"
MODECONF
        info "Default mode config created"
    fi

    if [ ! -f "$INSTALL_DIR/manager.conf" ]; then
        cat > "$INSTALL_DIR/manager.conf" << 'MGRCONF'
LISTEN_ADDR=":8080"
USERNAME="admin"
PASSWORD=""
MGRCONF
        info "Default manager config created"
    fi
}

create_dirs() {
    mkdir -p "$INSTALL_DIR"
    mkdir -p "$INSTALL_DIR/routing"
    mkdir -p /opt/var/run
    mkdir -p /opt/var/log
    mkdir -p "$INIT_DIR"
    mkdir -p "$HOOKS_DIR"
    mkdir -p /opt/etc/dnsmasq.d
}

main() {
    info "TrustTunnel for Keenetic installer"
    echo ""

    check_prerequisites

    local arch
    arch=$(detect_arch)
    info "Detected architecture: $arch"

    create_dirs
    download_client "$arch"
    download_manager "$arch"
    install_scripts
    create_default_config
    install_smart_routing_deps

    echo ""
    info "Installation complete!"
    echo ""
    echo "Next steps:"
    echo "  1. Configure the client: vi $INSTALL_DIR/trusttunnel_client.toml"
    echo "  2. Or run interactive setup: /opt/trusttunnel_client/configure.sh"
    echo "  3. Set a password for the web panel: vi $INSTALL_DIR/manager.conf"
    echo "  4. Start the services:"
    echo "     $INIT_DIR/S98trusttunnel-manager start"
    echo "     $INIT_DIR/S99trusttunnel start"
    echo "  5. Open web panel: http://<router-ip>:8080"
}

main "$@"
