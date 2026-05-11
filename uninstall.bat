@echo off
chcp 65001 >nul
setlocal EnableExtensions EnableDelayedExpansion

cd /d "%~dp0"

set "FULL=0"
set "YES=0"
set "REMOVE_ENV=0"
set "REMOVE_BACKUPS=0"
set "REMOVE_IMAGES=0"
set "REMOVE_BUILD_CACHE=0"

:parse_args
if "%~1"=="" goto args_done
if /I "%~1"=="--full" (
    set "FULL=1"
    set "YES=1"
    set "REMOVE_ENV=1"
    set "REMOVE_IMAGES=1"
    set "REMOVE_BUILD_CACHE=1"
    shift /1
    goto parse_args
)
if /I "%~1"=="--yes" (
    set "YES=1"
    shift /1
    goto parse_args
)
if /I "%~1"=="--remove-env" (
    set "REMOVE_ENV=1"
    shift /1
    goto parse_args
)
if /I "%~1"=="--remove-backups" (
    set "REMOVE_BACKUPS=1"
    shift /1
    goto parse_args
)
if /I "%~1"=="--remove-images" (
    set "REMOVE_IMAGES=1"
    shift /1
    goto parse_args
)
if /I "%~1"=="--remove-build-cache" (
    set "REMOVE_BUILD_CACHE=1"
    shift /1
    goto parse_args
)
if /I "%~1"=="/?" (
    call :print_help
    exit /b 0
)
if /I "%~1"=="--help" (
    call :print_help
    exit /b 0
)
echo ERROR: unknown option: %~1
exit /b 1

:args_done
call :main
set "EXIT_CODE=%ERRORLEVEL%"
if not "%EXIT_CODE%"=="0" (
    echo.
    echo UNINSTALL FAILED, exit code: %EXIT_CODE%
    echo.
    echo Useful diagnostics:
    echo   docker ps -a
    echo   docker network ls
    echo   docker volume ls
    echo.
    pause
    exit /b %EXIT_CODE%
)

echo.
echo Uninstall completed.
echo.
pause
exit /b 0

:main
call :check_requirements || exit /b 1

if "%YES%"=="0" (
    echo.
    echo Secure Voting uninstall
    echo Repository: %CD%
    echo.
    if "%FULL%"=="1" (
        echo FULL uninstall mode.
        echo Type DELETE to continue.
        set /p "CONFIRM=> "
        if /I not "!CONFIRM!"=="DELETE" (
            echo Uninstall cancelled.
            exit /b 1
        )
    ) else (
        echo Safe uninstall mode.
        echo Containers, networks, volumes and generated certificates will be removed.
        echo .env, backups, local images and Docker build cache will be preserved.
        echo Type YES to continue.
        set /p "CONFIRM=> "
        if /I not "!CONFIRM!"=="YES" (
            echo Uninstall cancelled.
            exit /b 1
        )
    )
)

echo.
echo == compose down ==
call :compose_down_default
call :compose_down_candidates

call :remove_known_containers
call :remove_known_networks
call :remove_known_volumes
call :remove_generated_files

if "%REMOVE_IMAGES%"=="1" call :remove_project_images
if "%REMOVE_BUILD_CACHE%"=="1" (
    echo.
    echo == prune Docker build cache ==
    docker builder prune -f
)

call :print_remaining_diagnostics
exit /b 0

:check_requirements
if not exist "docker-compose.yml" (
    echo ERROR: docker-compose.yml was not found.
    echo Run this file from the root directory of the secure-voting repository.
    exit /b 1
)

docker --version >nul 2>&1
if errorlevel 1 (
    echo ERROR: Docker was not found in PATH.
    echo Install Docker Desktop and restart the terminal.
    exit /b 1
)

docker compose version >nul 2>&1
if errorlevel 1 (
    echo ERROR: Docker Compose v2 is not available.
    echo Check that Docker Desktop is installed and running.
    exit /b 1
)

docker info >nul 2>&1
if errorlevel 1 (
    echo ERROR: Docker daemon is not available.
    echo Start Docker Desktop and try again.
    exit /b 1
)

exit /b 0

:compose_down_default
docker compose --profile prod --profile debug down -v --remove-orphans --timeout 20 >nul 2>&1
exit /b 0

:compose_down_candidates
for %%I in ("%CD%") do set "PROJECT_BASE=%%~nxI"
set "PROJECT_UNDERSCORE=%PROJECT_BASE:-=_%"

for %%P in ("%PROJECT_BASE%" "%PROJECT_UNDERSCORE%" "secure-voting" "secure_voting") do (
    docker compose --project-name "%%~P" --profile prod --profile debug down -v --remove-orphans --timeout 20 >nul 2>&1
)
exit /b 0

:remove_known_containers
echo.
echo == remove containers ==
for %%C in (
    ts-frontend
    go-backend
    go-worker
    postgres-db
    postgres-webui
    redis-cache
    redis-webui
    mongo-db
    mongodb-webui
    rust-compute
    go-compute-runner
    kafka
    kafka-ui
    kafka-init
) do (
    docker rm -f %%C >nul 2>&1
)
exit /b 0

