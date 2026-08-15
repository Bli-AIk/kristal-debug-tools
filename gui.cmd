@echo off
rem gui.cmd - start the kristal-debug-tools GUI without installing anything.
rem   - `gui` always runs release binaries; compile is only used by
rem     `gui-dev` / `gui-dev-release`.
rem   - Otherwise checks the latest GitHub release and downloads/updates the
rem     release binaries into the shared .tools\gui\ next to the Kristal engine
rem     (SHA256-verified). `just` is compiled into the kristal-run sidecar.
rem   - If the latest release's assets are not uploaded yet (e.g. release-please
rem     just cut the tag and CI is still building), the previous release is
rem     downloaded instead. The cached version is always shown and re-checked on
rem     the next run, so it self-heals back to the latest once it is ready.
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

rem Export the resolved roots to the GUI app in both modes. The GUI and the
rem kristal-run sidecar resolve the mod by walking up from cwd or reading
rem KDT_MOD_ROOT; compile mode runs from the shared .tools\gui\gui-src next
rem to the engine, so walking up can never reach the mod. Passing the roots
rem explicitly keeps gui-dev working from the new shared location. The
rem KRISTAL_ROOT fallback above is only for the cache dir (DL_DIR); export
rem it to the app only when a real engine (main.lua) was resolved, otherwise
rem clear it so the GUI reports "engine not found" accurately.
set "KDT_MOD_ROOT=%MOD_ROOT%"
if exist "%KRISTAL_ROOT%\main.lua" (
    set "KRISTAL_ROOT=%KRISTAL_ROOT%"
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
set "RELEASE_BASE=https://github.com/Bli-AIk/kristal-debug-tools-gui/releases/latest/download/"

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
    echo [kristal-debug-tools] Could not check for updates; verifying the cached build...
    call :verify-cached
    if errorlevel 1 (
        echo [kristal-debug-tools] Cached build failed checksum verification, re-downloading.
        goto need-download
    )
    rem Read the cached version on its own line so it is expanded before
    rem the message below runs (no delayed expansion needed).
    set "CACHED="
    if exist "%DL_DIR%\version.txt" set /p CACHED=<"%DL_DIR%\version.txt"
    if defined CACHED goto run-cached-known
    goto run-cached-unknown
)
if not exist "%DL_DIR%\version.txt" goto need-download
set "CACHED="
set /p CACHED=<"%DL_DIR%\version.txt"
if "%CACHED%"=="%LATEST%" (
    call :verify-cached
    if errorlevel 1 (
        echo [kristal-debug-tools] Cached build failed checksum verification, re-downloading.
        goto need-download
    )
    goto run-cached
)
goto need-download

:run-cached-known
echo [kristal-debug-tools] Using cached build %CACHED%.
goto run-cached

:run-cached-unknown
echo [kristal-debug-tools] Using cached build ^(version unknown^).
goto run-cached

:run-cached
"%DL_EXE%" %*
exit /b %ERRORLEVEL%

:need-download
echo [kristal-debug-tools] Downloading the GUI (latest release)...
call :download-assets "%RELEASE_BASE%"
if errorlevel 1 goto fallback
rem Record the version we actually downloaded: the PowerShell downloader
rem resolves the tag from the redirect URL when it can; otherwise fall back
rem to the API tag (LATEST). Writing version.txt unconditionally lets the
rem next run detect upgrades even when the API check failed this time.
if not exist "%DL_DIR%\version.txt" if defined LATEST (
    >"%DL_DIR%\version.txt" echo %LATEST%
)
"%DL_EXE%" %*
exit /b %ERRORLEVEL%

