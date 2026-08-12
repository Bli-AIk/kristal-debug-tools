#!/bin/sh
# Download the latest kristal-debug-tools-gui release binaries (Linux x64)
# into <mod-root>/.tools/gui/ and run the GUI. No Rust/Node/just needed.
# Checksums are verified against the release's checksums.txt.
set -eu

MOD_ROOT="${1:-$(pwd)}"
GUI_REPO_DIR="$(dirname "$0")/../kristal-debug-tools-gui"
LOCAL_BIN="$GUI_REPO_DIR/src-tauri/target/release/kristal-debug-tools-gui"
if [ -x "$LOCAL_BIN" ]; then
    shift 2>/dev/null || true
    exec "$LOCAL_BIN" "$@"
fi

DL_DIR="$MOD_ROOT/.tools/gui"
mkdir -p "$DL_DIR"
cd "$DL_DIR"

BASE="https://github.com/Bli-AIk/kristal-debug-tools-gui/releases/latest/download"
for f in kristal-debug-tools-gui-linux-x64 kristal-run-linux-x64 checksums.txt; do
    echo "[kristal-debug-tools] downloading $f"
    curl -fsSL -o "$f" "$BASE/$f"
done

sha256sum -c checksums.txt
chmod +x kristal-debug-tools-gui-linux-x64 kristal-run-linux-x64
exec ./kristal-debug-tools-gui-linux-x64
