#!/bin/sh
# Signs, loads and exercises acer-wmi-extras. Run as root on the host.
# The two writes are reversible and end in the current state (timeout 30 s, USB 30 %).
set -u
D=$(cd "$(dirname "$0")" && pwd)
K=/usr/src/kernels/$(uname -r)
S=/sys/devices/platform/acer-wmi-extras
rmmod acer_wmi_extras 2>/dev/null
"$K/scripts/sign-file" sha256 /etc/pki/akmods/private/private_key.priv /etc/pki/akmods/certs/public_key.der "$D/acer-wmi-extras.ko" || exit 1
insmod "$D/acer-wmi-extras.ko" || exit 1
sleep 0.5
dmesg | grep -E 'acer-wmi-extras' | tail -n 2
echo "--- attributes"; ls "$S" | grep -vE '^(driver|driver_override|modalias|power|subsystem|uevent|hwmon)$'
for a in kbd_backlight_timeout kbd_backlight_state usb_charging supported_profiles supported_profiles_mask; do
  [ -e "$S/$a" ] && printf '%-26s = %s\n' "$a" "$(cat "$S/$a")"
done
H=$(ls -d "$S"/hwmon/hwmon* 2>/dev/null | head -1)
if [ -n "$H" ]; then
  echo "--- hwmon $(cat "$H/name")"
  for f in temp1_input in0_input curr1_input; do printf '%-12s = %s\n' "$f" "$(cat "$H/$f" 2>&1)"; done
fi
echo "--- write test"
echo 0  > "$S/kbd_backlight_timeout" && printf 'timeout after 0  -> %s\n' "$(cat "$S/kbd_backlight_timeout")"
echo 30 > "$S/kbd_backlight_timeout" && printf 'timeout after 30 -> %s\n' "$(cat "$S/kbd_backlight_timeout")"
echo 20 > "$S/usb_charging" && printf 'usb after 20     -> %s\n' "$(cat "$S/usb_charging")"
echo 30 > "$S/usb_charging" && printf 'usb after 30     -> %s\n' "$(cat "$S/usb_charging")"
echo 15 > "$S/usb_charging" 2>/dev/null && echo "BUG: 15 accepted" || echo "usb 15 rejected (ok)"
echo "done"
