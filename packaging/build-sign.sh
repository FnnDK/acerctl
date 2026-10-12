#!/bin/sh
# Rebuilds and signs the out-of-tree modules for the running kernel.
# Usage (on the host):  sudo ./packaging/build-sign.sh [module ...]
# Build runs as the invoking user (SUDO_USER); signing needs root (akmods MOK key).
set -eu
ROOT=$(cd "$(dirname "$0")/.." && pwd)
KVER=$(uname -r)
KDIR=/lib/modules/$KVER/build
KEY=/etc/pki/akmods/private/private_key.priv
CERT=/etc/pki/akmods/certs/public_key.der
MODULES=${*:-"acer-wmi-battery acer-wmi-extras acer-wmi-lab"}

[ -d "$KDIR" ] || { echo "kernel-devel for $KVER not found ($KDIR)"; exit 1; }
for m in $MODULES; do
  d="$ROOT/drivers/$m"
  [ -d "$d" ] || { echo "no such module dir: $d"; exit 1; }
  echo "== building $m for $KVER"
  if [ -n "${SUDO_USER:-}" ]; then
    sudo -u "$SUDO_USER" make -s -C "$d" KDIR="$KDIR"
  else
    make -s -C "$d" KDIR="$KDIR"
  fi
  if [ "$(id -u)" = 0 ] && [ -r "$KEY" ]; then
    echo "== signing $m"
    "$KDIR/scripts/sign-file" sha256 "$KEY" "$CERT" "$d/$m.ko"
    modinfo -F signer "$d/$m.ko"
    # refresh the installed copy used by acer-wmi-modules.service (if installed)
    if [ -d /var/lib/acer-wmi-modules ] && [ "$m" != acer-wmi-lab ]; then
      install -m 0644 "$d/$m.ko" "/var/lib/acer-wmi-modules/$m.ko"
      restorecon "/var/lib/acer-wmi-modules/$m.ko" 2>/dev/null || true
      echo "== installed to /var/lib/acer-wmi-modules/$m.ko"
    fi
  else
    echo "== not root or no MOK key: $m.ko left unsigned"
  fi
done
