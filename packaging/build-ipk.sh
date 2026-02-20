#!/bin/bash
set -eo pipefail
trap 'echo "FAILED at line $LINENO (exit code $?)" >&2' ERR

ARCH="$1"
VERSION="${2:-dev}"
# Strip leading 'v' prefix for Debian-compatible versioning
VERSION="${VERSION#v}"
BUILD_DIR="build"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

if [ -z "$ARCH" ]; then
    echo "Usage: $0 <arch> [version]"
    echo "  arch: mipsel, mips, aarch64, armv7"
    exit 1
fi

# Map architecture names
case "$ARCH" in
    mipsel) BINARY_SUFFIX="linux-mipsel"; OPKG_ARCH="mipsel-3.4" ;;
    mips)   BINARY_SUFFIX="linux-mips";   OPKG_ARCH="mips-3.4" ;;
    aarch64) BINARY_SUFFIX="linux-aarch64"; OPKG_ARCH="aarch64-3.10" ;;
    armv7)  BINARY_SUFFIX="linux-armv7";  OPKG_ARCH="armv7-3.2" ;;
    *)      echo "Unknown arch: $ARCH"; exit 1 ;;
esac

BINARY="$BUILD_DIR/trusttunnel-manager-${BINARY_SUFFIX}"
if [ ! -f "$BINARY" ]; then
    echo "Binary not found: $BINARY"
    echo "Run 'make build-all' first."
    exit 1
fi

PKG_NAME="trusttunnel-manager_${VERSION}_${OPKG_ARCH}.ipk"
WORK="$BUILD_DIR/ipk-${ARCH}"

echo "Building $PKG_NAME..."

rm -rf "$WORK"
mkdir -p "$WORK/control" "$WORK/data"

# Prepare data tree
DATA="$WORK/data"
mkdir -p "$DATA/opt/trusttunnel_client"
mkdir -p "$DATA/opt/etc/init.d"
mkdir -p "$DATA/opt/etc/ndm/wan.d"
mkdir -p "$DATA/opt/etc/ndm/iflayerchanged.d"
mkdir -p "$DATA/opt/etc/ndm/netfilter.d"
mkdir -p "$DATA/opt/etc/ndm/schedule.d"
mkdir -p "$DATA/opt/etc/ndm/button.d"
mkdir -p "$DATA/opt/trusttunnel_client/routing"
mkdir -p "$DATA/opt/etc/dnsmasq.d"

# Ship default conffiles so opkg can track them from the start
cat > "$DATA/opt/trusttunnel_client/mode.conf" << 'MODEOF'
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
MODEOF

cat > "$DATA/opt/trusttunnel_client/manager.conf" << 'MGREOF'
LISTEN_ADDR=":8080"
# AUTH_MODE: "ndm" (Keenetic router accounts, default), "none" (disabled)
AUTH_MODE="ndm"
MGREOF

touch "$DATA/opt/trusttunnel_client/trusttunnel_client.toml"
touch "$DATA/opt/trusttunnel_client/routing/domains.txt"

# Copy manager binary
cp "$BINARY" "$DATA/opt/trusttunnel_client/trusttunnel-manager"
chmod 755 "$DATA/opt/trusttunnel_client/trusttunnel-manager"

# Copy scripts
cp "$PROJECT_DIR/scripts/install.sh" "$DATA/opt/trusttunnel_client/"
cp "$PROJECT_DIR/scripts/uninstall.sh" "$DATA/opt/trusttunnel_client/"
cp "$PROJECT_DIR/scripts/configure.sh" "$DATA/opt/trusttunnel_client/"
cp "$PROJECT_DIR/scripts/ndms-compat.sh" "$DATA/opt/trusttunnel_client/"
chmod 755 "$DATA/opt/trusttunnel_client/install.sh"
chmod 755 "$DATA/opt/trusttunnel_client/uninstall.sh"
chmod 755 "$DATA/opt/trusttunnel_client/configure.sh"
chmod 755 "$DATA/opt/trusttunnel_client/ndms-compat.sh"

cp "$PROJECT_DIR/scripts/smart-routing.sh" "$DATA/opt/trusttunnel_client/"
chmod 755 "$DATA/opt/trusttunnel_client/smart-routing.sh"

# Copy init scripts
cp "$PROJECT_DIR/scripts/init.d/S98trusttunnel-manager" "$DATA/opt/etc/init.d/"
cp "$PROJECT_DIR/scripts/init.d/S99trusttunnel" "$DATA/opt/etc/init.d/"
chmod 755 "$DATA/opt/etc/init.d/S98trusttunnel-manager"
chmod 755 "$DATA/opt/etc/init.d/S99trusttunnel"

# Copy NDM hooks
cp "$PROJECT_DIR/scripts/hooks/wan.d/010-trusttunnel.sh" "$DATA/opt/etc/ndm/wan.d/"
cp "$PROJECT_DIR/scripts/hooks/iflayerchanged.d/trusttunnel.sh" "$DATA/opt/etc/ndm/iflayerchanged.d/"
cp "$PROJECT_DIR/scripts/hooks/netfilter.d/trusttunnel.sh" "$DATA/opt/etc/ndm/netfilter.d/"
cp "$PROJECT_DIR/scripts/hooks/schedule.d/trusttunnel.sh" "$DATA/opt/etc/ndm/schedule.d/"
cp "$PROJECT_DIR/scripts/hooks/button.d/trusttunnel.sh" "$DATA/opt/etc/ndm/button.d/"
chmod 755 "$DATA"/opt/etc/ndm/*/trusttunnel*.sh
chmod 755 "$DATA"/opt/etc/ndm/wan.d/010-trusttunnel.sh

# Prepare control files (strip any stray \r from Windows checkouts)
sed -e "s/@VERSION@/$VERSION/g" -e "s/@ARCH@/$OPKG_ARCH/g" -e 's/\r$//' \
    "$SCRIPT_DIR/control/control" > "$WORK/control/control"
sed 's/\r$//' "$SCRIPT_DIR/control/conffiles" > "$WORK/control/conffiles"
sed 's/\r$//' "$SCRIPT_DIR/control/postinst"  > "$WORK/control/postinst"
sed 's/\r$//' "$SCRIPT_DIR/control/prerm"     > "$WORK/control/prerm"
chmod 755 "$WORK/control/postinst" "$WORK/control/prerm"

# Build .ipk (Entware uses tar.gz outer format, NOT ar like standard OpenWrt)
printf "2.0\n" > "$WORK/debian-binary"

(cd "$WORK/control" && tar --format=gnu --numeric-owner --owner=0 --group=0 -czf ../control.tar.gz .)
(cd "$WORK/data" && tar --format=gnu --numeric-owner --owner=0 --group=0 -czf ../data.tar.gz .)

PKG_FILE="$PROJECT_DIR/$BUILD_DIR/$PKG_NAME"
rm -f "$PKG_FILE"

cd "$WORK"
tar --format=gnu --numeric-owner --owner=0 --group=0 \
    -czf "$PKG_FILE" ./debian-binary ./control.tar.gz ./data.tar.gz

echo "Package contents verification:"
tar tzf "$PKG_FILE"
echo "Control archive contents:"
tar tzf control.tar.gz

# Cleanup
cd "$PROJECT_DIR"
rm -rf "$WORK"

echo ""
echo "Built: $BUILD_DIR/$PKG_NAME"
echo "Size: $(du -h "$BUILD_DIR/$PKG_NAME" | cut -f1)"
