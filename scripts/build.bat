@echo off
setlocal

REM This script builds the CPU Monitor application for Windows.

REM Find the directory where the script is located and go to the project root
set "SCRIPT_DIR=%~dp0"
set "PROJECT_ROOT=%SCRIPT_DIR%\.."
cd /d "%PROJECT_ROOT%"

REM Configuration
set "EXECUTABLE_NAME=cpu-monitor.exe"
set "SOURCE_DIR=.\cmd\cpu-monitor"
set "DIST_DIR=.\dist"

echo Creating distribution directory...
if not exist "%DIST_DIR%" mkdir "%DIST_DIR%"

echo Building the application for Windows...
go build -mod=vendor -o "%DIST_DIR%\%EXECUTABLE_NAME%" -ldflags="-H windowsgui" "%SOURCE_DIR%"

if %errorlevel% neq 0 (
    echo Build failed. Please ensure Go is installed and configured correctly.
    pause
    exit /b 1
)

echo Build successful!
echo The executable is located at: %DIST_DIR%\%EXECUTABLE_NAME%
pause