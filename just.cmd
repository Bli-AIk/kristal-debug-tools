@echo off
rem just.cmd — run `just` on Windows without installing anything.
rem
rem   - If `just.exe` is already on PATH, it is used as-is.
rem   - Otherwise the pinned official binary is downloaded once (SHA256
rem     verified) into .tools\just\ next to this script and reused after.
rem   - All arguments are forwarded unchanged.
rem
rem Messages are English on purpose: cmd reads batch files in the OEM
rem codepage, where UTF-8 Chinese renders as mojibake. The GUI speaks Chinese.
setlocal EnableExtensions
set "JUST_VERSION=1.58.0"
set "JUST_ZIP=just-1.58.0-x86_64-pc-windows-msvc.zip"
set "JUST_HASH=759f16fb7aa17c5c8b9594b6d4a8c1a6630dfd042cf2b3ff84841454d3d188dc"
set "JUST_DIR=%~dp0.tools\just"
set "JUST_EXE=%JUST_DIR%\just.exe"
set "MARKER=1.58.0 %JUST_HASH%"

rem 1) System just. `where just.exe` (not `where just`) so we never match
rem    .cmd shims — including this very file if the repo root is on PATH.
where just.exe >nul 2>nul
if not errorlevel 1 (
    call just %*
    exit /b %ERRORLEVEL%
)

rem 2) Cached copy with a matching marker -> forward. A mismatched or missing
rem    marker means a stale download; drop it and re-fetch.
if exist "%JUST_EXE%" (
    if exist "%JUST_DIR%\version.txt" (
        set /p cached=<"%JUST_DIR%\version.txt"
        if "%cached%"=="%MARKER%" goto :forward
    )
    del /q "%JUST_EXE%" 2>nul
)

rem 3) First run / stale cache: download, verify, extract. PowerShell is
rem    guaranteed on Windows 10+; no -ExecutionPolicy Bypass is needed since
rem    execution policy only applies to script files, not -Command strings.
:download
echo [kristal-debug-tools] Downloading just %JUST_VERSION% (~2.1 MB)...
if not exist "%JUST_DIR%" mkdir "%JUST_DIR%"
powershell -NoProfile -Command ^
  "$ProgressPreference='SilentlyContinue';" ^
  "$z='%JUST_DIR%\just.zip';" ^
  "$h='%JUST_HASH%';" ^
  "Invoke-WebRequest -Uri 'https://github.com/casey/just/releases/download/%JUST_VERSION%/%JUST_ZIP%' -OutFile $z;" ^
  "$a=(Get-FileHash $z -Algorithm SHA256).Hash.ToLower();" ^
  "if($a -ne $h){Write-Error ('SHA256 mismatch: '+$a); exit 1};" ^
  "Expand-Archive -Path $z -DestinationPath '%JUST_DIR%' -Force;" ^
  "Remove-Item $z -Force"
if errorlevel 1 (
    echo [kristal-debug-tools] Download failed. Install just manually (https://just.systems) and retry.
    exit /b 1
)
> "%JUST_DIR%\version.txt" echo %MARKER%

:forward
"%JUST_EXE%" %*
exit /b %ERRORLEVEL%
