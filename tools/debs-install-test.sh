#!/bin/sh
# Installs the .deb packages in throwaway Debian/Ubuntu containers and checks
# them: dependencies resolve, the binaries start, the app finds its libraries,
# DKMS builds the modules for the image's kernel headers, and removal works.
# No systemd runs in the containers, so the service is only installed, not
# started. Needs podman (or CONTAINER=docker).
#   sh tools/debs-install-test.sh DIR [image...]   # DIR holds the .deb files
set -eu
DEBS=$(cd "$1" && pwd); shift
CONTAINER=${CONTAINER:-podman}
OUT=${OUT:-$(mktemp -d)}
IMAGES=${*:-"debian:12 debian:13 ubuntu:22.04 ubuntu:24.04 ubuntu:26.04"}
mkdir -p "$OUT"
echo "logs: $OUT"
# shellcheck disable=SC2016
CHECK='set -e
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
case $(. /etc/os-release; echo $ID) in debian) H=linux-headers-amd64 ;; *) H=linux-headers-generic ;; esac
apt-get install -y -qq $H >/dev/null
cd /debs
apt-get install -y -qq ./acerctl_*.deb ./acerctl-tray_*.deb ./nitro-control-center_*.deb ./acerctl-dkms_*.deb >/dev/null
echo "acerd $(acerd --version)"
acerctl help >/dev/null && echo "acerctl ok"
missing=$(ldd /usr/bin/nitro-control-center /usr/bin/acerd /usr/bin/acer-tray | grep "not found" || true)
[ -z "$missing" ] && echo "libraries ok" || { echo "missing: $missing"; exit 1; }
V=$(dpkg-query -W -f="\${Version}" acerctl-dkms)
K=$(ls /lib/modules | sort -V | tail -1)
dkms build -m acerctl -v "$V" -k "$K" >/tmp/dkms.log 2>&1 || { tail -20 /tmp/dkms.log; exit 1; }
echo "dkms built for $K: $(ls /var/lib/dkms/acerctl/$V/$K/x86_64/module/ | tr "\n" " ")"
test -f /usr/lib/systemd/system/acerd.service && test -f /usr/share/dbus-1/system-services/io.github.fnndk.Acerd1.service
apt-get purge -y -qq nitro-control-center acerctl-tray acerctl acerctl-dkms >/dev/null
! test -e /usr/bin/acerd && echo "purge ok"'
for img in $IMAGES; do
  log="$OUT/$(echo "$img" | tr ':/' '__').log"
  if $CONTAINER run --rm -v "$DEBS:/debs:ro,z" "$img" sh -c "$CHECK" >"$log" 2>&1; then
    printf '%-14s ok    %s\n' "$img" "$(grep -m1 '^dkms built' "$log")"
  else
    printf '%-14s FAIL\n' "$img"
    tail -6 "$log" | sed 's/^/               /'
  fi
done
