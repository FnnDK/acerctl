#!/bin/sh
# Builds the companion modules (acer-wmi-extras, acer-wmi-battery) against the
# kernel headers of each supported distribution, in throwaway containers.
# Only compiles: nothing is loaded. Needs podman or docker (from a distrobox,
# the host's podman); CONTAINER overrides.
#   sh tools/modules-matrix.sh                 # every target below
#   sh tools/modules-matrix.sh debian:13 ...   # some of them
# Prints one line per target and keeps the logs in $OUT (default: .ci/modules).
# Exits 1 when a target fails; with STRICT=1 (CI) code warnings fail it too.
set -eu
R=$(cd "$(dirname "$0")/.." && pwd)
if [ -z "${CONTAINER:-}" ]; then
  if command -v podman >/dev/null 2>&1; then CONTAINER=podman
  elif command -v docker >/dev/null 2>&1; then CONTAINER=docker
  elif command -v distrobox-host-exec >/dev/null 2>&1; then CONTAINER="distrobox-host-exec podman"
  else echo "needs podman or docker"; exit 1; fi
fi
OUT=${OUT:-$R/.ci/modules}
MODULES="acer-wmi-extras acer-wmi-battery"

# target: image|setup command|glob of the kernel build directory
targets() {
  cat <<'EOF'
debian:12|apt-get update -qq && apt-get install -y -qq make gcc linux-headers-amd64|/usr/src/linux-headers-*-amd64
debian:13|apt-get update -qq && apt-get install -y -qq make gcc linux-headers-amd64|/usr/src/linux-headers-*-amd64
ubuntu:22.04|apt-get update -qq && apt-get install -y -qq make gcc linux-headers-generic|/usr/src/linux-headers-*-generic
ubuntu:24.04|apt-get update -qq && apt-get install -y -qq make gcc linux-headers-generic|/usr/src/linux-headers-*-generic
ubuntu:24.04-hwe|apt-get update -qq && apt-get install -y -qq make gcc linux-headers-generic-hwe-24.04|/usr/src/linux-headers-*-generic
ubuntu:26.04|apt-get update -qq && apt-get install -y -qq make gcc linux-headers-generic|/usr/src/linux-headers-*-generic
fedora:44|dnf -y -q install make gcc kernel-devel|/usr/src/kernels/*
EOF
}

mkdir -p "$OUT"
echo "logs: $OUT"
: > "$OUT/failed"
# a copy, so the bind mount does not relabel the checkout (SELinux :z)
mkdir -p "$OUT/src"
for m in $MODULES; do cp -r "$R/drivers/$m" "$OUT/src/"; done
targets | while IFS='|' read -r target setup kdirs; do
  if [ $# -gt 0 ]; then
    case " $* " in *" $target "*) ;; *) continue ;; esac
  fi
  image=${target%-hwe}
  case $image in fedora:*) image=registry.fedoraproject.org/$image ;; esac
  log="$OUT/$(echo "$target" | tr ':/' '__').log"
  # the newest kernel build directory the headers package installed
  build="for k in $kdirs; do kdir=\$k; done; echo \"kernel: \${kdir##*/}\";
    for m in $MODULES; do cp -r /src/\$m /tmp/\$m && make -s -C \"\$kdir\" M=/tmp/\$m modules || exit 1; done"
  if $CONTAINER run --rm -v "$OUT/src:/src:ro,z" "$image" sh -c "$setup >/dev/null 2>&1 && $build" >"$log" 2>&1; then
    printf '%-18s ok    %s\n' "$target" "$(grep -m1 '^kernel:' "$log")"
    # code warnings only; a different compiler or pahole in the container is expected
    if grep -E 'warning:' "$log" | grep -v -E 'compiler differs|pahole version' | sed 's/^/                    /' | grep .; then
      [ "${STRICT:-0}" = 1 ] && echo "$target (warnings)" >> "$OUT/failed"
    fi
  else
    echo "$target" >> "$OUT/failed"
    printf '%-18s FAIL  %s\n' "$target" "$(grep -m1 '^kernel:' "$log" || echo 'setup failed')"
    grep -m3 -E 'error|Error|E: ' "$log" | sed 's/^/                    /'
  fi
done
# the loop runs in a subshell (pipe): failures go through a file
[ -s "$OUT/failed" ] && { echo "failed: $(tr '\n' ' ' < "$OUT/failed")"; exit 1; }
exit 0
