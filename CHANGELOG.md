# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.2] — 2026-04-20
### Fixed
- Update-check TTL is now actually enforced. `BackgroundCheck` stats the
  cache file and skips the network call entirely when the cached result
  is younger than 24h. Previously every invocation of `yk`/`yt` made a
  GitHub API request, which could exhaust the anonymous rate limit on
  tight shell loops.
- `MoveToTop` (used by `--pick`) now preserves the original `Created`
  timestamp instead of resetting it. An entry you promote with
  `--pick 3` is still subject to age-based filters like
  `--drain --days 7`.
- `--json` is now order-independent. `yk --json --list` used to silently
  drop `--json`; both orderings now produce JSON output for `--list` and
  `--stats`.
- Unknown `--` flags error out with a helpful message instead of being
  silently treated as the user's command. `yk --lsit file` used to try
  to exec a literal `--lsit`; it now reports `unknown flag "--lsit"`.
  Single-dash tokens still pass through so `yk ls -la` keeps working.
- Secure drain now zeros the `.json` metadata sidecar in addition to the
  `.bin` payload. The sidecar's `Source` field often leaks the original
  command (e.g. an AWS CLI invocation) and deserved the same treatment.
- `--drain --days 0` and `--drain --hours 0` now error out instead of
  silently falling through to "drain everything."
- A non-zero-exit command that produced partial stdout now surfaces a
  stderr notice like `pushed 42 bytes despite non-zero exit (code 1)
  from "grep foo file"` before committing the partial output.
- `go.mod` relaxed from `go 1.26.1` to `go 1.22`, matching the actual
  minimum stdlib floor the code needs.
## [0.1.1] — 2026-04-19
### Added
- `--report [title] [body]` files an issue on the upstream repo, preferring
  the `gh` CLI when available and authenticated, otherwise opening the
  GitHub web issue form with fields prefilled.
- `--show` now defaults to index 0 (top of the stack) when called without a
  numeric argument, mirroring `--peek`.
- `install.sh --local` derives the version from `git describe --tags
  --always --dirty` so `yt --version` reports a useful string like
  `v0.1.0-3-gccedc7e[-dirty]` instead of the opaque `v0.0.0-dev` sentinel.
### Fixed
- Installer's sha256 verification no longer spuriously fails because it was
  matching sibling `.sbom.json` rows in the checksum file.
- Installer re-runs no longer corrupt `config.json` into invalid JSON when
  patching `installed_version`. The generated config now keeps a
  never-patched field last so trailing commas stay valid.
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

[Unreleased]: https://github.com/CoreyRDean/yoink-n-yeet/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/CoreyRDean/yoink-n-yeet/releases/tag/v0.1.2
[0.1.1]: https://github.com/CoreyRDean/yoink-n-yeet/releases/tag/v0.1.1
[0.1.0]: https://github.com/CoreyRDean/yoink-n-yeet/releases/tag/v0.1.0
