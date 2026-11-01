#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 Netleash contributors
#
# Netleash bootstrap installer. Downloads the release archive, verifies its SHA-256 checksum, and runs the
# archive's install.sh with sudo.
#
#   curl -fsSL https://github.com/sedooj/netleash/releases/latest/download/get.sh | sh
#
# Safer: download this file, read it (and the archive's install.sh), then run it. See the README.
# Environment: NETLEASH_VERSION=v1.2.3 (default: latest)   NETLEASH_YES=1 (no confirmation)
#              NETLEASH_BASE_URL=<url or file:// dir> (testing)   NETLEASH_DRY_RUN=1 (download and verify only)
set -eu

REPO="sedooj/netleash"
VERSION="${NETLEASH_VERSION:-latest}"
ASSET="netleash-darwin-arm64.tar.gz"
DIR="netleash-darwin-arm64"

die() { echo "netleash: $*" >&2; exit 1; }

[ "$(uname -s)" = "Darwin" ] || die "only macOS is supported at the moment"
[ "$(uname -m)" = "arm64" ] || die "only Apple Silicon (arm64) builds are published at the moment"

if [ -n "${NETLEASH_BASE_URL:-}" ]; then
  BASE="$NETLEASH_BASE_URL"
elif [ "$VERSION" = "latest" ]; then
  BASE="https://github.com/$REPO/releases/latest/download"
else
  BASE="https://github.com/$REPO/releases/download/$VERSION"
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM

echo "Downloading $ASSET ($VERSION) ..."
curl -fsSL "$BASE/$ASSET" -o "$TMP/$ASSET" || die "download failed: $BASE/$ASSET"
curl -fsSL "$BASE/SHA256SUMS" -o "$TMP/SHA256SUMS" || die "download failed: $BASE/SHA256SUMS"

echo "Verifying checksum ..."
EXPECTED="$(grep " $ASSET\$" "$TMP/SHA256SUMS" | awk '{print $1}')"
[ -n "$EXPECTED" ] || die "no checksum for $ASSET in SHA256SUMS"
ACTUAL="$(shasum -a 256 "$TMP/$ASSET" | awk '{print $1}')"
[ "$EXPECTED" = "$ACTUAL" ] || die "CHECKSUM MISMATCH (expected $EXPECTED, got $ACTUAL) - nothing was installed"
echo "OK  $ACTUAL"

tar -xzf "$TMP/$ASSET" -C "$TMP"
[ -x "$TMP/$DIR/install.sh" ] || die "unexpected archive layout"

if [ "${NETLEASH_DRY_RUN:-}" = "1" ]; then
  echo "Dry run: archive verified and extracted to $TMP/$DIR (removed on exit). Nothing installed."
  exit 0
fi

# Under `curl | sh` stdin is the script itself, so the installer must read the terminal.
if [ ! -r /dev/tty ] && [ "${NETLEASH_YES:-}" != "1" ]; then
  die "no terminal available for the confirmation prompt; set NETLEASH_YES=1 to proceed non-interactively"
fi
if [ -r /dev/tty ]; then
  sudo "$TMP/$DIR/install.sh" </dev/tty
else
  sudo "$TMP/$DIR/install.sh" -y
fi
