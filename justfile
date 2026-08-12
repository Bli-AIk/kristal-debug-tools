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

# Run the GUI for end users (checks for a newer release and downloads it;
# `just gui bin|compile` picks/remembers the source).
# zh_hans: 启动图形界面：无需 just/Rust/Node，优先本地构建，否则自动检测并下载最新 release
gui *args:
    @{{ if os() == "windows" { "\"" + justfile_directory() + "/gui.cmd\"" } else { "sh \"" + justfile_directory() + "/bin/gui-download.sh\" \"" + invocation_directory() + "\"" } }} {{ args }}

# Developer mode: run the Tauri GUI from source (needs Rust + Node).
# The GUI repo is cloned on demand into .tools/gui-src — not a submodule.
# zh_hans: 开发者模式：源码运行 GUI（需要 Rust + Node）。源码会按需 clone 到 .tools/gui-src。
gui-dev:
    @{{ if os() == "windows" { "\"" + justfile_directory() + "/gui.cmd\"" } else { "sh \"" + justfile_directory() + "/bin/gui-download.sh\" \"" + invocation_directory() + "\"" } }} compile

# Developer mode with a Rust release build (first compile is slower, the
# running app is faster).
# zh_hans: 开发者模式，Rust 用 release 构建（首次编译慢一点，跑起来更快）
gui-dev-release:
    @{{ if os() == "windows" { "\"" + justfile_directory() + "/gui.cmd\"" } else { "sh \"" + justfile_directory() + "/bin/gui-download.sh\" \"" + invocation_directory() + "\"" } }} compile-release

alias l := run
