#!/bin/sh
# yoink-n-yeet installer
#
# Examples:
#   curl -fsSL https://raw.githubusercontent.com/CoreyRDean/yoink-n-yeet/main/install.sh | bash
#   ./install.sh --channel nightly
#   ./install.sh --version v0.1.0
#   ./install.sh --local              # build from current working tree
#   ./install.sh --prefix /usr/local  # install location override
#
# Environment variables honored:
#   PREFIX             where to install (default: $HOME/.local)
#   BINDIR             binary dir (default: $PREFIX/bin)
#   YNY_NO_COMPLETIONS set to skip shell completion install
#
# Design notes:
# * Prints every step to stderr. stdout is reserved for the binary path on success.
# * Fails fast on unsupported OS/arch.
# * Idempotent: re-running upgrades in place and refreshes symlinks.
# * Non-interactive safe (curl | bash): never prompts unless stdin is a TTY.

set -eu

REPO="CoreyRDean/yoink-n-yeet"
APP="yoink-n-yeet"
CHANNEL="stable"
VERSION=""
LOCAL=0

log()  { printf '[install] %s\n' "$*" >&2; }
die()  { printf '[install] error: %s\n' "$*" >&2; exit 1; }

# ---------- argv ----------
while [ $# -gt 0 ]; do
    case "$1" in
        --channel)  CHANNEL="${2:?}"; shift 2 ;;
        --version)  VERSION="${2:?}"; shift 2 ;;
        --local)    LOCAL=1; shift ;;
        --prefix)   PREFIX="${2:?}"; shift 2 ;;
        -h|--help)
            sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *) die "unknown arg: $1 (see --help)" ;;
    esac
done

PREFIX="${PREFIX:-$HOME/.local}"
BINDIR="${BINDIR:-$PREFIX/bin}"
SHAREDIR="$PREFIX/share/$APP"

mkdir -p "$BINDIR" "$SHAREDIR"

# ---------- platform ----------
uname_s=$(uname -s)
uname_m=$(uname -m)
case "$uname_s" in
    Darwin)  GOOS=darwin  ;;
    Linux)   GOOS=linux   ;;
    MINGW*|MSYS*|CYGWIN*) GOOS=windows ;;
    *) die "unsupported OS: $uname_s" ;;
esac
case "$uname_m" in
    x86_64|amd64) GOARCH=amd64 ;;
    arm64|aarch64) GOARCH=arm64 ;;
    *) die "unsupported arch: $uname_m" ;;
esac
log "platform: $GOOS/$GOARCH"
log "prefix:   $PREFIX"

# ---------- local source path ----------
if [ "$LOCAL" -eq 1 ]; then
    [ -f go.mod ] || die "--local must be run from inside the repo (go.mod not found)"
    command -v go >/dev/null 2>&1 || die "go toolchain required for --local"
    COMMIT=$(git rev-parse HEAD 2>/dev/null || echo unknown)
    DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    REPO_PATH=$(pwd -P)
    VERSION="${VERSION:-v0.0.0-dev}"

    log "building from source ($COMMIT)"
    go build -trimpath -ldflags "\
        -X github.com/CoreyRDean/yoink-n-yeet/internal/buildinfo.Version=$VERSION \
        -X github.com/CoreyRDean/yoink-n-yeet/internal/buildinfo.Commit=$COMMIT \
        -X github.com/CoreyRDean/yoink-n-yeet/internal/buildinfo.Channel=local \
        -X github.com/CoreyRDean/yoink-n-yeet/internal/buildinfo.Date=$DATE \
        -X github.com/CoreyRDean/yoink-n-yeet/internal/buildinfo.RepoPath=$REPO_PATH" \
        -o "$BINDIR/$APP" ./cmd/yoink-n-yeet

    CHANNEL=local
    cp -f install.sh uninstall.sh "$SHAREDIR/" 2>/dev/null || true

