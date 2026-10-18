#!/bin/sh
# acerctl hardware report: collects what the firmware and the kernel expose on
# an Acer laptop, so support for it can be added without access to the
# machine. Run it as root on the laptop and attach the resulting .tar.gz to a
# "Hardware report" issue (see docs/hardware-reports.md):
#
#   sudo sh tools/survey.sh --probes    # recommended: plus read-only firmware calls
#   sudo sh tools/survey.sh             # data only, nothing to build
#
# Guarantees:
# - Read-only. Nothing is written to the firmware and no setting is changed.
#   --probes calls only "Get" WMI methods through drivers/acer-wmi-lab, which
#   is loaded for the duration of the run and unloaded afterwards.
# - No personal data: no hostname, user name, serial numbers, UUIDs, MAC
#   addresses or the MSDM table (Windows product key). Every file is plain
#   text or a raw firmware table and can be reviewed before sending.
set -u
REPORT_VERSION=3
HERE=$(cd "$(dirname "$0")" && pwd)
LAB_SRC=$HERE/../drivers/acer-wmi-lab
CALL=/sys/devices/platform/acer-wmi-lab/call
ISSUE_URL=https://github.com/FnnDK/acerctl/issues/new?template=hardware-report.yml

usage() {
  cat <<EOF
usage: sudo sh $0 [--probes] [--lab-module FILE.ko] [-o DIR]
  --probes             recommended: also run read-only firmware calls (builds and loads
                       drivers/acer-wmi-lab temporarily; needs make, a C
                       compiler and the kernel headers, or --lab-module)
  --lab-module FILE    use an already built acer-wmi-lab.ko (implies --probes)
  -o DIR               where to write the report (default: current directory)
EOF
}

PROBES=0 LAB_KO="" DEST=$PWD
while [ $# -gt 0 ]; do
  case "$1" in
    --probes) PROBES=1 ;;
    --lab-module) shift; LAB_KO=${1:-}; PROBES=1 ;;
    -o) shift; DEST=${1:-$PWD} ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; exit 2 ;;
  esac
  shift
done
[ "$(id -u)" = 0 ] || echo "warning: not root; DMI, ACPI tables, BMOF and dmesg will be missing" >&2

MODEL=$(tr -c 'A-Za-z0-9._\n-' '_' < /sys/class/dmi/id/product_name 2>/dev/null || echo unknown)
NAME=acer-report-$MODEL-$(date +%Y%m%d-%H%M%S)
OUT=$DEST/$NAME
mkdir -p "$OUT" || exit 1

# sec <file> <command...>: run a command, keep stdout+stderr in <file>.
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
val() { cat "$1" 2>/dev/null | tr -d '\n'; }

echo "collecting into $OUT"

