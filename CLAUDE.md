# CLAUDE.md — LLM & agent reference

This file is the structured landing page for automated agents working on
**yoink-n-yeet**. If you are a human, you probably want
[README.md](./README.md) or [INTENT.md](./INTENT.md) instead.

## One-paragraph overview

`yoink-n-yeet` is a cross-platform Go CLI that turns the system clipboard into
a persistent stack. The same binary is invoked as `yoink`/`yk` (push) or
`yeet`/`yt` (pop); a symlink or `argv[0]` dispatch selects the default
action. Entries are stored as timestamped `<unix_nanos>.bin` payloads plus
`<unix_nanos>.json` metadata under the platform's data dir. The OS clipboard
always mirrors the top of the stack.

## File map

```
cmd/yoink-n-yeet/main.go         # entry point, argv[0] dispatch, flag parsing
internal/stack/                  # stack CRUD (push/pop/show/peek/list/drain)
internal/clipboard/              # pluggable OS clipboard backends
internal/config/                 # JSON config at platform config dir
internal/stats/                  # append-only stats.jsonl + rollup
internal/update/                 # async update check + self-update apply
internal/buildinfo/              # version, commit, channel, build date (ldflags)
internal/redact/                 # secret redaction for --list previews
internal/platform/paths.go       # cross-platform data/config/cache paths
install.sh                       # POSIX sh installer (curl | bash compatible)
uninstall.sh                     # POSIX sh uninstaller
.github/workflows/               # CI, CodeQL, release, nightly
.goreleaser.yaml                 # release build/sign/publish config
completions/                     # bash/zsh/fish completions
```

## Build & test

```sh
# Build for the current platform (produces ./yoink-n-yeet)
go build ./cmd/yoink-n-yeet

# Run tests
go test ./...

# Run linters
golangci-lint run

# Install from local working tree (records channel=local)
./install.sh --local
```

Build is reproducible given a commit. Version metadata is injected via
`-ldflags`:

```sh
go build -ldflags "\
  -X github.com/CoreyRDean/yoink-n-yeet/internal/buildinfo.Version=v0.1.0 \
  -X github.com/CoreyRDean/yoink-n-yeet/internal/buildinfo.Commit=$(git rev-parse HEAD) \
  -X github.com/CoreyRDean/yoink-n-yeet/internal/buildinfo.Channel=local \
  -X github.com/CoreyRDean/yoink-n-yeet/internal/buildinfo.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  ./cmd/yoink-n-yeet
```

## Command reference

Default action is determined by `filepath.Base(os.Args[0])`:

- `yoink` / `yk` → push
- `yeet` / `yt` → pop

Every flag below works on either name.

| Flag | Behavior |
|------|----------|
| (none, args) | `yoink <cmd> [args]` — run `<cmd>`, push stdout |
| (none, pipe) | `... \| yoink` — push stdin |
| (none, no stdin, no args) | `yeet` — pop top to stdout |
| `--list [--json]` | Print stack (0 = top) with timestamps, sizes, redacted previews |
| `--show N` | Print entry N to stdout, no consumption (0 = top) |
| `--peek` | Alias for `--show 0` |
| `--dry` | Preview + interactive prompt (push/trash or pop/cancel) |
| `--drain` / `--clear` | Remove all entries (best-effort secure overwrite), with confirmation |
| `--drain --days N` | Remove entries older than N days |
| `--drain --hours N` | Remove entries older than N hours |
| `--stats [--json]` | Usage summary (push/pop counts, depth, ages, top commands) |
| `--doctor` | Platform + backend diagnostics |
| `--version` | Version, channel, commit, build date (+ repo path & dirty state for local) |
| `--update [stable\|nightly]` | Self-update (stable default); local channel re-runs installer |
| `--stable` | Shortcut for `--update stable` |
| `--auto-update on\|off\|status` | Toggle or inspect background auto-update (default off) |
| `--no-update-check` | Skip the async update check for this invocation |
| `--uninstall` | Remove the binary, symlinks, completions, and optionally data |

## Storage layout

- **Data**: entries as `<dataDir>/stack/<unix_nanos>.{bin,json}`
- **Config**: `<configDir>/config.json`
- **Stats**: `<dataDir>/stats.jsonl` (append-only)
- **Cache**: `<cacheDir>/latest_version.json` (async update check)

`<dataDir>`, `<configDir>`, `<cacheDir>` resolve via
`internal/platform.Paths()`:

- Linux: `$XDG_DATA_HOME/yoink-n-yeet`, `$XDG_CONFIG_HOME/yoink-n-yeet`, `$XDG_CACHE_HOME/yoink-n-yeet`
- macOS: `~/Library/Application Support/yoink-n-yeet`, `~/Library/Preferences/yoink-n-yeet`, `~/Library/Caches/yoink-n-yeet`
- Windows: `%LOCALAPPDATA%\yoink-n-yeet\{data,config,cache}`

## Channels

| Channel | Source | Updates via |
|---------|--------|-------------|
| `stable` | Latest GitHub Release (non-prerelease) | `yk --update` |
| `nightly` | Latest prerelease tagged `v*-nightly.*` | `yk --update nightly` |
| `local` | `./install.sh --local` from a working tree | `yk --update` re-runs installer against current working tree |

## CI

- `ci.yml` — runs on every PR + push to main: `go vet`, `golangci-lint`, `go test ./...` across ubuntu/macos/windows × go 1.22/1.23
- `codeql.yml` — SAST on PR + weekly schedule
- `release.yml` — on tag `v*.*.*`: goreleaser cross-compiles, checksums, cosign keyless signs via GitHub OIDC, creates release, bumps Homebrew tap
- `nightly.yml` — daily cron: tags `v<current>-nightly.<unix>`, goreleaser prerelease

## Contribution conventions

- Branch from `main`, open a PR, request review from `@CoreyRDean`.
- Commit style: short, imperative, present-tense subject. Include a body when the change is non-obvious.
- All PRs must pass CI and CodeQL before merge.
- Squash or rebase merges only; no merge commits on `main`.
- Include `Co-Authored-By: Oz <oz-agent@warp.dev>` in commit messages and PR descriptions when an AI agent assisted.
- Keep commits small and logically atomic. Don't mix unrelated changes.
- Add or update tests for any change that touches behavior.

## Useful invariants for agents

- The binary must never write to stdout anything except the requested payload. Update banners, warnings, prompts, progress → stderr.
- Push with `"$@"` preserves user's exact command. Only stdout is captured; stderr streams through.
- Stack entries are binary-safe. Never decode or re-encode.
- Update check is always non-blocking and best-effort; any failure is silent.
- Redaction is best-effort and only applies to `--list` previews, never to `--show N` / `--peek`, which emit raw bytes.

## Pointers

- [INTENT.md](./INTENT.md) — durable intent (read first if unsure what the project is)
- [CONTRIBUTING.md](./CONTRIBUTING.md) — issue/PR workflow
- [SECURITY.md](./SECURITY.md) — responsible disclosure
- [CHANGELOG.md](./CHANGELOG.md) — release history
