#!/bin/sh
# Run the kristal-debug-tools GUI without installing anything. No Rust/
# Node/just needed (just is compiled into the kristal-run sidecar).
#   - `just gui` always runs the release binaries; compile mode is only
#     used by `just gui-dev` / `just gui-dev-release`.
#   - Release binaries are auto-checked against the latest GitHub release
#     and re-downloaded when a newer version exists (SHA256-verified).
#   - If the latest release's assets are not uploaded yet (e.g. release-please
#     just cut the tag and CI is still building), the previous release is
#     downloaded instead. The cached version is always shown and re-checked on
#     the next run, so it self-heals back to the latest once it is ready.
set -eu

MOD_ROOT="${1:-$(pwd)}"
MODE_ARG="${2:-}"

# Shared tools dir, hosted next to the Kristal engine so the GUI cache is shared
# across mods and the mod tree stays clean. Resolution is local-first, mirroring
# bin/kristal-run and the build scripts (build-helper/lib.sh): the nearest engine
# by walking up from the mod root (main.lua + src/kristal.lua) wins, so a mod
# living inside its own engine fork (e.g. el-mods/ inside kristal-el) is never
# hijacked by a KRISTAL_ROOT inherited from the shell profile. Explicit
# KRISTAL_ROOT / THRASH_MACHINE_KRISTAL_DIR (skipping the mod-root .build/Kristal
# clone) are only a fallback for mods outside an engine tree; the final fallback
# is the mod root itself.
TOOLS_DIR="$MOD_ROOT/.tools"
ENGINE_ROOT=""
dir="$MOD_ROOT"
while :; do
    if [ -f "$dir/main.lua" ] && [ -f "$dir/src/kristal.lua" ]; then
        TOOLS_DIR="$dir/.tools"
        ENGINE_ROOT="$dir"
        break
    fi
    parent=$(dirname "$dir")
    [ "$parent" = "$dir" ] && break
    dir=$parent
done
if [ -z "$ENGINE_ROOT" ]; then
    for candidate in "${KRISTAL_ROOT:-}" "${THRASH_MACHINE_KRISTAL_DIR:-}"; do
        [ -n "$candidate" ] || continue
        [ "$candidate" = "$MOD_ROOT/.build/Kristal" ] && continue
        if [ -f "$candidate/main.lua" ]; then
            TOOLS_DIR="$candidate/.tools"
            ENGINE_ROOT="$candidate"
            break
        fi
    done
fi

# Export the resolved roots to the GUI app in both modes. The GUI and the
# kristal-run sidecar resolve the mod by walking up from cwd or reading
# KDT_MOD_ROOT; the GUI binary is launched from the shared .tools/gui next
# to the engine, so walking up can never reach the mod there. Passing the
# roots explicitly keeps bin and compile modes working. Export KRISTAL_ROOT
# only when a real engine was resolved, so the GUI reports "engine not
# found" accurately instead of showing a bogus path.
export KDT_MOD_ROOT="$MOD_ROOT"
if [ -n "$ENGINE_ROOT" ]; then
    export KRISTAL_ROOT="$ENGINE_ROOT"
else
    unset KRISTAL_ROOT
fi

# Test hook: print the resolved roots and exit without downloading/launching.
if [ "${KRISTAL_DEBUG_TOOLS_GUI_PRINT_ENV:-0}" = "1" ]; then
    printf 'KDT_MOD_ROOT=%s\n' "$KDT_MOD_ROOT"
    printf 'KRISTAL_ROOT=%s\n' "${KRISTAL_ROOT:-}"
    exit 0
fi
# GUI source is optional and cloned into the shared tools dir (gui-src).
GUI_REPO_DIR="$TOOLS_DIR/gui-src"

DL_DIR="$TOOLS_DIR/gui"
mkdir -p "$DL_DIR"
VERSION_FILE="$DL_DIR/version.txt"
GUI_REPO="https://github.com/Bli-AIk/kristal-debug-tools-gui.git"
RELEASE_BASE="https://github.com/Bli-AIk/kristal-debug-tools-gui/releases/latest/download"
RELEASE_API="https://api.github.com/repos/Bli-AIk/kristal-debug-tools-gui/releases/latest"
RELEASES_API="https://api.github.com/repos/Bli-AIk/kristal-debug-tools-gui/releases?per_page=10"
DOWNLOAD_BASE="https://github.com/Bli-AIk/kristal-debug-tools-gui/releases/download"

latest_version() {
    curl -fsSL --max-time 10 -H "User-Agent: kristal-debug-tools-gui" "$RELEASE_API" \
        | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
        | head -1
}

# Tag of the second-newest non-draft, non-prerelease release (used as the
# fallback while the newest release's assets are still being built).
previous_version() {
    curl -fsSL --max-time 10 -H "User-Agent: kristal-debug-tools-gui" "$RELEASES_API" \
        | tr ',' '\n' \
        | awk '
            /^[[:space:]]*"tag_name"/ { tag=$0; sub(/^.*"tag_name"[[:space:]]*:[[:space:]]*"/, "", tag); sub(/".*$/, "", tag) }
            /^[[:space:]]*"draft"[[:space:]]*:/ { draft=$0; sub(/^.*"draft"[[:space:]]*:[[:space:]]*/, "", draft); sub(/[,}].*$/, "", draft) }
            /^[[:space:]]*"prerelease"[[:space:]]*:/ { pre=$0; sub(/^.*"prerelease"[[:space:]]*:[[:space:]]*/, "", pre); sub(/[,}].*$/, "", pre) }
            /^[[:space:]]*"published_at"/ && tag != "" { if (draft == "false" && pre == "false") { n++; if (n == 2) { print tag; exit } } }
        ' \
        | tail -1
}

