#!/bin/bash
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 Netleash contributors
#
# Builds the release layout into dist/netleash-darwin-<arch>/ and packs dist/netleash-darwin-<arch>.tar.gz.
# Requires Go and (for the menu bar app) Xcode or the Command Line Tools. No sudo needed.
#   NETLEASH_VERSION=v1.2.3 ./build.sh
set -euo pipefail
cd "$(dirname "$0")"

VERSION="${NETLEASH_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
case "$(uname -m)" in arm64) ARCH=arm64 ;; x86_64) ARCH=amd64 ;; *) echo "unsupported architecture"; exit 1 ;; esac
NAME="netleash-darwin-$ARCH"
OUT="dist/$NAME"
rm -rf dist && mkdir -p "$OUT"

echo "==> Go binaries ($VERSION)"
export CGO_ENABLED=0 GOOS=darwin GOARCH="$ARCH"
go test ./...
go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$OUT/netleash" ./cmd/netleash
go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$OUT/netleash-exec" ./cmd/netleash-exec

echo "==> Menu bar app"
if command -v swiftc >/dev/null; then
  APP="$OUT/Netleash.app"
  mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
  swiftc -O -file-prefix-map "$PWD=." app/Netleash/main.swift -framework AppKit -o "$APP/Contents/MacOS/Netleash"
  cat > "$APP/Contents/Info.plist" <<P
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
 <key>CFBundleName</key><string>Netleash</string>
 <key>CFBundleDisplayName</key><string>Netleash</string>
 <key>CFBundleIdentifier</key><string>org.netleash.app</string>
 <key>CFBundleExecutable</key><string>Netleash</string>
 <key>CFBundleIconFile</key><string>icon.icns</string>
 <key>CFBundlePackageType</key><string>APPL</string>
 <key>CFBundleShortVersionString</key><string>$VERSION</string>
 <key>LSUIElement</key><true/>
 <key>LSMinimumSystemVersion</key><string>13.0</string>
</dict></plist>
P
  codesign -f -s - "$APP" 2>/dev/null || true
else
  echo "swiftc not found: building without the menu bar app (the CLI and daemon still work)"
fi

cp packaging/install.sh packaging/uninstall.sh packaging/panic.sh packaging/resume.sh LICENSE "$OUT/"
chmod 755 "$OUT"/*.sh

tar -C dist -czf "dist/$NAME.tar.gz" "$NAME"
( cd dist && shasum -a 256 "$NAME.tar.gz" > SHA256SUMS )
echo "OK: dist/$NAME.tar.gz"
cat dist/SHA256SUMS
echo
echo "Install from this build:  cd dist/$NAME && sudo ./install.sh"
