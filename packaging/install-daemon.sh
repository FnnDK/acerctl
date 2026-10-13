#!/bin/sh
# Builds nitrod and nitroctl and installs the binaries, D-Bus policy, polkit actions and
# the systemd unit into paths that are writable on Fedora Silverblue
# (/usr/local = /var/usrlocal, /etc). Go lives in the distrobox, root on the
# host, so the two steps run in different places:
#   sh packaging/install-daemon.sh build                        # in the container
#   distrobox-host-exec sudo sh packaging/install-daemon.sh     # on the host
# The default (all) builds when go is available and otherwise installs the
# binary already built by the first step.
set -eu
R=$(cd "$(dirname "$0")/.." && pwd)
BIN=/usr/local/bin/nitrod
VERSION=$(cd "$R" && git describe --tags --always --dirty 2>/dev/null || echo 0.1.0-dev)

build() {
  echo "building nitrod $VERSION"
  (cd "$R/daemon" && CGO_CFLAGS="-Wno-deprecated-declarations" go build -trimpath -ldflags "-s -w -X main.Version=$VERSION" -o "$R/daemon/nitrod" ./cmd/nitrod)
  (cd "$R/daemon" && go build -trimpath -ldflags "-s -w" -o "$R/daemon/nitroctl" ./cmd/nitroctl)
}

# The project used to be called nitrosense (nitrosensed, nitrosense-cli,
# org.nitrosense.*): remove that install and keep its state and scenarios.
migrate_nitrosense() {
  if [ -e /etc/systemd/system/nitrosensed.service ]; then
    echo "removing the old nitrosensed install"
    systemctl disable --now nitrosensed.service || true
    rm -f /etc/systemd/system/nitrosensed.service /usr/local/bin/nitrosensed /usr/local/bin/nitrosense-cli \
      /etc/dbus-1/system.d/org.nitrosense.Daemon1.conf /usr/local/share/polkit-1/actions/org.nitrosense.daemon.policy
  fi
  if [ -d /var/lib/nitrosense ] && [ ! -e /var/lib/nitrod ]; then
    mv /var/lib/nitrosense /var/lib/nitrod
    echo "moved /var/lib/nitrosense to /var/lib/nitrod"
  fi
}

install_all() {
  [ "$(id -u)" -eq 0 ] || { echo "run as root (distrobox-host-exec sudo ...)"; exit 1; }
  migrate_nitrosense
  install -D -m 0755 "$R/daemon/nitrod" "$BIN"
  install -D -m 0755 "$R/daemon/nitroctl" /usr/local/bin/nitroctl
  install -D -m 0644 "$R/packaging/io.github.fnndk.Nitrod1.conf" /etc/dbus-1/system.d/io.github.fnndk.Nitrod1.conf
  install -D -m 0644 "$R/packaging/io.github.fnndk.nitrod.policy" /usr/local/share/polkit-1/actions/io.github.fnndk.nitrod.policy
  install -D -m 0644 "$R/packaging/nitrod.service" /etc/systemd/system/nitrod.service
  if command -v restorecon >/dev/null 2>&1; then
    restorecon -v "$BIN" /usr/local/bin/nitroctl /etc/dbus-1/system.d/io.github.fnndk.Nitrod1.conf \
      /usr/local/share/polkit-1/actions/io.github.fnndk.nitrod.policy /etc/systemd/system/nitrod.service || true
  fi
  systemctl daemon-reload
  # polkitd only watches action directories that existed when it started;
  # a restart makes it read /usr/local/share/polkit-1/actions.
  systemctl try-restart polkit.service
  systemctl reload dbus.service 2>/dev/null || systemctl reload dbus-broker.service 2>/dev/null || true
  systemctl enable nitrod.service
  systemctl restart nitrod.service
  sleep 1
  systemctl --no-pager --lines=5 status nitrod.service || true
  echo
  echo "check: nitroctl status"
}

case "${1:-all}" in
  build) build ;;
  install) install_all ;;
  all)
    if command -v go >/dev/null 2>&1; then
      build
    elif [ -x "$R/daemon/nitrod" ]; then
      echo "go not found here; installing the binary built earlier ($("$R/daemon/nitrod" --version))"
    else
      echo "go not found and no daemon/nitrod: run 'sh packaging/install-daemon.sh build' in the container first"; exit 1
    fi
    install_all ;;
  *) echo "usage: $0 [build|install|all]"; exit 2 ;;
esac
