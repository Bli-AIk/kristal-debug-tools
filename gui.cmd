@echo off
rem gui.cmd - start the kristal-debug-tools GUI without installing anything.
rem   - `gui` always runs release binaries; compile is only used by
rem     `gui-dev` / `gui-dev-release`.
rem   - Release binaries are selected from the detected Kristal VERSION and
rem     downloaded from that exact tag into the shared .tools\gui\ next to the
rem     Kristal engine (SHA256-verified). `just` is compiled into the
rem     kristal-run sidecar.
rem   - Compile mode clones the GUI source repo on demand; the GUI is no
rem     longer a required submodule.
setlocal EnableExtensions

rem Resolve the real mod root (%~dp0..\.. may still contain ".." components).
set "MOD_ROOT="
for /f "usebackq delims=" %%M in (`powershell -NoProfile -Command "(Resolve-Path '%~dp0..\..').Path"`) do set "MOD_ROOT=%%M"
if not defined MOD_ROOT set "MOD_ROOT=%~dp0..\.."

rem Shared tools dir, hosted next to the Kristal engine so the GUI cache is
rem shared across mods and the mod tree stays clean. Resolution is local-first,
rem mirroring bin\kristal-run: the nearest engine by walking up from the mod
rem root (main.lua + src\kristal.lua) wins, so a mod living inside its own
rem engine fork (e.g. el-mods\ inside kristal-el) is never hijacked by a
rem KRISTAL_ROOT inherited from the shell profile. Explicit KRISTAL_ROOT /
rem THRASH_MACHINE_KRISTAL_DIR (skipping the mod-root .build\Kristal clone) are
rem only a fallback for mods outside an engine tree; the final fallback is the
rem mod root itself.
set "KRISTAL_ROOT_ENV=%KRISTAL_ROOT%"
if defined KRISTAL_ROOT_ENV if not exist "%KRISTAL_ROOT_ENV%\main.lua" set "KRISTAL_ROOT_ENV="
if defined KRISTAL_ROOT_ENV if /i "%KRISTAL_ROOT_ENV%"=="%MOD_ROOT%\.build\Kristal" set "KRISTAL_ROOT_ENV="
set "KRISTAL_ROOT="
call :find_kristal "%MOD_ROOT%"
if not defined KRISTAL_ROOT if defined THRASH_MACHINE_KRISTAL_DIR if exist "%THRASH_MACHINE_KRISTAL_DIR%\main.lua" if /i not "%THRASH_MACHINE_KRISTAL_DIR%"=="%MOD_ROOT%\.build\Kristal" set "KRISTAL_ROOT=%THRASH_MACHINE_KRISTAL_DIR%"
if not defined KRISTAL_ROOT if defined KRISTAL_ROOT_ENV set "KRISTAL_ROOT=%KRISTAL_ROOT_ENV%"
set "ENGINE_ROOT="
if defined KRISTAL_ROOT if exist "%KRISTAL_ROOT%\main.lua" set "ENGINE_ROOT=%KRISTAL_ROOT%"
if not defined KRISTAL_ROOT set "KRISTAL_ROOT=%MOD_ROOT%"
set "DL_DIR=%KRISTAL_ROOT%\.tools\gui"
set "GUI_DIR=%KRISTAL_ROOT%\.tools\gui-src"

rem Export the resolved roots to the GUI app in both modes. The GUI and the
rem kristal-run sidecar resolve the mod by walking up from cwd or reading
rem KDT_MOD_ROOT; compile mode runs from the shared .tools\gui-src next
rem to the engine, so walking up can never reach the mod. Passing the roots
rem explicitly keeps gui-dev working from the new shared location. The
rem KRISTAL_ROOT fallback above is only for the cache dir (DL_DIR); export
rem it to the app only when a real engine (main.lua) was resolved, otherwise
rem clear it so the GUI reports "engine not found" accurately.
set "KDT_MOD_ROOT=%MOD_ROOT%"
if defined ENGINE_ROOT (
    set "KRISTAL_ROOT=%ENGINE_ROOT%"
) else (
    set "KRISTAL_ROOT="
)

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
set "GUI_REPO=https://github.com/Bli-AIk/kristal-debug-tools-gui.git"
set "DOWNLOAD_BASE=https://github.com/Bli-AIk/kristal-debug-tools-gui/releases/download"
call :select-version
if errorlevel 1 exit /b 1
set "RELEASE_BASE=%DOWNLOAD_BASE%/%GUI_RELEASE_TAG%/"

