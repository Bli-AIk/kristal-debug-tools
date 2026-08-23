#!/bin/sh
# Run the kristal-debug-tools GUI without installing anything. No Rust/
# Node/just needed (just is compiled into the kristal-run sidecar).
#   - `just gui` always runs the release binaries; compile mode is only
#     used by `just gui-dev` / `just gui-dev-release`.
#   - Release binaries are selected from the detected Kristal VERSION and
#     downloaded from that exact release tag (SHA256-verified).
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
GUI_REPO="https://github.com/Bli-AIk/kristal-debug-tools-gui.git"
DOWNLOAD_BASE="https://github.com/Bli-AIk/kristal-debug-tools-gui/releases/download"

select_gui_version() {
    if [ -z "$ENGINE_ROOT" ] || [ ! -f "$ENGINE_ROOT/VERSION" ]; then
        echo "[kristal-debug-tools] Could not detect a Kristal VERSION. GUI download is supported only for Kristal 0.10.0 and 0.11.0-dev." >&2
        return 1
    fi

    ENGINE_VERSION=$(sed -n '1p' "$ENGINE_ROOT/VERSION" | tr -d '\r')
    case "$ENGINE_VERSION" in
        0.10.0|v0.10.0)
            GUI_RELEASE_TAG="v0.1.5"
            GUI_SOURCE_REF="v0.1.5"
            GUI_SOURCE_KIND="tag"
            ;;
        0.11.0-dev|v0.11.0-dev)
            GUI_RELEASE_TAG="v0.2.0"
            GUI_SOURCE_REF="v0.2.0"
            GUI_SOURCE_KIND="tag"
            ;;
        *)
            echo "[kristal-debug-tools] Unsupported Kristal VERSION \"$ENGINE_VERSION\". GUI download is supported only for Kristal 0.10.0 and 0.11.0-dev." >&2
            return 1
            ;;
    esac
}

# Download + SHA256-verify one release's assets into $DL_DIR (atomically:
# nothing is overwritten until the whole set verifies). $1 = download base.
download_release() {
    base_url="$1"
    tmp_dir="$DL_DIR/.tmp-download-$$"
    rm -rf "$tmp_dir"
    mkdir -p "$tmp_dir"
    for f in "$GUI_BIN" "$GUI_SIDE" "$CHECKSUMS"; do
        echo "[kristal-debug-tools] downloading $f"
        if ! curl -fsSL -o "$tmp_dir/$f" "$base_url/$f"; then
            rm -rf "$tmp_dir"
            return 1
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
    printf '%s\n' "$GUI_RELEASE_TAG" > "$VERSION_FILE"
}

# Verify the cached binaries against the checksums file. Returns 0 when both
# pass, 1 otherwise.
verify_cached() {
    (cd "$DL_DIR" && sha256sum -c "$CHECKSUMS" >/dev/null 2>&1)
}

