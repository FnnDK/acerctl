#!/bin/sh
# Builds the GUI (nitro-control-center, Go + Wails) and the session agent
# (nitro-tray: tray icon and Nitro key) and installs them with the desktop
# entry, icon and agent autostart (/usr/local and /etc, writable on Fedora
# Silverblue).
# Node, Wails and the WebKitGTK headers live in the distrobox; installing needs
# root on the host, which only has to provide the webkit2gtk-4.1 runtime:
#   sh packaging/install-gui.sh build                         # in the container
#   distrobox-host-exec sudo sh packaging/install-gui.sh      # on the host
#   distrobox-host-exec sudo sh packaging/install-gui.sh uninstall
set -eu
R=$(cd "$(dirname "$0")/.." && pwd)
ID=io.github.fnndk.NitroControlCenter
OUT="$R/gui/build/bin/nitro-control-center"
TRAY="$R/daemon/nitro-tray"
WAILS=$(command -v wails || echo "$HOME/go/bin/wails")

build() {
  echo "building nitro-control-center"
  # Debian/Fedora ship WebKitGTK 4.1 only (libsoup3): Wails needs the webkit2_41 tag.
  (cd "$R/gui" && "$WAILS" build -tags webkit2_41 -trimpath -ldflags "-s -w" -clean)
  echo "building nitro-tray"
  (cd "$R/daemon" && go build -trimpath -ldflags "-s -w" -o "$TRAY" ./cmd/nitro-tray)
}

install_all() {
  [ "$(id -u)" -eq 0 ] || { echo "run as root (distrobox-host-exec sudo ...)"; exit 1; }
  for f in "$OUT" "$TRAY"; do
    [ -x "$f" ] || { echo "no $f: run 'sh packaging/install-gui.sh build' in the container first"; exit 1; }
  done
  install -D -m 0755 "$OUT" /usr/local/bin/nitro-control-center
  install -D -m 0755 "$TRAY" /usr/local/bin/nitro-tray
  install -D -m 0644 "$R/packaging/io.github.fnndk.NitroTray.desktop" /etc/xdg/autostart/io.github.fnndk.NitroTray.desktop
  install -D -m 0644 "$R/packaging/$ID.desktop" "/usr/local/share/applications/$ID.desktop"
  install -D -m 0644 "$R/assets/logo/n-icon-cut.svg" "/usr/local/share/icons/hicolor/scalable/apps/$ID.svg"
  if command -v restorecon >/dev/null 2>&1; then
    restorecon -R /usr/local/bin/nitro-control-center /usr/local/bin/nitro-tray /usr/local/share/applications \
      /usr/local/share/icons /etc/xdg/autostart/io.github.fnndk.NitroTray.desktop || true
  fi
  command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database -q /usr/local/share/applications || true
  echo "installed: nitro-control-center (launcher: Nitro Control Center) and nitro-tray (starts at login;"
  echo "opening the app starts it now; GNOME shows the icon with the AppIndicator extension)"
}

uninstall() {
  [ "$(id -u)" -eq 0 ] || { echo "run as root (distrobox-host-exec sudo ...)"; exit 1; }
  pkill -x nitro-tray 2>/dev/null || true
  rm -fv /usr/local/bin/nitro-control-center /usr/local/bin/nitro-tray "/usr/local/share/applications/$ID.desktop" \
    "/usr/local/share/icons/hicolor/scalable/apps/$ID.svg" /etc/xdg/autostart/io.github.fnndk.NitroTray.desktop
  command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database -q /usr/local/share/applications || true
  # These belong to the user (Settings → Start with the session; tray and key settings).
  for f in /home/*/.config/autostart/$ID.desktop /var/home/*/.config/autostart/$ID.desktop \
    /home/*/.config/nitroctl /var/home/*/.config/nitroctl; do
    if [ -e "$f" ]; then echo "left in place (user setting): $f"; fi
  done
  echo "removed nitro-control-center"
}

case "${1:-install}" in
  build) build ;;
  install) install_all ;;
  uninstall) uninstall ;;
  *) echo "usage: $0 [build|install|uninstall]"; exit 2 ;;
esac
