#!/bin/sh
# Read-only hardware survey of an Acer laptop, to decide what nitroctl can
# support on it. Run as root on the machine being surveyed:
#
#   sudo sh survey.sh [output dir]
#
# Nothing is written to the firmware, no module is loaded or reloaded and no
# setting is changed. If acer-wmi-lab is already loaded, the read-only probe
# batches (probe.sh, probe2.sh) are run too; probe3.sh is never run (it writes).
#
# Serial numbers, UUIDs and the MSDM table (Windows product key) are left out.
# The result is a directory plus a .tar.gz next to it; do not commit it.
set -u
HERE=$(cd "$(dirname "$0")" && pwd)
MODEL=$(tr -c 'A-Za-z0-9._\n-' '_' < /sys/class/dmi/id/product_name 2>/dev/null || echo unknown)
OUT=${1:-$PWD/acer-survey-$MODEL-$(date +%Y%m%d-%H%M%S)}
mkdir -p "$OUT" || exit 1
[ "$(id -u)" = 0 ] || echo "warning: not root; DMI, ACPI tables, BMOF and dmesg will be missing" >&2

# sec "<file>" <command...>: run a command, keep stdout+stderr in <file>.
sec() { f=$1; shift; { printf '$ %s\n' "$*"; "$@"; } >> "$OUT/$f" 2>&1; printf '\n' >> "$OUT/$f"; }
# dump <file> <paths...>: "path = value" for every readable regular file.
dump() {
  f=$1; shift
  for p in "$@"; do
    [ -f "$p" ] && [ -r "$p" ] || continue
    v=$(timeout 5 cat "$p" 2>&1 | tr '\n' ' ' | cut -c1-300)
    printf '%s = %s\n' "$p" "$v"
  done >> "$OUT/$f"
}
have() { command -v "$1" >/dev/null 2>&1; }

echo "collecting into $OUT"

# --- system ---------------------------------------------------------------
sec system.txt uname -a
sec system.txt cat /etc/os-release
sec system.txt sh -c 'sed "s/UUID=[^ ]*/UUID=x/g" /proc/cmdline'
dump system.txt /sys/kernel/security/lockdown /sys/module/module/parameters/sig_enforce
have mokutil && sec system.txt mokutil --sb-state
have mokutil && sec system.txt sh -c 'mokutil --list-enrolled | grep -E "Subject:|CN="'
have lscpu && sec system.txt lscpu
have lspci && sec system.txt lspci -nnk
have lsusb && sec system.txt lsusb
sec system.txt sh -c 'grep -rsn -iE "acer|wmi" /etc/modprobe.d /usr/lib/modprobe.d /run/modprobe.d'
sec system.txt sh -c 'lsmod | grep -iE "acer|wmi|platform_profile|nvidia|nouveau|i915|xe|amdgpu"'
have rpm-ostree && sec system.txt rpm-ostree status
have powerprofilesctl && sec system.txt powerprofilesctl
have tuned-adm && sec system.txt tuned-adm active

# --- DMI (no serials/UUIDs) ------------------------------------------------
for k in bios_vendor bios_version bios_date bios_release ec_firmware_release \
         sys_vendor product_name product_version product_family product_sku \
         board_vendor board_name board_version chassis_type chassis_vendor modalias; do
  dump dmi.txt /sys/class/dmi/id/$k
done
have dmidecode && sec dmi.txt sh -c \
  'dmidecode -t 0,1,2,3,11 | grep -viE "serial|uuid|asset tag"'
# OEM types 170-172: acer_wmi reads type 0xAA (170) for its capabilities.
have dmidecode && sec dmi.txt sh -c 'dmidecode -t 170,171,172 -u'

