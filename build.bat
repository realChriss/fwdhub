@echo off
setlocal enabledelayedexpansion
cd /d "%~dp0"
set "VERSION=%~1"
if "%VERSION%"=="" set VERSION=dev

go run scripts/winicon.go || goto done

set CGO_ENABLED=0
for %%t in (linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64) do (
  for /f "tokens=1,2 delims=/" %%a in ("%%t") do (
    set GOOS=%%a
    set GOARCH=%%b
    set EXT=
    if "%%a"=="windows" set EXT=.exe
    echo building %%t
    go build -trimpath -ldflags "-s -w -X main.version=%VERSION%" -o "dist/fwdhub-%%a-%%b!EXT!" ./src || goto done
  )
)

:done
set ERR=%errorlevel%
del /q src\rsrc_windows_*.syso 2>nul
exit /b %ERR%
