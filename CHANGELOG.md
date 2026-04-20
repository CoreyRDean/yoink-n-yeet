# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] — 2026-04-19

### Added

- Initial public release.
- `yoink` / `yk` push and `yeet` / `yt` pop; single binary dispatched via
  `argv[0]`.
- Cross-platform clipboard backends for macOS (`pbcopy`/`pbpaste`), Linux
  Wayland (`wl-copy`/`wl-paste`), Linux X11 (`xclip`/`xsel`), Windows
  (`clip.exe` + PowerShell).
- Timestamped, binary-safe stack storage under the platform data dir.
- Flags: `--list`, `--show N`, `--peek`, `--dry`, `--drain` / `--clear`,
  `--drain --days N` / `--drain --hours N`, `--stats`, `--doctor`,
  `--version`, `--update`, `--stable`, `--auto-update`, `--uninstall`,
  `--no-update-check`.
- Async update check (24h TTL, stderr-only banner).
- Three channels: `stable`, `nightly`, `local`.
- `install.sh` (POSIX sh) supporting `curl | bash`, pinned version,
  nightly, and `--local` from a working tree.
- `uninstall.sh` and `--uninstall` wiring.
- CI: matrix build/test/lint on ubuntu/macos/windows, CodeQL SAST,
  Dependabot, goreleaser-driven releases with sha256 + cosign keyless
  signatures, daily nightly prereleases.

### Notes

- Homebrew tap publication is wired up in `.goreleaser.yaml` but commented
  out until `CoreyRDean/homebrew-tap` is created and a `HOMEBREW_TAP_TOKEN`
  PAT is registered as a repo secret. Will be enabled in a follow-up patch
  release.
- MIT license; CODEOWNERS; issue and PR templates; branch protection
  requiring review.

[Unreleased]: https://github.com/CoreyRDean/yoink-n-yeet/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/CoreyRDean/yoink-n-yeet/releases/tag/v0.1.0
