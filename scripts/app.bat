@echo off
setlocal

REM Find the directory where the script is located and go to the project root
set "SCRIPT_DIR=%~dp0"
cd /d "%SCRIPT_DIR%\.."

REM Configuration
set "EXECUTABLE=cpu-monitor.exe"
set "SOURCE_DIR=.\cmd\cpu-monitor"
set "PID_FILE=%TEMP%\cpu-monitor.pid"
set "LOG_FILE=%TEMP%\cpu-monitor.log"

REM Function to check if the process is running
:is_running
if exist "%PID_FILE%" (
    set /p PID=<"%PID_FILE%"
    tasklist /fi "pid eq %PID%" 2>nul | find /i "%PID%" >nul
    if !errorlevel! equ 0 (
        exit /b 0
    )
)
exit /b 1

REM Function to start the application
:start
call :is_running
if %errorlevel% equ 0 (
    echo CPU Monitor is already running.
    exit /b 1
)

if not exist "%EXECUTABLE%" (
    echo Executable not found. Building the application...
    go build -o "%EXECUTABLE%" -ldflags="-H windowsgui" "%SOURCE_DIR%"
    if !errorlevel! neq 0 (
        echo Build failed. Please ensure Go is installed and configured correctly.
        pause
        exit /b 1
    )
)

echo Starting CPU Monitor...
start "" /b "%EXECUTABLE%" >"%LOG_FILE%" 2>&1
for /f "tokens=2" %%i in ('wmic process call create "%EXECUTABLE%" ^| find "ProcessId"') do set PID=%%i
echo %PID% > "%PID_FILE%"
echo CPU Monitor started with PID %PID%.
goto:eof

REM Function to stop the application
:stop
call :is_running
if %errorlevel% neq 1 (
    echo CPU Monitor is not running.
    exit /b 1
)

echo Stopping CPU Monitor...
set /p PID=<"%PID_FILE%"
taskkill /pid %PID% /f
del "%PID_FILE%"
echo CPU Monitor stopped.
goto:eof

REM Function to check the status of the application
:status
call :is_running
if %errorlevel% equ 0 (
    set /p PID=<"%PID_FILE%"
    echo CPU Monitor is running with PID %PID%.
) else (
    echo CPU Monitor is not running.
)
goto:eof

REM Main script logic
set "ACTION=%1"
if "%ACTION%"=="" set "ACTION=start"

if "%ACTION%"=="start" (
    call :start
) else if "%ACTION%"=="stop" (
    call :stop
) else if "%ACTION%"=="status" (
    call :status
) else (
    echo Usage: %0 {start^|stop^|status}
    exit /b 1
)

endlocal