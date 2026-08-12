export KRISTAL_MOD_ROOT := invocation_directory()

default: run

run *args:
    @"{{ justfile_directory() }}/bin/kristal-run{{ if os() == "windows" { ".exe" } else { "" } }}" {{ args }}

test:
    @KRISTAL_DEBUG_TOOLS_TEST_PROJECT_ROOT="{{ invocation_directory() }}" "{{ justfile_directory() }}/tests/smoke.sh"

test-go:
    @cd {{ justfile_directory() }}/gui && go test ./...

alias l := run

# The GUI (gui/) is for end users who don't need just at all:
#   - Windows: run just.cmd for `just` commands, or download the GUI exe
#     from Releases — it embeds its own just and opens game/tasks in a new
#     terminal window.
#   - Developers build it directly with Go instead of recipes here:
#       cd gui && go build ./cmd/kristal-debug-tools-gui
#     Windows cross-build (embeds just.exe; CI fetches and SHA256-verifies
#     it into gui/internal/justbin/ first):
#       GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags kdt_embed_just \
#         -ldflags "-s -w -H windowsgui" ./cmd/kristal-debug-tools-gui