# --- system ---------------------------------------------------------------
sec system.txt uname -srvm
sec system.txt cat /etc/os-release
# Mask disk/LVM names (may carry the hostname), UUIDs and the ostree deployment hash.
sec system.txt sh -c 'sed -E -e "s/(root|resume|rd\.lvm\.[a-z]+|rd\.luks\.[a-z]+)=[^ ]*/\1=x/g" -e "s/[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}/x/g" -e "s/[0-9a-f]{32,}/x/g" /proc/cmdline'
dump system.txt /sys/kernel/security/lockdown /sys/module/module/parameters/sig_enforce
have mokutil && sec system.txt mokutil --sb-state
have lscpu && sec system.txt sh -c 'lscpu | grep -E "^(Model name|Vendor ID|CPU\(s\)|Thread|Core)"'
have lspci && sec system.txt lspci -nnk
have lsusb && sec system.txt lsusb
sec system.txt sh -c 'lsmod | grep -iE "acer|wmi|platform_profile|nvidia|nouveau|amdgpu|i915|xe"'
sec system.txt sh -c 'grep -rsn -iE "acer|wmi" /etc/modprobe.d /usr/lib/modprobe.d /run/modprobe.d'
# Other Acer tools (Linuwu-Sense, Div-Acer-Manager-Max, acer-predator-turbo):
# they blacklist acer_wmi or make their own WMI calls, which can corrupt ours.
other_tools() {
  for m in linuwu_sense facer; do
    [ -d /sys/module/$m ] && printf 'module %s loaded; ' "$m"
  done
  if have dkms; then
    d=$(dkms status 2>/dev/null | grep -iE 'linuwu|facer|damx|predator' | cut -d, -f1 | sort -u | paste -sd ' ' -)
    [ -n "$d" ] && printf 'dkms: %s; ' "$d"
  fi
  if have systemctl; then
    u=$(systemctl list-unit-files --no-legend 2>/dev/null | awk '{print $1}' | grep -iE 'damx|linuwu|facer|predator|nitrosense' | paste -sd ' ' -)
    [ -n "$u" ] && printf 'units: %s; ' "$u"
  fi
  f=$(for x in /etc/modprobe.d/* /etc/modules-load.d/* /usr/lib/modules-load.d/*; do
        [ -f "$x" ] && grep -qiE 'linuwu|facer' "$x" && printf '%s\n' "$x"; done | paste -sd ' ' -)
  [ -n "$f" ] && printf 'files: %s; ' "$f"
  return 0
}
sec system.txt other_tools
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
  printf '## %s (%s)\n' "$h" "$(val "$h/name")" >> "$OUT/hwmon.txt"
  dump hwmon.txt "$h"/*_input "$h"/*_label "$h"/pwm* "$h"/*_max "$h"/*_crit
done

for p in /sys/class/power_supply/*; do
  printf '## %s\n' "$p" >> "$OUT/power_supply.txt"
  ls "$p" >> "$OUT/power_supply.txt" 2>&1
  for a in "$p"/*; do
    case "$(basename "$a")" in *serial*) continue;; esac
    dump power_supply.txt "$a"
  done
done

for l in /sys/class/leds/*; do
  dump leds.txt "$l/brightness" "$l/max_brightness" "$l/trigger"
done

dump misc.txt /sys/class/firmware-attributes/*/attributes/*/current_value
sec misc.txt sh -c 'ls /sys/class/firmware-attributes/ 2>&1'
have rfkill && sec misc.txt rfkill list
# Input devices without the Uniq line (Bluetooth addresses, serials).
sec input.txt sh -c 'grep -v "^U: Uniq=" /proc/bus/input/devices'
# HID devices: keyboards and lighting controllers (vendor:product, name).
sec hid.txt sh -c 'for d in /sys/bus/hid/devices/*; do grep -hE "^HID_(ID|NAME)=" $d/uevent 2>/dev/null | tr "\n" " "; echo; done'
# HID report descriptors: some models switch modes or lighting through a
# vendor collection of the embedded controller's HID device (Acer = 1025).
for d in /sys/bus/hid/devices/*; do
  [ -r "$d/report_descriptor" ] || continue
  printf '## %s %s\n' "$(basename "$d")" "$(grep -h '^HID_NAME=' "$d/uevent" 2>/dev/null)" >> "$OUT/hid.txt"
  od -An -tx1 -v "$d/report_descriptor" >> "$OUT/hid.txt"
done
have dmesg && sec dmesg.txt sh -c \
  'dmesg | grep -iE "acer|wmi|platform.profile|ec:|thermal|battery|ACPI (BIOS )?Error"'

# --- optional: read-only firmware calls through acer-wmi-lab --------------
G=7A4DDFE7-5B5D-40B4-8595-4408E0CC7F56   # AcerGamingFunction
A=61EF69EA-865C-4BC3-A502-A0DEBA0CB531   # APGeAction (method 2 = GetFunction)
B=79772EC5-04B1-4BFD-843C-61E7F77B6CC9   # battery control
U=FE1DBBDA-3014-4856-870C-5B3A744BF341   # UtilityFunction
E=1EA0BC57-521E-461B-A7A9-5D36F141A82A   # AcerGenericMethod

# Only "Get" methods; method IDs as used by acer_wmi and the public Acer
# Linux projects. Format: <guid var> <method> <input hex> <label>.
probe_list() {
  cat <<EOF
G 5 00000000 GetGamingSysInfo supported sensors
G 5 02000000 GetGamingSysInfo AC status
G 5 01010000 GetGamingSysInfo sensor 1 (CPU temp)
G 5 01020000 GetGamingSysInfo sensor 2 (CPU fan)
G 5 01060000 GetGamingSysInfo sensor 6 (GPU fan)
G 5 010a0000 GetGamingSysInfo sensor 10 (GPU temp)
G 3 00000000 GetGamingProfile 0
G 3 01000000 GetGamingProfile 1
G 9 00000000 GetGamingProfileSetting 0
G 9 01000000 GetGamingProfileSetting 1
G 4 00000000 GetGamingLED 0
G 4 01000000 GetGamingLED 1
G 11 00000000 GetGamingLEDBehavior 0
G 13 00000000 GetGamingLEDColor 0
G 15 01000000 GetGamingFanBehavior CPU
G 15 04000000 GetGamingFanBehavior GPU
G 15 08000000 GetGamingFanBehavior bitmap 0x08
G 17 01000000 GetGamingFanSpeed CPU
G 17 04000000 GetGamingFanSpeed GPU
G 19 - GetGamingFanTable
G 21 00000000 GetGamingKBBacklight 0
G 21 01000000 GetGamingKBBacklight 1
G 7 01000000 GetGamingRgbKb zone 1
G 7 0f000000 GetGamingRgbKb all zones
G 25 00000000 GetCPUOverclockingProfile
A 2 01840800 APGe GetFunction keyboard backlight timeout
A 2 04000000 APGe GetFunction USB charging
A 2 07000000 APGe GetFunction 7
E 2 01000000 Generic GetFunction 1
E 2 02000000 Generic GetFunction 2
B 20 01010000 Battery health control status
B 20 01020000 Battery health control query 2
B 22 01000000 Battery function data mask 1
B 22 02000000 Battery function data mask 2
B 19 0000000001000000 Battery info idx 0
B 19 0100000001000000 Battery info idx 1
B 19 0200000001000000 Battery info idx 2
B 19 0300000001000000 Battery info idx 3
B 19 0800000001000000 Battery info idx 8 (temperature)
B 19 0900000001000000 Battery info idx 9 (voltage)
B 19 0a00000001000000 Battery info idx 10 (current)
U 11 00000000 Utility LED indicator status
U 14 00000000 Utility second display status
EOF
  i=0
  while [ $i -le 40 ]; do
    printf 'G 23 %02x000000 GetGamingMiscSetting index %d\n' $i $i
    i=$((i+1))
  done
  for i in 1 2 3 5 6 8 9 10 11 12; do
    printf 'A 2 %02x000000 APGe GetFunction %d\n' $i $i
  done
}

probe() {
  : > "$OUT/probes.txt"
  probe_list | while read -r g m hex label; do
    eval "guid=\$$g"
    [ "$hex" = - ] && hex=""
    printf '### %s\n' "$label" >> "$OUT/probes.txt"
    printf '%s %s %s\n' "$guid" "$m" "$hex" > "$CALL" 2>>"$OUT/probes.txt" || printf 'write error\n' >> "$OUT/probes.txt"
    cat "$CALL" >> "$OUT/probes.txt"; printf '\n' >> "$OUT/probes.txt"
  done
}

PROBE_NOTE="not requested (run again with --probes: it is the most useful part)"
LOADED_BY_US=0
if [ "$PROBES" = 1 ]; then
  if pgrep -x acerd >/dev/null 2>&1; then
    # Concurrent WMI calls corrupt each other's results on some firmware.
    PROBE_NOTE="skipped: acerd is running (stop it with 'systemctl stop acerd' and run again)"
  elif [ -d /sys/module/linuwu_sense ] || [ -d /sys/module/facer ]; then
    PROBE_NOTE="skipped: another Acer WMI module is loaded (linuwu_sense/facer); unload it or its service and run again"
  else
    if [ ! -w "$CALL" ] && [ -z "$LAB_KO" ]; then
      KDIR=/lib/modules/$(uname -r)/build
      [ -d "$KDIR" ] || KDIR=/usr/src/kernels/$(uname -r)
      if ! have make || { ! have cc && ! have gcc; }; then
        PROBE_NOTE="skipped: no make/C compiler here; build drivers/acer-wmi-lab elsewhere (e.g. a toolbox) and pass --lab-module"
      elif [ ! -d "$KDIR" ]; then
        PROBE_NOTE="skipped: no kernel headers for $(uname -r) (install kernel-devel/linux-headers, or use --lab-module)"
      elif [ ! -f "$LAB_SRC/acer-wmi-lab.c" ]; then
        PROBE_NOTE="skipped: drivers/acer-wmi-lab not found next to this script (clone the whole repository)"
      else
        BUILD=$(mktemp -d)
        cp "$LAB_SRC/acer-wmi-lab.c" "$LAB_SRC/Makefile" "$BUILD/"
        if make -C "$BUILD" KDIR="$KDIR" > "$OUT/lab-build.log" 2>&1; then
          LAB_KO=$BUILD/acer-wmi-lab.ko
        else
          PROBE_NOTE="skipped: building acer-wmi-lab failed (see lab-build.log)"
        fi
      fi
    fi
    if [ ! -w "$CALL" ] && [ -n "$LAB_KO" ]; then
      if insmod "$LAB_KO" 2> "$OUT/lab-load.log"; then
        LOADED_BY_US=1
      elif [ "$(val /sys/module/module/parameters/sig_enforce)" = Y ]; then
        PROBE_NOTE="skipped: Secure Boot only accepts signed modules (sign acer-wmi-lab.ko with an enrolled MOK, or disable Secure Boot for the test)"
      else
        PROBE_NOTE="skipped: insmod failed (see lab-load.log)"
      fi
    fi
    if [ -w "$CALL" ]; then
      echo "running read-only firmware calls"
      probe
      PROBE_NOTE="run: $(grep -c '^###' "$OUT/probes.txt") calls, $(grep -c 'AE_OK' "$OUT/probes.txt") returned AE_OK"
    fi
    [ "$LOADED_BY_US" = 1 ] && rmmod acer_wmi_lab 2>/dev/null
    [ -n "${BUILD:-}" ] && rm -rf "$BUILD"
  fi
fi

# --- summary --------------------------------------------------------------
# dmi_aa: DMI type 170 (0xAA, the hotkey function table acer_wmi reads) and
# whether it lists the keyboard backlight, decoded as acer-wmi-extras does:
# after the header, five u16 bitmaps of the functions present (communication,
# application, media, display, others), then { code, 2, u16 bit } entries;
# code 0x84 is the keyboard backlight.
dmi_aa() {
  have dmidecode || { echo "unknown (no dmidecode)"; return; }
  # shellcheck disable=SC2046
  set -- $(dmidecode -t 170 -u 2>/dev/null |
    awk '/Header and Data:/ { on = 1; next } on && /^\t\t[0-9A-Fa-f][0-9A-Fa-f]( |$)/ { print; next } on { exit }')
  [ $# -ge 14 ] || { echo "absent"; return; }
  others=$(( 0x${13} | (0x${14} << 8) ))
  shift 14
  while [ $# -ge 4 ]; do
    if [ $((0x$1)) -eq 132 ] && [ $((0x$2)) -eq 2 ]; then
      bit=$(( 0x$3 | (0x$4 << 8) ))
      if [ $((others & bit)) -ne 0 ]; then kb=present; else kb=absent; fi
      printf 'present; keyboard backlight %s (others 0x%04x, 0x84 -> bit 0x%04x)\n' "$kb" "$others" "$bit"
      return
    fi
    shift 4
  done
  printf 'present; keyboard backlight not listed (others 0x%04x)\n' "$others"
}
guid_name() {
  case "$1" in
    7A4DDFE7-5B5D-40B4-8595-4408E0CC7F56) echo "AcerGamingFunction (modes, fans, sensors, lighting)" ;;
    61EF69EA-865C-4BC3-A502-A0DEBA0CB531) echo "APGeAction (keyboard timeout, USB charging)" ;;
    79772EC5-04B1-4BFD-843C-61E7F77B6CC9) echo "battery control (charge limit, calibration)" ;;
    FE1DBBDA-3014-4856-870C-5B3A744BF341) echo "UtilityFunction" ;;
    1EA0BC57-521E-461B-A7A9-5D36F141A82A) echo "AcerGenericMethod" ;;
    676AA15E-6A47-4D9F-A2CC-1E6D18D14026) echo "acer_wmi events (hotkeys)" ;;
    6AF4F258-B401-42FD-BE91-3D4AC2D7C0D3) echo "acer_wmi WMID v1" ;;
    95764E09-FB56-4E83-B31A-37761F60994A) echo "acer_wmi WMID device detection" ;;
    67C3371D-95A3-4C37-BB61-DD47B491DAAB) echo "acer_wmi AMW0" ;;
    05901221-D566-11D1-B2F0-00A0C9062910) echo "binary MOF" ;;
    *) echo "?" ;;
  esac
}
{
  echo "acerctl hardware report v$REPORT_VERSION, $(date +%F)"
  echo
  echo "model:        $(val /sys/class/dmi/id/sys_vendor) $(val /sys/class/dmi/id/product_name) ($(val /sys/class/dmi/id/product_family))"
  echo "board:        $(val /sys/class/dmi/id/board_name)"
  echo "BIOS / EC:    $(val /sys/class/dmi/id/bios_version) ($(val /sys/class/dmi/id/bios_date)) / EC $(val /sys/class/dmi/id/ec_firmware_release)"
  echo "CPU:          $(grep -m1 'model name' /proc/cpuinfo | cut -d: -f2- | sed 's/^ //')"
  echo "GPU:          $(lspci 2>/dev/null | grep -E 'VGA|3D' | cut -d: -f3- | sed 's/^ //' | tr '\n' ';')"
  echo "system:       $(. /etc/os-release 2>/dev/null; echo "$PRETTY_NAME"), kernel $(uname -r)"
  echo "secure boot:  $(mokutil --sb-state 2>/dev/null | head -1 || echo unknown); sig_enforce=$(val /sys/module/module/parameters/sig_enforce)"
  echo
  if [ -d /sys/module/acer_wmi ]; then
    echo "acer_wmi:     loaded ($(for p in /sys/module/acer_wmi/parameters/*; do printf '%s=%s ' "$(basename "$p")" "$(val "$p")"; done))"
  else
    bl=$(grep -lsE '^[[:space:]]*(blacklist[[:space:]]+acer[_-]wmi|install[[:space:]]+acer[_-]wmi)' /etc/modprobe.d/* /usr/lib/modprobe.d/* /run/modprobe.d/* | tr '\n' ' ')
    grep -qE 'blacklist=[^ ]*acer[_-]wmi' /proc/cmdline && bl="$bl kernel command line"
    echo "acer_wmi:     not loaded${bl:+ (blocked by: $bl)}"
  fi
  pp=""; for d in /sys/class/platform-profile/*; do [ -d "$d" ] && pp="$pp$(val "$d/name"): $(val "$d/choices") [$(val "$d/profile")]; "; done
  echo "profiles:     ${pp:-none}"
  fans=""; for f in /sys/class/hwmon/hwmon*/fan*_input; do [ -r "$f" ] && fans="$fans$(val "$(dirname "$f")/name")/$(basename "$f")=$(val "$f") "; done
  echo "fans:         ${fans:-none visible}"
  bat=""; for b in /sys/class/power_supply/BAT*; do
    [ -d "$b" ] || continue
    bat="$bat$(basename "$b") $(val "$b/capacity")% $(val "$b/status")"
    for a in charge_control_start_threshold charge_control_end_threshold charge_types; do
      [ -r "$b/$a" ] && bat="$bat, $a=$(val "$b/$a")"
    done
    bat="$bat; "
  done
  echo "battery:      ${bat:-none}"
  kbd=$(ls -d /sys/class/leds/*kbd_backlight* 2>/dev/null | xargs -r -n1 basename | tr '\n' ' ')
  echo "kbd LED:      ${kbd:-none}"
  hid=$(grep -lsh '^HID_ID=.*:00001025:' /sys/bus/hid/devices/*/uevent | xargs -r -n1 dirname | xargs -r -n1 basename | tr '\n' ' ')
  echo "Acer HID:     ${hid:-none}"
  echo "DMI type 170: $(dmi_aa)"
  ot=$(other_tools)
  echo "other tools:  ${ot:-none found}"
  echo
  echo "WMI devices:"
  for d in /sys/bus/wmi/devices/*; do
    [ -d "$d" ] || continue
    g=$(val "$d/guid")
    drv=$(basename "$(readlink "$d/driver" 2>/dev/null)" 2>/dev/null)
    printf '  %s  %-16s %s\n' "$g" "${drv:--}" "$(guid_name "$g")"
  done
  echo
  echo "firmware probes: $PROBE_NOTE"
} > "$OUT/summary.txt" 2>/dev/null

cat > "$OUT/README.txt" <<EOF
acerctl hardware report v$REPORT_VERSION. Every file can be opened with a text
editor except the raw firmware tables (acpi/*.dat, bmof-*.bin).

summary.txt         the short version; paste it into the issue
system.txt          kernel, distribution, Secure Boot, PCI/USB devices, modules
dmi.txt             model and BIOS identification (serials and UUIDs removed)
wmi.txt, bmof-*     WMI devices and the firmware's WMI class definitions
acpi/               DSDT and SSDT tables (MSDM, with the Windows key, is not copied)
acer_wmi.txt, platform_profile.txt, hwmon.txt, power_supply.txt, leds.txt,
misc.txt, input.txt, hid.txt, dmesg.txt
                    what the kernel already exposes for this laptop
probes.txt          read-only WMI calls (only with --probes)
issue-url.txt       link to a new "Hardware report" issue with the text fields filled in

Not collected: hostname, user name, serial numbers, UUIDs, MAC addresses.
EOF

# Last line of defense: mask anything shaped like a MAC address.
for f in "$OUT"/*.txt; do
  sed -i -E 's/([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}/xx:xx:xx:xx:xx:xx/g' "$f"
done

# Issue link with the form's text fields already filled in (GitHub prefills
# input/textarea fields from the URL; only the archive is left to attach).
urlencode() {
  LC_ALL=C awk 'BEGIN { for (i = 0; i < 256; i++) ord[sprintf("%c", i)] = i }
    { s = (NR > 1 ? "\n" : "") $0; for (i = 1; i <= length(s); i++) { c = substr(s, i, 1)
        if (c ~ /[A-Za-z0-9._~-]/) printf "%s", c; else printf "%%%02X", ord[c] } }'
}
PRODUCT="$(val /sys/class/dmi/id/sys_vendor) $(val /sys/class/dmi/id/product_name)"
LINK="$ISSUE_URL&title=$(printf 'Hardware report: %s' "$PRODUCT" | urlencode)"
LINK="$LINK&model=$(printf '%s' "$(val /sys/class/dmi/id/product_name)" | urlencode)"
SUMMARY_Q="&summary=$(urlencode < "$OUT/summary.txt")"
# Stay well below GitHub's URL limit (414 URI Too Long).
[ $(( ${#LINK} + ${#SUMMARY_Q} )) -lt 7000 ] && LINK="$LINK$SUMMARY_Q"
printf '%s\n' "$LINK" > "$OUT/issue-url.txt"

tar -czf "$OUT.tar.gz" -C "$DEST" "$NAME"
if [ -n "${SUDO_USER:-}" ]; then chown -R "$SUDO_USER": "$OUT" "$OUT.tar.gz"; fi
echo
cat "$OUT/summary.txt"
echo
echo "report: $OUT.tar.gz"
[ "$PROBES" = 1 ] || echo "tip: run 'sudo sh $0 --probes' instead; the firmware probes are the most useful part of the report"
echo "review it if you like, then open this link (model and summary already filled in)"
echo "and drag the .tar.gz into the form:"
echo
echo "$LINK"
