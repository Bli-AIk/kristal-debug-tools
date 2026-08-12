#!/bin/sh
# Run the kristal-debug-tools GUI without installing anything. No Rust/
# Node/just needed (just is compiled into the kristal-run sidecar).
#   - `just gui` always runs the release binaries; compile mode is only
#     used by `just gui-dev` / `just gui-dev-release`.
#   - Release binaries are auto-checked against the latest GitHub release
#     and re-downloaded when a newer version exists (SHA256-verified).
set -eu

MOD_ROOT="${1:-$(pwd)}"
MODE_ARG="${2:-}"
# GUI source is optional and cloned into <mod-root>/.tools/gui-src
# (.tools/ is ignored), not into libraries/.
GUI_REPO_DIR="$MOD_ROOT/.tools/gui-src"

DL_DIR="$MOD_ROOT/.tools/gui"
mkdir -p "$DL_DIR"
VERSION_FILE="$DL_DIR/version.txt"
GUI_REPO="https://github.com/Bli-AIk/kristal-debug-tools-gui.git"
RELEASE_BASE="https://github.com/Bli-AIk/kristal-debug-tools-gui/releases/latest/download"
RELEASE_API="https://api.github.com/repos/Bli-AIk/kristal-debug-tools-gui/releases/latest"

latest_version() {
    curl -fsSL --max-time 10 -H "User-Agent: kristal-debug-tools-gui" "$RELEASE_API" \
        | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
        | head -1
}

case "$MODE_ARG" in
    compile|compile-release) MODE="$MODE_ARG" ;;
    *) MODE=bin ;;
esac

if [ "$MODE" = compile ] || [ "$MODE" = compile-release ]; then
    if [ ! -e "$GUI_REPO_DIR/.git" ]; then
        if [ -e "$GUI_REPO_DIR" ]; then
            echo "[kristal-debug-tools] $GUI_REPO_DIR exists but is not a git checkout; remove it or clone manually." >&2
        else
            echo "[kristal-debug-tools] Cloning GUI source for local compile..."
            if ! git clone --depth 1 "$GUI_REPO" "$GUI_REPO_DIR"; then
                echo "[kristal-debug-tools] Clone failed, falling back to release binaries."
            fi
        fi
    fi
    # tauri dev only compiles the main bin; build the kristal-run sidecar
    # first or the task list comes up empty.
    CARGO_FLAG=""
    DEV_FLAG=""
    if [ "$MODE" = compile-release ]; then
        CARGO_FLAG="--release"
        DEV_FLAG="--release"
    fi
    if [ -e "$GUI_REPO_DIR/.git" ] && (cd "$GUI_REPO_DIR" && { [ -d node_modules ] || npm ci; } \
        && cargo build $CARGO_FLAG --manifest-path src-tauri/Cargo.toml --bin kristal-run \
        && npm run tauri dev -- $DEV_FLAG); then
        exit 0
    fi
    echo "[kristal-debug-tools] Local compile failed, falling back to release binaries."
fi

cd "$DL_DIR"

GUI_BIN="kristal-debug-tools-gui-linux-x64"
GUI_SIDE="kristal-run-linux-x64"
CHECKSUMS="checksums.txt"
LATEST="$(latest_version 2>/dev/null || true)"

need_download=false
if [ ! -f "$GUI_BIN" ] || [ ! -f "$GUI_SIDE" ] || [ ! -f "$CHECKSUMS" ]; then
    need_download=true
elif [ -n "$LATEST" ] && [ "$(cat "$VERSION_FILE" 2>/dev/null || true)" != "$LATEST" ]; then
    need_download=true
fi

if [ "$need_download" = false ]; then
    if [ -z "$LATEST" ]; then
        echo "[kristal-debug-tools] Could not check for updates, using cached build."
    fi
    exec "./$GUI_BIN"
fi

echo "[kristal-debug-tools] Downloading the GUI (latest release)..."
for f in "$GUI_BIN" "$GUI_SIDE" "$CHECKSUMS"; do
    echo "[kristal-debug-tools] downloading $f"
    curl -fsSL -o "$f" "$RELEASE_BASE/$f"
done

sha256sum -c "$CHECKSUMS"
chmod +x "$GUI_BIN" "$GUI_SIDE"
if [ -n "$LATEST" ]; then
    printf '%s\n' "$LATEST" > "$VERSION_FILE"
fi
exec "./$GUI_BIN"
