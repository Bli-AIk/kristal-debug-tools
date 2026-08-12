#!/bin/sh
# Run the kristal-debug-tools GUI without installing anything. No Rust/
# Node/just needed (just is compiled into the kristal-run sidecar).
#   - A local release build is used when present.
#   - The choice between release binaries and local compile is asked once
#     and remembered in <mod-root>/.tools/gui/settings.json; `just gui
#     bin|compile` overrides it.
#   - Release binaries are auto-checked against the latest GitHub release
#     and re-downloaded when a newer version exists (SHA256-verified).
set -eu

MOD_ROOT="${1:-$(pwd)}"
MODE_ARG="${2:-}"
# GUI source is optional and cloned into <mod-root>/.tools/gui-src
# (.tools/ is ignored), not into libraries/.
GUI_REPO_DIR="$MOD_ROOT/.tools/gui-src"
LOCAL_BIN="$GUI_REPO_DIR/src-tauri/target/release/kristal-debug-tools-gui"
if [ -x "$LOCAL_BIN" ]; then
    shift 2>/dev/null || true
    exec "$LOCAL_BIN" "$@"
fi

DL_DIR="$MOD_ROOT/.tools/gui"
mkdir -p "$DL_DIR"
SETTINGS="$DL_DIR/settings.json"
VERSION_FILE="$DL_DIR/version.txt"
GUI_REPO="https://github.com/Bli-AIk/kristal-debug-tools-gui.git"
RELEASE_BASE="https://github.com/Bli-AIk/kristal-debug-tools-gui/releases/latest/download"
RELEASE_API="https://api.github.com/repos/Bli-AIk/kristal-debug-tools-gui/releases/latest"

read_mode() {
    # extract "mode": "..." from the JSON without requiring jq
    sed -n 's/.*"mode"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$SETTINGS" 2>/dev/null | head -1
}

write_mode() {
    if [ -f "$SETTINGS" ] && grep -q '"mode"' "$SETTINGS" 2>/dev/null; then
        sed -i "s/\"mode\"[[:space:]]*:[[:space:]]*\"[^\"]*\"/\"mode\": \"$1\"/" "$SETTINGS"
    elif [ -f "$SETTINGS" ]; then
        # settings.json exists without a mode key — append it as the
        # last member of the top-level object (plain one-level JSON)
        sed -i "\$s/^}/,\n  \"mode\": \"$1\"\n}/" "$SETTINGS"
    else
        echo "{\"mode\": \"$1\"}" > "$SETTINGS"
    fi
}

latest_version() {
    curl -fsSL --max-time 10 -H "User-Agent: kristal-debug-tools-gui" "$RELEASE_API" \
        | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
        | head -1
}

case "$MODE_ARG" in
    bin|compile) MODE="$MODE_ARG"; write_mode "$MODE" ;;
    *)
        MODE="$(read_mode)"
        if [ -z "$MODE" ] && command -v cargo >/dev/null 2>&1 && command -v node >/dev/null 2>&1; then
            echo "[kristal-debug-tools] Detected a local compile toolchain (cargo + node)."
            printf "[B] use release binaries (default)  [C] compile and run locally: "
            read -t 5 ans || ans=B
            case "$ans" in
                [Cc]*) MODE=compile ;;
                *) MODE=bin ;;
            esac
            write_mode "$MODE"
            echo "[kristal-debug-tools] Remembered (edit .tools/gui/settings.json or pass bin|compile to change)."
        fi
        [ -z "$MODE" ] && MODE=bin
        ;;
esac

if [ "$MODE" = compile ]; then
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
    if [ -e "$GUI_REPO_DIR/.git" ] && (cd "$GUI_REPO_DIR" && { [ -d node_modules ] || npm ci; } \
        && cargo build --manifest-path src-tauri/Cargo.toml --bin kristal-run \
        && npm run tauri dev); then
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