# --- WMI ------------------------------------------------------------------
for d in /sys/bus/wmi/devices/*; do
  [ -d "$d" ] || continue
  drv=$(basename "$(readlink "$d/driver" 2>/dev/null)" 2>/dev/null)
  printf '%s  driver=%s\n' "$(basename "$d")" "${drv:-none}" >> "$OUT/wmi.txt"
  dump wmi.txt "$d/guid" "$d/object_id" "$d/notify_id" "$d/instance_count" \
               "$d/expensive" "$d/setable" "$d/modalias"
  printf '\n' >> "$OUT/wmi.txt"
  # Binary MOF: the firmware's own WMI class and method definitions.
  [ -r "$d/bmof" ] && cat "$d/bmof" > "$OUT/bmof-$(basename "$d").bin" 2>/dev/null
done

# --- ACPI tables (DSDT/SSDT only; MSDM holds the Windows key) --------------
if [ -r /sys/firmware/acpi/tables/DSDT ]; then
  mkdir -p "$OUT/acpi"
  for t in /sys/firmware/acpi/tables/DSDT /sys/firmware/acpi/tables/SSDT*; do
    [ -r "$t" ] && cat "$t" > "$OUT/acpi/$(basename "$t").dat"
  done
fi
sec acpi-ids.txt sh -c 'for d in /sys/bus/acpi/devices/*; do h=$(cat $d/hid 2>/dev/null); case "$h" in 1025*|ACER*|PNP0C14|PNP0C09|PNP0C0A|ACPI0003) echo "$(basename $d) $h";; esac; done'

# --- kernel drivers already present ---------------------------------------
dump acer_wmi.txt /sys/module/acer_wmi/parameters/*
sec acer_wmi.txt sh -c 'ls -l /sys/devices/platform/acer-wmi/ 2>&1'
for d in /sys/class/platform-profile/*; do
  dump platform_profile.txt "$d/name" "$d/choices" "$d/profile"
done
dump platform_profile.txt /sys/firmware/acpi/platform_profile_choices /sys/firmware/acpi/platform_profile

for h in /sys/class/hwmon/hwmon*; do
  printf '## %s (%s)\n' "$h" "$(cat "$h/name" 2>/dev/null)" >> "$OUT/hwmon.txt"
  dump hwmon.txt "$h"/*_input "$h"/*_label "$h"/pwm* "$h"/*_max "$h"/*_crit
done

for p in /sys/class/power_supply/*; do
  printf '## %s\n' "$p" >> "$OUT/power_supply.txt"
  ls "$p" >> "$OUT/power_supply.txt" 2>&1
  for a in "$p"/*; do
    case "$(basename "$a")" in serial_number|*serial*) continue;; esac
    dump power_supply.txt "$a"
  done
done

for l in /sys/class/leds/*; do
  dump leds.txt "$l/brightness" "$l/max_brightness" "$l/trigger"
done

dump misc.txt /sys/class/firmware-attributes/*/attributes/*/current_value
sec misc.txt sh -c 'ls /sys/class/firmware-attributes/ 2>&1'
have rfkill && sec misc.txt rfkill list
sec input.txt cat /proc/bus/input/devices
have dmesg && sec dmesg.txt sh -c \
  'dmesg | grep -iE "acer|wmi|platform.profile|ec:|thermal|battery|ACPI (BIOS )?Error"'

# --- optional: firmware Get calls through acer-wmi-lab --------------------
if [ -w /sys/devices/platform/acer-wmi-lab/call ]; then
  echo "acer-wmi-lab loaded: running read-only probe batches"
  sh "$HERE/probe.sh" "$OUT/probe.txt"
  sh "$HERE/probe2.sh" "$OUT/probe2.txt"
else
  echo "acer-wmi-lab not loaded: firmware probes skipped (build, sign and insmod acer-wmi-lab.ko to include them)" \
    | tee "$OUT/probe-skipped.txt"
fi

tar -czf "$OUT.tar.gz" -C "$(dirname "$OUT")" "$(basename "$OUT")"
if [ -n "${SUDO_USER:-}" ]; then chown -R "$SUDO_USER": "$OUT" "$OUT.tar.gz"; fi
echo "done: $OUT.tar.gz"
