#!/bin/sh
# Third batch: validates the APGe SET format with two reversible writes.
#  - keyboard backlight timeout: off -> on (30 s)   [restores the current state: on]
#  - USB charging off-state: 20 % -> 30 %            [restores the current state: 30 %]
# Run as root on the host. Watch the keyboard backlight: it must stay as it is.
set -u
CALL=/sys/devices/platform/acer-wmi-lab/call
OUT=${1:-$(cd "$(dirname "$0")" && pwd)/probe3-$(date +%Y%m%d-%H%M%S).txt}
A=61EF69EA-865C-4BC3-A502-A0DEBA0CB531   # APGeAction: method 1 = SetFunction, 2 = GetFunction
[ -w "$CALL" ] || { echo "acer-wmi-lab not loaded ($CALL)"; exit 1; }
: > "$OUT"
call() { printf '### %s\n' "$1" >> "$OUT"; printf '%s %s %s\n' "$2" "$3" "$4" > "$CALL" 2>>"$OUT" || printf 'write error\n' >> "$OUT"; cat "$CALL" >> "$OUT"; printf '\n' >> "$OUT"; sleep 0.3; }

call "KB timeout GET (before)"              $A 2 "01840800"
call "KB timeout SET off  (0x088402)"       $A 1 "0284080000000000"
call "KB timeout GET (after off)"           $A 2 "01840800"
call "KB timeout SET on 30s (0x1E0000088402)" $A 1 "02840800001e0000"
call "KB timeout GET (after on)"            $A 2 "01840800"

call "USB charging GET (before)"            $A 2 "04000000"
call "USB charging SET 20% (0x140F04)"      $A 1 "040f140000000000"
call "USB charging GET (after 20%)"         $A 2 "04000000"
call "USB charging SET 30% (0x1E0F04)"      $A 1 "040f1e0000000000"
call "USB charging GET (after 30%)"         $A 2 "04000000"

[ -n "${SUDO_USER:-}" ] && chown "$SUDO_USER": "$OUT"
echo "wrote $OUT ($(grep -c '^###' "$OUT") calls)"