if /i "%KRISTAL_DEBUG_TOOLS_GUI_PRINT_SELECTION%"=="1" (
    echo ENGINE_VERSION=%ENGINE_VERSION%
    echo GUI_RELEASE_TAG=%GUI_RELEASE_TAG%
    echo GUI_SOURCE_REF=%GUI_SOURCE_REF%
    echo GUI_SOURCE_KIND=%GUI_SOURCE_KIND%
    echo GUI_RELEASE_URL=%RELEASE_BASE%
    exit /b 0
)

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
call :prepare-source
if errorlevel 1 (
    echo [kristal-debug-tools] gui-dev could not prepare the compatible source checkout.
    exit /b 1
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
goto compile-fail-message

:compile-fail
set "DEV_ERR=%ERRORLEVEL%"
popd >nul 2>nul

:compile-fail-message
echo [kristal-debug-tools] Local compile failed (%DEV_ERR%), falling back to release binaries.
goto download

:download
if exist "%DL_EXE%" if exist "%DL_SIDE%" if exist "%DL_SUMS%" goto check-version
goto need-download

:check-version
if not exist "%DL_DIR%\version.txt" goto need-download
set "CACHED="
set /p CACHED=<"%DL_DIR%\version.txt"
if /i "%CACHED%"=="%GUI_RELEASE_TAG%" (
    call :verify-cached
    if errorlevel 1 (
        echo [kristal-debug-tools] Cached build failed checksum verification, re-downloading.
        goto need-download
    )
    goto run-cached
)
goto need-download

:run-cached
echo [kristal-debug-tools] Using verified GUI release %GUI_RELEASE_TAG% for Kristal %ENGINE_VERSION%.
if defined MODE (
    "%DL_EXE%"
) else (
    "%DL_EXE%" %*
)
exit /b %ERRORLEVEL%

:need-download
echo [kristal-debug-tools] Downloading GUI release %GUI_RELEASE_TAG% for Kristal %ENGINE_VERSION%...
call :download-assets "%RELEASE_BASE%"
if errorlevel 1 (
    echo [kristal-debug-tools] Could not download or verify GUI release %GUI_RELEASE_TAG%. Check your network or build locally.
    exit /b 1
)
>"%DL_DIR%\version.txt" echo %GUI_RELEASE_TAG%
goto run-cached

rem Downloads one release's three assets to .tmp names, SHA256-verifies them
rem against the checksums file, then moves them into place atomically.
rem %1 = base URL of the selected release assets.
:download-assets
if not exist "%DL_DIR%" mkdir "%DL_DIR%"
del /q "%DL_DIR%\*.tmp" 2>nul
powershell -NoProfile -Command ^
  "$ProgressPreference='SilentlyContinue';" ^
  "$base='%~1';" ^
  "$dir='%DL_DIR%';" ^
  "$files=@('kristal-debug-tools-gui-windows-%ARCH%.exe','kristal-run-windows-%ARCH%.exe','checksums-windows-%ARCH%.txt');" ^
  "foreach($f in $files){ Invoke-WebRequest -Uri ($base+$f) -OutFile (Join-Path $dir ($f+'.tmp')) -UseBasicParsing };" ^
  "$sums=Get-Content (Join-Path $dir ('checksums-windows-%ARCH%.txt'+'.tmp'));" ^
  "foreach($f in $files){ if($f -eq 'checksums-windows-%ARCH%.txt'){continue};" ^
  "  $h=(Get-FileHash (Join-Path $dir ($f+'.tmp')) -Algorithm SHA256).Hash.ToLower();" ^
  "  if(-not ($sums -match $h)){ Write-Error ('checksum mismatch: '+$f); exit 1 } };" ^
  "foreach($f in $files){ Move-Item -Force (Join-Path $dir ($f+'.tmp')) (Join-Path $dir $f) }"
exit /b %ERRORLEVEL%

rem Verifies the cached binaries against the checksums file. Sets errorlevel
rem to 0 when both pass, 1 otherwise.
:verify-cached
powershell -NoProfile -Command ^
  "$s=Get-Content '%DL_SUMS%';" ^
  "$ok=$true;" ^
  "foreach($f in @('%DL_EXE%','%DL_SIDE%')){ $h=(Get-FileHash $f -Algorithm SHA256).Hash.ToLower(); if(-not ($s -match $h)){ $ok=$false } };" ^
  "if($ok){ exit 0 } else { Write-Error 'cached GUI failed checksum verification'; exit 1 }"
exit /b %ERRORLEVEL%

:select-version
if not defined ENGINE_ROOT (
    echo [kristal-debug-tools] Could not detect a Kristal VERSION. GUI download is supported only for Kristal 0.10.0 and 0.11.0-dev.
    exit /b 1
)
if not exist "%ENGINE_ROOT%\VERSION" (
    echo [kristal-debug-tools] Could not detect a Kristal VERSION. GUI download is supported only for Kristal 0.10.0 and 0.11.0-dev.
    exit /b 1
)
set "ENGINE_VERSION="
set /p ENGINE_VERSION=<"%ENGINE_ROOT%\VERSION"
if /i "%ENGINE_VERSION%"=="0.10.0" goto version-010
if /i "%ENGINE_VERSION%"=="v0.10.0" goto version-010
if /i "%ENGINE_VERSION%"=="0.11.0-dev" goto version-011
if /i "%ENGINE_VERSION%"=="v0.11.0-dev" goto version-011
echo [kristal-debug-tools] Unsupported Kristal VERSION "%ENGINE_VERSION%". GUI download is supported only for Kristal 0.10.0 and 0.11.0-dev.
exit /b 1

:version-010
set "GUI_RELEASE_TAG=v0.1.5"
set "GUI_SOURCE_REF=v0.1.5"
set "GUI_SOURCE_KIND=tag"
exit /b 0

:version-011
set "GUI_RELEASE_TAG=v0.2.0"
set "GUI_SOURCE_REF=v0.2.0"
set "GUI_SOURCE_KIND=tag"
exit /b 0

:prepare-source
if not exist "%GUI_DIR%\.git" (
    if exist "%GUI_DIR%" (
        echo [kristal-debug-tools] "%GUI_DIR%" exists but is not a git checkout; remove it or clone manually.
        exit /b 1
    )
    echo [kristal-debug-tools] Cloning GUI source at %GUI_SOURCE_REF%...
    git clone --depth 1 --branch "%GUI_SOURCE_REF%" "%GUI_REPO%" "%GUI_DIR%"
    exit /b %ERRORLEVEL%
)
for /f "usebackq delims=" %%S in (`git -C "%GUI_DIR%" status --porcelain`) do (
    echo [kristal-debug-tools] "%GUI_DIR%" has local changes; gui-dev will not switch or update it.
    exit /b 1
)
if /i "%GUI_SOURCE_KIND%"=="tag" goto prepare-source-tag
git -C "%GUI_DIR%" fetch origin "refs/heads/%GUI_SOURCE_REF%:refs/remotes/origin/%GUI_SOURCE_REF%"
if errorlevel 1 exit /b 1
git -C "%GUI_DIR%" show-ref --verify --quiet "refs/heads/%GUI_SOURCE_REF%"
if errorlevel 1 (
    git -C "%GUI_DIR%" switch --create "%GUI_SOURCE_REF%" "origin/%GUI_SOURCE_REF%"
) else (
    git -C "%GUI_DIR%" switch "%GUI_SOURCE_REF%"
)
if errorlevel 1 exit /b 1
git -C "%GUI_DIR%" merge-base --is-ancestor HEAD "origin/%GUI_SOURCE_REF%"
if errorlevel 1 (
    echo [kristal-debug-tools] "%GUI_DIR%" cannot fast-forward to %GUI_SOURCE_REF%; gui-dev will not overwrite local commits.
    exit /b 1
)
git -C "%GUI_DIR%" merge --ff-only "origin/%GUI_SOURCE_REF%"
exit /b %ERRORLEVEL%

:prepare-source-tag
git -C "%GUI_DIR%" fetch origin "refs/tags/%GUI_SOURCE_REF%:refs/tags/%GUI_SOURCE_REF%"
if errorlevel 1 exit /b 1
git -C "%GUI_DIR%" switch --detach "%GUI_SOURCE_REF%"
exit /b %ERRORLEVEL%

rem Walk up from a directory for the nearest Kristal engine (main.lua +
rem src\kristal.lua). Recursive; sets KRISTAL_ROOT, otherwise leaves it unset.
rem Parent is computed as "CAND\.." + %~fI, not %~dpI: on a directory with a
rem trailing backslash (what %~dpI produces), %~dpI returns the directory
rem itself, so the walk stops one level short of the engine.
:find_kristal
set "CAND=%~1"
if exist "%CAND%\main.lua" if exist "%CAND%\src\kristal.lua" (
    set "KRISTAL_ROOT=%CAND%"
    exit /b
)
for %%I in ("%CAND%\..") do set "PARENT=%%~fI"
if /i "%PARENT%"=="%CAND%" exit /b
call :find_kristal "%PARENT%"
exit /b
