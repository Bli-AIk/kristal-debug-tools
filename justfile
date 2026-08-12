export KRISTAL_MOD_ROOT := invocation_directory()

default: run

run *args:
    @"{{ justfile_directory() }}/bin/kristal-run{{ if os() == "windows" { ".exe" } else { "" } }}" {{ args }}

test:
    @KRISTAL_DEBUG_TOOLS_TEST_PROJECT_ROOT="{{ invocation_directory() }}" "{{ justfile_directory() }}/tests/smoke.sh"

test-go:
    @cd {{ justfile_directory() }}/gui && go test ./...

# Build and run the GUI for the host platform (developer convenience; end
# users use just.cmd, gui.cmd or the release binaries instead — the GUI is
# for people without just, so it embeds its own and opens game/tasks in a
# new terminal window).
gui:
    @mkdir -p {{ justfile_directory() }}/dist
    @cd {{ justfile_directory() }}/gui && go build -trimpath -ldflags "-s -w" \
        -o {{ justfile_directory() }}/dist/kristal-debug-tools-gui ./cmd/kristal-debug-tools-gui
    @{{ justfile_directory() }}/dist/kristal-debug-tools-gui

alias l := run

# Windows cross-build (embeds just.exe; CI fetches and SHA256-verifies it
# into gui/internal/justbin/ first):
#   GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags kdt_embed_just \
#     -ldflags "-s -w -H windowsgui" ./cmd/kristal-debug-tools-gui
