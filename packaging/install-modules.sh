#!/bin/sh
# Installs the companion modules into /var/lib/acer-wmi-modules (SELinux label
# modules_object_t) and enables the systemd unit that loads them at boot.
# Usage (on the host):  sudo ./packaging/install-modules.sh [module ...]
# Default modules: acer-wmi-battery acer-wmi-extras (build them first with
# packaging/build-sign.sh). The .ko files must be signed when the kernel
# enforces module signatures (Secure Boot / lockdown); otherwise unsigned
# builds are accepted.
set -eu
ROOT=$(cd "$(dirname "$0")/.." && pwd)
DEST=/var/lib/acer-wmi-modules
MODULES=${*:-"acer-wmi-battery acer-wmi-extras"}

# host_root: these scripts change the system, so they need root on the host;
# sudo inside a toolbox/distrobox container only reaches the container.
host_root() {
  if [ -e /run/.containerenv ] || [ -e /.dockerenv ]; then
    echo "this shell is inside a container: run it on the host (distrobox-host-exec sudo sh $0 ...)"; exit 1
  fi
  [ "$(id -u)" -eq 0 ] || { echo "run as root (sudo sh $0 ...)"; exit 1; }
}

host_root

sig_required() {
  [ "$(cat /sys/module/module/parameters/sig_enforce 2>/dev/null)" = Y ] && return 0
  grep -q '\[\(integrity\|confidentiality\)\]' /sys/kernel/security/lockdown 2>/dev/null
}

for m in $MODULES; do
  ko="$ROOT/drivers/$m/$m.ko"
  [ -f "$ko" ] || { echo "missing $ko (run packaging/build-sign.sh first)"; exit 1; }
  if ! modinfo -F signer "$ko" | grep -q .; then
    if sig_required; then
      echo "$ko is not signed and this kernel requires signed modules (run packaging/build-sign.sh as root)"; exit 1
    fi
    echo "note: $m.ko is unsigned (accepted: module signatures are not enforced here)"
  fi
done

mkdir -p "$DEST"
if command -v semanage >/dev/null 2>&1; then
  if ! semanage fcontext -l 2>/dev/null | grep -q "^$DEST(/\.\*)?"; then
    semanage fcontext -a -t modules_object_t "$DEST(/.*)?"
  fi
fi
for m in $MODULES; do
  install -m 0644 "$ROOT/drivers/$m/$m.ko" "$DEST/$m.ko"
done
restorecon -R "$DEST" 2>/dev/null || true
ls -Z "$DEST" 2>/dev/null || ls "$DEST"

install -m 0644 "$ROOT/packaging/acer-wmi-modules.service" /etc/systemd/system/acer-wmi-modules.service
systemctl daemon-reload
systemctl enable acer-wmi-modules.service
# reload through the unit so the boot path is what gets tested now
rmmod acer_wmi_extras 2>/dev/null || true
rmmod acer_wmi_battery 2>/dev/null || true
systemctl restart acer-wmi-modules.service
systemctl --no-pager --lines=6 status acer-wmi-modules.service || true
for m in $MODULES; do n=$(echo "$m" | tr - _); [ -d "/sys/module/$n" ] && echo "loaded: $m" || echo "NOT loaded: $m"; done
# UPower only looks for charge_control_end_threshold when a battery appears; at
# boot the unit runs before it, but now it is already running.
if [ -d /sys/module/acer_wmi_battery ] && systemctl -q is-active upower.service; then
  systemctl restart upower.service && echo "upower restarted (sees the charge limit again)"
fi
