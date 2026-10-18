#!/bin/sh
# Removes acerctl, however it was installed: the RPM/.deb packages (dnf,
# rpm-ostree, apt) and everything the install scripts put on the system
# (install-modules.sh, install-daemon.sh, install-gui.sh), including the
# installs made before the renames: nitrosense (nitrosensed, nitrosense-cli,
# org.nitrosense.*) and nitroctl (nitrod, nitroctl, nitro-tray, io.github.fnndk.Nitrod1).
#   sudo sh packaging/uninstall.sh           # keeps the state and the user settings
#   sudo sh packaging/uninstall.sh --purge   # also /var/lib/acerd, ~/.config/acerctl,
#                                            # the app data and /etc/modprobe.d/acerctl.conf
#   sudo sh packaging/uninstall.sh --scripts # only the script installs, packages and loaded
#                                            # modules stay (the README's install blocks run it)
#   sudo sh packaging/uninstall.sh --old     # only the installs from before the renames
#                                            # (install-daemon.sh and install-gui.sh run it)
set -eu

MODE=all PURGE=
for a in "$@"; do
  case $a in
    --purge) PURGE=1 ;;
    --scripts) MODE=scripts ;;
    --old) MODE=old ;;
    *) echo "usage: $0 [--purge | --scripts | --old]"; exit 2 ;;
  esac
done
[ "$MODE" != all ] && [ -n "$PURGE" ] && { echo "--$MODE keeps the state: use it without --purge"; exit 2; }
# host_root: these scripts change the system, so they need root on the host;
# sudo inside a toolbox/distrobox container only reaches the container.
host_root() {
  if [ -e /run/.containerenv ] || [ -e /.dockerenv ]; then
    echo "this shell is inside a container: run it on the host (distrobox-host-exec sudo sh $0 ...)"; exit 1
  fi
  [ "$(id -u)" -eq 0 ] || { echo "run as root (sudo sh $0 ...)"; exit 1; }
}

host_root

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

# drop_unit NAME [keep] stops, disables and removes a unit the scripts put in
# /etc/systemd/system; with keep it is not stopped (its ExecStop stays unrun).
drop_unit() {
  u=$1.service
  if [ -e "/etc/systemd/system/$u" ]; then
    if [ "${2:-}" = keep ]; then
      systemctl disable "$u" 2>/dev/null || true
    else
      systemctl disable --now "$u" 2>/dev/null || true
    fi
    rm_path "/etc/systemd/system/$u"
  fi
  # enable links left behind when the unit file went first (an interrupted install)
  for l in /etc/systemd/system/*.wants/"$u"; do
    if [ -L "$l" ] && [ ! -e "$l" ]; then rm_path "$l"; fi
  done
}

# stop_exe DIR NAME...: ends the processes running DIR/NAME, also once the file
# is gone (on Silverblue /usr/local/bin resolves to /var/usrlocal/bin)
stop_exe() {
  d=$1; shift
  ld=$(readlink -f "$d")
  for e in /proc/[0-9]*/exe; do
    t=$(readlink "$e" 2>/dev/null) || continue
    t=${t% (deleted)}
    for n in "$@"; do
      if [ "$t" = "$d/$n" ] || [ "$t" = "$ld/$n" ]; then
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
stop_exe /usr/local/bin nitro-tray
drop_unit nitrosensed
drop_unit nitrod
rm_path /usr/local/bin/nitrosensed /usr/local/bin/nitrosense-cli \
  /usr/local/bin/nitrod /usr/local/bin/nitroctl /usr/local/bin/nitro-tray \
  /etc/dbus-1/system.d/org.nitrosense.Daemon1.conf /etc/dbus-1/system.d/io.github.fnndk.Nitrod1.conf \
  /usr/local/share/polkit-1/actions/org.nitrosense.daemon.policy \
  /usr/local/share/polkit-1/actions/io.github.fnndk.nitrod.policy \
  /etc/xdg/autostart/io.github.fnndk.NitroTray.desktop

if [ "$MODE" != old ]; then
  stop_exe /usr/local/bin acer-tray $APPS
  drop_unit acerd
  # Stopping acer-wmi-modules unloads the modules. --scripts runs before the
  # packages go in, so the loaded ones stay until the packaged ones take over.
  if [ "$MODE" = scripts ]; then
    drop_unit acer-wmi-modules keep
  else
    drop_unit acer-wmi-modules
  fi
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
[ "$MODE" = scripts ] && { echo "script install removed"; exit 0; }

# Packages: their own package manager removes them (and their scriptlets stop
# and disable acerd). The akmods-built kmod-acerctl-<kernel> RPMs go too.
RPMS= DEBS= REBOOT=
for p in acerctl acerctl-tray $APPS akmod-acerctl acerctl-kmod-common acerctl-dkms; do
  if command -v rpm >/dev/null 2>&1 && rpm -q "$p" >/dev/null 2>&1; then
    RPMS="$RPMS $p"
  elif command -v dpkg-query >/dev/null 2>&1 &&
    dpkg-query -W -f '${Status}' "$p" 2>/dev/null | grep -q 'install ok installed'; then
    DEBS="$DEBS $p"
  fi
done
if [ -n "$RPMS" ] && [ -e /run/ostree-booted ]; then
  # Silverblue, Kinoite, Bluefin: gone from the next deployment; until the reboot
  # the packaged copies would keep running, so they stop now.
  # shellcheck disable=SC2086
  rpm-ostree uninstall --idempotent $RPMS
  systemctl disable --now acerd.service 2>/dev/null || true
  stop_exe /usr/bin acer-tray $APPS
  REBOOT=1
elif [ -n "$RPMS" ]; then
  RPMS="$RPMS $(rpm -qa --qf '%{NAME} ' 'kmod-acerctl*')"
  # shellcheck disable=SC2086
  dnf remove $RPMS || { echo "the packages were not removed; run it again to remove them"; exit 1; }
  stop_exe /usr/bin acer-tray $APPS
fi
if [ -n "$DEBS" ]; then
  # purge also drops the packages' state (/var/lib/acerd), as --purge does
  act=remove; [ -n "$PURGE" ] && act=purge
  # shellcheck disable=SC2086
  apt-get "$act" $DEBS || { echo "the packages were not removed; run it again to remove them"; exit 1; }
  stop_exe /usr/bin acer-tray $APPS
fi

for m in acer_wmi_extras acer_wmi_battery; do
  if [ -d "/sys/module/$m" ]; then rmmod "$m" && echo "unloaded $m" || echo "could not unload $m (gone after a reboot)"; fi
done

STATE="/var/lib/acerd /var/lib/nitrod /var/lib/nitrosense /etc/modprobe.d/acerctl.conf"
user_files() {
  for h in $(homes); do
    for f in "$h/.config/acerctl" "$h/.config/nitroctl"; do echo "$f"; done
    for id in $IDS; do echo "$h/.config/autostart/$id.desktop"; done
    for a in $APPS; do echo "$h/.cache/$a"; echo "$h/.local/share/$a"; done
  done
}

echo
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
if [ -n "$REBOOT" ]; then
  echo "acerctl removed; the packages go with the next reboot (systemctl reboot)"
else
  echo "acerctl removed"
fi
