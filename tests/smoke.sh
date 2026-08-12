#!/usr/bin/env bash
set -euo pipefail

root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
project_root=${KRISTAL_DEBUG_TOOLS_TEST_PROJECT_ROOT:-}
if [ -z "$project_root" ]; then
    project_root=$(CDPATH= cd -- "$root/../thrash-machine" && pwd -P)
fi

runner="$root/bin/kristal-run"
dry_run() {
    KRISTAL_MOD_ROOT="$project_root" \
    KRISTAL_DEBUG_TOOLS_DRY_RUN=1 \
    KRISTAL_ROOT= \
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
trap 'rm -rf "$precedence_root"' EXIT
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

printf '%s\n' 'kristal-debug-tools smoke: PASS'
