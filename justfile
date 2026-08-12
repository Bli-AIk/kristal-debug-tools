default: run

run *args:
    @KRISTAL_MOD_ROOT="{{ invocation_directory() }}" "{{ justfile_directory() }}/bin/kristal-run" {{ args }}

test:
    @KRISTAL_DEBUG_TOOLS_TEST_PROJECT_ROOT="{{ invocation_directory() }}" "{{ justfile_directory() }}/tests/smoke.sh"

# Run the Tauri GUI (developer convenience; the gui repo is a sibling
# submodule at libraries/kristal-debug-tools-gui).
gui:
    @cd "{{ justfile_directory() }}/../kristal-debug-tools-gui" && npm run tauri dev

alias l := run
