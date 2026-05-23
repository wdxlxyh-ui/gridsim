@echo off
chcp 65001 >nul

:: Switch to package root so web/dist resolves correctly
set DIR=%~dp0..
cd /d "%DIR%" || exit /b 1

if not exist "logs" mkdir logs
if not exist "config" mkdir config

:: Check if already running
tasklist /FI "IMAGENAME eq gridsim.exe" 2>nul | find /I "gridsim.exe" >nul
if not errorlevel 1 (
    echo IEC104 Sim is already running.
    echo Web UI: http://localhost:8989
    pause
    exit /b 0
)

echo Starting GridSim...
echo.

:: start /MIN creates a new, independent console window (minimized to taskbar)
:: The server process is NOT tied to this batch script's console,
:: so it will keep running after this window closes.
:: When you close the minimized "IEC104 Sim" window, the server stops.
start "IEC104 Sim" /MIN "bin\gridsim.exe" serve --http :8989 --config-dir config --log-dir logs --log info

:: Wait a moment then check if it started
timeout /t 2 /nobreak >nul
tasklist /FI "IMAGENAME eq gridsim.exe" 2>nul | find /I "gridsim.exe" >nul
if errorlevel 1 (
    echo Failed to start IEC104 Sim. Check logs\output.log for details.
    pause
    exit /b 1
)

echo IEC104 Sim started successfully.
echo.
echo   Web UI:      http://localhost:8989
echo   Server log:  click the "IEC104 Sim" window in the taskbar
echo   To stop:     run scripts\stop.bat, or close the IEC104 Sim window
echo.
pause