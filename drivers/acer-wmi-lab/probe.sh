#!/bin/sh
# Read-only probe batch for acer-wmi-lab. Run as root on the host.
# Only "Get" methods are called; nothing is written to the firmware.
set -u
CALL=/sys/devices/platform/acer-wmi-lab/call
OUT=${1:-$(cd "$(dirname "$0")" && pwd)/probe-$(date +%Y%m%d-%H%M%S).txt}
G=7A4DDFE7-5B5D-40B4-8595-4408E0CC7F56   # AcerGamingFunction (WMBH)
A=61EF69EA-865C-4BC3-A502-A0DEBA0CB531   # APGeAction (WMAA)
B=79772EC5-04B1-4BFD-843C-61E7F77B6CC9   # BatteryControl (WMBE)
U=FE1DBBDA-3014-4856-870C-5B3A744BF341   # UtilityFunction (WMBL)
E=1EA0BC57-521E-461B-A7A9-5D36F141A82A   # AcerGenericMethod (WMAE)

[ -w "$CALL" ] || { echo "acer-wmi-lab not loaded ($CALL)"; exit 1; }
: > "$OUT"
call() { # label guid method hex
  printf '### %s\n' "$1" >> "$OUT"
  printf '%s %s %s\n' "$2" "$3" "$4" > "$CALL" 2>>"$OUT" || printf 'write error\n' >> "$OUT"
  cat "$CALL" >> "$OUT"; printf '\n' >> "$OUT"
}

call "GetGamingSysInfo cmd0 supported sensors"   $G 5  "00000000"
call "GetGamingSysInfo cmd2 AC status"           $G 5  "02000000"
call "GetGamingSysInfo cmd1 sensor1 CPU temp"    $G 5  "01010000"
call "GetGamingSysInfo cmd1 sensor2 CPU fan"     $G 5  "01020000"
call "GetGamingSysInfo cmd1 sensor6 GPU fan"     $G 5  "01060000"
call "GetGamingSysInfo cmd1 sensor10 GPU temp"   $G 5  "010a0000"
call "GetGamingProfile(0)"                       $G 3  "00000000"
call "GetGamingProfile(1)"                       $G 3  "01000000"
call "GetGamingProfileSetting(0)"                $G 9  "00000000"
call "GetGamingProfileSetting(1)"                $G 9  "01000000"
call "GetGamingLED(0)"                           $G 4  "00000000"
call "GetGamingLED(1)"                           $G 4  "01000000"
call "GetGamingLEDBehavior(0)"                   $G 11 "00000000"
call "GetGamingLEDBehavior(1)"                   $G 11 "01000000"
call "GetGamingLEDColor(0)"                      $G 13 "00000000"
call "GetGamingLEDColor(1)"                      $G 13 "01000000"
call "GetGamingFanBehavior fan bitmap 1 (CPU)"   $G 15 "01000000"
call "GetGamingFanBehavior fan bitmap 4 (GPU)"   $G 15 "04000000"
call "GetGamingFanBehavior 0"                    $G 15 "00000000"
call "GetGamingFanSpeed fan 1 (CPU)"             $G 17 "01000000"
call "GetGamingFanSpeed fan 4 (GPU)"             $G 17 "04000000"
call "GetGamingFanTable (no input)"              $G 19 ""
call "GetGamingKBBacklight(1)"                   $G 21 "01000000"
call "GetGamingKBBacklight(0)"                   $G 21 "00000000"
call "GetGamingKBBacklight(2)"                   $G 21 "02000000"
call "GetGamingRgbKb zone1"                      $G 7  "01000000"
call "GetGamingRgbKb zone2"                      $G 7  "02000000"
call "GetGamingRgbKb zone3"                      $G 7  "04000000"
call "GetGamingRgbKb zone4"                      $G 7  "08000000"
i=0
while [ $i -le 20 ]; do
  h=$(printf '%02x000000' $i)
  call "GetGamingMiscSetting index $i" $G 23 "$h"
  i=$((i+1))
done
call "GetCPUOverclockingProfile"                 $G 25 "00000000"
call "APGe GetFunction 0x88401 (backlight timeout)" $A 2 "01840800"
call "APGe GetFunction 0x4 (USB charging)"       $A 2  "04000000"
call "APGe GetFunction 0x1"                      $A 2  "01000000"
call "APGe GetFunction 0x2"                      $A 2  "02000000"
call "APGe GetFunction 0x3"                      $A 2  "03000000"
call "APGe GetFunction 0x5"                      $A 2  "05000000"
call "APGe GetFunction 0x6"                      $A 2  "06000000"
call "APGe GetFunction 0x7"                      $A 2  "07000000"
call "APGe GetFunction 0x8"                      $A 2  "08000000"
call "Generic GetFunction 0x1"                   $E 2  "01000000"
call "Generic GetFunction 0x2"                   $E 2  "02000000"
call "Battery GetBatteryHealthControlStatus"     $B 20 "01010000"
call "Battery GetBatteryFunctionData mask 1"     $B 22 "01000000"
call "Battery GetBatteryFunctionData mask 2"     $B 22 "02000000"
call "Battery GetBattInfoInterface idx0 batt1"   $B 19 "0000000001000000"
call "Battery GetBattInfoInterface idx1 batt1"   $B 19 "0100000001000000"
call "Battery GetBattInfoInterface idx2 batt1"   $B 19 "0200000001000000"
call "Battery GetBattInfoInterface idx3 batt1"   $B 19 "0300000001000000"
call "Utility GetLedIndicatorSts(0)"             $U 11 "00000000"
call "Utility GetLedIndicatorSts(1)"             $U 11 "01000000"
call "Utility GetAcerDiagnosticStatus(0)"        $U 8  "0000"
call "Utility GetSecondDisplayDeviceStatus"      $U 14 "00000000"

[ -n "${SUDO_USER:-}" ] && chown "$SUDO_USER": "$OUT"
echo "wrote $OUT ($(grep -c '^###' "$OUT") calls)"
