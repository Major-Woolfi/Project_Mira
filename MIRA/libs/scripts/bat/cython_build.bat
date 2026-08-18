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

echo [CYTHON] Installing dependencies...
if exist "requirements.txt" (
    %PYTHON% -m pip install -r requirements.txt
)

%PYTHON% -m pip install cython

echo [CYTHON] Building extensions...
if exist "setup.py" (
    %PYTHON% setup.py build_ext --inplace
    %PYTHON% setup.py bdist_wheel
) else (
    echo [CYTHON] No setup.py found, compiling .pyx files directly...
    for %%f in (*.pyx) do (
        echo [CYTHON] Compiling %%f
        %PYTHON% -m cython %%f
    )
)

if exist "dist" (
    for %%f in ("dist\*.whl") do (
        echo [CYTHON] Copying %%f
        copy /Y "%%f" "%ARTIFACT_DIR%\" >nul
    )
)

if exist "*.pyd" (
    copy /Y *.pyd "%ARTIFACT_DIR%\" >nul
)

echo [CYTHON] Copying Go shared library...
if exist "..\build\engine-memory-go\mira_memory.dll" (
    copy /Y "..\build\engine-memory-go\mira_memory.dll" "%ARTIFACT_DIR%\" >nul
)
if exist "..\build\engine-memory-go\mira_memory.lib" (
    copy /Y "..\build\engine-memory-go\mira_memory.lib" "%ARTIFACT_DIR%\" >nul
)
if exist "..\build\engine-memory-go\mira_memory.h" (
    copy /Y "..\build\engine-memory-go\mira_memory.h" "%ARTIFACT_DIR%\" >nul
)

popd
endlocal
