@echo off
SETLOCAL EnableDelayedExpansion

:: Navigate to root directory if executed from the deployment folder
if exist "..\docker-compose.yml" (
    cd ..
)

echo ==========================================================
echo           Crystal Games Deployment Controller
echo ==========================================================
echo.
echo 1. Start the services (docker compose up -d)
echo 2. Stop the services (docker compose down)
echo 3. Rebuild and Start (docker compose up -d --build)
echo 4. View live logs (docker compose logs -f)
echo 5. View status (docker compose ps)
echo 6. Exit
echo.

set /p choice="Enter your choice (1-6): "

if "%choice%"=="1" (
    echo Starting services...
    docker compose up -d
) else if "%choice%"=="2" (
    echo Stopping services...
    docker compose down
) else if "%choice%"=="3" (
    echo Building and starting services...
    docker compose up -d --build
) else if "%choice%"=="4" (
    echo Streaming logs...
    docker compose logs -f
) else if "%choice%"=="5" (
    echo Service status:
    docker compose ps
) else if "%choice%"=="6" (
    echo Exiting...
) else (
    echo Invalid option selected.
)

echo.
pause
