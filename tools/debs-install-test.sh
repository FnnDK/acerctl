#!/bin/sh
# Installs the .deb packages in throwaway Debian/Ubuntu containers the way a
# user does and checks them: the image gets a kernel without its headers (as
# a fresh Debian has it), then packaging/install.sh installs the packages,
# with its downloads served from DIR and its container check skipped, and
# must add the headers so the package's own DKMS postinst builds the modules
# for that kernel. Then: the binaries start, the app finds its libraries, and
# removal works. No systemd runs in the containers, so the service is only
# installed, not started. Needs podman or docker (from a distrobox, the host's
# podman); CONTAINER overrides. Logs in $OUT (default: .ci/debs).
#   sh tools/debs-install-test.sh DIR [image...]   # DIR holds the .deb files
# Exits 1 when an image fails.
set -eu
R=$(cd "$(dirname "$0")/.." && pwd)
DEBS=$(cd "$1" && pwd); shift
if [ -z "${CONTAINER:-}" ]; then
  if command -v podman >/dev/null 2>&1; then CONTAINER=podman
  elif command -v docker >/dev/null 2>&1; then CONTAINER=docker
  elif command -v distrobox-host-exec >/dev/null 2>&1; then CONTAINER="distrobox-host-exec podman"
  else echo "needs podman or docker"; exit 1; fi
fi
OUT=${OUT:-$R/.ci/debs}
IMAGES=${*:-"debian:12 debian:13 ubuntu:22.04 ubuntu:24.04 ubuntu:26.04"}
mkdir -p "$OUT"
echo "logs: $OUT"
# shellcheck disable=SC2016
CHECK='set -e
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
# a kernel metapackage without its headers (on Ubuntu linux-image-virtual,
# which leaves out the firmware and extra modules of linux-generic)
case $(. /etc/os-release; echo $ID) in debian) M=amd64 ;; *) M=virtual ;; esac
apt-get install -y -qq --no-install-recommends linux-image-$M >/dev/null
K=$(ls /lib/modules | sort -V | tail -1)
! test -e /lib/modules/$K/build || { echo "the image already has headers for $K"; exit 1; }
# uname -r answers the installed kernel, not the host kernel
printf "#!/bin/sh\n[ \"\$1\" = -r ] && echo %s || exec /usr/bin/uname \"\$@\"\n" "$K" >/usr/local/bin/uname
# install.sh downloads with curl (or wget): serve the files from /debs
cat >/usr/local/bin/curl <<"EOF"
#!/bin/sh
# called as: curl -fsSL -o FILE URL
n=${4##*/}
if [ "$n" = SHA256SUMS ]; then cp /tmp/SHA256SUMS "$3"; else cp "/debs/$n" "$3"; fi
EOF
chmod +x /usr/local/bin/uname /usr/local/bin/curl
(cd /debs && sha256sum ./*.deb | sed "s| \./| |") >/tmp/SHA256SUMS
V=$(ls /debs/acerctl_*_amd64.deb | sed "s|.*/acerctl_\(.*\)_amd64\.deb|\1|")
sed -e "s|^VERSION=.*|VERSION=$V|" -e "s|/run/\.containerenv|/nonexistent|" -e "s|/\.dockerenv|/nonexistent|" /install.sh >/tmp/install.sh
echo "APT::Get::Assume-Yes \"true\";" >/etc/apt/apt.conf.d/99yes
sh /tmp/install.sh --nitro >/tmp/install.log 2>&1 || { tail -30 /tmp/install.log; exit 1; }
echo "headers: $(dpkg-query -W -f="\${Package} " linux-headers-$K linux-headers-$M)"
dkms status -m acerctl -k "$K" | grep -q ": installed" || { grep -iE -A2 "warning|error" /tmp/install.log; exit 1; }
modinfo -k "$K" -n acer_wmi_extras acer_wmi_battery >/dev/null
echo "dkms built for $K: $(ls /lib/modules/$K/updates/dkms/ | tr "\n" " ")"
echo "acerd $(acerd --version)"
acerctl help >/dev/null && echo "acerctl ok"
missing=$(ldd /usr/bin/nitro-control-center /usr/bin/acerd /usr/bin/acer-tray | grep "not found" || true)
[ -z "$missing" ] && echo "libraries ok" || { echo "missing: $missing"; exit 1; }
test -f /usr/lib/systemd/system/acerd.service && test -f /usr/share/dbus-1/system-services/io.github.fnndk.Acerd1.service
apt-get purge -y -qq nitro-control-center acerctl-tray acerctl acerctl-dkms >/dev/null
! test -e /usr/bin/acerd && ! ls /lib/modules/$K/updates/dkms/acer-wmi-* >/dev/null 2>&1 && echo "purge ok"'
failed=
for img in $IMAGES; do
  log="$OUT/$(echo "$img" | tr ':/' '__').log"
  if $CONTAINER run --rm -v "$DEBS:/debs:ro,z" -v "$R/packaging/install.sh:/install.sh:ro,z" "$img" sh -c "$CHECK" >"$log" 2>&1; then
    printf '%-14s ok    %s\n' "$img" "$(grep -m1 '^dkms built' "$log")"
  else
    printf '%-14s FAIL\n' "$img"
    tail -6 "$log" | sed 's/^/               /'
    failed="$failed $img"
  fi
done
[ -z "$failed" ] || { echo "failed:$failed"; exit 1; }
