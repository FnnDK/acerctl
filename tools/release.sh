#!/bin/sh
# Builds the release artifacts of a tag into dist/:
#   acerctl-<ver>.tar.gz                sources with vendored Go modules and the
#                                       built frontend (builds without network)
#   acerctl-<ver>-linux-x86_64.tar.gz   prebuilt acerd, acerctl, acer-tray and the
#                                       app, the install scripts and the kernel
#                                       modules as source (packaging/INSTALL-binary.md)
#   *.deb                               Debian/Ubuntu packages of the same binaries
#                                       (tools/build-debs.sh)
#   SHA256SUMS
# Everything is built from the tag, not from the working tree:
#   sh tools/release.sh v0.1.0
# Needs what building does (README, Installation): go, node/npm, wails and
# libwebkit2gtk-4.1-dev.
set -eu
R=$(cd "$(dirname "$0")/.." && pwd)
TAG=${1:?usage: sh tools/release.sh vX.Y.Z}
VER=${TAG#v}
ARCH=$(uname -m)
DIST="$R/dist"
WAILS=$(command -v wails || echo "$HOME/go/bin/wails")
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

git -C "$R" rev-parse -q --verify "refs/tags/$TAG" >/dev/null || { echo "no tag $TAG"; exit 1; }

echo "== sources of $TAG"
NAME="acerctl-$VER"
SRC="$WORK/$NAME"
git -C "$R" archive --format=tar --prefix="$NAME/" "$TAG" | tar -x -C "$WORK"
(cd "$SRC/daemon" && go mod vendor)
(cd "$SRC/gui" && go mod vendor)
(cd "$SRC/gui/frontend" && npm ci --no-audit --no-fund --loglevel=error && npm run build >/dev/null)
rm -rf "$SRC/gui/frontend/node_modules"
echo "$VER" > "$SRC/VERSION"
mkdir -p "$DIST"
tar -C "$WORK" -czf "$DIST/$NAME.tar.gz" "$NAME"

echo "== binaries (vendored modules, frontend already built)"
B="$WORK/build"
cp -a "$SRC" "$B"
(cd "$B/daemon" && CGO_CFLAGS="-Wno-deprecated-declarations" go build -mod=vendor -trimpath \
  -ldflags "-s -w -X main.Version=$VER" -o acerd ./cmd/acerd)
(cd "$B/daemon" && go build -mod=vendor -trimpath -ldflags "-s -w" -o acerctl ./cmd/acerctl)
(cd "$B/daemon" && go build -mod=vendor -trimpath -ldflags "-s -w" -o acer-tray ./cmd/acer-tray)
(cd "$B/gui" && GOFLAGS=-mod=vendor "$WAILS" build -s -m -tags webkit2_41 -trimpath -ldflags "-s -w" >/dev/null)

BIN="$WORK/$NAME-linux-$ARCH"
mkdir -p "$BIN/daemon" "$BIN/gui/build/bin" "$BIN/tools" "$BIN/assets"
cp "$B/daemon/acerd" "$B/daemon/acerctl" "$B/daemon/acer-tray" "$BIN/daemon/"
cp "$B/gui/build/bin/nitro-control-center" "$BIN/gui/build/bin/"
cp -a "$SRC/packaging" "$SRC/drivers" "$BIN/"
cp -a "$SRC/assets/logo" "$BIN/assets/"
cp "$SRC/tools/survey.sh" "$BIN/tools/"
cp "$SRC/README.md" "$SRC/LICENSE" "$SRC/VERSION" "$BIN/"
mv "$BIN/packaging/INSTALL-binary.md" "$BIN/INSTALL.md"
tar -C "$WORK" -czf "$DIST/$NAME-linux-$ARCH.tar.gz" "$NAME-linux-$ARCH"

echo "== Debian/Ubuntu packages"
D="$WORK/debbin"
mkdir -p "$D"
cp "$B/daemon/acerd" "$B/daemon/acerctl" "$B/daemon/acer-tray" "$D/"
cp "$B/gui/build/bin/nitro-control-center" "$D/control-center"
sh "$SRC/tools/build-debs.sh" "$D" "$SRC" "$VER" "$DIST" >/dev/null

cd "$DIST"
sha256sum "$NAME.tar.gz" "$NAME-linux-$ARCH.tar.gz" ./*_"${VER}"_*.deb | sed 's| \./| |' > SHA256SUMS
echo "== $DIST"
ls -l "$NAME.tar.gz" "$NAME-linux-$ARCH.tar.gz" ./*_"${VER}"_*.deb SHA256SUMS
