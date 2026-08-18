#!/usr/bin/env bash
set -euo pipefail

root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
project_root=${KRISTAL_DEBUG_TOOLS_TEST_PROJECT_ROOT:-}
runner="$root/bin/kristal-run"
fixture_root=$(mktemp -d)
fixture_engine="$fixture_root/engine"
mkdir -p "$fixture_engine/src"
: > "$fixture_engine/main.lua"
: > "$fixture_engine/src/kristal.lua"

if [ -z "$project_root" ] || [ ! -f "$project_root/mod.json" ]; then
    project_root="$fixture_engine/mod"
    mkdir -p "$project_root"
    printf '%s\n' '{}' > "$project_root/mod.json"
fi

dry_run() {
    KRISTAL_MOD_ROOT="$project_root" \
    KRISTAL_DEBUG_TOOLS_DRY_RUN=1 \
    KRISTAL_ROOT="$fixture_engine" \
    "$runner" "$@"
}

output=$(dry_run --wave 2 --tp 50 --mercy 100)
printf '%s\n' "$output" | grep -F -- '--wave 2' >/dev/null
printf '%s\n' "$output" | grep -F -- '--tp 50' >/dev/null
printf '%s\n' "$output" | grep -F -- '--mercy 100' >/dev/null

output=$(dry_run -wf 2)
printf '%s\n' "$output" | grep -F -- '--wave-force 2' >/dev/null

output=$(dry_run --encounter dummy --initial-tp=25 --initial-mercy=75)
printf '%s\n' "$output" | grep -F -- '--encounter dummy' >/dev/null
printf '%s\n' "$output" | grep -F -- '--tp 25' >/dev/null
printf '%s\n' "$output" | grep -F -- '--mercy 75' >/dev/null

output=$(dry_run --lang zh-hans)
printf '%s\n' "$output" | grep -F -- '--lang zh-hans' >/dev/null

output=$(dry_run --language=en)
printf '%s\n' "$output" | grep -F -- '--lang en' >/dev/null

output=$(dry_run -l zh-hans)
printf '%s\n' "$output" | grep -F -- '--lang zh-hans' >/dev/null

output=$(dry_run -- --custom value)
printf '%s\n' "$output" | grep -F -- '--custom value' >/dev/null

output=$(dry_run -- --disable-stdout-buffer)
printf '%s\n' "$output" | grep -F -- '--disable-stdout-buffer' >/dev/null

if dry_run --tp >/dev/null 2>&1; then
    printf '%s\n' 'missing-value validation failed' >&2
    exit 1
fi

if dry_run --language >/dev/null 2>&1; then
    printf '%s\n' 'language missing-value validation failed' >&2
    exit 1
fi

if dry_run --unknown >/dev/null 2>&1; then
    printf '%s\n' 'unknown-option validation failed' >&2
    exit 1
fi

precedence_root=$(mktemp -d)
gui_root=$(mktemp -d)
selection_root=$(mktemp -d)
trap 'rm -rf "$fixture_root" "$precedence_root" "$gui_root" "$selection_root"' EXIT
mkdir -p "$precedence_root/engine/src" "$precedence_root/engine/mod"
: > "$precedence_root/engine/main.lua"
: > "$precedence_root/engine/src/kristal.lua"
printf '%s\n' '{}' > "$precedence_root/engine/mod/mod.json"
output=$(
    KRISTAL_MOD_ROOT="$precedence_root/engine/mod" \
    KRISTAL_DEBUG_TOOLS_DRY_RUN=1 \
    KRISTAL_ROOT="$precedence_root/override" \
    "$runner"
)
printf '%s\n' "$output" | grep -Fqx "engine_root=$precedence_root/engine"

# gui-download.sh: resolved roots must be exported to the GUI app
# (KDT_MOD_ROOT always; KRISTAL_ROOT only when a real engine is found).
# KRISTAL_DEBUG_TOOLS_GUI_PRINT_ENV=1 prints the roots and exits before
# any download/launch.
make_engine() {
    mkdir -p "$1/src"
    : > "$1/main.lua"
    : > "$1/src/kristal.lua"
}

# 1. KRISTAL_ROOT is only a fallback: a mod outside any engine tree uses it.
make_engine "$gui_root/engine"
printf '%s\n' '{}' > "$gui_root/mod.json"
output=$(KRISTAL_DEBUG_TOOLS_GUI_PRINT_ENV=1 KRISTAL_ROOT="$gui_root/engine" THRASH_MACHINE_KRISTAL_DIR= "$root/bin/gui-download.sh" "$gui_root")
printf '%s\n' "$output" | grep -Fqx "KDT_MOD_ROOT=$gui_root"
printf '%s\n' "$output" | grep -Fqx "KRISTAL_ROOT=$gui_root/engine"

# 2. THRASH_MACHINE_KRISTAL_DIR is a fallback when KRISTAL_ROOT is unset.
output=$(KRISTAL_DEBUG_TOOLS_GUI_PRINT_ENV=1 KRISTAL_ROOT= THRASH_MACHINE_KRISTAL_DIR="$gui_root/engine" "$root/bin/gui-download.sh" "$gui_root")
printf '%s\n' "$output" | grep -Fqx "KRISTAL_ROOT=$gui_root/engine"

