#!/bin/sh
# Builds acerd and acerctl and installs the binaries, D-Bus policy, polkit actions and
# the systemd unit into paths that are writable on Fedora Silverblue
# (/usr/local = /var/usrlocal, /etc). Go lives in the distrobox, root on the
# host, so the two steps run in different places:
#   sh packaging/install-daemon.sh build                        # in the container
#   distrobox-host-exec sudo sh packaging/install-daemon.sh     # on the host
# The default (all) builds when go and the sources are there (not in a release
# binary archive) and otherwise installs the binary already built.
set -eu
R=$(cd "$(dirname "$0")/.." && pwd)
BIN=/usr/local/bin/acerd
VERSION=$(cd "$R" && git describe --tags --always --dirty 2>/dev/null || echo 0.1.0-dev)

build() {
  echo "building acerd $VERSION"
  (cd "$R/daemon" && CGO_CFLAGS="-Wno-deprecated-declarations" go build -trimpath -ldflags "-s -w -X main.Version=$VERSION" -o "$R/daemon/acerd" ./cmd/acerd)
  (cd "$R/daemon" && go build -trimpath -ldflags "-s -w" -o "$R/daemon/acerctl" ./cmd/acerctl)
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
  if [ -d /var/lib/nitrosense ] && [ ! -e /var/lib/acerd ]; then
    mv /var/lib/nitrosense /var/lib/acerd
    echo "moved /var/lib/nitrosense to /var/lib/acerd"
  fi
}

# Before the rename to acerctl the daemon was nitrod (io.github.fnndk.Nitrod1,
# /var/lib/nitrod, nitroctl): remove that install and keep its state.
migrate_nitrod() {
  if [ -e /etc/systemd/system/nitrod.service ]; then
    echo "removing the old nitrod install"
    systemctl disable --now nitrod.service || true
    rm -f /etc/systemd/system/nitrod.service /usr/local/bin/nitrod /usr/local/bin/nitroctl \
      /etc/dbus-1/system.d/io.github.fnndk.Nitrod1.conf /usr/local/share/polkit-1/actions/io.github.fnndk.nitrod.policy
  fi
  if [ -d /var/lib/nitrod ] && [ ! -e /var/lib/acerd ]; then
    mv /var/lib/nitrod /var/lib/acerd
    echo "moved /var/lib/nitrod to /var/lib/acerd"
  fi
}

install_all() {
  [ "$(id -u)" -eq 0 ] || { echo "run as root (distrobox-host-exec sudo ...)"; exit 1; }
  migrate_nitrosense
  migrate_nitrod
  install -D -m 0755 "$R/daemon/acerd" "$BIN"
  install -D -m 0755 "$R/daemon/acerctl" /usr/local/bin/acerctl
  install -D -m 0644 "$R/packaging/io.github.fnndk.Acerd1.conf" /etc/dbus-1/system.d/io.github.fnndk.Acerd1.conf
  install -D -m 0644 "$R/packaging/io.github.fnndk.Acerd1.service" /usr/local/share/dbus-1/system-services/io.github.fnndk.Acerd1.service
  install -D -m 0644 "$R/packaging/io.github.fnndk.acerd.policy" /usr/local/share/polkit-1/actions/io.github.fnndk.acerd.policy
  install -D -m 0644 "$R/packaging/acerd.service" /etc/systemd/system/acerd.service
  install -D -m 0644 "$R/packaging/61-acerctl-keyboard.hwdb" /etc/udev/hwdb.d/61-acerctl-keyboard.hwdb
  if command -v restorecon >/dev/null 2>&1; then
    restorecon -v "$BIN" /usr/local/bin/acerctl /etc/dbus-1/system.d/io.github.fnndk.Acerd1.conf \
      /usr/local/share/dbus-1/system-services/io.github.fnndk.Acerd1.service \
      /usr/local/share/polkit-1/actions/io.github.fnndk.acerd.policy /etc/systemd/system/acerd.service \
      /etc/udev/hwdb.d/61-acerctl-keyboard.hwdb || true
  fi
  # Sense key mapping for models systemd's hwdb does not list yet (acerd probes
  # the keymap at startup, so this goes before the restart below).
  systemd-hwdb update
  udevadm trigger --subsystem-match=input --action=change
  udevadm settle --timeout=5 || true
  systemctl daemon-reload
  # polkitd only watches action directories that existed when it started;
  # a restart makes it read /usr/local/share/polkit-1/actions.
  systemctl try-restart polkit.service
  systemctl reload dbus.service 2>/dev/null || systemctl reload dbus-broker.service 2>/dev/null || true
  systemctl enable acerd.service
  systemctl restart acerd.service
  sleep 1
  systemctl --no-pager --lines=5 status acerd.service || true
  echo
  echo "check: acerctl status"
}

case "${1:-all}" in
  build) build ;;
  install) install_all ;;
  all)
    if command -v go >/dev/null 2>&1 && [ -f "$R/daemon/go.mod" ]; then
      build
    elif [ -x "$R/daemon/acerd" ]; then
      echo "go not found here; installing the binary built earlier ($("$R/daemon/acerd" --version))"
    else
      echo "go not found and no daemon/acerd: run 'sh packaging/install-daemon.sh build' in the container first"; exit 1
    fi
    install_all ;;
  *) echo "usage: $0 [build|install|all]"; exit 2 ;;
esac
