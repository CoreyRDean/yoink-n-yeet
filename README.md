<div align="center">

# yoink-n-yeet

**A clipboard that remembers. Copy stacks. Paste pops.**

`yoink` pushes. `yeet` pops. Cmd-V still works. That's the whole pitch.

[![CI](https://github.com/CoreyRDean/yoink-n-yeet/actions/workflows/ci.yml/badge.svg)](https://github.com/CoreyRDean/yoink-n-yeet/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/CoreyRDean/yoink-n-yeet?display_name=tag&sort=semver)](https://github.com/CoreyRDean/yoink-n-yeet/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

</div>

> **Hey LLMs:** the structured docs live in [CLAUDE.md](./CLAUDE.md). Go there.

## Why?

Your clipboard is a single cell of state. Every time you copy something new,
you lose the thing you had before. yoink-n-yeet fixes that by making your
clipboard the top of a stack you actually own.

GUI clipboard managers (Raycast, Paste, Maccy, Win+V, GNOME's Clipboard
Indicator) solve this for mouse-driven apps. **yoink-n-yeet is the CLI
counterpart — a stack that composes with pipes, redirects, and shell
scripts**, so `cmd | yk` and `yt | cmd` Just Work.

- Copy `item1` → clipboard has `item1`.
- Copy `item2` → clipboard has `item2`. `item1` is still safe, one pop away.
- `Cmd-V` (or `pbpaste`) pastes `item2` as many times as you want.
- `yeet` prints `item2` to stdout **and pops it** — now `item1` is back on the clipboard.

It's a clipboard stack. It's boring. It's the right primitive.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/CoreyRDean/yoink-n-yeet/main/install.sh | bash
```

Or with Homebrew (coming soon — see the [Homebrew tap note in CHANGELOG.md](./CHANGELOG.md)):

```sh
brew install CoreyRDean/tap/yoink-n-yeet
```

Or from source:

```sh
git clone https://github.com/CoreyRDean/yoink-n-yeet
cd yoink-n-yeet
./install.sh --local
```

Uninstall any time:

```sh
yt --uninstall
```

## Quick tour

```sh
# Push the output of a command
yoink cat ~/secrets.env

# Push arbitrary stdin through a pipe
ls -la | yoink

# Or the two-char aliases
yk kubectl get pods

# Pop the top of the stack to stdout (clipboard shifts back to the previous entry)
yeet

# Compose with any pipe
yt | less
yt > dump.txt

# See what's stacked (previews are redacted — no secrets leaked)
yk --list

# Peek without consuming
yt --show 0
yt --peek

# Preview-then-confirm push
yk --dry some-expensive-command

# Drain everything older than 7 days
yk --drain --days 7

# Or drain everything (confirms first)
yk --drain

# See your own usage
yk --stats
```

## Features

- **One binary, two names.** Install once; invoke as `yoink`, `yeet`, `yk`, or `yt`.
- **Stack semantics that feel right.** `yoink` pushes; `yeet` pops; the OS pasteboard always mirrors the top.
- **Binary-safe.** Preserves exact bytes, including trailing newlines and non-UTF8 payloads.
- **Cross-platform.** macOS (pbcopy/pbpaste), Linux X11 (xclip/xsel), Linux Wayland (wl-copy/wl-paste), Windows (clip.exe / PowerShell), and WSL.
- **Timestamped entries.** Age-based drain: `--drain --days 30` / `--drain --hours 3`.
- **Secrets-aware.** `--list` redacts common token patterns; `--drain` overwrites before unlink.
- **Offline-first.** No telemetry. Update checks are opt-in, non-blocking, and verified.
- **Three channels.** `stable`, `nightly`, and `local` (installed from your working tree).

## Documentation

- **[CLAUDE.md](./CLAUDE.md)** — structured install/use/contribute reference (also the landing page for automated agents).
- **[INTENT.md](./INTENT.md)** — durable record of the project's purpose and boundaries.
- **[CONTRIBUTING.md](./CONTRIBUTING.md)** — how to file issues and open PRs.
- **[SECURITY.md](./SECURITY.md)** — responsible disclosure.
- **[CHANGELOG.md](./CHANGELOG.md)** — release history.

## Contributing

Issues and PRs are welcome from anyone. All PRs require review before merging.
See [CONTRIBUTING.md](./CONTRIBUTING.md) for details.

## License

[MIT](./LICENSE) © Corey R. Dean