# 3. local-first: the nearest engine by walking up wins even when KRISTAL_ROOT
#    points at a different engine (the regression this fixes: a shell-wide
#    KRISTAL_ROOT used to hijack mods living inside their own engine fork).
mkdir -p "$gui_root/engine/mods/foo"
printf '%s\n' '{}' > "$gui_root/engine/mods/foo/mod.json"
output=$(KRISTAL_DEBUG_TOOLS_GUI_PRINT_ENV=1 KRISTAL_ROOT="$gui_root/other-engine" THRASH_MACHINE_KRISTAL_DIR= "$root/bin/gui-download.sh" "$gui_root/engine/mods/foo")
printf '%s\n' "$output" | grep -Fqx "KDT_MOD_ROOT=$gui_root/engine/mods/foo"
printf '%s\n' "$output" | grep -Fqx "KRISTAL_ROOT=$gui_root/engine"

# 4. no engine anywhere -> KRISTAL_ROOT stays empty (GUI reports not found).
mkdir -p "$gui_root/standalone"
printf '%s\n' '{}' > "$gui_root/standalone/mod.json"
output=$(KRISTAL_DEBUG_TOOLS_GUI_PRINT_ENV=1 KRISTAL_ROOT= THRASH_MACHINE_KRISTAL_DIR= "$root/bin/gui-download.sh" "$gui_root/standalone")
printf '%s\n' "$output" | grep -Fqx "KDT_MOD_ROOT=$gui_root/standalone"
printf '%s\n' "$output" | grep -Fqx 'KRISTAL_ROOT='

# 5. the mod-root .build/Kristal clone is skipped in favor of the real engine
#    found by walking up.
mkdir -p "$gui_root/engine/mods/bar/.build/Kristal/src"
make_engine "$gui_root/engine/mods/bar/.build/Kristal"
printf '%s\n' '{}' > "$gui_root/engine/mods/bar/mod.json"
output=$(KRISTAL_DEBUG_TOOLS_GUI_PRINT_ENV=1 KRISTAL_ROOT="$gui_root/engine/mods/bar/.build/Kristal" "$root/bin/gui-download.sh" "$gui_root/engine/mods/bar")
printf '%s\n' "$output" | grep -Fqx "KRISTAL_ROOT=$gui_root/engine"

# gui-download.sh selects a fixed GUI release and source ref from the real
# engine VERSION. It must not fall back to GitHub's global latest release.
make_engine "$selection_root/engine"
mkdir -p "$selection_root/project"
printf '%s\n' '{}' > "$selection_root/project/mod.json"

printf '%s\n' '0.10.0' > "$selection_root/engine/VERSION"
output=$(KRISTAL_DEBUG_TOOLS_GUI_PRINT_SELECTION=1 KRISTAL_ROOT="$selection_root/engine" "$root/bin/gui-download.sh" "$selection_root/project")
printf '%s\n' "$output" | grep -Fqx 'ENGINE_VERSION=0.10.0'
printf '%s\n' "$output" | grep -Fqx 'GUI_RELEASE_TAG=v0.1.5'
printf '%s\n' "$output" | grep -Fqx 'GUI_SOURCE_REF=v0.1.5'
printf '%s\n' "$output" | grep -Fqx 'GUI_SOURCE_KIND=tag'
printf '%s\n' "$output" | grep -Fqx 'GUI_RELEASE_URL=https://github.com/Bli-AIk/kristal-debug-tools-gui/releases/download/v0.1.5'

printf '%s\n' '0.11.0-dev' > "$selection_root/engine/VERSION"
output=$(KRISTAL_DEBUG_TOOLS_GUI_PRINT_SELECTION=1 KRISTAL_ROOT="$selection_root/engine" "$root/bin/gui-download.sh" "$selection_root/project")
printf '%s\n' "$output" | grep -Fqx 'ENGINE_VERSION=0.11.0-dev'
printf '%s\n' "$output" | grep -Fqx 'GUI_RELEASE_TAG=v0.2.0'
printf '%s\n' "$output" | grep -Fqx 'GUI_SOURCE_REF=feat/v0.11-dev'
printf '%s\n' "$output" | grep -Fqx 'GUI_SOURCE_KIND=branch'
printf '%s\n' "$output" | grep -Fqx 'GUI_RELEASE_URL=https://github.com/Bli-AIk/kristal-debug-tools-gui/releases/download/v0.2.0'

printf '%s\n' '0.12.0-dev' > "$selection_root/engine/VERSION"
if output=$(KRISTAL_DEBUG_TOOLS_GUI_PRINT_SELECTION=1 KRISTAL_ROOT="$selection_root/engine" "$root/bin/gui-download.sh" "$selection_root/project" 2>&1); then
    printf '%s\n' 'unsupported VERSION unexpectedly selected a GUI build' >&2
    exit 1
fi
printf '%s\n' "$output" | grep -F 'Unsupported Kristal VERSION "0.12.0-dev"' >/dev/null

if grep -Eq 'releases/latest|previous_version|RELEASE_API|RELEASES_API' "$root/bin/gui-download.sh" "$root/gui.cmd"; then
    printf '%s\n' 'GUI downloader still contains a latest/previous-release path' >&2
    exit 1
fi

printf '%s\n' 'kristal-debug-tools smoke: PASS'
