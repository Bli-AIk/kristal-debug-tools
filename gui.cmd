@echo off
rem gui.cmd - start the kristal-debug-tools GUI without installing anything.
rem   - Uses the locally built exe (libraries\kristal-debug-tools-gui\src-tauri\target\release)
rem     when present, so development builds work offline.
rem   - Otherwise downloads the latest release binaries into .tools\gui\.
rem   - `just` is compiled into the kristal-run sidecar - no just install needed.
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
