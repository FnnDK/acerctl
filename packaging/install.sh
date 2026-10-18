#!/bin/sh
# Installs acerctl from the release packages with the system's package manager:
# the daemon and CLI, the kernel modules (akmod on Fedora, DKMS on Debian/Ubuntu)
# and, unless --no-gui, the tray and the app of the laptop's product line.
#   sudo sh install.sh              # product line from the laptop's DMI product name
#   sudo sh install.sh --nitro      # or --acer, --predator
#   sudo sh install.sh --no-gui     # daemon, CLI and modules only
#   sh install.sh -n [...]          # dry run: download and check, print what it would run
# Running it again upgrades: it replaces an earlier version and an install made
# with the scripts, the app of another product line too, and with --no-gui it
# removes an installed app and tray (they require the same acerctl version).
# Fedora 44 (dnf; rpm-ostree on Silverblue, Kinoite, Bluefin), Debian 13 and
# Ubuntu 24.04/26.04 (apt), x86_64.
set -eu

# the release it installs, bumped with the specs (the release workflow checks it)
VERSION=0.1.5
REPO=https://github.com/FnnDK/acerctl
UNINSTALL=https://raw.githubusercontent.com/FnnDK/acerctl/main/packaging/uninstall.sh
APPS="acer-control-center nitro-control-center predator-control-center"

usage() {
  echo "usage: sudo sh $0 [--acer | --nitro | --predator | --no-gui] [-n | --dry-run]"
}

LINE= GUI=1 DRY=
for a in "$@"; do
  case $a in
    --acer|--nitro|--predator) LINE=${a#--} ;;
    --no-gui) GUI= ;;
    -n|--dry-run) DRY=1 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

die() { echo "$*" >&2; exit 1; }

# run CMD...: runs it, or only prints it in a dry run
run() {
  if [ -n "$DRY" ]; then echo "would run: $*"; else "$@"; fi
}

# The packages change the host: a toolbox/distrobox shell only reaches the
# container (its sudo too). A dry run needs no root.
if [ -e /run/.containerenv ] || [ -e /.dockerenv ]; then
  die "this shell is inside a container: run it on the host (distrobox-host-exec sudo sh $0 ...)"
fi
[ -n "$DRY" ] || [ "$(id -u)" -eq 0 ] || die "run as root (sudo sh $0 ...)"
[ "$(uname -m)" = x86_64 ] || die "the packages are built for x86_64"

# Product line: Nitro and Predator by their product name, every other line gets
# the Acer app (the same rule as install-gui.sh).
if [ -z "$LINE" ]; then
  case "$(cat /sys/class/dmi/id/product_name 2>/dev/null)" in
    Nitro*) LINE=nitro ;;
    Predator*) LINE=predator ;;
    *) LINE=acer ;;
  esac
fi
APP=$LINE-control-center
case $LINE in
  acer) NAME="Acer Control Center" ;;
  nitro) NAME="Nitro Control Center" ;;
  predator) NAME="Predator Control Center" ;;
esac

if [ -e /run/ostree-booted ]; then
  PM=rpm-ostree
elif command -v dnf >/dev/null 2>&1; then
  PM=dnf
elif command -v apt-get >/dev/null 2>&1; then
  PM=apt
else
  die "no dnf, rpm-ostree or apt-get: see Building from source in the README"
fi
case $PM in
  dnf|rpm-ostree)
    [ "$(rpm -E %fedora)" = 44 ] || die "the RPMs are built for Fedora 44"
    R=$VERSION-1.fc44
    FILES="acerctl-$R.x86_64.rpm akmod-acerctl-$R.x86_64.rpm acerctl-kmod-common-$R.noarch.rpm"
    if [ -n "$GUI" ]; then FILES="$FILES acerctl-tray-$R.x86_64.rpm $APP-$R.x86_64.rpm"; fi ;;
  apt)
    FILES="acerctl_${VERSION}_amd64.deb acerctl-dkms_${VERSION}_all.deb"
    if [ -n "$GUI" ]; then FILES="$FILES acerctl-tray_${VERSION}_amd64.deb ${APP}_${VERSION}_amd64.deb"; fi ;;
