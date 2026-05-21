#!/bin/sh
#
# agx install script.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/kiddingbaby/agx/main/scripts/install.sh | sh
#
# Env overrides:
#   AGX_VERSION       release tag (default: latest, e.g. v0.1.0)
#   AGX_INSTALL_DIR   target directory (default: $HOME/.local/bin)
#   AGX_REPO          GitHub repo (default: kiddingbaby/agx)
#
# Behaviour:
#   1. Detect OS / arch (linux|darwin × x86_64|arm64).
#   2. Resolve version (follows /releases/latest redirect; no API rate limit).
#   3. Download the archive + checksums.txt; verify SHA256.
#   4. Extract `agx` into AGX_INSTALL_DIR with mode 0755.
#   5. Warn if AGX_INSTALL_DIR is not on PATH.

set -eu

: "${AGX_VERSION:=latest}"
: "${AGX_INSTALL_DIR:=$HOME/.local/bin}"
: "${AGX_REPO:=kiddingbaby/agx}"

GITHUB="https://github.com/${AGX_REPO}"

log() { printf '%s\n' "agx-install: $*" >&2; }
die() { log "$*"; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

have curl || die "curl is required"
have tar  || die "tar is required"
have install || die "install(1) is required"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux|darwin) ;;
  *) die "unsupported OS: $OS (only linux / darwin)";;
esac

MACHINE=$(uname -m)
case "$MACHINE" in
  x86_64|amd64)  ARCH=x86_64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "unsupported arch: $MACHINE (only x86_64 / arm64)";;
esac

if [ "$AGX_VERSION" = "latest" ]; then
  log "resolving latest release"
  VERSION=$(curl -fsLI -o /dev/null -w '%{url_effective}\n' "${GITHUB}/releases/latest" | sed 's#.*/tag/##' | tr -d '\r\n')
  [ -n "$VERSION" ] || die "could not resolve latest version"
else
  VERSION="$AGX_VERSION"
fi
case "$VERSION" in v*) ;; *) VERSION="v${VERSION}" ;; esac

VER_NO_V=${VERSION#v}
ARCHIVE="agx_${VER_NO_V}_${OS}_${ARCH}.tar.gz"
ARCHIVE_URL="${GITHUB}/releases/download/${VERSION}/${ARCHIVE}"
CHECKSUMS_URL="${GITHUB}/releases/download/${VERSION}/checksums.txt"

TMP=$(mktemp -d 2>/dev/null || mktemp -d -t agx-install)
trap 'rm -rf "$TMP"' EXIT INT TERM HUP

log "downloading ${ARCHIVE}"
curl -fsSL -o "$TMP/$ARCHIVE" "$ARCHIVE_URL" \
  || die "failed to download $ARCHIVE_URL"
curl -fsSL -o "$TMP/checksums.txt" "$CHECKSUMS_URL" \
  || die "failed to download $CHECKSUMS_URL"

if have sha256sum; then
  SHA_CMD="sha256sum"
elif have shasum; then
  SHA_CMD="shasum -a 256"
else
  die "neither sha256sum nor shasum found"
fi

ACTUAL=$(cd "$TMP" && $SHA_CMD "$ARCHIVE" | awk '{print $1}')
EXPECTED=$(awk -v f="$ARCHIVE" '$2==f {print $1}' "$TMP/checksums.txt")
[ -n "$EXPECTED" ] || die "no checksum entry for $ARCHIVE in checksums.txt"
[ "$ACTUAL" = "$EXPECTED" ] || die "checksum mismatch (got $ACTUAL, want $EXPECTED)"
log "checksum OK"

tar -xzf "$TMP/$ARCHIVE" -C "$TMP" agx \
  || die "failed to extract agx from archive"

mkdir -p "$AGX_INSTALL_DIR"
install -m 0755 "$TMP/agx" "$AGX_INSTALL_DIR/agx"
log "installed ${VERSION} to ${AGX_INSTALL_DIR}/agx"

case ":$PATH:" in
  *":$AGX_INSTALL_DIR:"*) ;;
  *)
    log ""
    log "${AGX_INSTALL_DIR} is not on your PATH. Add to your shell rc:"
    log "  export PATH=\"${AGX_INSTALL_DIR}:\$PATH\""
    ;;
esac

"$AGX_INSTALL_DIR/agx" version 2>/dev/null || true
