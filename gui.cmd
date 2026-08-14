@echo off
rem gui.cmd - start the kristal-debug-tools GUI without installing anything.
rem   - `gui` always runs release binaries; compile is only used by
rem     `gui-dev` / `gui-dev-release`.
rem   - Otherwise checks the latest GitHub release and downloads/updates the
rem     release binaries into the shared .tools\gui\ next to the Kristal engine
rem     (SHA256-verified). `just` is compiled into the kristal-run sidecar.
rem   - Compile mode clones the GUI source repo on demand; the GUI is no
rem     longer a required submodule.
setlocal EnableExtensions

rem Resolve the real mod root (%~dp0..\.. may still contain ".." components).
set "MOD_ROOT="
for /f "usebackq delims=" %%M in (`powershell -NoProfile -Command "(Resolve-Path '%~dp0..\..').Path"`) do set "MOD_ROOT=%%M"
if not defined MOD_ROOT set "MOD_ROOT=%~dp0..\.."

rem Shared tools dir, hosted next to the Kristal engine so the GUI cache is
rem shared across mods and the mod tree stays clean. Resolution mirrors the
rem build scripts: explicit KRISTAL_ROOT / THRASH_MACHINE_KRISTAL_DIR (skipping
rem the mod-root .build\Kristal clone) → nearest engine by walking up from the
rem mod root (main.lua + src\kristal.lua) → fall back to the mod root.
if defined KRISTAL_ROOT if not exist "%KRISTAL_ROOT%\main.lua" set "KRISTAL_ROOT="
if defined KRISTAL_ROOT if /i "%KRISTAL_ROOT%"=="%MOD_ROOT%\.build\Kristal" set "KRISTAL_ROOT="
if defined THRASH_MACHINE_KRISTAL_DIR if exist "%THRASH_MACHINE_KRISTAL_DIR%\main.lua" if /i not "%THRASH_MACHINE_KRISTAL_DIR%"=="%MOD_ROOT%\.build\Kristal" set "KRISTAL_ROOT=%THRASH_MACHINE_KRISTAL_DIR%"
if not defined KRISTAL_ROOT call :find_kristal "%MOD_ROOT%"
if not defined KRISTAL_ROOT set "KRISTAL_ROOT=%MOD_ROOT%"
set "DL_DIR=%KRISTAL_ROOT%\.tools\gui"
set "GUI_DIR=%DL_DIR%\gui-src"

rem Detect host architecture (AMD64 or ARM64). cmd may run under x64
rem emulation on ARM64 Windows, so check PROCESSOR_ARCHITEW6432 and, as a
rem fallback, PowerShell's OSArchitecture (reports the OS arch, not the
rem emulated process arch).
set "ARCH=x64"
if /i "%PROCESSOR_ARCHITEW6432%"=="ARM64" set "ARCH=arm64"
if /i "%PROCESSOR_ARCHITECTURE%"=="ARM64" set "ARCH=arm64"
if /i "%ARCH%"=="x64" (
    for /f "usebackq delims=" %%A in (`powershell -NoProfile -Command "[System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture"`) do (
        if /i "%%A"=="Arm64" set "ARCH=arm64"
    )
)

set "DL_EXE=%DL_DIR%\kristal-debug-tools-gui-windows-%ARCH%.exe"
set "DL_SIDE=%DL_DIR%\kristal-run-windows-%ARCH%.exe"
set "DL_SUMS=%DL_DIR%\checksums-windows-%ARCH%.txt"

if /i "%~1"=="compile" (
    set "MODE=compile"
    goto compile
)
if /i "%~1"=="compile-release" (
    set "MODE=compile-release"
    goto compile
)
goto download

