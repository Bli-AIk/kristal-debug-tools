# Changelog

## [0.1.2](https://github.com/Bli-AIk/kristal-debug-tools/compare/v0.1.1...v0.1.2) (2026-08-15)


### chore

* drop bump-patch-for-minor-pre-major config ([fb6b7c8](https://github.com/Bli-AIk/kristal-debug-tools/commit/fb6b7c894cfc416e132741a291b506ca7ddaa7b3))

## [0.1.1](https://github.com/Bli-AIk/kristal-debug-tools/compare/v0.1.0...v0.1.1) (2026-08-14)


### Features

* **gui:** 图形界面一键启动 ([#3](https://github.com/Bli-AIk/kristal-debug-tools/issues/3)) ([af1ccb5](https://github.com/Bli-AIk/kristal-debug-tools/commit/af1ccb5f386bdda408ca6fee245ac39d793a3ffc))


### Bug Fixes

* **gui:** escape parens in gui.cmd compile echo ([6255642](https://github.com/Bli-AIk/kristal-debug-tools/commit/6255642c34e3ef81c2529e1131c7b2ce85370a20))
* **gui:** export roots to the app and fix the engine walk-up ([0fd6267](https://github.com/Bli-AIk/kristal-debug-tools/commit/0fd626732fb5797a157d7c37c0069107b099b1af))
* use lovec on Windows so the debug console can attach ([d6331a1](https://github.com/Bli-AIk/kristal-debug-tools/commit/d6331a1256e78de64fc5ebd0547aee0aa43424c5))


### Code Refactoring

* **gui:** download into shared .tools next to the Kristal engine ([667dbc5](https://github.com/Bli-AIk/kristal-debug-tools/commit/667dbc5eb68b8022e2ac4e59fc7c9d12f59cd737))

## 0.1.0 (2026-08-12)


### chore

* force release 0.1.0 ([2246c46](https://github.com/Bli-AIk/kristal-debug-tools/commit/2246c46e9f920bf8adf0b3e23d5aa0cef222b863))


### Features

* add reusable Kristal debug tools ([5f31e9c](https://github.com/Bli-AIk/kristal-debug-tools/commit/5f31e9c34c53368549cbd3f0448daa10f51682f1))
* support startup language selection ([8a46d2c](https://github.com/Bli-AIk/kristal-debug-tools/commit/8a46d2cf3fd31906081b5fac19878a7108082670))


### Bug Fixes

* handle missing mod options ([3ca28ab](https://github.com/Bli-AIk/kristal-debug-tools/commit/3ca28ab6bf509b10b1c7a9ce00381155832c7247))
* kristal-run 支持查找 kristal-el 引擎 ([beafcd0](https://github.com/Bli-AIk/kristal-debug-tools/commit/beafcd039634c57bf5b29d79f7a198feeeac7058))
* prefer nearest Kristal engine over override ([a4993c7](https://github.com/Bli-AIk/kristal-debug-tools/commit/a4993c78075bb4144faca9d4379ee5186bb860a7))
* quote project paths in just recipes ([d6f993a](https://github.com/Bli-AIk/kristal-debug-tools/commit/d6f993ad19122d651f4d4e517998f261d8934a3b))


### Code Refactoring

* 引擎发现改为向上遍历，去除硬编码库名 ([70246a8](https://github.com/Bli-AIk/kristal-debug-tools/commit/70246a8ba5663315a741cc65918ae0fcedb6b7fe))
