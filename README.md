# kristal-debug-tools

[![license](https://img.shields.io/badge/license-MIT%2FApache--2.0-blue)](LICENSE-APACHE) <img src="https://img.shields.io/github/repo-size/Bli-AIk/kristal-debug-tools.svg"/> <img src="https://img.shields.io/github/last-commit/Bli-AIk/kristal-debug-tools.svg"/> <img src="https://img.shields.io/github/v/release/Bli-AIk/kristal-debug-tools.svg"/> <br>
<img src="https://img.shields.io/badge/Deltarune-001225?style=for-the-badge&labelColor=001225&logo=undertale&logoColor=ff0000" /> <img src="https://img.shields.io/badge/Lua-2C2D72?style=for-the-badge&logo=lua&logoColor=white" /> <img src="https://img.shields.io/badge/Kristal-FF6B35?style=for-the-badge&logo=love2d&logoColor=white" />

**kristal-debug-tools** 是一套 Kristal 开发期的可复用调试工具，包含两部分：运行时库和命令行启动器。启动器让你不用改 `mod.lua` 就能带战斗调试参数启动项目——直接进遭遇战、指定 wave、塞初始 TP 和仁慈值。

| 简体中文 | English                 |
| -------- | ----------------------- |
| 简体中文 | [English](README_en.md) |

## Kristal 版本支持

| `kristal`                                                                                                                     | `kristal-debug-tools` |
| ----------------------------------------------------------------------------------------------------------------------------- | --------------------- |
| [v0.11.0-dev](https://github.com/KristalTeam/Kristal/commit/f62afea63ccab02f468c24ac0d096bd8a2c9aa81) (`f62afea`, 2026-08-16) | 0.1.3 – 0.1.4         |
| [v0.10.0](https://github.com/KristalTeam/Kristal/commit/752bc0688ba97ca8a256ba9125b7e05a1ca6edbd) (`752bc068`, 2026-06-23)    | 0.1.0 – 0.1.2         |

## 功能

- **运行时库** —— 挂在 Kristal 的调试系统上，全部行为默认仅在开发模式启用，玩家包不受影响
- **启动器**（`bin/kristal-run`）—— 用 `just` 以任意组合的调试参数启动项目，不改一行 `mod.lua`
- **语言选择** —— `--lang` 把启动语言传给项目的本地化库，方便直接检查不同语言的界面

## 安装

以子模块方式安装到 `libraries/kristal-debug-tools`：

```bash
git submodule add https://github.com/Bli-AIk/kristal-debug-tools.git libraries/kristal-debug-tools
git submodule update --init --recursive
```

以子模块方式安装为**建议方式**；也可以直接下载 [Release 源码](https://github.com/Bli-AIk/kristal-debug-tools/releases)，或克隆仓库最新代码（滚动更新）后放入 `libraries/kristal-debug-tools`。

在 `mod.json` 中配置库：

```json
"kristal-debug-tools": {
    "enabled": true,
    "only_dev": true,
    "default_encounter": "dummy",
    "initial_tp": null,
    "initial_mercy": null
}
```

## 用法

不改 `mod.lua` 直接启动：

```bash
just --justfile libraries/kristal-debug-tools/justfile run --wave 2 --tp 50
```

项目可以在自己的 `justfile` 里加一个薄的 `run` 配方来缩短命令。

### 图形界面（GUI）

没装 `just` 也能用——图形界面（[kristal-debug-tools-gui](https://github.com/Bli-AIk/kristal-debug-tools-gui)）把启动器、任务列表、章节配置都变成可视化操作：

```bash
just --justfile libraries/kristal-debug-tools/justfile gui
```

或者 Windows 双击库目录下的 `gui.cmd`。首次运行会按 Kristal 引擎的 `VERSION` 下载对应的固定 release 二进制（SHA256 校验）；不会跟随 GitHub 的全局最新版，以免旧引擎下载到不兼容的 GUI。当前支持 Kristal `0.10.0` 和 `0.11.0-dev`，其他版本会明确报错。**不需要 just / Rust / Node**；只需要 LÖVE 装好并进 PATH（Git Bash 进 PATH 后，GUI 里跑构建类任务也没问题）。`just` 已编译进程序本体。

`just gui` 固定使用 release 二进制（下载到 Kristal 引擎旁的共享 `.tools/gui/`），不询问、不记忆 bin/compile 模式，也不会误用源码编译出的 dev 二进制。GUI 不是子模块；源码模式（`just gui-dev`）会 checkout 与当前引擎匹配的 GUI ref，并且在源码目录有本地改动时停止，避免覆盖开发中的工作。想要 Rust release 构建就用 `just gui-dev-release`。

### 启动器选项

| 选项                                 | 说明                                                            |
| ------------------------------------ | --------------------------------------------------------------- |
| `--lang` / `--language` / `-l`       | 选择启动语言（如 `en`、`zh-hans`），传给项目的本地化库          |
| `--encounter` / `-e`                 | 直接进入某个遭遇战                                              |
| `--wave` / `-w`                      | 首个防御阶段使用的 wave：敌人 `waves` 列表的 1 基编号或 wave ID |
| `--wave-force` / `-wf`               | 每个防御阶段都重复该 wave                                       |
| `--tp` / `--initial-tp`              | 设置初始 TP                                                     |
| `--mercy` / `--initial-mercy` / `-m` | 设置初始敌人仁慈值（0–100）                                     |

Kristal 的 `--disable-stdout-buffer` 是可选引擎参数，启动器不会默认加入。需要时可用 `just --justfile libraries/kristal-debug-tools/justfile run -- --disable-stdout-buffer` 在 `--` 后透传。

所有运行时行为默认仅限开发模式；启动器本身不会进入面向玩家的发行包。

## 开发

```bash
just test    # 冒烟测试（tests/smoke.sh）
```

## 许可

本项目可任选以下许可证使用：

- [Apache License, Version 2.0](LICENSE-APACHE)
- [MIT License](LICENSE-MIT)