:compile
if not exist "%GUI_DIR%\.git" (
    if not exist "%GUI_DIR%" (
        echo [kristal-debug-tools] Cloning GUI source for local compile...
        git clone --depth 1 https://github.com/Bli-AIk/kristal-debug-tools-gui.git "%GUI_DIR%"
        if errorlevel 1 goto compile-fail
    ) else (
        echo [kristal-debug-tools] "%GUI_DIR%" exists but is not a git checkout.
        goto compile-fail
    )
)
set "RELEASE_FLAG="
if "%MODE%"=="compile-release" set "RELEASE_FLAG=--release"
if "%MODE%"=="compile-release" (
    echo [kristal-debug-tools] Compiling locally in release mode...
) else (
    echo [kristal-debug-tools] Compiling locally ^(npm run tauri dev^)...
)
pushd "%GUI_DIR%"
if not exist node_modules (
    call npm ci
    if errorlevel 1 goto compile-fail
)
rem tauri dev only compiles the main bin; the task list needs the sidecar.
call cargo build %RELEASE_FLAG% --manifest-path src-tauri\Cargo.toml --bin kristal-run
if errorlevel 1 goto compile-fail
call npm run tauri dev -- %RELEASE_FLAG%
set "DEV_ERR=%ERRORLEVEL%"
popd
if "%DEV_ERR%"=="0" exit /b 0
:compile-fail
echo [kristal-debug-tools] Local compile failed (%DEV_ERR%), falling back to release binaries.

:download
if exist "%DL_EXE%" if exist "%DL_SIDE%" if exist "%DL_SUMS%" goto check-version
goto need-download

:check-version
set "LATEST="
for /f "usebackq delims=" %%V in (`powershell -NoProfile -Command "try { (Invoke-RestMethod -Uri 'https://api.github.com/repos/Bli-AIk/kristal-debug-tools-gui/releases/latest' -Headers @{'User-Agent'='kristal-debug-tools-gui'} -TimeoutSec 10).tag_name } catch { '' }"`) do set "LATEST=%%V"
if not defined LATEST (
    echo [kristal-debug-tools] Could not check for updates, using cached build.
    goto run-cached
)
if not exist "%DL_DIR%\version.txt" goto need-download
set "CACHED="
set /p CACHED=<"%DL_DIR%\version.txt"
if "%CACHED%"=="%LATEST%" goto run-cached
goto need-download

:run-cached
"%DL_EXE%" %*
exit /b %ERRORLEVEL%

:need-download
echo [kristal-debug-tools] Downloading the GUI (latest release)...
if not exist "%DL_DIR%" mkdir "%DL_DIR%"
powershell -NoProfile -Command ^
  "$ProgressPreference='SilentlyContinue';" ^
  "$base='https://github.com/Bli-AIk/kristal-debug-tools-gui/releases/latest/download/';" ^
  "$dir='%DL_DIR%';" ^
  "$files=@('kristal-debug-tools-gui-windows-%ARCH%.exe','kristal-run-windows-%ARCH%.exe','checksums-windows-%ARCH%.txt');" ^
  "foreach($f in $files){ Invoke-WebRequest -Uri ($base+$f) -OutFile (Join-Path $dir $f) };" ^
  "$sums=Get-Content (Join-Path $dir 'checksums-windows-%ARCH%.txt');" ^
  "foreach($f in $files){ if($f -eq 'checksums-windows-%ARCH%.txt'){continue};" ^
  "  $h=(Get-FileHash (Join-Path $dir $f) -Algorithm SHA256).Hash.ToLower();" ^
  "  if(-not ($sums -match $h)){ Write-Error ('checksum mismatch: '+$f); exit 1 } }"
if errorlevel 1 (
    echo [kristal-debug-tools] Download or checksum failed. Check your network or build locally.
    exit /b 1
)
if defined LATEST (
    >"%DL_DIR%\version.txt" echo %LATEST%
)

"%DL_EXE%" %*
exit /b %ERRORLEVEL%

rem Walk up from a directory for the nearest Kristal engine (main.lua +
rem src\kristal.lua). Recursive; sets KRISTAL_ROOT, otherwise leaves it unset.
:find_kristal
set "CAND=%~1"
if exist "%CAND%\main.lua" if exist "%CAND%\src\kristal.lua" (
    set "KRISTAL_ROOT=%CAND%"
    exit /b
)
for %%I in ("%CAND%") do set "PARENT=%%~dpI"
if /i "%PARENT%"=="%CAND%" exit /b
call :find_kristal "%PARENT%"
exit /b
