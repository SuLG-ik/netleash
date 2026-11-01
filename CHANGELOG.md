# Changelog

Versions follow [Semantic Versioning](https://semver.org). The version of a release is the content of the `VERSION`
file and the git tag `vX.Y.Z`; local builds are marked `X.Y.Z-dev+<commit>`.

## 0.1.0 — 2026-11-01

First public version. Supported setup: Claude desktop app + AmneziaVPN (Kill Switch off), macOS 26, Apple Silicon.

- Kernel-level confinement of the protected app's network to the VPN tunnel (`pf` anchor, dedicated group).
- Root daemon and CLI in Go (`netleash`), setuid helper (`netleash-exec`), menu bar app in Swift.
- DNS leak protection with automatic detection of the VPN's DNS servers.
- Emergency rollback (`netleash panic`) and restore (`netleash resume`).
- Release archive with SHA-256 checksums and a `curl | sh` bootstrap installer.
- Original logo, app icon and menu bar glyphs.
