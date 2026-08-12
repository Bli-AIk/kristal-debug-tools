# kristal-debug-tools

[![license](https://img.shields.io/badge/license-MIT%2FApache--2.0-blue)](LICENSE-APACHE) <img src="https://img.shields.io/github/repo-size/Bli-AIk/kristal-debug-tools.svg"/> <img src="https://img.shields.io/github/last-commit/Bli-AIk/kristal-debug-tools.svg"/> <img src="https://img.shields.io/github/v/release/Bli-AIk/kristal-debug-tools.svg"/> <br>
<img src="https://img.shields.io/badge/Deltarune-001225?style=for-the-badge&labelColor=001225&logo=undertale&logoColor=ff0000" /> <img src="https://img.shields.io/badge/Lua-2C2D72?style=for-the-badge&logo=lua&logoColor=white" /> <img src="https://img.shields.io/badge/Kristal-FF6B35?style=for-the-badge&logo=love2d&logoColor=white" />

**kristal-debug-tools** is a set of reusable development tools for Kristal v0.10, in two parts: a runtime library and a command-line launcher. The launcher starts your project with battle debugging options without touching `mod.lua` — jump straight into an encounter, pick a wave, inject starting TP and mercy.

| English | 简体中文                |
| ------- | ----------------------- |
| English | [简体中文](./README.md) |

## Kristal Version Support

| `kristal`                                                                                                                  | `kristal-debug-tools` |
| -------------------------------------------------------------------------------------------------------------------------- | --------------------- |
| [v0.10.0](https://github.com/KristalTeam/Kristal/commit/752bc0688ba97ca8a256ba9125b7e05a1ca6edbd) (`752bc068`, 2026-06-23) | 0.1.0                 |
| [v0.10.0](https://github.com/KristalTeam/Kristal/commit/752bc0688ba97ca8a256ba9125b7e05a1ca6edbd) (`752bc068`, 2026-06-23) | 0.2.0                 |

## Features

- **Runtime library** — hooks into Kristal's debug system; every behavior is development-only by default, so player-facing packages are unaffected
- **Launcher** (`bin/kristal-run`) — start a project through `just` with any combination of debugging arguments, without editing `mod.lua`
- **Language selection** — `--lang` forwards the startup language to the project's localization library, handy for checking UI in different languages

## Install

Install it as a submodule at `libraries/kristal-debug-tools`:

```bash
git submodule add https://github.com/Bli-AIk/kristal-debug-tools.git libraries/kristal-debug-tools
git submodule update --init --recursive
```

Installing as a submodule is the **recommended** way; you can also download the [release source](https://github.com/Bli-AIk/kristal-debug-tools/releases), or clone the latest code (rolling updates) and place it in `libraries/kristal-debug-tools`.

Configure the library in `mod.json`:

```json
"kristal-debug-tools": {
    "enabled": true,
    "only_dev": true,
    "default_encounter": "dummy",
    "initial_tp": null,
    "initial_mercy": null
}
```

## Usage

Start the project without changing its `mod.lua`:

```bash
just --justfile libraries/kristal-debug-tools/justfile run --wave 2 --tp 50
```

Projects may add a thin `run` recipe in their own `justfile` to shorten that command.

### Launcher options

| Option                               | Description                                                                                      |
| ------------------------------------ | ------------------------------------------------------------------------------------------------ |
| `--lang` / `--language` / `-l`       | Select the startup language (e.g. `en`, `zh-hans`), passed to the project's localization library |
| `--encounter` / `-e`                 | Start directly in an encounter                                                                   |
| `--wave` / `-w`                      | Wave for the first defending phase: a 1-based position in an enemy's `waves` list, or a wave ID  |
| `--wave-force` / `-wf`               | Repeat the same wave for every defending phase                                                   |
| `--tp` / `--initial-tp`              | Set starting TP                                                                                  |
| `--mercy` / `--initial-mercy` / `-m` | Set starting enemy mercy from 0 to 100                                                           |

All runtime behavior is development-only by default. The launcher itself is not included in player-facing packages.

## Windows

Windows users don't need `just` or bash preinstalled — the core workflow (launching the game) has a native implementation:

### GUI (recommended)

Download `kristal-debug-tools-gui-windows-x64.exe` from the [Releases](https://github.com/Bli-AIk/kristal-debug-tools/releases) page and run it. A single-window UI opens (Edge WebView2, nothing to install):

- **Tasks panel** — lists every recipe of the justfile; run them with one click and watch output stream live
- **Launch game** — form fields for `--lang / --encounter / --wave / --wave-force / --tp / --mercy`
- **Runs log** — command, duration and exit code of past runs; cancel anytime

Requirements: [LÖVE](https://love2d.org) installed (`Program Files\LOVE` or on PATH). The binaries are unsigned — choose "More info → Run anyway" if SmartScreen complains.

### Command line

`just.cmd` at the repo root bootstraps `just`: it uses a system install if present, otherwise downloads the pinned official binary (SHA256-verified) into `.tools\just\` on first run:

```bat
just.cmd --justfile libraries\kristal-debug-tools\justfile run --wave 2 --tp 50
```

`kristal-run.exe` (`kristal-run-windows-x64.exe` in the same Release) is a native Windows port of `bin/kristal-run` with identical behavior: drop it into `bin\` and `just run` works, or call it directly:

```bat
kristal-run.exe --wave 2 --tp 50
```

Note: `tests/smoke.sh` is a bash script — running `just test` on Windows needs Git Bash. The core workflow (launching the game) does not depend on bash.

## Development

```bash
just test    # smoke test (tests/smoke.sh)
```

## License

Licensed under either of

- Apache License, Version 2.0 ([LICENSE-APACHE](LICENSE-APACHE))
- MIT license ([LICENSE-MIT](LICENSE-MIT))

at your option.
