# Run the mod with a local Kristal checkout and shared debug tools.
# zh_hans: 用本地 Kristal 引擎启动 mod（带共享调试工具）
default: run

# Run the mod with debug launcher arguments (see bin/kristal-run --help).
# zh_hans: 启动游戏，可带调试参数（如 -w 波次、-tp 初始 TP、-m mercy）
run *args:
    @KRISTAL_MOD_ROOT="{{ invocation_directory() }}" "{{ justfile_directory() }}/bin/kristal-run" {{ args }}

# Run the smoke test suite (needs bash).
# zh_hans: 运行冒烟测试（需要 bash）
test:
    @KRISTAL_DEBUG_TOOLS_TEST_PROJECT_ROOT="{{ invocation_directory() }}" "{{ justfile_directory() }}/tests/smoke.sh"

# Run the GUI for end users (downloads release binaries on first use;
# `just gui bin|compile` picks/remembers the source).
# zh_hans: 启动图形界面：无需 just/Rust/Node，优先本地构建，否则自动下载最新 release
gui *args:
    @{{ if os() == "windows" { "\"" + justfile_directory() + "/gui.cmd\"" } else { "sh \"" + justfile_directory() + "/bin/gui-download.sh\" \"" + invocation_directory() + "\"" } }} {{ args }}

# Developer mode: run the Tauri GUI from source (needs Rust + Node).
# The kristal-run sidecar is built first — tauri dev only compiles the
# main bin, and the task list needs the sidecar.
# zh_hans: 开发者模式：源码运行 GUI（需要 Rust + Node）。会先构建 kristal-run sidecar（任务列表依赖它）
gui-dev:
    @cd "{{ justfile_directory() }}/../kristal-debug-tools-gui" && (test -d node_modules || npm ci) && cargo build --manifest-path src-tauri/Cargo.toml --bin kristal-run && npm run tauri dev

alias l := run