# Download + SHA256-verify one release's assets into $DL_DIR (atomically:
# nothing is overwritten until the whole set verifies). $1 = download base.
# Writes version.txt when the release tag can be resolved from the redirect
# URL (i.e. even when the API check failed earlier).
download_release() {
    base_url="$1"
    tmp_dir="$DL_DIR/.tmp-download-$$"
    rm -rf "$tmp_dir"
    mkdir -p "$tmp_dir"
    location=""
    for f in "$GUI_BIN" "$GUI_SIDE" "$CHECKSUMS"; do
        echo "[kristal-debug-tools] downloading $f"
        if ! curl -fsSL -o "$tmp_dir/$f" "$base_url/$f"; then
            rm -rf "$tmp_dir"
            return 1
        fi
        if [ -z "$location" ]; then
            # GitHub first redirects latest/download to /releases/download/<tag>/...
            # before bouncing to a CDN; the tag comes from that first Location.
            location=$(curl -sSI --max-time 15 "$base_url/$f" 2>/dev/null | tr -d '\r' | awk 'tolower($1)=="location:" {print $2; exit}' || true)
        fi
    done
    if ! (cd "$tmp_dir" && sha256sum -c "$CHECKSUMS"); then
        rm -rf "$tmp_dir"
        return 1
    fi
    mv -f "$tmp_dir/$GUI_BIN" "$DL_DIR/$GUI_BIN"
    mv -f "$tmp_dir/$GUI_SIDE" "$DL_DIR/$GUI_SIDE"
    mv -f "$tmp_dir/$CHECKSUMS" "$DL_DIR/$CHECKSUMS"
    chmod +x "$DL_DIR/$GUI_BIN" "$DL_DIR/$GUI_SIDE"
    rm -rf "$tmp_dir"
    tag=$(printf '%s\n' "$location" | sed -n 's#.*/releases/download/\([^/]*\)/.*#\1#p')
    if [ -n "$tag" ]; then
        printf '%s\n' "$tag" > "$VERSION_FILE"
    fi
}

# Verify the cached binaries against the checksums file. Returns 0 when both
# pass, 1 otherwise.
verify_cached() {
    (cd "$DL_DIR" && sha256sum -c "$CHECKSUMS" >/dev/null 2>&1)
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
elif [ -n "$LATEST" ]; then
    # Re-download when the recorded version differs from the latest OR when
    # the cached files no longer match their checksums.
    if [ "$(cat "$VERSION_FILE" 2>/dev/null || true)" != "$LATEST" ] || ! verify_cached; then
        need_download=true
    fi
fi

if [ "$need_download" = false ]; then
    if [ -z "$LATEST" ]; then
        if verify_cached; then
            VERSION="$(cat "$VERSION_FILE" 2>/dev/null || true)"
            if [ -n "$VERSION" ]; then
                echo "[kristal-debug-tools] Could not check for updates; using verified cached build $VERSION."
            else
                echo "[kristal-debug-tools] Could not check for updates; using verified cached build (version unknown)."
            fi
        else
            echo "[kristal-debug-tools] Cached build failed checksum verification, re-downloading." >&2
            need_download=true
        fi
    fi
    if [ "$need_download" = false ]; then
        exec "./$GUI_BIN"
    fi
fi

echo "[kristal-debug-tools] Downloading the GUI (latest release)..."
if download_release "$RELEASE_BASE"; then
    if [ -n "$LATEST" ] && [ ! -f "$VERSION_FILE" ]; then
        printf '%s\n' "$LATEST" > "$VERSION_FILE"
    fi
    exec "./$GUI_BIN"
fi

# The latest release's assets are not uploaded yet (e.g. CI is still
# building them), so fall back to the previous release.
PREV="$(previous_version 2>/dev/null || true)"
if [ -z "$PREV" ]; then
    echo "[kristal-debug-tools] The latest release is not ready and no previous release was found. Try again later." >&2
    exit 1
fi
if [ -n "$LATEST" ]; then
    echo "[kristal-debug-tools] The latest release ($LATEST) is not ready yet; falling back to previous release $PREV."
else
    echo "[kristal-debug-tools] The latest release is not ready yet; falling back to previous release $PREV."
fi

# If the previous release is already cached and still intact, use it without
# re-downloading (still shown, and re-checked on the next run).
if [ "$(cat "$VERSION_FILE" 2>/dev/null || true)" = "$PREV" ] \
   && [ -f "$GUI_BIN" ] && [ -f "$GUI_SIDE" ] && [ -f "$CHECKSUMS" ] \
   && verify_cached; then
    echo "[kristal-debug-tools] Previous release $PREV is already downloaded and verified."
    exec "./$GUI_BIN"
fi

echo "[kristal-debug-tools] Downloading previous release $PREV..."
if download_release "$DOWNLOAD_BASE/$PREV"; then
    printf '%s\n' "$PREV" > "$VERSION_FILE"
    exec "./$GUI_BIN"
fi

echo "[kristal-debug-tools] Could not download the latest or previous release. Check your network or build locally." >&2
exit 1
