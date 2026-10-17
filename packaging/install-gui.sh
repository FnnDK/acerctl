#!/bin/sh
# Builds the GUI (Go + Wails) and the session agent (acer-tray: tray icon and
# Sense key) and installs them with the desktop entry, icon and agent autostart
# (/usr/local and /etc, writable on Fedora Silverblue).
# The same build ships in three flavors (branding only; capabilities still
# come from the firmware, see daemon/flavor): nitro-control-center (red [N])
# on Nitro, predator-control-center (blue-grey [P]) on Predator and
# acer-control-center (green [a]) on the other lines. FLAVOR=nitro|predator|acer
# overrides the choice.
# Node, Wails and the WebKitGTK headers live in the distrobox; installing needs
# root on the host, which only has to provide the webkit2gtk-4.1 runtime:
#   sh packaging/install-gui.sh build                         # in the container
#   distrobox-host-exec sudo sh packaging/install-gui.sh      # on the host
#   distrobox-host-exec sudo sh packaging/install-gui.sh uninstall
# (packaging/uninstall.sh removes the whole install, daemon and modules too)
set -eu
R=$(cd "$(dirname "$0")/.." && pwd)
OUT="$R/gui/build/bin/nitro-control-center"
TRAY="$R/daemon/acer-tray"
WAILS=$(command -v wails || echo "$HOME/go/bin/wails")
AUTOSTART=/etc/xdg/autostart/io.github.fnndk.AcerTray.desktop

build() {
  echo "building the GUI"
  # Debian/Fedora ship WebKitGTK 4.1 only (libsoup3): Wails needs the webkit2_41 tag.
  (cd "$R/gui" && "$WAILS" build -tags webkit2_41 -trimpath -ldflags "-s -w" -clean)
  echo "building acer-tray"
  (cd "$R/daemon" && go build -trimpath -ldflags "-s -w" -o "$TRAY" ./cmd/acer-tray)
}

# flavor: by product line (product name in DMI).
flavor() {
  case "${FLAVOR:-}" in
    nitro|predator|acer) echo "$FLAVOR"; return ;;
    "") ;;
    *) echo "FLAVOR must be nitro, predator or acer" >&2; exit 2 ;;
  esac
  case "$(cat /sys/class/dmi/id/product_name 2>/dev/null)" in
    Nitro*) echo nitro ;;
    Predator*) echo predator ;;
    *) echo acer ;;
  esac
}

# set_flavor sets APP (executable), ID (desktop file/icon), NAME and ICON_SVG.
set_flavor() {
  case "$1" in
    nitro) APP=nitro-control-center ID=io.github.fnndk.NitroControlCenter NAME="Nitro Control Center" ICON_SVG=n-icon-cut.svg ;;
    predator) APP=predator-control-center ID=io.github.fnndk.PredatorControlCenter NAME="Predator Control Center" ICON_SVG=p-icon-cut.svg ;;
    acer) APP=acer-control-center ID=io.github.fnndk.AcerControlCenter NAME="Acer Control Center" ICON_SVG=a-icon-cut.svg ;;
  esac
}

remove_flavor() {
  set_flavor "$1"
  rm -fv "/usr/local/bin/$APP" "/usr/local/share/applications/$ID.desktop" "/usr/local/share/icons/hicolor/scalable/apps/$ID.svg"
}

# Before the rename to acerctl the tray agent was nitro-tray and the user
# settings lived in ~/.config/nitroctl: remove the old agent, move the settings.
migrate_nitro_tray() {
  pkill -x nitro-tray 2>/dev/null || true
  rm -fv /usr/local/bin/nitro-tray /etc/xdg/autostart/io.github.fnndk.NitroTray.desktop
  for d in /home/*/.config/nitroctl /var/home/*/.config/nitroctl; do
    if [ -d "$d" ] && [ ! -e "${d%/nitroctl}/acerctl" ]; then
      mv -v "$d" "${d%/nitroctl}/acerctl"
    fi
  done
}

install_all() {
  [ "$(id -u)" -eq 0 ] || { echo "run as root (distrobox-host-exec sudo ...)"; exit 1; }
  for f in "$OUT" "$TRAY"; do
    [ -x "$f" ] || { echo "no $f: run 'sh packaging/install-gui.sh build' in the container first"; exit 1; }
  done
  migrate_nitro_tray
  FL=$(flavor)
  # one flavor per machine: the tray and the Sense key open whichever is installed
  for other in nitro predator acer; do
    [ "$other" = "$FL" ] || remove_flavor "$other"
  done
  set_flavor "$FL"
  install -D -m 0755 "$OUT" "/usr/local/bin/$APP"
  install -D -m 0755 "$TRAY" /usr/local/bin/acer-tray
  tmp=$(mktemp)
  sed -e "s/^Name=.*/Name=$NAME (tray)/" -e "s/^Icon=.*/Icon=$ID/" "$R/packaging/io.github.fnndk.AcerTray.desktop" > "$tmp"
  install -D -m 0644 "$tmp" "$AUTOSTART"
  rm -f "$tmp"
  # icon before the desktop entry: GNOME Shell resolves the icon as soon as the
  # entry appears and keeps a miss until the entry changes again
  install -D -m 0644 "$R/assets/logo/$ICON_SVG" "/usr/local/share/icons/hicolor/scalable/apps/$ID.svg"
  install -D -m 0644 "$R/packaging/$ID.desktop" "/usr/local/share/applications/$ID.desktop"
  if command -v restorecon >/dev/null 2>&1; then
    restorecon -R "/usr/local/bin/$APP" /usr/local/bin/acer-tray /usr/local/share/applications \
      /usr/local/share/icons "$AUTOSTART" || true
  fi
  command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database -q /usr/local/share/applications || true
  echo "installed: $APP (launcher: $NAME, flavor $FL) and acer-tray (starts at login;"
  echo "opening the app starts it now; a running agent keeps its old flavor until it restarts;"
  echo "GNOME shows the icon with the AppIndicator extension)"
}

uninstall() {
  [ "$(id -u)" -eq 0 ] || { echo "run as root (distrobox-host-exec sudo ...)"; exit 1; }
  pkill -x acer-tray 2>/dev/null || true
  remove_flavor nitro
  remove_flavor predator
  remove_flavor acer
  rm -fv /usr/local/bin/acer-tray "$AUTOSTART"
  command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database -q /usr/local/share/applications || true
  # These belong to the user (Settings → Start with the session; tray and key settings).
  for f in /home/*/.config/autostart/io.github.fnndk.*ControlCenter.desktop /var/home/*/.config/autostart/io.github.fnndk.*ControlCenter.desktop \
    /home/*/.config/acerctl /var/home/*/.config/acerctl; do
    if [ -e "$f" ]; then echo "left in place (user setting): $f"; fi
  done
  echo "removed the app and acer-tray"
}

case "${1:-install}" in
  build) build ;;
  install) install_all ;;
  uninstall) uninstall ;;
  *) echo "usage: $0 [build|install|uninstall]"; exit 2 ;;
esac
