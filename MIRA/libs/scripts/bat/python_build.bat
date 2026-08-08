@echo off
setlocal

set "PKG_DIR=%~1"
set "BUILD_DIR=%~2"
set "PYTHON=%~3"

if not defined PYTHON set "PYTHON=python"
if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"

pushd "%PKG_DIR%"

set "ARTIFACT_DIR=%BUILD_DIR%\%~n1"
if not exist "%ARTIFACT_DIR%" mkdir "%ARTIFACT_DIR%"

echo [PYTHON] Installing dependencies...
if exist "requirements.txt" (
    %PYTHON% -m pip install -r requirements.txt
)

if exist "pyproject.toml" (
    echo [PYTHON] Building wheel with pyproject.toml...
    %PYTHON% -m build --wheel
) else if exist "setup.py" (
    echo [PYTHON] Building with setup.py...
    %PYTHON% setup.py bdist_wheel
) else (
    echo [PYTHON] No build system found, copying source...
    if not exist "%ARTIFACT_DIR%\src" mkdir "%ARTIFACT_DIR%\src"
    xcopy /Y /I /E "*.py" "%ARTIFACT_DIR%\src\"
    goto :python_done
)

if exist "dist" (
    for %%f in ("dist\*.whl") do (
        echo [PYTHON] Copying %%f
        copy /Y "%%f" "%ARTIFACT_DIR%\" >nul
    )
)

:python_done
popd
endlocal
