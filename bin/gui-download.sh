#!/bin/sh
# Run the kristal-debug-tools GUI without installing anything. No Rust/
# Node/just needed (just is compiled into the kristal-run sidecar).
#   - A local release build is used when present.
#   - The choice between release binaries and local compile is asked once
#     and remembered in <mod-root>/.tools/gui/.mode; `just gui bin|compile`
#     overrides it.
#   - Release binaries are downloaded on first use (SHA256-verified).
set -eu

MOD_ROOT="${1:-$(pwd)}"
MODE_ARG="${2:-}"
# this script lives in <lib>/bin; the gui repo is a sibling submodule at
# <mod-root>/libraries/kristal-debug-tools-gui — two levels up.
GUI_REPO_DIR="$(dirname "$0")/../../kristal-debug-tools-gui"
LOCAL_BIN="$GUI_REPO_DIR/src-tauri/target/release/kristal-debug-tools-gui"
if [ -x "$LOCAL_BIN" ]; then
    shift 2>/dev/null || true
    exec "$LOCAL_BIN" "$@"
fi

DL_DIR="$MOD_ROOT/.tools/gui"
mkdir -p "$DL_DIR"
SETTINGS="$DL_DIR/settings.json"

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
    # tauri dev only compiles the main bin; build the kristal-run sidecar
    # first or the task list comes up empty.
    if (cd "$GUI_REPO_DIR" && { [ -d node_modules ] || npm ci; } \
        && cargo build --manifest-path src-tauri/Cargo.toml --bin kristal-run \
        && npm run tauri dev); then
        exit 0
    fi
    echo "[kristal-debug-tools] Local compile failed, falling back to release binaries."
fi

cd "$DL_DIR"

BASE="https://github.com/Bli-AIk/kristal-debug-tools-gui/releases/latest/download"
for f in kristal-debug-tools-gui-linux-x64 kristal-run-linux-x64 checksums.txt; do
    echo "[kristal-debug-tools] downloading $f"
    curl -fsSL -o "$f" "$BASE/$f"
done

sha256sum -c checksums.txt
chmod +x kristal-debug-tools-gui-linux-x64 kristal-run-linux-x64
exec ./kristal-debug-tools-gui-linux-x64
