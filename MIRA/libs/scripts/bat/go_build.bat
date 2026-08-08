@echo off
setlocal enabledelayedexpansion

set "PKG_DIR=%~1"
set "BUILD_DIR=%~2"
set "GO_PATH=%~3"

if not defined GO_PATH set "GO_PATH=C:\Program Files\Go\bin"
if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"

set "PATH=%GO_PATH%;%PATH%"

pushd "%PKG_DIR%"

echo [GO] Tidying modules...
go mod tidy

echo [GO] Building libraries...
go build ./...

set "ARTIFACT_DIR=%BUILD_DIR%\%~n1"
if not exist "!ARTIFACT_DIR!" mkdir "!ARTIFACT_DIR!"

echo [GO] Checking for main packages...
set "MAIN_FOUND=0"

if exist "main.go" (
    echo [GO] Found main.go in root
    go build -o "!ARTIFACT_DIR!\%~n1.exe" .
    set "MAIN_FOUND=1"
)

if exist "cmd\" (
    for /d %%d in ("cmd\*") do (
        if exist "%%d\main.go" (
            echo [GO] Found main.go in %%d
            set "CMD_NAME=%%~nd"
            set "CMD_PATH=%%d"
            set "CMD_PATH=!CMD_PATH:\=/!"
            go build -o "!ARTIFACT_DIR!\!CMD_NAME!.exe" "./!CMD_PATH!"
            set "MAIN_FOUND=1"
        )
    )
)

if exist "go.mod" (
    copy /Y go.mod "!ARTIFACT_DIR!\go.mod" >nul
    if exist "go.sum" copy /Y go.sum "!ARTIFACT_DIR!\go.sum" >nul
)

if "!MAIN_FOUND!"=="0" (
    echo [GO] No main package found, library only
)

popd
endlocal
