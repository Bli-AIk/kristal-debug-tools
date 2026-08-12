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
MODE_FILE="$DL_DIR/.mode"

case "$MODE_ARG" in
    bin|compile) MODE="$MODE_ARG"; printf '%s\n' "$MODE" > "$MODE_FILE" ;;
    *)
        if [ -f "$MODE_FILE" ]; then
            MODE="$(cat "$MODE_FILE")"
        elif command -v cargo >/dev/null 2>&1 && command -v node >/dev/null 2>&1; then
            echo "[kristal-debug-tools] Detected a local compile toolchain (cargo + node)."
            printf "[B] use release binaries (default)  [C] compile and run locally: "
            read -t 5 ans || ans=B
            case "$ans" in
                [Cc]*) MODE=compile ;;
                *) MODE=bin ;;
            esac
            printf '%s\n' "$MODE" > "$MODE_FILE"
            echo "[kristal-debug-tools] Remembered (delete .tools/gui/.mode or pass bin|compile to change)."
        else
            MODE=bin
        fi
        ;;
esac

if [ "$MODE" = compile ]; then
    if (cd "$GUI_REPO_DIR" && { [ -d node_modules ] || npm ci; } && npm run tauri dev); then
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
