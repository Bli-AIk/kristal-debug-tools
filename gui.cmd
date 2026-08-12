@echo off
rem gui.cmd - start the kristal-debug-tools GUI without installing anything.
rem   - `gui` always runs release binaries; compile is only used by
rem     `gui-dev` / `gui-dev-release`.
rem   - Otherwise checks the latest GitHub release and downloads/updates the
rem     release binaries into .tools\gui\ (SHA256-verified). `just` is
rem     compiled into the kristal-run sidecar.
rem   - Compile mode clones the GUI source repo on demand; the GUI is no
rem     longer a required submodule.
setlocal EnableExtensions
set "DL_DIR=%~dp0..\..\.tools\gui"
set "GUI_DIR=%DL_DIR%\gui-src"
set "DL_EXE=%DL_DIR%\kristal-debug-tools-gui-windows-x64.exe"
set "DL_SIDE=%DL_DIR%\kristal-run-windows-x64.exe"

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
    echo [kristal-debug-tools] Compiling locally (npm run tauri dev)...
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
if exist "%DL_EXE%" if exist "%DL_SIDE%" if exist "%DL_DIR%\checksums.txt" goto check-version
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
  "$files=@('kristal-debug-tools-gui-windows-x64.exe','kristal-run-windows-x64.exe','checksums.txt');" ^
  "foreach($f in $files){ Invoke-WebRequest -Uri ($base+$f) -OutFile (Join-Path $dir $f) };" ^
  "$sums=Get-Content (Join-Path $dir 'checksums.txt');" ^
  "foreach($f in $files){ if($f -eq 'checksums.txt'){continue};" ^
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