:fallback
rem The latest release's assets are not uploaded yet (e.g. CI is still
rem building them), so fall back to the previous release.
set "PREV="
for /f "usebackq delims=" %%V in (`powershell -NoProfile -Command "$r=Invoke-RestMethod -Uri 'https://api.github.com/repos/Bli-AIk/kristal-debug-tools-gui/releases?per_page=10' -Headers @{'User-Agent'='kristal-debug-tools-gui'} -TimeoutSec 10; $v=@($r).Where({ -not $_.draft -and -not $_.prerelease }); if($v.Count -ge 2){ $v[1].tag_name }"`) do set "PREV=%%V"
if not defined PREV (
    echo [kristal-debug-tools] The latest release is not ready and no previous release was found. Try again later.
    exit /b 1
)
if defined LATEST (
    echo [kristal-debug-tools] The latest release ^(%LATEST%^) is not ready yet; falling back to previous release %PREV%.
) else (
    echo [kristal-debug-tools] The latest release is not ready yet; falling back to previous release %PREV%.
)

rem If the previous release is already cached and still intact, use it
rem without re-downloading (still shown, and re-checked next run).
set "CACHED="
if exist "%DL_DIR%\version.txt" set /p CACHED=<"%DL_DIR%\version.txt"
if "%CACHED%"=="%PREV%" if exist "%DL_EXE%" if exist "%DL_SIDE%" if exist "%DL_SUMS%" (
    call :verify-cached
    if not errorlevel 1 (
        echo [kristal-debug-tools] Previous release %PREV% is already downloaded and verified.
        goto run-cached
    )
)
echo [kristal-debug-tools] Downloading previous release %PREV%...
call :download-assets "https://github.com/Bli-AIk/kristal-debug-tools-gui/releases/download/%PREV%/"
if errorlevel 1 (
    echo [kristal-debug-tools] Could not download the latest or previous release. Check your network or build locally.
    exit /b 1
)
>"%DL_DIR%\version.txt" echo %PREV%
"%DL_EXE%" %*
exit /b %ERRORLEVEL%

rem Downloads one release's three assets to .tmp names, SHA256-verifies them
rem against the checksums file, then moves them into place atomically. Also
rem writes version.txt when the release tag can be resolved from the redirect
rem URL. %1 = base URL of the release assets.
:download-assets
if not exist "%DL_DIR%" mkdir "%DL_DIR%"
del /q "%DL_DIR%\*.tmp" 2>nul
powershell -NoProfile -Command ^
  "$ProgressPreference='SilentlyContinue';" ^
  "$base='%~1';" ^
  "$dir='%DL_DIR%';" ^
  "$files=@('kristal-debug-tools-gui-windows-%ARCH%.exe','kristal-run-windows-%ARCH%.exe','checksums-windows-%ARCH%.txt');" ^
  "$tag=$null;" ^
  "foreach($f in $files){ $r=Invoke-WebRequest -Uri ($base+$f) -OutFile (Join-Path $dir ($f+'.tmp')) -UseBasicParsing;" ^
  "  if($null -eq $tag){ try { Invoke-WebRequest -Uri ($base+$f) -Method Head -MaximumRedirection 0 -UseBasicParsing | Out-Null } catch { $resp=$_.Exception.Response; if($resp -and $resp.Headers){ $loc=$resp.Headers['Location']; if($loc){ $m=[regex]::Match([string]$loc,'/releases/download/([^/]+)/'); if($m.Success){ $tag=$m.Groups[1].Value } } } } } };" ^
  "$sums=Get-Content (Join-Path $dir ('checksums-windows-%ARCH%.txt'+'.tmp'));" ^
  "foreach($f in $files){ if($f -eq 'checksums-windows-%ARCH%.txt'){continue};" ^
  "  $h=(Get-FileHash (Join-Path $dir ($f+'.tmp')) -Algorithm SHA256).Hash.ToLower();" ^
  "  if(-not ($sums -match $h)){ Write-Error ('checksum mismatch: '+$f); exit 1 } };" ^
  "foreach($f in $files){ Move-Item -Force (Join-Path $dir ($f+'.tmp')) (Join-Path $dir $f) };" ^
  "if($tag){ Set-Content -Path (Join-Path $dir 'version.txt') -Value $tag }"
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
