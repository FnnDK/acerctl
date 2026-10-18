#!/bin/sh
# Runs acerd against one fixture tree (daemon/internal/hw/testdata/models/*/root)
# and prints a transcript: the status, a fixed list of acerctl commands with
# their output and exit status, the status again and every file acerd wrote.
# tools/models-test.sh runs it on the installed packages in containers and
# compares the transcript with the model's acerctl.txt; --update writes those
# from the binaries built here.
#   sh tools/models-run.sh FIXTURE_ROOT
# ACERD and ACERCTL name the binaries (default: from PATH); BUS=--session uses
# the session bus, otherwise the system bus (root, as acerd.service runs).
set -eu
SRC=$(cd "$1" && pwd)
ACERD=${ACERD:-acerd} ACERCTL=${ACERCTL:-acerctl} BUS=${BUS:-}
W=$(mktemp -d)
PID=
trap 'if [ -n "$PID" ]; then kill "$PID" 2>/dev/null; wait "$PID" 2>/dev/null; fi; rm -rf "$W"' EXIT
cp -a "$SRC" "$W/root"
mkdir "$W/state"

$ACERD $BUS --no-polkit --root "$W/root" --state "$W/state/state.json" >"$W/acerd.log" 2>&1 &
PID=$!
i=0
until grep -q "acerd ready" "$W/acerd.log"; do
  i=$((i + 1))
  if [ $i -gt 100 ] || ! kill -0 "$PID" 2>/dev/null; then
    echo "acerd did not start:" >&2
    cat "$W/acerd.log" >&2
    exit 1
  fi
  sleep 0.1
done
sleep 1 # the state restore runs right after "ready"

# The version is the build's, not the model's.
status() { $ACERCTL $BUS --json status 2>&1 | sed 's/"Version": "[^"]*"/"Version": "VERSION"/'; }
run() {
  echo "\$ acerctl $*"
  if out=$($ACERCTL $BUS "$@" 2>&1); then rc=0; else rc=$?; fi
  [ -z "$out" ] || printf '%s\n' "$out"
  echo "[exit $rc]"
}

echo "# status"
status
echo "# commands"
run setup
run mode quiet
run mode performance
run fan all max
run fan cpu 40
run fan all auto
run kbd timeout 0
run usb-charging 20
run battery limit off
run battery limit on
run battery calibrate start
echo "# status after"
status
echo "# files written"
(cd "$W/root" && find . -type f | sort) | while read -r f; do
  cmp -s "$W/root/$f" "$SRC/$f" 2>/dev/null || printf '%s = %s\n' "${f#.}" "$(cat "$W/root/$f")"
done
