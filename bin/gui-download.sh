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

# Shared tools dir, hosted next to the Kristal engine so the GUI cache is shared
# across mods and the mod tree stays clean. Resolution mirrors the build scripts
# (build-helper/lib.sh): explicit KRISTAL_ROOT / THRASH_MACHINE_KRISTAL_DIR
# (skipping the mod-root .build/Kristal clone) → nearest engine by walking up
# from the mod root (main.lua + src/kristal.lua) → fall back to the mod root.
TOOLS_DIR="$MOD_ROOT/.tools"
for candidate in "${KRISTAL_ROOT:-}" "${THRASH_MACHINE_KRISTAL_DIR:-}"; do
    [ -n "$candidate" ] || continue
    [ "$candidate" = "$MOD_ROOT/.build/Kristal" ] && continue
    [ -f "$candidate/main.lua" ] && { TOOLS_DIR="$candidate/.tools"; break; }
done
if [ "$TOOLS_DIR" = "$MOD_ROOT/.tools" ]; then
    dir="$MOD_ROOT"
    while :; do
        if [ -f "$dir/main.lua" ] && [ -f "$dir/src/kristal.lua" ]; then
            TOOLS_DIR="$dir/.tools"
            break
        fi
        parent=$(dirname "$dir")
        [ "$parent" = "$dir" ] && break
        dir=$parent
    done
fi
# GUI source is optional and cloned into the shared tools dir (gui-src).
GUI_REPO_DIR="$TOOLS_DIR/gui-src"

DL_DIR="$TOOLS_DIR/gui"
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

# Detect host architecture for the matching release assets.
# macOS keeps the x64 name (no macOS assets are produced); only Linux
# switches to arm64 on aarch64 hosts.
ARCH="x64"
case "$(uname -s)" in
    Linux)
        case "$(uname -m)" in
            aarch64|arm64) ARCH="arm64" ;;
        esac
        ;;
esac
GUI_BIN="kristal-debug-tools-gui-linux-${ARCH}"
GUI_SIDE="kristal-run-linux-${ARCH}"
CHECKSUMS="checksums-linux-${ARCH}.txt"
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
