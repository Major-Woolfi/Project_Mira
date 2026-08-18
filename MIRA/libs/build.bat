@echo off
setlocal enabledelayedexpansion

set "LIBS_DIR=%~dp0"
set "SRC_DIR=%LIBS_DIR%src"
set "BUILD_DIR=%LIBS_DIR%build"

if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"

set "GO_PATH=C:\Program Files\Go\bin"
set "PYTHON=python"

echo Scanning "%SRC_DIR%" for packages...

for /f "delims=" %%p in ('dir /b /ad "%SRC_DIR%"') do (
    set "PKG_DIR=%SRC_DIR%\%%p"
    set "PKG_NAME=%%p"

    if exist "!PKG_DIR!\go.mod" (
        echo.
        echo ========================================
        echo Processing Go: !PKG_NAME!
        echo ========================================
        call "%LIBS_DIR%scripts\bat\go_build.bat" "!PKG_DIR!" "%BUILD_DIR%" "%GO_PATH%"
    )
)

for /f "delims=" %%p in ('dir /b /ad "%SRC_DIR%"') do (
    set "PKG_DIR=%SRC_DIR%\%%p"
    set "PKG_NAME=%%p"

    if exist "!PKG_DIR!\setup.py" (
        echo.
        echo ========================================
        echo Processing Cython: !PKG_NAME!
        echo ========================================
        call "%LIBS_DIR%scripts\bat\cython_build.bat" "!PKG_DIR!" "%BUILD_DIR%" "%PYTHON%"
    ) else if exist "!PKG_DIR!\pyproject.toml" (
        echo.
        echo ========================================
        echo Processing Python: !PKG_NAME!
        echo ========================================
        call "%LIBS_DIR%scripts\bat\python_build.bat" "!PKG_DIR!" "%BUILD_DIR%" "%PYTHON%"
    )
)

echo.
echo ========================================
echo Build complete. Artifacts in "%BUILD_DIR%"
echo ========================================
pause
endlocal
