@echo off
setlocal enabledelayedexpansion

set "base_dir=%~dp0.."
for %%i in ("%base_dir%") do set "base_dir=%%~fi"
set "results_dir=unit_tests\results"
set "report_file=%results_dir%\syntax_test.md"

if not exist "%results_dir%" mkdir "%results_dir%"

echo # Syntax Test Report > "%report_file%"
echo. >> "%report_file%"
echo Date: %date% %time% >> "%report_file%"
echo. >> "%report_file%"

for /r "%base_dir%" %%f in (*.py) do (
    set "rel=%%f"
    call set "rel=%%rel:%base_dir%\=%%"
    set "rel=.\!rel!"
    echo Testing !rel!
    python -m py_compile "%%f" >nul 2>&1
    if !errorlevel! equ 0 (
        echo - !rel!: SUCCESS >> "%report_file%"
    ) else (
        echo - !rel!: FAILED >> "%report_file%"
        echo   Error details: >> "%report_file%"
        python -m py_compile "%%f" 2>>&1 | findstr /v "Compiling" >> "%report_file%"
        echo. >> "%report_file%"
    )
)

echo Consolidating compiled files to root __pycache__
for /d /r "%base_dir%" %%d in (__pycache__) do (
    if not "%%d"=="%base_dir%\__pycache__" (
        move "%%d\*.pyc" "%base_dir%\__pycache__\" >nul 2>&1
        rmdir "%%d" 2>nul
    )
)

echo. >> "%report_file%"
echo Report generated successfully. >> "%report_file%"

echo Syntax test completed. Check %report_file% for results.