default: run

run *args:
    @KRISTAL_MOD_ROOT="{{ invocation_directory() }}" "{{ justfile_directory() }}/bin/kristal-run" {{ args }}

test:
    @KRISTAL_DEBUG_TOOLS_TEST_PROJECT_ROOT="{{ invocation_directory() }}" "{{ justfile_directory() }}/tests/smoke.sh"

# Run the GUI for end users: no Rust/Node/just needed (just is compiled
# into the kristal-run sidecar). Uses a local dev build when present, else
# downloads the latest release binaries (cached in .tools/gui/). The
# bin|compile choice is asked once and remembered; `just gui bin|compile`
# overrides it.
gui *args:
    @{{ if os() == "windows" { "\"" + justfile_directory() + "/gui.cmd\"" } else { "sh \"" + justfile_directory() + "/bin/gui-download.sh\" \"" + invocation_directory() + "\"" } }} {{ args }}

# Developer mode: run the Tauri GUI from source (needs Rust + Node).
# The kristal-run sidecar is built first — tauri dev only compiles the
# main bin, and the task list needs the sidecar.
gui-dev:
    @cd "{{ justfile_directory() }}/../kristal-debug-tools-gui" && (test -d node_modules || npm ci) && cargo build --manifest-path src-tauri/Cargo.toml --bin kristal-run && npm run tauri dev

alias l := run
