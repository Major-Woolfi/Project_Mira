@echo off
setlocal enabledelayedexpansion

set "LIBS_DIR=%~dp0"
set "SRC_DIR=%LIBS_DIR%src"
set "BUILD_DIR=%LIBS_DIR%build"

if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"

set "GO_PATH=C:\Program Files\Go\bin"
set "PYTHON=python"

echo Scanning "%SRC_DIR%" for packages...

for /d %%p in ("%SRC_DIR%\*") do (
    set "PKG_DIR=%%p"
    set "PKG_NAME=%%~nxp"
    echo.
    echo ========================================
    echo Processing: !PKG_NAME!
    echo ========================================

    if exist "!PKG_DIR!\go.mod" (
        echo Detected: Go package
        call "%LIBS_DIR%scripts\bat\go_build.bat" "!PKG_DIR!" "%BUILD_DIR%" "%GO_PATH%"
        goto :continue
    )

    if exist "!PKG_DIR!\pyproject.toml" (
        echo Detected: Python package (pyproject.toml)
        call "%LIBS_DIR%scripts\bat\python_build.bat" "!PKG_DIR!" "%BUILD_DIR!" "%PYTHON%"
        goto :continue
    )

    if exist "!PKG_DIR!\setup.py" (
        echo Detected: Cython package (setup.py)
        call "%LIBS_DIR%scripts\bat\cython_build.bat" "!PKG_DIR!" "%BUILD_DIR!" "%PYTHON%"
        goto :continue
    )

    echo Skipping !PKG_NAME! - unknown package type

:continue
)

echo.
echo ========================================
echo Build complete. Artifacts in "%BUILD_DIR%"
echo ========================================
pause
endlocal
