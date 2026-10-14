#!/bin/sh
# Builds the Fedora RPMs (packaging/rpm/acerctl.spec and the akmod) from the
# source archive of tools/release.sh, in a throwaway Fedora container, and
# copies them with their SRPMs into DIST.
#   sh tools/build-rpms.sh DIST VERSION
# The specs must carry VERSION; FORCE_VERSION=1 (tools/ci.sh, untagged
# commits) puts it in copies of them instead. Needs podman or docker (from a
# distrobox, the host's podman); CONTAINER overrides, FEDORA picks the release.
set -eu
R=$(cd "$(dirname "$0")/.." && pwd)
DIST=$(cd "$1" && pwd) VER=$2
FEDORA=${FEDORA:-44}
if [ -z "${CONTAINER:-}" ]; then
  if command -v podman >/dev/null 2>&1; then CONTAINER=podman
  elif command -v docker >/dev/null 2>&1; then CONTAINER=docker
  elif command -v distrobox-host-exec >/dev/null 2>&1; then CONTAINER="distrobox-host-exec podman"
  else echo "needs podman or docker"; exit 1; fi
fi
case $VER in *-*) echo "RPM versions cannot contain '-' ($VER)"; exit 1 ;; esac
[ -f "$DIST/acerctl-$VER.tar.gz" ] || { echo "no $DIST/acerctl-$VER.tar.gz (tools/release.sh first)"; exit 1; }

W="$R/.ci/rpm"
rm -rf "$W"
mkdir -p "$W/SOURCES" "$W/SPECS"
cp "$DIST/acerctl-$VER.tar.gz" "$W/SOURCES/"
for s in "$R"/packaging/rpm/*.spec; do
  if ! grep -q "^Version: *$VER$" "$s"; then
    [ "${FORCE_VERSION:-0}" = 1 ] || { echo "$s: Version is not $VER"; exit 1; }
  fi
  sed "s/^Version:.*/Version:        $VER/" "$s" > "$W/SPECS/${s##*/}"
done

echo "== RPMs in fedora:$FEDORA (log: $W/build.log)"
$CONTAINER run --rm -v "$W:/work:z" "registry.fedoraproject.org/fedora:$FEDORA" sh -c '
  set -e
  dnf -y -q install rpm-build dnf-plugins-core kmodtool
  dnf -y -q builddep /work/SPECS/acerctl.spec
  rpmbuild -ba --define "_topdir /work" /work/SPECS/acerctl.spec
  rpmbuild -ba --define "_topdir /work" /work/SPECS/acerctl-kmod.spec' > "$W/build.log" 2>&1 \
  || { tail -20 "$W/build.log"; exit 1; }
# everything but the empty kmod-acerctl metapackage
find "$W/RPMS" "$W/SRPMS" -name '*.rpm' ! -name 'kmod-acerctl-*' -exec cp {} "$DIST/" \; -print | sed 's|.*/|  |'
