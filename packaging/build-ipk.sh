#!/bin/bash
set -e

ARCH="$1"
VERSION="${2:-dev}"
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

# Copy manager binary
cp "$BINARY" "$DATA/opt/trusttunnel_client/trusttunnel-manager"
chmod 755 "$DATA/opt/trusttunnel_client/trusttunnel-manager"

# Copy scripts
cp "$PROJECT_DIR/scripts/install.sh" "$DATA/opt/trusttunnel_client/"
cp "$PROJECT_DIR/scripts/configure.sh" "$DATA/opt/trusttunnel_client/"
cp "$PROJECT_DIR/scripts/smart-routing.sh" "$DATA/opt/trusttunnel_client/"
chmod 755 "$DATA/opt/trusttunnel_client/install.sh"
chmod 755 "$DATA/opt/trusttunnel_client/configure.sh"
chmod 755 "$DATA/opt/trusttunnel_client/smart-routing.sh"

# Create routing directory
mkdir -p "$DATA/opt/trusttunnel_client/routing"
mkdir -p "$DATA/opt/etc/dnsmasq.d"

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

# Prepare control files
sed -e "s/@VERSION@/$VERSION/g" -e "s/@ARCH@/$OPKG_ARCH/g" \
    "$SCRIPT_DIR/control/control" > "$WORK/control/control"
cp "$SCRIPT_DIR/control/conffiles" "$WORK/control/"
cp "$SCRIPT_DIR/control/postinst" "$WORK/control/"
cp "$SCRIPT_DIR/control/prerm" "$WORK/control/"
chmod 755 "$WORK/control/postinst" "$WORK/control/prerm"

# Build .ipk (ar archive)
echo "2.0" > "$WORK/debian-binary"

cd "$PROJECT_DIR/$WORK/control"
tar czf ../control.tar.gz ./*
cd "$PROJECT_DIR/$WORK/data"
tar czf ../data.tar.gz ./*
cd "$PROJECT_DIR/$WORK"

ar rc "$PROJECT_DIR/$BUILD_DIR/$PKG_NAME" debian-binary control.tar.gz data.tar.gz

# Cleanup
cd "$PROJECT_DIR"
rm -rf "$WORK"

echo "Built: $BUILD_DIR/$PKG_NAME"
echo "Size: $(du -h "$BUILD_DIR/$PKG_NAME" | cut -f1)"
