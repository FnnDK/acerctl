#!/bin/sh
# Builds the Debian/Ubuntu packages from binaries already compiled (the GUI
# needs a newer Go than these distributions ship, so tools/release.sh builds
# them with the official toolchain; they only need glibc >= 2.34):
#   acerctl, acerctl-tray, nitro-/predator-/acer-control-center (amd64) and
#   acerctl-dkms (all), from the templates in packaging/deb/.
# Usage: sh tools/build-debs.sh BIN SRC VERSION OUT
#   BIN: acerd, acerctl, acer-tray and control-center (the app)
#   SRC: a source tree (packaging/, drivers/, assets/, LICENSE)
set -eu
BIN=$1 SRC=$2 VER=$3 OUT=$4
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$OUT"

# fill KEY=VALUE pairs into a template
fill() {
  f=$1; shift
  cp "$f" "$WORK/fill"
  for kv in "$@"; do
    k=${kv%%=*} v=${kv#*=}
    sed -i "s|@$k@|$v|g" "$WORK/fill"
  done
  cat "$WORK/fill"
}

# pkg NAME TREE CONTROL [scripts dir]: control file, maintainer scripts, conffiles
pkg() {
  name=$1 tree=$2 control=$3 scripts=${4:-}
  mkdir -p "$tree/DEBIAN"
  size=$(du -sk --exclude=DEBIAN "$tree" | cut -f1)
  shift 3; [ $# -gt 0 ] && shift
  fill "$control" VERSION="$VER" SIZE="$size" "$@" > "$tree/DEBIAN/control"
  if [ -n "$scripts" ]; then
    for s in postinst prerm postrm; do
      [ -f "$scripts/$s" ] || continue
      fill "$scripts/$s" VERSION="$VER" > "$tree/DEBIAN/$s"
      chmod 0755 "$tree/DEBIAN/$s"
    done
  fi
  if [ -d "$tree/etc" ]; then
    (cd "$tree" && find etc -type f | sed 's|^|/|') > "$tree/DEBIAN/conffiles"
  fi
  dpkg-deb --root-owner-group -Zxz --build "$tree" "$OUT/${name}_${VER}_$(sed -n 's/^Architecture: //p' "$tree/DEBIAN/control").deb" >/dev/null
  echo "$OUT/${name}_${VER}_$(sed -n 's/^Architecture: //p' "$tree/DEBIAN/control").deb"
}

doc() { install -Dm0644 "$SRC/LICENSE" "$1/usr/share/doc/$2/copyright"; }

# acerctl: daemon, CLI, unit, D-Bus, polkit, hwdb
t="$WORK/acerctl"
install -Dm0755 "$BIN/acerd" "$t/usr/bin/acerd"
install -Dm0755 "$BIN/acerctl" "$t/usr/bin/acerctl"
install -Dm0644 "$SRC/packaging/acerd.service" "$t/usr/lib/systemd/system/acerd.service"
sed -i 's|/usr/local/bin/|/usr/bin/|' "$t/usr/lib/systemd/system/acerd.service"
install -Dm0644 "$SRC/packaging/io.github.fnndk.Acerd1.conf" "$t/usr/share/dbus-1/system.d/io.github.fnndk.Acerd1.conf"
install -Dm0644 "$SRC/packaging/io.github.fnndk.Acerd1.service" "$t/usr/share/dbus-1/system-services/io.github.fnndk.Acerd1.service"
install -Dm0644 "$SRC/packaging/io.github.fnndk.acerd.policy" "$t/usr/share/polkit-1/actions/io.github.fnndk.acerd.policy"
install -Dm0644 "$SRC/packaging/61-acerctl-keyboard.hwdb" "$t/usr/lib/udev/hwdb.d/61-acerctl-keyboard.hwdb"
doc "$t" acerctl
pkg acerctl "$t" "$SRC/packaging/deb/acerctl/control" "$SRC/packaging/deb/acerctl"

# acerctl-tray
t="$WORK/acerctl-tray"
install -Dm0755 "$BIN/acer-tray" "$t/usr/bin/acer-tray"
doc "$t" acerctl-tray
pkg acerctl-tray "$t" "$SRC/packaging/deb/acerctl-tray/control"

# the app, one package per flavor: it picks its branding from its file name
for f in Nitro:nitro:n:"Acer Nitro laptops":"Nitro (red [N])" \
         Predator:predator:p:"Acer Predator laptops":"Predator (blue-grey [P])" \
         Acer:acer:a:"the other Acer lines (Aspire, Swift...)":"Acer (green [a])"; do
  name=${f%%:*}; rest=${f#*:}; flav=${rest%%:*}; rest=${rest#*:}; logo=${rest%%:*}; rest=${rest#*:}
  line=${rest%%:*}; brand=${rest#*:}
  app=$flav-control-center id=io.github.fnndk.${name}ControlCenter
  others=$(for o in nitro predator acer; do [ $o = "$flav" ] || printf '%s-control-center, ' $o; done | sed 's/, $//')
  t="$WORK/$app"
  install -Dm0755 "$BIN/control-center" "$t/usr/bin/$app"
  install -Dm0644 "$SRC/packaging/$id.desktop" "$t/usr/share/applications/$id.desktop"
  install -Dm0644 "$SRC/assets/logo/$logo-icon-cut.svg" "$t/usr/share/icons/hicolor/scalable/apps/$id.svg"
  mkdir -p "$t/etc/xdg/autostart"
  sed -e "s|^Name=.*|Name=$name Control Center (tray)|" -e "s|^Icon=.*|Icon=$id|" -e 's|/usr/local/bin/|/usr/bin/|' \
    "$SRC/packaging/io.github.fnndk.AcerTray.desktop" > "$t/etc/xdg/autostart/$id.Tray.desktop"
  doc "$t" "$app"
  pkg "$app" "$t" "$SRC/packaging/deb/app/control" "" APP="$app" NAME="$name Control Center" \
    LINE="$line" BRAND="$brand" OTHERS="$others"
done

# acerctl-dkms: module sources where dh_dkms puts them
t="$WORK/acerctl-dkms"
for m in acer-wmi-extras acer-wmi-battery; do
  mkdir -p "$t/usr/src/acerctl-$VER/$m"
  install -m0644 "$SRC/drivers/$m/$m.c" "$SRC/drivers/$m/Makefile" "$t/usr/src/acerctl-$VER/$m/"
done
fill "$SRC/packaging/deb/acerctl-dkms/dkms.conf" VERSION="$VER" > "$t/usr/src/acerctl-$VER/dkms.conf"
# acer-wmi-battery loads by its WMI alias; acer-wmi-extras has none
install -dm0755 "$t/usr/lib/modules-load.d"
echo acer_wmi_extras > "$t/usr/lib/modules-load.d/acerctl.conf"
install -Dm0644 "$SRC/drivers/LICENSE" "$t/usr/share/doc/acerctl-dkms/copyright"
pkg acerctl-dkms "$t" "$SRC/packaging/deb/acerctl-dkms/control" "$SRC/packaging/deb/acerctl-dkms"