:remove_known_networks
echo.
echo == remove networks ==
for %%I in ("%CD%") do set "PROJECT_BASE=%%~nxI"
set "PROJECT_UNDERSCORE=%PROJECT_BASE:-=_%"

for %%P in ("%PROJECT_BASE%" "%PROJECT_UNDERSCORE%" "secure-voting" "secure_voting") do (
    for %%N in (
        app_network
        db_network
        rpc_network
        redis_network
        kafka_network
        debug_network
    ) do (
        docker network rm "%%~P_%%N" >nul 2>&1
        docker network rm "%%~P-%%N" >nul 2>&1
    )
)
exit /b 0

:remove_known_volumes
echo.
echo == remove volumes ==
for %%I in ("%CD%") do set "PROJECT_BASE=%%~nxI"
set "PROJECT_UNDERSCORE=%PROJECT_BASE:-=_%"

for %%P in ("%PROJECT_BASE%" "%PROJECT_UNDERSCORE%" "secure-voting" "secure_voting") do (
    for %%V in (
        db-data
        redis-data
        mongo-data
        kafka-data
        pgadmin-data
        redisinsight-data
    ) do (
        docker volume rm -f "%%~P_%%V" >nul 2>&1
        docker volume rm -f "%%~P-%%V" >nul 2>&1
    )
)

if exist "db-data" rmdir /s /q "db-data" >nul 2>&1
if exist "cache" rmdir /s /q "cache" >nul 2>&1
if exist "mongo-data" rmdir /s /q "mongo-data" >nul 2>&1
if exist "iggy" rmdir /s /q "iggy" >nul 2>&1

exit /b 0

:remove_generated_files
echo.
echo == remove generated files ==

if exist "scripts\certs\out" rmdir /s /q "scripts\certs\out" >nul 2>&1
if exist ".ci-artifacts" rmdir /s /q ".ci-artifacts" >nul 2>&1

if "%REMOVE_ENV%"=="1" (
    if exist ".env" del /f /q ".env" >nul 2>&1
)

if "%REMOVE_BACKUPS%"=="1" (
    if exist ".backups" rmdir /s /q ".backups" >nul 2>&1
)

exit /b 0

:remove_project_images
echo.
echo == remove local project images ==

for %%I in ("%CD%") do set "PROJECT_BASE=%%~nxI"
set "PROJECT_UNDERSCORE=%PROJECT_BASE:-=_%"

for %%P in ("%PROJECT_BASE%" "%PROJECT_UNDERSCORE%" "secure-voting" "secure_voting") do (
    for /f "usebackq delims=" %%I in (`docker images -q --filter "label=com.docker.compose.project=%%~P" 2^>nul`) do (
        docker image rm -f %%I >nul 2>&1
    )

    for %%M in (
        "%%~P-frontend"
        "%%~P-backend"
        "%%~P-compute"
        "%%~P_frontend"
        "%%~P_backend"
        "%%~P_compute"
    ) do (
        docker image rm -f %%~M >nul 2>&1
    )
)

exit /b 0

:print_remaining_diagnostics
echo.
echo == remaining secure-voting docker objects ==

echo Containers:
docker ps -a --format "{{.Names}}" | findstr /R /C:"^ts-frontend$" /C:"^go-backend$" /C:"^go-worker$" /C:"^postgres-db$" /C:"^postgres-webui$" /C:"^redis-cache$" /C:"^redis-webui$" /C:"^mongo-db$" /C:"^mongodb-webui$" /C:"^rust-compute$" /C:"^go-compute-runner$" /C:"^kafka$" /C:"^kafka-ui$" /C:"^kafka-init$"

echo.
echo Networks:
docker network ls --format "{{.Name}}" | findstr /I "secure-voting secure_voting app_network db_network rpc_network redis_network kafka_network debug_network"

echo.
echo Volumes:
docker volume ls --format "{{.Name}}" | findstr /I "secure-voting secure_voting db-data redis-data mongo-data kafka-data pgadmin-data redisinsight-data"

echo.
if "%REMOVE_ENV%"=="0" (
    echo .env was preserved. Use --remove-env or --full to delete it.
)
if "%REMOVE_BACKUPS%"=="0" (
    echo .backups was preserved if it existed. Use --remove-backups to delete it.
)

exit /b 0

:print_help
echo Usage:
echo   uninstall.bat
echo   uninstall.bat --yes
echo   uninstall.bat --full
echo   uninstall.bat --remove-env
echo   uninstall.bat --remove-images
echo   uninstall.bat --remove-build-cache
echo   uninstall.bat --remove-backups
echo.
echo Default safe mode:
echo   removes containers, networks, volumes and generated certificates.
echo   preserves .env, backups, images and Docker build cache.
echo.
echo Full mode:
echo   uninstall.bat --full
echo   removes .env, local project images and Docker build cache too.
echo.
echo Requirements:
echo   Windows CMD, Docker Desktop, Docker Compose v2.
echo   Git Bash and WSL are not used.
exit /b 0
