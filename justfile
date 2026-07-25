default: run

run *args:
    @KRISTAL_MOD_ROOT={{ invocation_directory() }} {{ justfile_directory() }}/bin/kristal-run {{ args }}

test:
    @KRISTAL_DEBUG_TOOLS_TEST_PROJECT_ROOT={{ invocation_directory() }} {{ justfile_directory() }}/tests/smoke.sh

alias l := run
