#!/bin/sh
# The model x distribution matrix: the packages of a release build (the RPMs
# and .debs in DIST, from tools/release.sh and tools/build-rpms.sh) installed
# in Fedora 44 and Ubuntu 24.04 containers, and in each one acerd run against
# the fixture tree of every known model (daemon/internal/hw/testdata/models:
# Nitro ANV15-52, Aspire AG15-71P, Predator PH16-71). For each pair:
#   - transcript: tools/models-run.sh on the system bus, with the packaged
#     binaries and D-Bus policy, against the model's acerctl.txt;
#   - boot modules: the companion modules built by akmods (Fedora) or DKMS
#     (Ubuntu) for the image's kernel, and the ones that load at boot, through
#     the WMI alias of one of the model's devices or modules-load.d, against
#     the model's boot-modules.txt;
#   - the D-Bus activation file is installed (a call without acerd tries to
#     start it instead of failing with ServiceUnknown).
#   sh tools/models-test.sh DIST [image...]   default: fedora:44 ubuntu:24.04
#   sh tools/models-test.sh --update          rewrite the acerctl.txt files (and the
#                                             expected.json of the Go test) from the
#                                             code here, on a private session bus
set -eu
R=$(cd "$(dirname "$0")/.." && pwd)
M="$R/daemon/internal/hw/testdata/models"
MODELS=$(cd "$M" && for d in */root; do echo "${d%/root}"; done)

if [ "${1:-}" = --update ]; then
  B=$(mktemp -d)
  trap 'rm -rf "$B"' EXIT
  (cd "$R/daemon" && go build -o "$B/acerd" ./cmd/acerd && go build -o "$B/acerctl" ./cmd/acerctl &&
    go test ./internal/hw -run TestModels -update)
  for m in $MODELS; do
    dbus-run-session -- env BUS=--session ACERD="$B/acerd" ACERCTL="$B/acerctl" \
      sh "$R/tools/models-run.sh" "$M/$m/root" >"$M/$m/acerctl.txt"
    echo "updated $m"
  done
  exit 0
fi

DIST=$(cd "${1:?usage: $0 DIST [image...] | --update}" && pwd); shift
IMAGES=${*:-"fedora:44 ubuntu:24.04"}
if [ -z "${CONTAINER:-}" ]; then
  if command -v podman >/dev/null 2>&1; then CONTAINER=podman
  elif command -v docker >/dev/null 2>&1; then CONTAINER=docker
  elif command -v distrobox-host-exec >/dev/null 2>&1; then CONTAINER="distrobox-host-exec podman"
  else echo "needs podman or docker"; exit 1; fi
fi
OUT=${OUT:-$R/.ci/models}

CHECK='set -e
. /etc/os-release
case $ID in
fedora)
  dnf -y -q install diffutils findutils kmod dbus-daemon kernel-core kernel-devel >/dev/null
  dnf -y -q install /dist/acerctl-[0-9]*.x86_64.rpm /dist/akmod-acerctl-*.rpm /dist/acerctl-kmod-common-*.rpm >/dev/null
  K=$(ls /usr/src/kernels | sort -V | tail -1)
  akmods --force --kernels "$K" >/out/modules.log 2>&1 || { tail -20 /out/modules.log; exit 1; }
  ;;
*)
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  apt-get install -y -qq linux-headers-generic diffutils kmod >/dev/null
  apt-get install -y -qq /dist/acerctl_*.deb /dist/acerctl-dkms_*.deb >/dev/null
  K=$(ls /lib/modules | sort -V | tail -1)
  V=$(dpkg-query -W -f="\${Version}" acerctl-dkms)
  dkms status -m acerctl -v "$V" -k "$K" | grep -q installed ||
    dkms install -m acerctl -v "$V" -k "$K" >/out/modules.log 2>&1 || { tail -20 /out/modules.log; exit 1; }
  ;;
esac
echo "kernel $K: $(modinfo -k "$K" -F version acer_wmi_battery >/dev/null && modinfo -k "$K" -n acer_wmi_battery acer_wmi_extras | tr "\n" " ")"

mkdir -p /run/dbus
dbus-uuidgen --ensure=/etc/machine-id
dbus-daemon --system --fork
if acerctl status >/out/activation.txt 2>&1; then echo "acerctl answered before acerd was started"; exit 1; fi
if grep -q "is not running" /out/activation.txt; then echo "no D-Bus activation file"; exit 1; fi

onboot=$(cat /usr/lib/modules-load.d/*.conf /etc/modules-load.d/*.conf 2>/dev/null | grep -x "acer_wmi_[a-z]*" || true)
for d in /models/*/root; do
  m=$(basename "$(dirname "$d")")
  aliases=$(cat "$d"/sys/bus/wmi/devices/*/modalias)
  { for a in $aliases; do modprobe -S "$K" -R "$a" 2>/dev/null || true; done; echo "$onboot"; } |
    grep -x "acer_wmi_[a-z]*" | sort -u >/out/$m.boot
  sh /models-run.sh "$d" >/out/$m.txt 2>&1 || echo "models-run.sh failed for $m" >>/out/$m.txt
done'

failed=
for img in $IMAGES; do
  o="$OUT/$(echo "$img" | tr ':/' '__')"
  rm -rf "$o" && mkdir -p "$o"
  if ! $CONTAINER run --rm -v "$DIST:/dist:ro,z" -v "$M:/models:ro,z" -v "$R/tools/models-run.sh:/models-run.sh:ro,z" \
    -v "$o:/out:z" "$img" sh -c "$CHECK" >"$o/setup.log" 2>&1; then
    printf '%-14s FAIL  setup (%s)\n' "$img" "$o/setup.log"
    tail -6 "$o/setup.log" | sed 's/^/               /'
    failed="$failed $img"
    continue
  fi
  for m in $MODELS; do
    bad=
    cmp -s "$M/$m/acerctl.txt" "$o/$m.txt" || bad="$bad transcript"
    cmp -s "$M/$m/boot-modules.txt" "$o/$m.boot" || bad="$bad boot-modules"
    if [ -z "$bad" ]; then
      printf '%-14s %-10s ok\n' "$img" "$m"
    else
      printf '%-14s %-10s FAIL:%s (diff -u %s %s)\n' "$img" "$m" "$bad" "$M/$m/acerctl.txt" "$o/$m.txt"
      failed="$failed $img/$m"
    fi
  done
done
[ -z "$failed" ] || { echo "failed:$failed"; exit 1; }
