#!/bin/sh
# Removes everything the install scripts put on the system (install-modules.sh,
# install-daemon.sh, install-gui.sh), including the installs made before the
# renames: nitrosense (nitrosensed, nitrosense-cli, org.nitrosense.*) and
# nitroctl (nitrod, nitroctl, nitro-tray, io.github.fnndk.Nitrod1).
#   sudo sh packaging/uninstall.sh           # keeps the state and the user settings
#   sudo sh packaging/uninstall.sh --purge   # also /var/lib/acerd, ~/.config/acerctl,
#                                            # the app data and /etc/modprobe.d/acerctl.conf
#   sudo sh packaging/uninstall.sh --old     # only the installs from before the renames
#                                            # (install-daemon.sh and install-gui.sh run it)
# RPM/.deb packages are left alone: the script says how to remove them.
set -eu

MODE=all PURGE=
for a in "$@"; do
  case $a in
    --purge) PURGE=1 ;;
    --old) MODE=old ;;
    *) echo "usage: $0 [--purge | --old]"; exit 2 ;;
  esac
done
[ "$MODE" = old ] && [ -n "$PURGE" ] && { echo "--old keeps the state: use it without --purge"; exit 2; }
[ "$(id -u)" -eq 0 ] || { echo "run as root (distrobox-host-exec sudo ...)"; exit 1; }

APPS="nitro-control-center predator-control-center acer-control-center"
IDS="io.github.fnndk.NitroControlCenter io.github.fnndk.PredatorControlCenter io.github.fnndk.AcerControlCenter"
CHANGED= HWDB=

rm_path() {
  for p in "$@"; do
    if [ -e "$p" ] || [ -L "$p" ]; then
      rm -rf -- "$p"
      echo "removed $p"
      CHANGED=1
    fi
  done
}

