#!/bin/sh
# Builds nitrosensed and installs binary, D-Bus policy, polkit actions and
# the systemd unit into paths that are writable on Fedora Silverblue
# (/usr/local = /var/usrlocal, /etc). Run as root on the host:
#   distrobox-host-exec sudo sh packaging/install-daemon.sh
# Build only (no root):  sh packaging/install-daemon.sh build
set -eu
R=$(cd "$(dirname "$0")/.." && pwd)
BIN=/usr/local/bin/nitrosensed
VERSION=$(cd "$R" && git describe --tags --always --dirty 2>/dev/null || echo 0.1.0-dev)

build() {
  echo "building nitrosensed $VERSION"
  (cd "$R/daemon" && go build -trimpath -ldflags "-s -w -X main.Version=$VERSION" -o "$R/daemon/nitrosensed" ./cmd/nitrosensed)
}

install_all() {
  [ "$(id -u)" -eq 0 ] || { echo "run as root (distrobox-host-exec sudo ...)"; exit 1; }
  install -D -m 0755 "$R/daemon/nitrosensed" "$BIN"
  install -D -m 0644 "$R/packaging/org.nitrosense.Daemon1.conf" /etc/dbus-1/system.d/org.nitrosense.Daemon1.conf
  install -D -m 0644 "$R/packaging/org.nitrosense.daemon.policy" /usr/local/share/polkit-1/actions/org.nitrosense.daemon.policy
  install -D -m 0644 "$R/packaging/nitrosensed.service" /etc/systemd/system/nitrosensed.service
  if command -v restorecon >/dev/null 2>&1; then
    restorecon -v "$BIN" /etc/dbus-1/system.d/org.nitrosense.Daemon1.conf \
      /usr/local/share/polkit-1/actions/org.nitrosense.daemon.policy /etc/systemd/system/nitrosensed.service || true
  fi
  systemctl daemon-reload
  systemctl reload dbus.service 2>/dev/null || systemctl reload dbus-broker.service 2>/dev/null || true
  systemctl enable --now nitrosensed.service
  sleep 1
  systemctl --no-pager --lines=5 status nitrosensed.service || true
  echo
  echo "check: busctl get-property org.nitrosense.Daemon1 /org/nitrosense/Daemon1 org.nitrosense.Daemon1.System Capabilities"
}

case "${1:-all}" in
  build) build ;;
  install) install_all ;;
  all) build; install_all ;;
  *) echo "usage: $0 [build|install|all]"; exit 2 ;;
esac
