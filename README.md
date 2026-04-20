<div align="center">

# yoink-n-yeet

**The clipboard, as a stack. For your terminal. `yoink` pushes, `yeet` pops.**

A tiny, cross-platform CLI clipboard manager that keeps a history of everything you copy, and actually *composes with pipes*. Built in Go, one static binary, zero runtime dependencies. macOS, Linux (X11 + Wayland), Windows, WSL.

[![CI](https://github.com/CoreyRDean/yoink-n-yeet/actions/workflows/ci.yml/badge.svg)](https://github.com/CoreyRDean/yoink-n-yeet/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/CoreyRDean/yoink-n-yeet?display_name=tag&sort=semver)](https://github.com/CoreyRDean/yoink-n-yeet/releases)
[![Downloads](https://img.shields.io/github/downloads/CoreyRDean/yoink-n-yeet/total)](https://github.com/CoreyRDean/yoink-n-yeet/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Go Report](https://goreportcard.com/badge/github.com/CoreyRDean/yoink-n-yeet)](https://goreportcard.com/report/github.com/CoreyRDean/yoink-n-yeet)

```sh
brew install CoreyRDean/tap/yoink-n-yeet
```

</div>

> **For AI agents / automation:** structured docs live in [CLAUDE.md](./CLAUDE.md).

## Why does this exist?

The system clipboard is a single cell of state. Copy something new, lose the thing you had before. For mouse-driven apps, GUI clipboard managers (Raycast, Paste, Maccy, Win+V, GNOME Clipboard Indicator) solve this fine. **For the terminal, they don't help at all** — they can't pipe into `grep`, they can't be scripted, they can't be composed with `ssh`, `kubectl`, `aws`, or your build tools.

`yoink-n-yeet` is the CLI counterpart. It turns the system clipboard into the top of a persistent stack you actually own, and gives you two verbs that work the way shell verbs should:

- **`yoink`** (short: `yk`) — push command output or stdin onto the stack
- **`yeet`** (short: `yt`) — pop the top of the stack to stdout

Every flag works on either name, so `yk --list` and `yt --list` are the same thing. Your OS clipboard (`Cmd-V`, `Ctrl-V`, `pbpaste`, `xclip -o`) always mirrors the top of the stack, so pasting into GUI apps keeps working exactly as it does today.

## Install

**Homebrew** (macOS + Linux, recommended):

```sh
brew install CoreyRDean/tap/yoink-n-yeet
```

**One-line install** (macOS + Linux, no Homebrew required):

```sh
curl -fsSL https://raw.githubusercontent.com/CoreyRDean/yoink-n-yeet/main/install.sh | bash
```

**From source** (any platform with Go 1.22+):

```sh
git clone https://github.com/CoreyRDean/yoink-n-yeet
cd yoink-n-yeet
./install.sh --local
```

**Windows**: grab the prebuilt zip for your arch from the [latest release](https://github.com/CoreyRDean/yoink-n-yeet/releases/latest) and drop `yoink-n-yeet.exe` on your PATH. Native Windows clipboard via `clip.exe` / `Get-Clipboard`; WSL works too.

Uninstall any time with `yt --uninstall` (or `brew uninstall yoink-n-yeet`).

## 30-second tour

```sh
# Push the output of any command onto the stack
yk cat ~/.ssh/config
yk kubectl get pods -A
yk aws sts get-caller-identity

# Pipe into it
git log --oneline | yk

# Import whatever's on the OS clipboard (from any app)
yk -c

# See the stack (previews are secret-redacted)
yk --list

# Pop the top entry to stdout
yt

# Or peek without consuming
yt --peek

# Pipe popped content into something else
yt | pbcopy
yt | base64 | curl -X POST --data-binary @- https://paste.example/

# Arrow-key picker to promote any entry to the top
yk --pick

# Age-based cleanup
yk --drain --days 7

# File an issue without leaving the terminal
yk --report "macOS Sequoia: --pick cursor bleeds" "…repro steps…"
```

That's the whole idea. It's boring. It's the right primitive.

## Features

- **Stack semantics that feel right.** LIFO, binary-safe, timestamped. The OS clipboard always mirrors the top.
- **Pipe-native, script-friendly.** Update banners and prompts go to stderr; only the requested payload hits stdout.
- **One binary, four names.** `yoink`, `yeet`, `yk`, `yt` — install once, invoke however your muscle memory wants.
- **Cross-platform clipboard.** macOS (`pbcopy`/`pbpaste`), Linux Wayland (`wl-copy`/`wl-paste`), Linux X11 (`xclip` / `xsel`), Windows (`clip.exe` + PowerShell), and WSL — auto-detected.
- **Secrets-aware by default.** `--list` previews redact GitHub tokens, AWS keys, Slack tokens, JWTs, PEM private keys, OpenAI keys, common `password:=…` patterns.
- **Interactive picker.** `yk --pick` launches a zero-dep arrow-key TUI. Vim-style `j/k` also works.
- **Offline-first.** No telemetry. Update checks are opt-in, non-blocking, and checksum-verified.
- **Three release channels.** `stable`, `nightly`, and `local` (installed from your working tree — `--version` tells you exactly which commit, and whether your tree is dirty).
- **Supply-chain hardening.** Every release is cosign-keyless signed via GitHub OIDC, ships an SBOM per artifact, and has SHA256 checksums the installer verifies.

## Commands

Default action is decided by which name you invoked: `yoink`/`yk` push, `yeet`/`yt` pop. Every flag is valid on either name.

| Flag | What it does |
|------|--------------|
| *(no flags + args)* | `yoink cmd [args]` runs `cmd` and pushes its stdout |
| *(no flags + pipe)* | `... \| yoink` pushes stdin |
| *(no flags, no stdin)* | `yeet` pops the top of the stack to stdout |
| `-c`, `--cb` | On yoink: import the OS clipboard. On yeet: stream the OS clipboard to stdout without touching the stack. |
| `--list [--json]` | Render the stack (0 = top) with timestamps, sizes, redacted previews |
| `--show [N\|first\|last]` | Print entry N (default: top) to stdout, non-consuming |
| `--peek` | Alias for `--show first` |
| `--pick [N\|first\|last]` | Move an entry to the top. No arg on a TTY launches the arrow-key picker. |
| `--dry` | Preview + prompt before pushing or popping |
| `--drain`, `--clear` | Wipe the stack (confirms; best-effort secure overwrite on both payload and metadata) |
| `--drain --days N` | Drop entries older than N days (N > 0) |
| `--drain --hours N` | Drop entries older than N hours (N > 0) |
| `--stats [--json]` | Usage summary: push/pop counts, avg age at pop, top source commands, per-day histogram |
| `--doctor` | Platform + clipboard-backend diagnostics |
| `--report [title] [body]` | File an issue on the upstream repo (uses `gh` CLI if available, otherwise opens the browser issue form with fields prefilled) |
| `--version` | Version, channel, commit, build date (plus repo path + dirty state for local builds) |
| `--update [stable\|nightly]` | Self-update. `--stable` is a shortcut for `--update stable`. |
| `--auto-update on\|off\|status` | Toggle the opt-in background auto-update (default off) |
| `--no-update-check` | Skip the async update check for this invocation |
| `--uninstall` | Remove the binary, symlinks, completions, optionally data |

## Compared to other clipboard tools

| Tool | History / stack | CLI-native | Pipe-friendly | Cross-platform |
|------|-----------------|------------|---------------|----------------|
| `pbcopy` / `pbpaste`, `xclip`, `clip.exe` | ❌ | ✅ | ✅ | platform-specific |
| Raycast / Paste / Maccy / Alfred | ✅ | ❌ | ❌ | macOS |
| Win+V | ✅ | ❌ | ❌ | Windows |
| CopyQ / GPaste / clipmenu | ✅ | partial | ❌ | Linux |
| **yoink-n-yeet** | ✅ | ✅ | ✅ | macOS + Linux + Windows + WSL |

If you already use a GUI clipboard manager, keep using it — `yoink-n-yeet` is additive, not a replacement. What it's for is the gap those tools don't fill: **scripted, shell-composable clipboard state on the terminal**.

## FAQ

### How do I save clipboard history in the terminal?

Install `yoink-n-yeet`, then pipe into `yk` or run `yk <command>`. Each push adds an entry; `yk --list` shows the history; `yk --stats` shows usage. History persists across terminal sessions at `~/Library/Application Support/yoink-n-yeet/stack` on macOS and `$XDG_DATA_HOME/yoink-n-yeet/stack` on Linux.

### How do I pipe to the clipboard from a shell script?

```sh
some-command | yk          # push into the stack + sync OS clipboard
some-command | pbcopy      # direct to OS clipboard, no history
some-command | yk -c       # direct to OS clipboard via yoink (no-op on stack)
```

### How do I paste the OS clipboard in a script?

```sh
yt                  # pop top of stack to stdout (consuming)
yt --peek           # print top without popping
yt -c               # print current OS clipboard (bypasses stack entirely)
pbpaste | ...       # direct OS clipboard read, no stack interaction
```

### Does `Cmd-V` / `Ctrl-V` still work?

Yes. The OS clipboard always mirrors the top of the stack, so pasting in GUI apps works exactly as before. If another app clobbers the clipboard (e.g. a Slack notification copied a link), run `yt --peek | pbcopy` to re-sync.

### Is it safe for sensitive data?

For a single-user workstation, yes — entries are stored with `0o600` permissions (user-only). `--list` previews redact common token/secret patterns. `--drain` does a best-effort secure overwrite of both payload and metadata. But entries are **not** encrypted at rest; don't use this on shared or multi-user systems for payloads whose cleartext exposure would be a problem. See [SECURITY.md](./SECURITY.md) for the full rundown.

### Does this replace `pbcopy` / `pbpaste` / `xclip`?

No, it's a wrapper on top of them. `yoink-n-yeet` shells out to the native clipboard tool on each platform. If those aren't installed (e.g. a fresh Linux box with neither `wl-clipboard` nor `xclip`), `yt --doctor` tells you what's missing.

### Does it phone home?

No telemetry. The only network I/O is the opt-in update check, which fetches `/releases/latest` from the GitHub API once every 24 hours per machine. You can disable it with `yt --no-update-check` per invocation or by editing `config.json`.

### How do I uninstall?

```sh
yt --uninstall              # interactive, prompts before removing data
brew uninstall yoink-n-yeet # if installed via Homebrew
```

## Documentation

- **[INTENT.md](./INTENT.md)** — durable record of what this project is and isn't. Read this before filing a feature request.
- **[CLAUDE.md](./CLAUDE.md)** — structured reference for AI agents working on this repo.
- **[CONTRIBUTING.md](./CONTRIBUTING.md)** — issue/PR workflow.
- **[SECURITY.md](./SECURITY.md)** — responsible disclosure, threat model.
- **[CHANGELOG.md](./CHANGELOG.md)** — release history.

## Roadmap

Out of scope forever: GUI / tray icon, cross-device sync, cloud, telemetry.

Planned for v0.2+:

- `yt --sync` — re-mirror the top of the stack back to the OS clipboard after another app clobbered it
- `--tag <name>` — named entries with tag-based filtering (`yk --drain --tag throwaway`)
- `--swap` / `--dup` / `--rotate` — stack ergonomics
- Scoop + winget manifests for first-class Windows install
- SLSA provenance + reproducible builds
- Optional `--tui` interactive browser

Open an issue if you want to push something up the list.

## Contributing

Issues and PRs are welcome from anyone. Merges require one approving review. See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full workflow.

If you're filing a bug, `yt --report` is the fastest path — it prefills the issue form with your version and environment.

## License

[MIT](./LICENSE) © Corey R. Dean

---

<sub>**Keywords:** clipboard stack, clipboard manager CLI, clipboard history terminal, command line clipboard, pbcopy alternative, xclip alternative, bash zsh clipboard, developer clipboard tool, pipe-friendly clipboard, cross-platform clipboard, Go CLI, macOS Linux Windows WSL clipboard.</sub>