# drop_unit stops, disables and removes a unit the scripts put in /etc/systemd/system.
drop_unit() {
  u=$1.service
  if [ -e "/etc/systemd/system/$u" ]; then
    systemctl disable --now "$u" 2>/dev/null || true
    rm_path "/etc/systemd/system/$u"
  fi
  # enable links left behind when the unit file went first (an interrupted install)
  for l in /etc/systemd/system/*.wants/"$u"; do
    if [ -L "$l" ] && [ ! -e "$l" ]; then rm_path "$l"; fi
  done
}

# stop_local NAME...: ends the processes running /usr/local/bin/NAME (packaged
# copies in /usr/bin are left alone; on Silverblue the link resolves to /var/usrlocal)
stop_local() {
  lb=$(readlink -f /usr/local/bin)
  for e in /proc/[0-9]*/exe; do
    t=$(readlink "$e" 2>/dev/null) || continue
    t=${t% (deleted)}
    for n in "$@"; do
      if [ "$t" = "/usr/local/bin/$n" ] || [ "$t" = "$lb/$n" ]; then
        pid=${e#/proc/}
        kill "${pid%/exe}" 2>/dev/null || true
      fi
    done
  done
}

# homes of the regular users, for the per-user settings
homes() {
  getent passwd | awk -F: '$3 >= 1000 && $3 < 65534 && $6 != "/" { print $6 }' | sort -u
}

# Installs from before the renames. /var/lib/nitrod and ~/.config/nitroctl stay:
# the install scripts move them to the new names, --purge removes them.
stop_local nitro-tray
drop_unit nitrosensed
drop_unit nitrod
rm_path /usr/local/bin/nitrosensed /usr/local/bin/nitrosense-cli \
  /usr/local/bin/nitrod /usr/local/bin/nitroctl /usr/local/bin/nitro-tray \
  /etc/dbus-1/system.d/org.nitrosense.Daemon1.conf /etc/dbus-1/system.d/io.github.fnndk.Nitrod1.conf \
  /usr/local/share/polkit-1/actions/org.nitrosense.daemon.policy \
  /usr/local/share/polkit-1/actions/io.github.fnndk.nitrod.policy \
  /etc/xdg/autostart/io.github.fnndk.NitroTray.desktop

if [ "$MODE" = all ]; then
  stop_local acer-tray $APPS
  drop_unit acerd
  drop_unit acer-wmi-modules
  rm_path /usr/local/bin/acerd /usr/local/bin/acerctl /usr/local/bin/acer-tray \
    /etc/dbus-1/system.d/io.github.fnndk.Acerd1.conf \
    /usr/local/share/dbus-1/system-services/io.github.fnndk.Acerd1.service \
    /usr/local/share/polkit-1/actions/io.github.fnndk.acerd.policy \
    /etc/xdg/autostart/io.github.fnndk.AcerTray.desktop /var/lib/acer-wmi-modules
  if [ -e /etc/udev/hwdb.d/61-acerctl-keyboard.hwdb ]; then
    rm_path /etc/udev/hwdb.d/61-acerctl-keyboard.hwdb
    HWDB=1
  fi
  for a in $APPS; do rm_path "/usr/local/bin/$a"; done
  for id in $IDS; do
    rm_path "/usr/local/share/applications/$id.desktop" "/usr/local/share/icons/hicolor/scalable/apps/$id.svg"
  done
  if command -v semanage >/dev/null 2>&1 &&
    semanage fcontext -l 2>/dev/null | grep -q '^/var/lib/acer-wmi-modules(/\.\*)?'; then
    semanage fcontext -d '/var/lib/acer-wmi-modules(/.*)?' && echo "removed the SELinux label of /var/lib/acer-wmi-modules"
  fi
fi

if [ -n "$CHANGED" ]; then
  systemctl daemon-reload
  systemctl reload dbus.service 2>/dev/null || systemctl reload dbus-broker.service 2>/dev/null || true
  command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database -q /usr/local/share/applications || true
fi
if [ -n "$HWDB" ]; then
  systemd-hwdb update
  udevadm trigger --subsystem-match=input --action=change
fi
[ "$MODE" = old ] && exit 0

# Packages (RPM, .deb) own their files: only say how to remove them.
PKGS= REMOVE=
for p in acerctl acerctl-tray $APPS akmod-acerctl acerctl-kmod-common acerctl-dkms; do
  if command -v rpm >/dev/null 2>&1 && rpm -q "$p" >/dev/null 2>&1; then
    PKGS="$PKGS $p" REMOVE="sudo dnf remove"
    [ -e /run/ostree-booted ] && REMOVE="sudo rpm-ostree uninstall"
  elif command -v dpkg-query >/dev/null 2>&1 &&
    dpkg-query -W -f '${Status}' "$p" 2>/dev/null | grep -q 'install ok installed'; then
    PKGS="$PKGS $p" REMOVE="sudo apt purge"
  fi
done

# The companion modules stay loaded until reboot unless they go now; the ones a
# package installs (akmod, DKMS) are left to it.
if [ -z "$PKGS" ]; then
  for m in acer_wmi_extras acer_wmi_battery; do
    if [ -d "/sys/module/$m" ]; then rmmod "$m" && echo "unloaded $m" || echo "could not unload $m (gone after a reboot)"; fi
  done
fi

STATE="/var/lib/acerd /var/lib/nitrod /var/lib/nitrosense /etc/modprobe.d/acerctl.conf"
user_files() {
  for h in $(homes); do
    for f in "$h/.config/acerctl" "$h/.config/nitroctl"; do echo "$f"; done
    for id in $IDS; do echo "$h/.config/autostart/$id.desktop"; done
    for a in $APPS; do echo "$h/.cache/$a"; echo "$h/.local/share/$a"; done
  done
}

echo
if [ -n "$PKGS" ]; then
  echo "installed as packages (left in place):$PKGS"
  echo "  remove them with: $REMOVE$PKGS"
  echo "  or keep them and start the packaged daemon: sudo systemctl enable --now acerd"
  [ -n "$PURGE" ] && echo "  --purge skipped: the packages use the state and the settings; run it again after removing them"
  PURGE=
fi
if [ -n "$PURGE" ]; then
  if [ -e /etc/modprobe.d/acerctl.conf ]; then echo "acer_wmi keeps the options of acerctl.conf until the next reboot"; fi
  for f in $STATE $(user_files); do rm_path "$f"; done
else
  for f in $STATE $(user_files); do
    if [ -e "$f" ]; then echo "kept (sudo sh $0 --purge removes it): $f"; fi
  done
fi
for f in /etc/modprobe.d/*.conf; do
  if [ "$f" != /etc/modprobe.d/acerctl.conf ] && [ -f "$f" ] && grep -q '^[[:space:]]*options[[:space:]]\+acer_wmi' "$f"; then
    echo "kept (not written by acerctl): acer_wmi options in $f"
  fi
done
echo "acerctl removed"
