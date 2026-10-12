#!/bin/sh
# Builds nitrosensed and installs binary, D-Bus policy, polkit actions and
# the systemd unit into paths that are writable on Fedora Silverblue
# (/usr/local = /var/usrlocal, /etc). Go lives in the distrobox, root on the
# host, so the two steps run in different places:
#   sh packaging/install-daemon.sh build                        # in the container
#   distrobox-host-exec sudo sh packaging/install-daemon.sh     # on the host
# The default (all) builds when go is available and otherwise installs the
# binary already built by the first step.
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
  # polkitd only watches action directories that existed when it started;
  # a restart makes it read /usr/local/share/polkit-1/actions.
  systemctl try-restart polkit.service
  systemctl reload dbus.service 2>/dev/null || systemctl reload dbus-broker.service 2>/dev/null || true
  systemctl enable nitrosensed.service
  systemctl restart nitrosensed.service
  sleep 1
  systemctl --no-pager --lines=5 status nitrosensed.service || true
  echo
  echo "check: busctl get-property org.nitrosense.Daemon1 /org/nitrosense/Daemon1 org.nitrosense.Daemon1.System Capabilities"
}

case "${1:-all}" in
  build) build ;;
  install) install_all ;;
  all)
    if command -v go >/dev/null 2>&1; then
      build
    elif [ -x "$R/daemon/nitrosensed" ]; then
      echo "go not found here; installing the binary built earlier ($("$R/daemon/nitrosensed" --version))"
    else
      echo "go not found and no daemon/nitrosensed: run 'sh packaging/install-daemon.sh build' in the container first"; exit 1
    fi
    install_all ;;
  *) echo "usage: $0 [build|install|all]"; exit 2 ;;
esac
