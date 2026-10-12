#!/bin/sh
# Second read-only probe batch (corrected inputs + sweeps). Run as root on the host.
set -u
CALL=/sys/devices/platform/acer-wmi-lab/call
OUT=${1:-$(cd "$(dirname "$0")" && pwd)/probe2-$(date +%Y%m%d-%H%M%S).txt}
G=7A4DDFE7-5B5D-40B4-8595-4408E0CC7F56
A=61EF69EA-865C-4BC3-A502-A0DEBA0CB531
B=79772EC5-04B1-4BFD-843C-61E7F77B6CC9
[ -w "$CALL" ] || { echo "acer-wmi-lab not loaded ($CALL)"; exit 1; }
: > "$OUT"
call() { printf '### %s\n' "$1" >> "$OUT"; printf '%s %s %s\n' "$2" "$3" "$4" > "$CALL" 2>>"$OUT" || printf 'write error\n' >> "$OUT"; cat "$CALL" >> "$OUT"; printf '\n' >> "$OUT"; }

# fan behavior with fan-id bitmaps (CPU id1 -> bit0, GPU id4 -> bit3)
call "FanBehavior bitmap 0x08 (GPU)"        $G 15 "08000000"
call "FanBehavior bitmap 0x09 (CPU+GPU)"    $G 15 "09000000"
call "FanBehavior bitmap 0x02"              $G 15 "02000000"
call "FanBehavior bitmap 0x10"              $G 15 "10000000"
# fan speed with other ids
for id in 2 3 5 8; do call "FanSpeed fan id $id" $G 17 "$(printf '%02x000000' $id)"; done
# keyboard backlight: alternative inputs
for h in 00010000 01010000 03000000 04000000 10000000 ff000000 00000100 01000100; do
  call "GetGamingKBBacklight in=$h" $G 21 "$h"
done
# RGB kb: alternative inputs
for h in 00000000 0f000000 ff000000 00010000; do call "GetGamingRgbKb in=$h" $G 7 "$h"; done
# gaming profile: more inputs
for h in 02000000 03000000 ff000000 00010000; do call "GetGamingProfile in=$h" $G 3 "$h"; done
# misc settings 21..40 and high indexes
i=21; while [ $i -le 40 ]; do call "MiscSetting index $i" $G 23 "$(printf '%02x000000' $i)"; i=$((i+1)); done
for i in 48 64 80 96 112 128 160 192 224 240 255; do call "MiscSetting index $i" $G 23 "$(printf '%02x000000' $i)"; done
# misc setting 8 and 7 with a sub-index in byte1
for h in 08010000 08020000 07010000; do call "MiscSetting in=$h" $G 23 "$h"; done
# fan table with other inputs
for h in 01000000 02000000 ff000000; do call "GetGamingFanTable in=$h" $G 19 "$h"; done
# APGe function sweep 0x09..0x20 (function id in byte0)
i=9; while [ $i -le 32 ]; do call "APGe GetFunction fn $i" $A 2 "$(printf '%02x000000' $i)"; i=$((i+1)); done
# APGe function 1 with other sub-parameters (0x88401 known = backlight timeout)
for h in 01840800 01840000 01800800 01000800 01840900 01850800 01840a00 02840800 03840800; do call "APGe GetFunction in=$h" $A 2 "$h"; done
# APGe function 3 and 7 variations
for h in 03010000 03020000 07010000 07020000; do call "APGe GetFunction in=$h" $A 2 "$h"; done
# battery info indexes 4..12
i=4; while [ $i -le 12 ]; do call "Battery GetBattInfoInterface idx $i batt1" $B 19 "$(printf '%02x00000001000000' $i)"; i=$((i+1)); done
call "Battery GetBatteryFunctionData mask 4"  $B 22 "04000000"
call "Battery GetBatteryFunctionData mask 0"  $B 22 "00000000"
call "Battery HealthControlStatus query 2"    $B 20 "01020000"
call "Battery HealthControlStatus query 3"    $B 20 "01030000"
[ -n "${SUDO_USER:-}" ] && chown "$SUDO_USER": "$OUT"
echo "wrote $OUT ($(grep -c '^###' "$OUT") calls)"
