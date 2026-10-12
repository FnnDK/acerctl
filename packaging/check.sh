#!/bin/sh
# Read-only health check of the kernel layer (run anywhere; no root needed).
S=/sys/devices/platform/acer-wmi-extras
echo "kernel: $(uname -r)"
echo "acer_wmi: predator_v4=$(cat /sys/module/acer_wmi/parameters/predator_v4 2>/dev/null) force_caps=$(cat /sys/module/acer_wmi/parameters/force_caps 2>/dev/null)"
for m in acer_wmi acer_wireless acer_wmi_battery acer_wmi_extras acer_wmi_lab; do [ -d /sys/module/$m ] && echo "loaded: $m" || echo "absent: $m"; done
pp=$(ls -d /sys/class/platform-profile/platform-profile-* 2>/dev/null | head -1)
[ -n "$pp" ] && echo "platform-profile: $(cat $pp/profile) [choices: $(cat $pp/choices)]" || echo "platform-profile: none"
for h in /sys/class/hwmon/hwmon*; do
  case "$(cat $h/name 2>/dev/null)" in
    acer) echo "hwmon acer: fan1=$(cat $h/fan1_input) fan2=$(cat $h/fan2_input) temp1=$(cat $h/temp1_input) pwm=$(ls $h | grep -c '^pwm[0-9]$') pwm1_enable=$(cat $h/pwm1_enable 2>/dev/null)";;
    acer_battery) echo "hwmon acer_battery: temp=$(cat $h/temp1_input) mV=$(cat $h/in0_input) mA=$(cat $h/curr1_input)";;
  esac
done
[ -d "$S" ] && echo "extras: timeout=$(cat $S/kbd_backlight_timeout) kbd_on=$(cat $S/kbd_backlight_state) usb=$(cat $S/usb_charging) profiles=[$(cat $S/supported_profiles)]"
B=/sys/bus/wmi/drivers/acer-wmi-battery
[ -d "$B" ] && echo "battery: health_mode=$(cat $B/health_mode) calibration=$(cat $B/calibration_mode) temp=$(cat $B/temperature)"
command -v systemctl >/dev/null && systemctl is-enabled acer-wmi-modules.service 2>/dev/null | sed 's/^/unit acer-wmi-modules: /'
command -v tuned-adm >/dev/null && tuned-adm active 2>/dev/null
exit 0
