@echo off
rem gui.cmd - start the kristal-debug-tools GUI without installing anything.
rem   - Uses the locally built exe (libraries\kristal-debug-tools-gui\src-tauri\target\release)
rem     when present, so development builds work offline.
rem   - Detects a local compile toolchain (cargo + node) and asks whether to
rem     run from source instead; falls back to release binaries.
rem   - Otherwise downloads the latest release binaries into .tools\gui\
rem     (SHA256-verified). `just` is compiled into the kristal-run sidecar.
setlocal EnableExtensions
set "GUI_DIR=%~dp0..\kristal-debug-tools-gui"
set "LOCAL_EXE=%GUI_DIR%\src-tauri\target\release\kristal-debug-tools-gui.exe"
set "DL_DIR=%~dp0..\..\.tools\gui"
set "DL_EXE=%DL_DIR%\kristal-debug-tools-gui-windows-x64.exe"
set "DL_SIDE=%DL_DIR%\kristal-run-windows-x64.exe"

if exist "%LOCAL_EXE%" (
    "%LOCAL_EXE%" %*
    exit /b %ERRORLEVEL%
)

rem The bin/compile choice is asked once and remembered in
rem .tools\gui\settings.json; `gui.cmd bin|compile` overrides it.
set "DL_DIR=%~dp0..\..\.tools\gui"
set "SETTINGS=%DL_DIR%\settings.json"
set "MODE="
set "MODE_ARG="
if /i "%~1"=="bin" set "MODE_ARG=bin"
if /i "%~1"=="compile" set "MODE_ARG=compile"

if defined MODE_ARG (
    set "MODE=%MODE_ARG%"
    if not exist "%DL_DIR%" mkdir "%DL_DIR%"
    powershell -NoProfile -Command ^
      "$s=if(Test-Path '%SETTINGS%'){(Get-Content '%SETTINGS%'|ConvertFrom-Json)}else{[pscustomobject]@{}};" ^
      "$s.mode='%MODE%'; $s|ConvertTo-Json|Set-Content '%SETTINGS%'"
    shift
    goto run-mode
)

for /f "usebackq delims=" %%M in (`powershell -NoProfile -Command "(Get-Content '%SETTINGS%' -ErrorAction SilentlyContinue | ConvertFrom-Json).mode"`) do set "MODE=%%M"
if defined MODE goto run-mode

where cargo >nul 2>&1
if errorlevel 1 goto set-bin
where node >nul 2>&1
if errorlevel 1 goto set-bin
echo [kristal-debug-tools] Detected a local compile toolchain (cargo + node).
choice /c BC /n /t 5 /d B /m "[B] use release binaries (default)  [C] compile and run locally: "
if not exist "%DL_DIR%" mkdir "%DL_DIR%"
set "MODE=bin"
if errorlevel 2 set "MODE=compile"
powershell -NoProfile -Command ^
  "$s=if(Test-Path '%SETTINGS%'){(Get-Content '%SETTINGS%'|ConvertFrom-Json)}else{[pscustomobject]@{}};" ^
  "$s.mode='%MODE%'; $s|ConvertTo-Json|Set-Content '%SETTINGS%'"
echo [kristal-debug-tools] Remembered (edit .tools\gui\settings.json or pass bin^|compile to change).
goto run-mode

:set-bin
set "MODE=bin"
goto run-mode

:run-mode
if "%MODE%"=="compile" goto compile
goto download

:compile
echo [kristal-debug-tools] Compiling locally (npm run tauri dev)...
pushd "%GUI_DIR%"
if not exist node_modules (
    call npm ci
    if errorlevel 1 goto compile-fail
)
rem tauri dev only compiles the main bin; the task list needs the sidecar.
call cargo build --manifest-path src-tauri\Cargo.toml --bin kristal-run
if errorlevel 1 goto compile-fail
call npm run tauri dev
set "DEV_ERR=%ERRORLEVEL%"
popd
if "%DEV_ERR%"=="0" exit /b 0
:compile-fail
echo [kristal-debug-tools] Local compile failed (%DEV_ERR%), falling back to release binaries.

:download
if exist "%DL_EXE%" (
    "%DL_EXE%" %*
    exit /b %ERRORLEVEL%
)

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

"%DL_EXE%" %*
exit /b %ERRORLEVEL%