prepare_gui_source() {
    if [ ! -e "$GUI_REPO_DIR/.git" ]; then
        if [ -e "$GUI_REPO_DIR" ]; then
            echo "[kristal-debug-tools] $GUI_REPO_DIR exists but is not a git checkout; remove it or clone manually." >&2
            return 1
        fi
        echo "[kristal-debug-tools] Cloning GUI source at $GUI_SOURCE_REF..."
        git clone --depth 1 --branch "$GUI_SOURCE_REF" "$GUI_REPO" "$GUI_REPO_DIR" || return 1
        return 0
    fi

    if ! source_status=$(git -C "$GUI_REPO_DIR" status --porcelain); then
        echo "[kristal-debug-tools] Could not inspect $GUI_REPO_DIR." >&2
        return 1
    fi
    if [ -n "$source_status" ]; then
        echo "[kristal-debug-tools] $GUI_REPO_DIR has local changes; gui-dev will not switch or update it." >&2
        return 1
    fi

    if [ "$GUI_SOURCE_KIND" = "tag" ]; then
        git -C "$GUI_REPO_DIR" fetch origin "refs/tags/$GUI_SOURCE_REF:refs/tags/$GUI_SOURCE_REF" || return 1
        git -C "$GUI_REPO_DIR" switch --detach "$GUI_SOURCE_REF" || return 1
        return 0
    fi

    git -C "$GUI_REPO_DIR" fetch origin "refs/heads/$GUI_SOURCE_REF:refs/remotes/origin/$GUI_SOURCE_REF" || return 1
    if git -C "$GUI_REPO_DIR" show-ref --verify --quiet "refs/heads/$GUI_SOURCE_REF"; then
        git -C "$GUI_REPO_DIR" switch "$GUI_SOURCE_REF" || return 1
    else
        git -C "$GUI_REPO_DIR" switch --create "$GUI_SOURCE_REF" "origin/$GUI_SOURCE_REF" || return 1
    fi
    if ! git -C "$GUI_REPO_DIR" merge-base --is-ancestor HEAD "origin/$GUI_SOURCE_REF"; then
        echo "[kristal-debug-tools] $GUI_REPO_DIR cannot fast-forward to $GUI_SOURCE_REF; gui-dev will not overwrite local commits." >&2
        return 1
    fi
    git -C "$GUI_REPO_DIR" merge --ff-only "origin/$GUI_SOURCE_REF" || return 1
}

if ! select_gui_version; then
    exit 1
fi
RELEASE_BASE="$DOWNLOAD_BASE/$GUI_RELEASE_TAG"

# Test hook: print the version mapping and exit before downloading/launching.
if [ "${KRISTAL_DEBUG_TOOLS_GUI_PRINT_SELECTION:-0}" = "1" ]; then
    printf 'ENGINE_VERSION=%s\n' "$ENGINE_VERSION"
    printf 'GUI_RELEASE_TAG=%s\n' "$GUI_RELEASE_TAG"
    printf 'GUI_SOURCE_REF=%s\n' "$GUI_SOURCE_REF"
    printf 'GUI_SOURCE_KIND=%s\n' "$GUI_SOURCE_KIND"
    printf 'GUI_RELEASE_URL=%s\n' "$RELEASE_BASE"
    exit 0
fi

# GUI source is optional and cloned into the shared tools dir (gui-src).
GUI_REPO_DIR="$TOOLS_DIR/gui-src"
DL_DIR="$TOOLS_DIR/gui"
mkdir -p "$DL_DIR"
VERSION_FILE="$DL_DIR/version.txt"

case "$MODE_ARG" in
    compile|compile-release) MODE="$MODE_ARG" ;;
    *) MODE=bin ;;
esac

if [ "$MODE" = compile ] || [ "$MODE" = compile-release ]; then
    if ! prepare_gui_source; then
        echo "[kristal-debug-tools] gui-dev could not prepare the compatible source checkout." >&2
        exit 1
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

need_download=false
if [ ! -f "$GUI_BIN" ] || [ ! -f "$GUI_SIDE" ] || [ ! -f "$CHECKSUMS" ]; then
    need_download=true
elif [ "$(cat "$VERSION_FILE" 2>/dev/null || true)" != "$GUI_RELEASE_TAG" ] || ! verify_cached; then
    need_download=true
fi

if [ "$need_download" = false ]; then
    echo "[kristal-debug-tools] Using verified GUI release $GUI_RELEASE_TAG for Kristal $ENGINE_VERSION."
    exec "./$GUI_BIN"
fi

echo "[kristal-debug-tools] Downloading GUI release $GUI_RELEASE_TAG for Kristal $ENGINE_VERSION..."
if download_release "$RELEASE_BASE"; then
    exec "./$GUI_BIN"
fi
echo "[kristal-debug-tools] Could not download or verify GUI release $GUI_RELEASE_TAG. Check your network or build locally." >&2
exit 1
