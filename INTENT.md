# Intent

This document is the durable record of *why* this project exists, *what* it must
remain true to, and *what it intentionally is not*. Everything else — code,
docs, infrastructure — is downstream of this file. When in doubt, re-read it.

## Purpose

The system clipboard is a single cell of state. Anyone who has ever copied
something, gone to paste, and realized they clobbered the thing they actually
wanted has felt the absence of a stack.

**yoink-n-yeet** turns the system clipboard into the top of a persistent,
user-owned stack. Copying pushes; pasting (the consuming kind) pops. The OS
pasteboard always mirrors the top of the stack, so Cmd-V / Ctrl-V still work
exactly as users expect.

Two personalities ship in one binary:

- **`yoink`** (short: `yk`) — push command output or stdin onto the stack.
- **`yeet`** (short: `yt`) — pop the top of the stack to stdout.

Every flag works on either name.

## Principles

1. **Composable with pipes.** Every invocation must cooperate with shell
   pipelines. Update banners, prompts, and errors go to stderr; only the
   requested payload goes to stdout.
2. **Binary-safe.** Clipboard payloads are bytes, not strings. No
   transformation, no re-encoding, no trailing-newline games.
3. **Fast cold start.** The CLI must feel invisible. The async update check
   never blocks a command.
4. **Offline-first.** No telemetry, no analytics, no phone-home beyond the
   opt-in update check against GitHub Releases.
5. **Secrets are real.** Clipboard contents frequently contain tokens, keys,
   and passwords. Previews redact; drain overwrites; auto-update is off by
   default and always checksum-verified.
6. **One binary, one install, one uninstall.** Users install with one command
   and remove cleanly with one command. No stray files, no surprise processes.
7. **Cross-platform by construction.** macOS, Linux (X11 + Wayland), Windows,
   WSL. Platform-specific logic lives behind a single `Clipboard` interface.
8. **Local installs are first-class.** Contributors can install directly from a
   working tree and get a `--version` that honestly reports where the binary
   came from, including uncommitted changes.

## Non-Goals

- A clipboard manager with a GUI or tray icon. There are good ones already.
- Cross-device clipboard sync. Out of scope forever.
- Rich structured clipboards (MIME types, images). v0.1.0 is text/bytes only.
- Silent self-modifying binaries. Auto-update is opt-in and verified.
- Telemetry of any kind.

## Success Criteria

- `curl -fsSL <install-url> | bash` installs a working `yoink`/`yeet` on a
  clean macOS or Linux machine in under 10 seconds.
- A user can do `yoink cat secrets.env`, `yoink some-command`, and then
  `yeet`, `yeet` to pop each item in reverse order.
- `yt --list` makes the stack visible without leaking secrets.
- `yt --version` honestly reports channel, commit, and (for local installs)
  repo path and dirty state.
- Any contributor can file an issue or PR; merges require review.

## Scope Boundary for v0.1.0

Included: push/pop semantics, `--list`, `--show`, `--peek`, `--dry`,
`--drain`, time-filtered drain, `--stats`, `--doctor`, `--version`,
`--update`, `--stable`, `--auto-update`, `--uninstall`,
cross-platform clipboard backends, install/uninstall scripts, CI, cosign
keyless signing, Homebrew tap.

Deferred: Scoop/winget manifests, SLSA provenance attestation, interactive
TUI, named/tagged entries, stack manipulation beyond push/pop (swap, dup,
rotate), pre-commit hooks, automated changelog via git-cliff.