# ---------- release path ----------
else
    command -v curl >/dev/null 2>&1 || die "curl is required"
    command -v tar  >/dev/null 2>&1 || die "tar is required"

    # Resolve a concrete tag.
    if [ -z "$VERSION" ]; then
        api="https://api.github.com/repos/$REPO/releases/latest"
        if [ "$CHANNEL" = "nightly" ]; then
            api="https://api.github.com/repos/$REPO/releases"
        fi
        log "resolving latest $CHANNEL release"
        if [ "$CHANNEL" = "stable" ]; then
            VERSION=$(curl -fsSL "$api" | grep -E '"tag_name":' | head -1 | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
        else
            VERSION=$(curl -fsSL "$api" | grep -E '"tag_name":' | grep -- '-nightly\.' | head -1 | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
        fi
        [ -n "$VERSION" ] || die "could not resolve latest $CHANNEL version"
    fi
    log "installing $VERSION"

    ext="tar.gz"
    [ "$GOOS" = "windows" ] && ext="zip"
    archive="${APP}_${VERSION#v}_${GOOS}_${GOARCH}.${ext}"
    url="https://github.com/$REPO/releases/download/$VERSION/$archive"
    sums_url="https://github.com/$REPO/releases/download/$VERSION/${APP}_${VERSION#v}_checksums.txt"

    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT
    log "downloading $archive"
    curl -fsSL -o "$tmp/$archive" "$url" || die "download failed: $url"

    # Verify sha256 against the published checksum file.
    if curl -fsSL -o "$tmp/SHA256SUMS" "$sums_url"; then
        log "verifying sha256"
        (cd "$tmp" && grep "$archive" SHA256SUMS | shasum -a 256 -c -) \
            || die "sha256 mismatch for $archive"
    else
        log "warning: checksum file not available ($sums_url); skipping verification"
    fi

    # Extract into tmp, then copy the binary in.
    (cd "$tmp" && if [ "$ext" = "zip" ]; then unzip -q "$archive"; else tar -xzf "$archive"; fi)
    if [ -f "$tmp/$APP" ]; then
        install -m 0755 "$tmp/$APP" "$BINDIR/$APP"
    elif [ -f "$tmp/$APP.exe" ]; then
        install -m 0755 "$tmp/$APP.exe" "$BINDIR/$APP.exe"
    else
        die "binary not found in archive"
    fi

    # Also stash the install/uninstall scripts for --uninstall to reuse.
    cp -f "$tmp/install.sh"   "$SHAREDIR/install.sh"   2>/dev/null || true
    cp -f "$tmp/uninstall.sh" "$SHAREDIR/uninstall.sh" 2>/dev/null || true
fi

# ---------- symlinks ----------
ensure_symlink() {
    target="$1"; link="$2"
    # Clean any existing file/symlink, then create a fresh one.
    rm -f "$link"
    ln -s "$target" "$link"
}
for n in yoink yeet yk yt; do
    ensure_symlink "$BINDIR/$APP" "$BINDIR/$n"
done
log "installed: $BINDIR/{yoink,yeet,yk,yt} → $APP"

# ---------- shell completions (optional) ----------
if [ "${YNY_NO_COMPLETIONS:-}" != "1" ]; then
    # bash
    for d in "$HOME/.local/share/bash-completion/completions" /etc/bash_completion.d; do
        if [ -w "$d" ] || mkdir -p "$d" 2>/dev/null; then
            cat >"$d/$APP" 2>/dev/null <<'EOF' || true
_yoink_n_yeet_complete() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local flags="--list --show --peek --dry --drain --clear --days --hours --stats --doctor --version --update --stable --auto-update --no-update-check --uninstall --json --help"
  COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
}
complete -F _yoink_n_yeet_complete yoink yeet yk yt yoink-n-yeet
EOF
            log "installed bash completions to $d"
            break
        fi
    done
    # zsh
    for d in "$HOME/.local/share/zsh/site-functions" "$HOME/.zfunc"; do
        if [ -w "$d" ] || mkdir -p "$d" 2>/dev/null; then
            cat >"$d/_$APP" 2>/dev/null <<'EOF' || true
#compdef yoink yeet yk yt yoink-n-yeet
_arguments '*:flag:(--list --show --peek --dry --drain --clear --days --hours --stats --doctor --version --update --stable --auto-update --no-update-check --uninstall --json --help)'
EOF
            log "installed zsh completions to $d"
            break
        fi
    done
fi

# ---------- config ----------
data_config_dir() {
    case "$GOOS" in
        darwin) printf '%s\n' "$HOME/Library/Preferences/$APP" ;;
        *)      printf '%s\n' "${XDG_CONFIG_HOME:-$HOME/.config}/$APP" ;;
    esac
}
CFG_DIR=$(data_config_dir)
mkdir -p "$CFG_DIR"
CFG_FILE="$CFG_DIR/config.json"

if [ ! -f "$CFG_FILE" ]; then
    INSTALLED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    if [ "$LOCAL" -eq 1 ]; then
        cat >"$CFG_FILE" <<EOF
{
  "version": 1,
  "channel": "local",
  "auto_update": false,
  "update_check": true,
  "preview_width": 80,
  "max_depth": 0,
  "local_repo_path": "$REPO_PATH",
  "installed_at": "$INSTALLED_AT",
  "installed_version": "$VERSION"
}
EOF
    else
        cat >"$CFG_FILE" <<EOF
{
  "version": 1,
  "channel": "$CHANNEL",
  "auto_update": false,
  "update_check": true,
  "preview_width": 80,
  "max_depth": 0,
  "installed_at": "$INSTALLED_AT",
  "installed_version": "$VERSION"
}
EOF
    fi
    log "wrote config: $CFG_FILE"
else
    # Re-run: patch channel + installed_* metadata without destroying user prefs.
    tmp_cfg=$(mktemp)
    INSTALLED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    awk -v ch="$CHANNEL" -v ia="$INSTALLED_AT" -v iv="$VERSION" -v rp="${REPO_PATH:-}" '
        BEGIN { patched_ch=0; patched_ia=0; patched_iv=0; patched_rp=0 }
        /"channel"[[:space:]]*:/        { print "  \"channel\": \"" ch "\","; patched_ch=1; next }
        /"installed_at"[[:space:]]*:/   { print "  \"installed_at\": \"" ia "\","; patched_ia=1; next }
        /"installed_version"[[:space:]]*:/ { print "  \"installed_version\": \"" iv "\","; patched_iv=1; next }
        /"local_repo_path"[[:space:]]*:/ { if (rp != "") { print "  \"local_repo_path\": \"" rp "\","; patched_rp=1 } else { next } ; next }
        { print }
    ' "$CFG_FILE" >"$tmp_cfg"
    mv "$tmp_cfg" "$CFG_FILE"
    log "updated config: $CFG_FILE"
fi

# ---------- done ----------
if ! printf '%s' "$PATH" | tr ':' '\n' | grep -Fx "$BINDIR" >/dev/null; then
    log ""
    log "NOTE: $BINDIR is not on your PATH yet. Add this to your shell rc:"
    log "    export PATH=\"$BINDIR:\$PATH\""
fi

log "done."
printf '%s\n' "$BINDIR/$APP"
