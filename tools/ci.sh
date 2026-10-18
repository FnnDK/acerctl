#!/bin/sh
# Continuous integration, the same steps here and in .github/workflows/:
#   sh tools/ci.sh              code, modules, release (HEAD), debs (Debian 13, Ubuntu 24.04),
#                               RPMs and the model matrix
#   sh tools/ci.sh full         the same with every Debian/Ubuntu image
#   sh tools/ci.sh STEP [args]  one step:
#     code                  gofmt, vet and tests; frontend type check and build; app build; sh -n
#     modules [target...]   tools/modules-matrix.sh, code warnings fail
#     release               tools/release.sh of HEAD as 0.0.0~ci into .ci/dist
#     debs [image...]       tools/debs-install-test.sh on .ci/dist
#     rpms                  tools/build-rpms.sh on .ci/dist
#     models [image...]     tools/models-test.sh on .ci/dist: the known models (fixtures) on the
#                           packages installed in Fedora 44 and Ubuntu 24.04
# Needs go, node/npm, wails and libwebkit2gtk-4.1-dev, plus podman or docker
# (from a distrobox, the host's podman). The release build archives HEAD:
# uncommitted changes are not in it.
set -eu
R=$(cd "$(dirname "$0")/.." && pwd)
cd "$R"
VER=0.0.0~ci
DIST="$R/.ci/dist"
step() { printf '\n== %s\n' "$*"; }

code() {
  step "code"
  (cd daemon && { test -z "$(gofmt -l .)" || { gofmt -l .; exit 1; }; } && go vet ./... && go test ./...)
  (cd gui/frontend && npm ci --no-audit --no-fund --loglevel=error && npm run build >/dev/null)
  (cd gui && { test -z "$(gofmt -l .)" || { gofmt -l .; exit 1; }; } &&
    CGO_CFLAGS=-Wno-deprecated-declarations go vet -tags desktop,production,webkit2_41 . &&
    CGO_CFLAGS=-Wno-deprecated-declarations go build -tags desktop,production,webkit2_41 -o /dev/null .)
  for f in tools/*.sh packaging/*.sh; do sh -n "$f"; done
  echo "code ok"
}
modules() { step "modules"; STRICT=1 sh tools/modules-matrix.sh "$@"; }
release() {
  step "release build of HEAD"
  [ -z "$(git status --porcelain)" ] || echo "note: uncommitted changes are not in this build"
  rm -rf "$DIST"
  DIST="$DIST" sh tools/release.sh HEAD "$VER"
}
debs() { step "debs"; sh tools/debs-install-test.sh "$DIST" "$@"; }
rpms() { step "rpms"; FORCE_VERSION=1 sh tools/build-rpms.sh "$DIST" "$VER"; }
models() { step "models"; sh tools/models-test.sh "$DIST" "$@"; }

case "${1:-all}" in
  all) code; modules; release; debs debian:13 ubuntu:24.04; rpms; models ;;
  full) code; modules; release; debs; rpms; models ;;
  code|modules|release|debs|rpms|models) s=$1; shift; $s "$@" ;;
  *) sed -n '2,16p' "$0"; exit 2 ;;
esac
step "ok"
