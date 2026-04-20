#!/bin/sh
# yoink-n-yeet uninstaller
#
# Usage:
#   yt --uninstall          # invoked from the installed binary
#   ./uninstall.sh          # directly
#   ./uninstall.sh --purge  # also remove stack data, stats, and config
#   ./uninstall.sh --yes    # skip confirmation prompts

set -eu

REPO="CoreyRDean/yoink-n-yeet"
APP="yoink-n-yeet"
PURGE=0
ASSUME_YES=0

log() { printf '[uninstall] %s\n' "$*" >&2; }

while [ $# -gt 0 ]; do
    case "$1" in
        --purge) PURGE=1; shift ;;
        --yes|-y) ASSUME_YES=1; shift ;;
        -h|--help)
            sed -n '2,10p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *) log "unknown arg: $1 (ignored)"; shift ;;
    esac
done

PREFIX="${PREFIX:-$HOME/.local}"
BINDIR="${BINDIR:-$PREFIX/bin}"
SHAREDIR="$PREFIX/share/$APP"

prompt() {
    msg="$1"
    if [ "$ASSUME_YES" -eq 1 ] || [ ! -t 0 ]; then
        return 0
    fi
    printf '%s [y/N] ' "$msg" >&2
    read -r ans || ans=""
    case "$ans" in y|Y|yes|YES) return 0 ;; *) return 1 ;; esac
}

if prompt "remove binary + symlinks from $BINDIR?"; then
    for n in yoink yeet yk yt "$APP" "$APP.exe"; do
        rm -f "$BINDIR/$n" && log "removed $BINDIR/$n" || true
    done
fi

rm -rf "$SHAREDIR" 2>/dev/null || true

# Remove completion files where we might have installed them.
for f in \
    "$HOME/.local/share/bash-completion/completions/$APP" \
    "/etc/bash_completion.d/$APP" \
    "$HOME/.local/share/zsh/site-functions/_$APP" \
    "$HOME/.zfunc/_$APP"
do
    rm -f "$f" 2>/dev/null && log "removed $f" || true
done

# --purge also wipes user data + config + cache.
if [ "$PURGE" -eq 1 ] || prompt "remove stack data, stats, config, and cache (--purge)?"; then
    case "$(uname -s)" in
        Darwin)
            rm -rf "$HOME/Library/Application Support/$APP" \
                   "$HOME/Library/Preferences/$APP" \
                   "$HOME/Library/Caches/$APP" 2>/dev/null || true
            ;;
        Linux|FreeBSD)
            rm -rf "${XDG_DATA_HOME:-$HOME/.local/share}/$APP" \
                   "${XDG_CONFIG_HOME:-$HOME/.config}/$APP" \
                   "${XDG_CACHE_HOME:-$HOME/.cache}/$APP" 2>/dev/null || true
            ;;
    esac
    log "purged user data"
fi

log "done."
