# kristal-debug-tools

Reusable development tooling for Kristal v0.10 projects. The repository contains
both the runtime library and the command-line launcher used to start a project
with battle debugging options.

## Install

Add this repository as a submodule at `libraries/kristal-debug-tools`:

```bash
git submodule add https://github.com/Bli-AIk/kristal-debug-tools.git libraries/kristal-debug-tools
git submodule update --init --recursive
```

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

The project can be started without changing its `mod.lua`:

```bash
just --justfile libraries/kristal-debug-tools/justfile run --wave 2 --tp 50
```

Projects may add a thin `run` recipe in their own `justfile` to shorten that
command.

## Options

- `--lang`, `--language`, `-l`: select the startup language, such as `en` or
  `zh-hans`. The value is passed to the project localization library.
- `--encounter`, `-e`: start directly in an encounter.
- `--wave`, `-w`: select a wave for the first defending phase. Values can be a
  one-based position in an enemy's `waves` list or a wave ID.
- `--wave-force`, `-wf`: select the same wave for every defending phase.
- `--tp`, `--initial-tp`, `-tp`: set starting TP.
- `--mercy`, `--initial-mercy`, `-m`: set starting enemy mercy from 0 to 100.

All runtime behavior is development-only by default. The launcher itself is not
included in player-facing packages.
