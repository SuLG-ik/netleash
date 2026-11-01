#!/bin/bash
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 Netleash contributors
# Regenerates svg/, png/ and Netleash.icns from generate.py (macOS only: uses AppKit's SVG renderer).
set -euo pipefail
cd "$(dirname "$0")"
python3 generate.py
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
swiftc -O tools/svg2png.swift -o "$TMP/svg2png"
R="$TMP/svg2png"
mkdir -p png
$R svg/icon.svg png/icon-1024.png 1024
$R svg/mark.svg png/mark.png 1024
$R svg/mark-dark.svg png/mark-dark.png 1024
$R svg/logo-horizontal.svg png/logo-horizontal.png 1800
$R svg/logo-horizontal-dark.svg png/logo-horizontal-dark.png 1800
$R svg/social-preview.svg png/social-preview.png 1280
for s in up down warn; do $R svg/menubar-$s.svg png/menubar-$s@2x.png 36; $R svg/menubar-$s.svg png/menubar-$s.png 18; done
# macOS icon set
ICONSET="$TMP/Netleash.iconset"; mkdir -p "$ICONSET"
for s in 16 32 128 256 512; do
  $R svg/icon.svg "$ICONSET/icon_${s}x${s}.png" $s
  $R svg/icon.svg "$ICONSET/icon_${s}x${s}@2x.png" $((s*2))
done
iconutil -c icns "$ICONSET" -o Netleash.icns
echo "OK: svg/ png/ Netleash.icns"
