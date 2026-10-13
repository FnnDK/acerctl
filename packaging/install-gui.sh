#!/bin/sh
# Builds the GUI (nitro-control-center, Go + Wails) and installs it with its
# desktop entry and icon under /usr/local (writable on Fedora Silverblue).
# Node, Wails and the WebKitGTK headers live in the distrobox; installing needs
# root on the host, which only has to provide the webkit2gtk-4.1 runtime:
#   sh packaging/install-gui.sh build                         # in the container
#   distrobox-host-exec sudo sh packaging/install-gui.sh      # on the host
#   distrobox-host-exec sudo sh packaging/install-gui.sh uninstall
set -eu
R=$(cd "$(dirname "$0")/.." && pwd)
ID=io.github.fnndk.NitroControlCenter
OUT="$R/gui/build/bin/nitro-control-center"
WAILS=$(command -v wails || echo "$HOME/go/bin/wails")

build() {
  echo "building nitro-control-center"
  # Debian/Fedora ship WebKitGTK 4.1 only (libsoup3): Wails needs the webkit2_41 tag.
  (cd "$R/gui" && "$WAILS" build -tags webkit2_41 -trimpath -ldflags "-s -w" -clean)
}

install_all() {
  [ "$(id -u)" -eq 0 ] || { echo "run as root (distrobox-host-exec sudo ...)"; exit 1; }
  [ -x "$OUT" ] || { echo "no $OUT: run 'sh packaging/install-gui.sh build' in the container first"; exit 1; }
  install -D -m 0755 "$OUT" /usr/local/bin/nitro-control-center
  install -D -m 0644 "$R/packaging/$ID.desktop" "/usr/local/share/applications/$ID.desktop"
  install -D -m 0644 "$R/assets/logo/n-icon-cut.svg" "/usr/local/share/icons/hicolor/scalable/apps/$ID.svg"
  if command -v restorecon >/dev/null 2>&1; then
    restorecon -R /usr/local/bin/nitro-control-center /usr/local/share/applications /usr/local/share/icons || true
  fi
  command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database -q /usr/local/share/applications || true
  echo "installed: nitro-control-center (launcher: Nitro Control Center)"
}

uninstall() {
  [ "$(id -u)" -eq 0 ] || { echo "run as root (distrobox-host-exec sudo ...)"; exit 1; }
  rm -fv /usr/local/bin/nitro-control-center "/usr/local/share/applications/$ID.desktop" \
    "/usr/local/share/icons/hicolor/scalable/apps/$ID.svg"
  command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database -q /usr/local/share/applications || true
  # The autostart entry belongs to the user (Settings → Start with the session).
  for f in /home/*/.config/autostart/$ID.desktop /var/home/*/.config/autostart/$ID.desktop; do
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
