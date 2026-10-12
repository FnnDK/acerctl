#!/bin/sh
# Installs the signed companion modules into /var/lib/acer-wmi-modules (SELinux
# label modules_object_t) and enables the systemd unit that loads them at boot.
# Usage (on the host):  sudo ./packaging/install-modules.sh
set -eu
ROOT=$(cd "$(dirname "$0")/.." && pwd)
DEST=/var/lib/acer-wmi-modules
MODULES="acer-wmi-battery acer-wmi-extras"

mkdir -p "$DEST"
if command -v semanage >/dev/null 2>&1; then
  if ! semanage fcontext -l 2>/dev/null | grep -q "^$DEST(/\.\*)?"; then
    semanage fcontext -a -t modules_object_t "$DEST(/.*)?"
  fi
fi
for m in $MODULES; do
  ko="$ROOT/drivers/$m/$m.ko"
  [ -f "$ko" ] || { echo "missing $ko (run packaging/build-sign.sh first)"; exit 1; }
  modinfo -F signer "$ko" | grep -q . || { echo "$ko is not signed (run packaging/build-sign.sh)"; exit 1; }
  install -m 0644 "$ko" "$DEST/$m.ko"
done
restorecon -R "$DEST"
ls -Z "$DEST"

install -m 0644 "$ROOT/packaging/acer-wmi-modules.service" /etc/systemd/system/acer-wmi-modules.service
systemctl daemon-reload
systemctl enable acer-wmi-modules.service
# reload through the unit so the boot path is what gets tested now
rmmod acer_wmi_extras 2>/dev/null || true
rmmod acer_wmi_battery 2>/dev/null || true
systemctl restart acer-wmi-modules.service
systemctl --no-pager --lines=6 status acer-wmi-modules.service || true
for m in $MODULES; do n=$(echo "$m" | tr - _); [ -d "/sys/module/$n" ] && echo "loaded: $m" || echo "NOT loaded: $m"; done
