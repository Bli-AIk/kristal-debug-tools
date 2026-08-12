export KRISTAL_MOD_ROOT := invocation_directory()

JUST_VERSION := "1.58.0"
JUST_ZIP := "just-" + JUST_VERSION + "-x86_64-pc-windows-msvc.zip"

default: run

run *args:
    @"{{ justfile_directory() }}/bin/kristal-run{{ if os() == "windows" { ".exe" } else { "" } }}" {{ args }}

test:
    @KRISTAL_DEBUG_TOOLS_TEST_PROJECT_ROOT="{{ invocation_directory() }}" "{{ justfile_directory() }}/tests/smoke.sh"

test-go:
    @cd {{ justfile_directory() }}/gui && go test ./...

alias l := run

# --- GUI / Windows tooling (recipe bodies are lazy: safe on POSIX) ---

download-just:
    @mkdir -p {{ justfile_directory() }}/gui/internal/justbin
    @cd /tmp && curl -fsSL -o {{ JUST_ZIP }} "https://github.com/casey/just/releases/download/{{ JUST_VERSION }}/{{ JUST_ZIP }}"
    @curl -fsSL "https://github.com/casey/just/releases/download/{{ JUST_VERSION }}/SHA256SUMS" \
        | grep -F "{{ JUST_ZIP }}" | (cd /tmp && sha256sum -c -)
    @unzip -o /tmp/{{ JUST_ZIP }} just.exe -d {{ justfile_directory() }}/gui/internal/justbin
    @rm /tmp/{{ JUST_ZIP }}

gui-build:
    @just download-just
    @mkdir -p {{ justfile_directory() }}/dist
    @cd {{ justfile_directory() }}/gui && go build -trimpath -ldflags "-s -w" \
        -o {{ justfile_directory() }}/dist/kristal-debug-tools-gui ./cmd/kristal-debug-tools-gui
    @cd {{ justfile_directory() }}/gui && go build -trimpath -ldflags "-s -w" \
        -o {{ justfile_directory() }}/dist/kristal-run ./cmd/kristal-run

gui-run:
    @just gui-build
    @{{ justfile_directory() }}/dist/kristal-debug-tools-gui

gui-build-windows:
    @just download-just
    @mkdir -p {{ justfile_directory() }}/dist
    @cd {{ justfile_directory() }}/gui && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -H windowsgui" -tags kdt_embed_just \
        -o {{ justfile_directory() }}/dist/kristal-debug-tools-gui-windows-x64.exe ./cmd/kristal-debug-tools-gui
    @cd {{ justfile_directory() }}/gui && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" \
        -o {{ justfile_directory() }}/dist/kristal-run-windows-x64.exe ./cmd/kristal-run
