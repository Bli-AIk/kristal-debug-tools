# kristal-debug-tools

[![license](https://img.shields.io/badge/license-MIT%2FApache--2.0-blue)](LICENSE-APACHE) <img src="https://img.shields.io/github/repo-size/Bli-AIk/kristal-debug-tools.svg"/> <img src="https://img.shields.io/github/last-commit/Bli-AIk/kristal-debug-tools.svg"/> <img src="https://img.shields.io/github/v/release/Bli-AIk/kristal-debug-tools.svg"/> <br>
<img src="https://img.shields.io/badge/Deltarune-001225?style=for-the-badge&labelColor=001225&logo=undertale&logoColor=ff0000" /> <img src="https://img.shields.io/badge/Lua-2C2D72?style=for-the-badge&logo=lua&logoColor=white" /> <img src="https://img.shields.io/badge/Kristal-FF6B35?style=for-the-badge&logo=love2d&logoColor=white" />

**kristal-debug-tools** is a set of reusable development tools for Kristal, in two parts: a runtime library and a command-line launcher. The launcher starts your project with battle debugging options without touching `mod.lua` — jump straight into an encounter, pick a wave, inject starting TP and mercy.

| English | 简体中文                |
| ------- | ----------------------- |
| English | [简体中文](./README.md) |

## Kristal Version Support

| `kristal`                                                                                                                     | `kristal-debug-tools` |
| ----------------------------------------------------------------------------------------------------------------------------- | --------------------- |
| [v0.11.0-dev](https://github.com/KristalTeam/Kristal/commit/f62afea63ccab02f468c24ac0d096bd8a2c9aa81) (`f62afea`, 2026-08-16) | 0.1.3                 |
| [v0.10.0](https://github.com/KristalTeam/Kristal/commit/752bc0688ba97ca8a256ba9125b7e05a1ca6edbd) (`752bc068`, 2026-06-23)    | 0.1.0 – 0.1.2         |

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

Kristal's `--disable-stdout-buffer` is optional and is not added by the launcher by default. Pass it through after `--` when needed: `just --justfile libraries/kristal-debug-tools/justfile run -- --disable-stdout-buffer`.

All runtime behavior is development-only by default. The launcher itself is not included in player-facing packages.

## Development

```bash
just test    # smoke test (tests/smoke.sh)
```

## License

Licensed under either of

- Apache License, Version 2.0 ([LICENSE-APACHE](LICENSE-APACHE))
- MIT license ([LICENSE-MIT](LICENSE-MIT))

at your option.
