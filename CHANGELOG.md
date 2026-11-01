# Changelog

Versions follow [Semantic Versioning](https://semver.org). The version of a release is the content of the `VERSION`
file and the git tag `vX.Y.Z`; local builds are marked `X.Y.Z-dev+<commit>`.

## 0.2.0 — unreleased

- `netleash update`: checks GitHub for a newer release and, after confirmation, downloads it, verifies its SHA-256
  checksum and runs its installer. Requests go **through the VPN** by default (`--direct` to bypass; `--check`,
  `--json`, `--yes`, `--download-only`, `--version` available).
- Menu bar app: "Check for updates…" with a one-click update flow, and an opt-in daily check (off by default;
  silent when the VPN is down).
- Notifications (VPN down/up, unprotected launch closed, update available) are now shown by the Netleash app under its
  own name and icon instead of "Script Editor". The daemon publishes events in its status file; if the menu bar app is
  not running it falls back to the old mechanism. macOS asks for notification permission on first start.
- Unit tests for version comparison, checksum verification and safe archive extraction.

## 0.1.0 — 2026-11-01

First public version. Supported setup: Claude desktop app + AmneziaVPN (Kill Switch off), macOS 26, Apple Silicon.

- Kernel-level confinement of the protected app's network to the VPN tunnel (`pf` anchor, dedicated group).
- Root daemon and CLI in Go (`netleash`), setuid helper (`netleash-exec`), menu bar app in Swift.
- DNS leak protection with automatic detection of the VPN's DNS servers.
- Emergency rollback (`netleash panic`) and restore (`netleash resume`).
- Release archive with SHA-256 checksums and a `curl | sh` bootstrap installer.
- Original logo, app icon and menu bar glyphs.