esac
if [ -n "$GUI" ]; then echo "acerctl $VERSION: $APP with $PM"; else echo "acerctl $VERSION: no GUI, with $PM"; fi

# installed NAME...: prints the ones the package manager has installed
installed() {
  for p in "$@"; do
    if [ "$PM" = apt ]; then
      if dpkg-query -W -f '${Status}' "$p" 2>/dev/null | grep -q 'install ok installed'; then echo "$p"; fi
    elif rpm -q --quiet "$p"; then
      echo "$p"
    fi
  done
}

# fetch URL FILE: with curl, or wget (Debian and Ubuntu may have only that)
fetch() {
  if command -v curl >/dev/null 2>&1; then curl -fsSL -o "$2" "$1"
  else wget -q -O "$2" "$1"
  fi
}

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
fetch "$REPO/releases/download/v$VERSION/SHA256SUMS" "$TMP/SHA256SUMS" || die "could not download the $VERSION checksums"
PKGS=
for f in $FILES; do
  fetch "$REPO/releases/download/v$VERSION/$f" "$TMP/$f" || die "could not download $f"
  grep -q " $f\$" "$TMP/SHA256SUMS" || die "$f is not in the release's SHA256SUMS"
  PKGS="$PKGS $TMP/$f"
done
(cd "$TMP" && sha256sum --quiet --ignore-missing -c SHA256SUMS) || die "checksum mismatch: nothing installed"

# An install made with the scripts would shadow the packages (its unit in
# /etc/systemd/system, /usr/local first in PATH): remove it first, with the
# uninstall.sh next to this script in a checkout, else the one on main.
SCRIPTS=
for p in /usr/local/bin/acerd /usr/local/bin/acer-tray /usr/local/bin/nitrod \
  /usr/local/bin/nitro-tray /usr/local/bin/nitrosensed /var/lib/acer-wmi-modules; do
  if [ -e "$p" ]; then SCRIPTS=1; fi
done
if [ -n "$SCRIPTS" ]; then
  U=$(dirname "$0")/uninstall.sh
  [ -f "$(dirname "$0")/install-daemon.sh" ] || { U=$TMP/uninstall.sh; fetch "$UNINSTALL" "$U"; }
  run sh "$U" --scripts
fi

# With --no-gui, an installed app and tray go: they require this acerctl version.
DROP=
if [ -z "$GUI" ]; then DROP=$(installed acerctl-tray $APPS); fi

# shellcheck disable=SC2086
case $PM in
  rpm-ostree)
    # rpm-ostree cannot upgrade local packages in place ("cannot install both"):
    # every installed acerctl package leaves in the same transaction, which also
    # covers another line's app and DROP.
    OLD=
    for p in $(installed acerctl acerctl-tray akmod-acerctl acerctl-kmod-common $APPS); do
      OLD="$OLD --uninstall=$p"
    done
    run rpm-ostree install --idempotent $OLD $PKGS ;;
  dnf)
    if [ -n "$DROP" ]; then run dnf remove $DROP; fi
    # the apps conflict with each other: --allowerasing swaps another line's app
    run dnf install --allowerasing $PKGS ;;
  apt)
    # NAME- removes NAME in the same transaction
    run apt-get install $PKGS $(for p in $DROP; do echo "$p-"; done) ;;
esac

if [ -n "$GUI" ]; then
  NEXT="open $NAME; on its first start it lists anything still missing"
else
  NEXT="acerctl setup lists what is left to set up (acerctl setup apply makes it)"
fi
if [ -n "$DRY" ]; then
  echo "dry run: nothing changed"
elif [ "$PM" = rpm-ostree ]; then
  echo "installed: reboot to start using it (systemctl reboot), then $NEXT"
else
  echo "installed: $NEXT"
fi
