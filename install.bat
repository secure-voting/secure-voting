@echo off
chcp 65001 >nul
setlocal EnableExtensions EnableDelayedExpansion

cd /d "%~dp0"

set "FRESH=1"
set "RESET_ENV=1"
set "WITH_DEBUG=0"
set "PRUNE_BUILD_CACHE=0"
set "SKIP_BUILD=0"
set "WAIT_TIMEOUT_SECONDS=420"

:parse_args
if "%~1"=="" goto args_done
if /I "%~1"=="--fresh" (
    set "FRESH=1"
    shift /1
    goto parse_args
)
if /I "%~1"=="--reset-env" (
    set "RESET_ENV=1"
    shift /1
    goto parse_args
)
if /I "%~1"=="--keep-existing" (
    set "FRESH=0"
    set "RESET_ENV=0"
    shift /1
    goto parse_args
)
if /I "%~1"=="--with-debug" (
    set "WITH_DEBUG=1"
    shift /1
    goto parse_args
)
if /I "%~1"=="--prune-build-cache" (
    set "PRUNE_BUILD_CACHE=1"
    shift /1
    goto parse_args
)
if /I "%~1"=="--skip-build" (
    set "SKIP_BUILD=1"
    shift /1
    goto parse_args
)
if /I "%~1"=="--wait-timeout" (
    if "%~2"=="" (
        echo ERROR: --wait-timeout requires a value.
        exit /b 1
    )
    set "WAIT_TIMEOUT_SECONDS=%~2"
    shift /1
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
    echo INSTALL FAILED, exit code: %EXIT_CODE%
    echo.
    echo Useful diagnostics:
    echo   docker compose ps
    echo   docker compose logs --tail=200 backend
    echo   docker compose logs --tail=200 db mongo cache kafka
    echo   docker system df
    echo.
    pause
    exit /b %EXIT_CODE%
)

echo.
echo Installation completed.
echo Frontend: https://127.0.0.1:8080
echo Backend health: http://127.0.0.1:3001/health
echo.
pause
exit /b 0

:main
call :check_requirements || exit /b 1

if "%FRESH%"=="1" (
    call :fresh_cleanup || exit /b 1
)

call :prepare_env || exit /b 1
call :generate_certs || exit /b 1

if "%PRUNE_BUILD_CACHE%"=="1" (
    echo.
    echo == prune Docker build cache ==
    docker builder prune -f
)

call :compose_up || exit /b 1
call :wait_stack || exit /b 1
call :print_summary
exit /b 0

:check_requirements
if not exist "docker-compose.yml" (
    echo ERROR: docker-compose.yml was not found.
    echo Run this file from the root directory of the secure-voting repository.
    exit /b 1
)

where powershell >nul 2>&1
if errorlevel 1 (
    echo ERROR: PowerShell was not found.
    echo This native Windows installer uses PowerShell only for .env editing and random secrets.
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

:fresh_cleanup
echo.
echo == fresh cleanup ==

call :compose_down_default
call :compose_down_candidates
call :remove_known_containers
call :remove_known_networks
call :remove_known_volumes

if exist "scripts\certs\out" rmdir /s /q "scripts\certs\out" >nul 2>&1
if exist ".ci-artifacts" rmdir /s /q ".ci-artifacts" >nul 2>&1

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
echo == remove stale containers ==
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
echo == remove stale networks ==
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
echo == remove stale volumes ==
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

:prepare_env
echo.
echo == prepare .env ==

set "NEW_ENV=0"

if "%RESET_ENV%"=="1" (
    if exist ".env" del /f /q ".env" >nul 2>&1
)

if not exist ".env" (
    if exist ".env.example" (
        copy /y ".env.example" ".env" >nul
    ) else (
        > ".env" (
            echo POSTGRES_PASSWORD=
            echo REDIS_PASSWORD=
            echo MONGO_INITDB_ROOT_PASSWORD=
            echo BOOTSTRAP_ADMIN_EMAIL=admin@example.com
            echo BOOTSTRAP_ADMIN_PASSWORD=
            echo BOOTSTRAP_RESEARCHER_EMAIL=researcher@example.com
            echo BOOTSTRAP_RESEARCHER_PASSWORD=
            echo WRITE_RATE_LIMIT=30
            echo WRITE_RATE_LIMIT_TTL=1m
            echo AUTH_RATE_LIMIT=10
            echo AUTH_RATE_LIMIT_TTL=1m
            echo ADMIN_TRUSTED_CIDRS=
            echo COMPUTE_GRPC_ADDR=rust-compute:50051
            echo COMPUTE_TLS=true
            echo COMPUTE_TLS_CA=/certs/ca.pem
            echo COMPUTE_TLS_SERVER_NAME=rust-compute
            echo FRONTEND_TLS_HOSTS=localhost,127.0.0.1,ts-frontend
            echo EMAIL_VERIFICATION_MODE=dev
            echo SMTP_HOST=
            echo SMTP_PORT=587
            echo SMTP_USERNAME=
            echo SMTP_PASSWORD=
            echo SMTP_FROM_EMAIL=
            echo SMTP_FROM_NAME="Secure Voting"
            echo SMTP_TLS_MODE=starttls
        )
    )
    set "NEW_ENV=1"
)

if "%RESET_ENV%"=="1" call :set_secret_env POSTGRES_PASSWORD
if "%RESET_ENV%"=="1" call :set_secret_env REDIS_PASSWORD
if "%RESET_ENV%"=="1" call :set_secret_env MONGO_INITDB_ROOT_PASSWORD
if "%RESET_ENV%"=="1" call :set_secret_env BOOTSTRAP_ADMIN_PASSWORD
if "%RESET_ENV%"=="1" call :set_secret_env BOOTSTRAP_RESEARCHER_PASSWORD

if "%RESET_ENV%"=="0" if "%NEW_ENV%"=="1" call :set_secret_env POSTGRES_PASSWORD
if "%RESET_ENV%"=="0" if "%NEW_ENV%"=="1" call :set_secret_env REDIS_PASSWORD
if "%RESET_ENV%"=="0" if "%NEW_ENV%"=="1" call :set_secret_env MONGO_INITDB_ROOT_PASSWORD
if "%RESET_ENV%"=="0" if "%NEW_ENV%"=="1" call :set_secret_env BOOTSTRAP_ADMIN_PASSWORD
if "%RESET_ENV%"=="0" if "%NEW_ENV%"=="1" call :set_secret_env BOOTSTRAP_RESEARCHER_PASSWORD

if "%RESET_ENV%"=="0" if "%NEW_ENV%"=="0" call :ensure_secret_nonempty POSTGRES_PASSWORD
if "%RESET_ENV%"=="0" if "%NEW_ENV%"=="0" call :ensure_secret_nonempty REDIS_PASSWORD
if "%RESET_ENV%"=="0" if "%NEW_ENV%"=="0" call :ensure_secret_nonempty MONGO_INITDB_ROOT_PASSWORD
if "%RESET_ENV%"=="0" if "%NEW_ENV%"=="0" call :ensure_secret_nonempty BOOTSTRAP_ADMIN_PASSWORD
if "%RESET_ENV%"=="0" if "%NEW_ENV%"=="0" call :ensure_secret_nonempty BOOTSTRAP_RESEARCHER_PASSWORD

call :ensure_env_value BOOTSTRAP_ADMIN_EMAIL admin@example.com
call :ensure_env_value BOOTSTRAP_RESEARCHER_EMAIL researcher@example.com
call :ensure_env_value WRITE_RATE_LIMIT 30
call :ensure_env_value WRITE_RATE_LIMIT_TTL 1m
call :ensure_env_value AUTH_RATE_LIMIT 10
call :ensure_env_value AUTH_RATE_LIMIT_TTL 1m
call :ensure_env_value ADMIN_TRUSTED_CIDRS ""
call :ensure_env_value COMPUTE_GRPC_ADDR rust-compute:50051
call :ensure_env_value COMPUTE_TLS true
call :ensure_env_value COMPUTE_TLS_CA /certs/ca.pem
call :ensure_env_value COMPUTE_TLS_SERVER_NAME rust-compute
call :ensure_env_value FRONTEND_TLS_HOSTS localhost,127.0.0.1,ts-frontend
call :ensure_env_value EMAIL_VERIFICATION_MODE dev
call :ensure_env_value SMTP_HOST ""
call :ensure_env_value SMTP_PORT 587
call :ensure_env_value SMTP_USERNAME ""
call :ensure_env_value SMTP_PASSWORD ""
call :ensure_env_value SMTP_FROM_EMAIL ""
call :ensure_env_value SMTP_FROM_NAME "\"Secure Voting\""
call :ensure_env_value SMTP_TLS_MODE starttls

exit /b 0

:set_secret_env
set "SV_KEY=%~1"
powershell -NoProfile -ExecutionPolicy Bypass -Command "$p='.env'; $k=$env:SV_KEY; $bytes=New-Object byte[] 32; [Security.Cryptography.RandomNumberGenerator]::Fill($bytes); $v=([BitConverter]::ToString($bytes)).Replace('-','').ToLowerInvariant(); $lines=@(); if(Test-Path $p){$lines=[IO.File]::ReadAllLines($p)}; $found=$false; $out=New-Object System.Collections.Generic.List[string]; foreach($line in $lines){ if($line -match ('^'+[regex]::Escape($k)+'=')){ $out.Add($k+'='+$v); $found=$true } else { $out.Add($line) } }; if(-not $found){ $out.Add($k+'='+$v) }; $enc=New-Object System.Text.UTF8Encoding($false); [IO.File]::WriteAllLines($p,[string[]]$out,$enc)"
if errorlevel 1 exit /b 1
exit /b 0

:ensure_secret_nonempty
set "SV_KEY=%~1"
powershell -NoProfile -ExecutionPolicy Bypass -Command "$p='.env'; $k=$env:SV_KEY; $lines=@(); if(Test-Path $p){$lines=[IO.File]::ReadAllLines($p)}; $current=''; foreach($line in $lines){ if($line -match ('^'+[regex]::Escape($k)+'=')){ $current=$line.Substring($k.Length+1); break } }; if([string]::IsNullOrWhiteSpace($current)){ $bytes=New-Object byte[] 32; [Security.Cryptography.RandomNumberGenerator]::Fill($bytes); $v=([BitConverter]::ToString($bytes)).Replace('-','').ToLowerInvariant(); $found=$false; $out=New-Object System.Collections.Generic.List[string]; foreach($line in $lines){ if($line -match ('^'+[regex]::Escape($k)+'=')){ $out.Add($k+'='+$v); $found=$true } else { $out.Add($line) } }; if(-not $found){ $out.Add($k+'='+$v) }; $enc=New-Object System.Text.UTF8Encoding($false); [IO.File]::WriteAllLines($p,[string[]]$out,$enc) }"
if errorlevel 1 exit /b 1
exit /b 0

:ensure_env_value
set "SV_KEY=%~1"
set "SV_VALUE=%~2"
powershell -NoProfile -ExecutionPolicy Bypass -Command "$p='.env'; $k=$env:SV_KEY; $default=$env:SV_VALUE; $lines=@(); if(Test-Path $p){$lines=[IO.File]::ReadAllLines($p)}; $found=$false; $changed=$false; $out=New-Object System.Collections.Generic.List[string]; foreach($line in $lines){ if($line -match ('^'+[regex]::Escape($k)+'=')){ $found=$true; $value=$line.Substring($k.Length+1); if([string]::IsNullOrEmpty($value) -and -not [string]::IsNullOrEmpty($default)){ $out.Add($k+'='+$default); $changed=$true } else { $out.Add($line) } } else { $out.Add($line) } }; if(-not $found){ $out.Add($k+'='+$default); $changed=$true }; if($changed){ $enc=New-Object System.Text.UTF8Encoding($false); [IO.File]::WriteAllLines($p,[string[]]$out,$enc) }"
if errorlevel 1 exit /b 1
exit /b 0

:get_env_value
set "SV_KEY=%~1"
set "%~2="
for /f "usebackq delims=" %%A in (`powershell -NoProfile -ExecutionPolicy Bypass -Command "$p='.env'; $k=$env:SV_KEY; if(Test-Path $p){ foreach($line in [IO.File]::ReadAllLines($p)){ if($line -match ('^'+[regex]::Escape($k)+'=')){ $line.Substring($k.Length+1); break } } }"`) do set "%~2=%%A"
exit /b 0

:generate_certs
echo.
echo == generate fresh TLS certificates ==

set "CERT_OUT=%CD%\scripts\certs\out"
set "OPENSSL_IMAGE=alpine/openssl:latest"
set "KEYTOOL_IMAGE=eclipse-temurin:21-jre"

if exist "%CERT_OUT%" rmdir /s /q "%CERT_OUT%" >nul 2>&1
mkdir "%CERT_OUT%" || exit /b 1

docker pull %OPENSSL_IMAGE% >nul
if errorlevel 1 exit /b 1
docker pull %KEYTOOL_IMAGE% >nul
if errorlevel 1 exit /b 1

call :openssl genrsa -out /work/ca.key 4096 || exit /b 1
call :openssl req -x509 -new -nodes -key /work/ca.key -sha256 -days 3650 -subj "/C=NL/ST=Noord-Holland/L=Amsterdam/O=secure-voting/OU=dev/CN=secure-voting-dev-ca" -addext "basicConstraints=critical,CA:TRUE" -addext "keyUsage=critical,keyCertSign,cRLSign" -addext "subjectKeyIdentifier=hash" -out /work/ca.pem || exit /b 1

call :write_compute_ext
call :gen_server_cert compute "/C=NL/ST=Noord-Holland/L=Amsterdam/O=secure-voting/OU=compute/CN=rust-compute" || exit /b 1

call :write_frontend_ext || exit /b 1
call :gen_server_cert frontend "/C=NL/ST=Noord-Holland/L=Amsterdam/O=secure-voting/OU=frontend/CN=localhost" || exit /b 1

call :write_db_ext
call :gen_server_cert db "/C=NL/ST=Noord-Holland/L=Amsterdam/O=secure-voting/OU=postgres/CN=db" || exit /b 1

call :write_redis_ext
call :gen_server_cert redis "/C=NL/ST=Noord-Holland/L=Amsterdam/O=secure-voting/OU=redis/CN=cache" || exit /b 1

call :write_mongo_ext
call :gen_server_cert mongo "/C=NL/ST=Noord-Holland/L=Amsterdam/O=secure-voting/OU=mongo/CN=mongo" || exit /b 1

call :write_kafka_ext
call :gen_server_cert kafka "/C=NL/ST=Noord-Holland/L=Amsterdam/O=secure-voting/OU=kafka/CN=kafka" || exit /b 1

call :openssl pkcs12 -export -in /work/kafka.pem -inkey /work/kafka.key -certfile /work/ca.pem -out /work/kafka.keystore.p12 -name kafka -passout pass:changeit || exit /b 1

docker run --rm -v "%CERT_OUT%:/work" %KEYTOOL_IMAGE% keytool -importcert -noprompt -alias secure-voting-dev-ca -file /work/ca.pem -keystore /work/kafka.truststore.p12 -storetype PKCS12 -storepass changeit
if errorlevel 1 exit /b 1

> "%CERT_OUT%\kafka_keystore_creds" echo changeit
> "%CERT_OUT%\kafka_sslkey_creds" echo changeit
> "%CERT_OUT%\kafka_truststore_creds" echo changeit

copy /b "%CERT_OUT%\mongo.pem"+"%CERT_OUT%\mongo.key" "%CERT_OUT%\mongo.server.pem" >nul
powershell -NoProfile -ExecutionPolicy Bypass -Command "$bytes=New-Object byte[] 512; [Security.Cryptography.RandomNumberGenerator]::Fill($bytes); $text=[Convert]::ToBase64String($bytes); $enc=New-Object System.Text.UTF8Encoding($false); [IO.File]::WriteAllText((Join-Path $env:CERT_OUT 'mongo.keyfile'), $text + [Environment]::NewLine, $enc)"
if errorlevel 1 exit /b 1

for %%F in (
    ca.pem
    compute.pem
    frontend.pem
    db.pem
    redis.pem
    mongo.pem
    kafka.pem
) do (
    call :openssl x509 -checkend 2592000 -noout -in /work/%%F || exit /b 1
)

echo OK:
echo   CA:              %CERT_OUT%\ca.pem
echo   COMPUTE CERT:    %CERT_OUT%\compute.pem
echo   FRONTEND CERT:   %CERT_OUT%\frontend.pem
echo   POSTGRES CERT:   %CERT_OUT%\db.pem
echo   REDIS CERT:      %CERT_OUT%\redis.pem
echo   MONGO CERT:      %CERT_OUT%\mongo.pem
echo   KAFKA CERT:      %CERT_OUT%\kafka.pem
echo   KAFKA KEYSTORE:  %CERT_OUT%\kafka.keystore.p12
echo   KAFKA TRUSTSTORE %CERT_OUT%\kafka.truststore.p12

exit /b 0

:openssl
docker run --rm -v "%CERT_OUT%:/work" %OPENSSL_IMAGE% %*
exit /b %ERRORLEVEL%

:gen_server_cert
set "CERT_NAME=%~1"
set "CERT_SUBJECT=%~2"
call :openssl genrsa -out /work/%CERT_NAME%.key 4096 || exit /b 1
call :openssl req -new -key /work/%CERT_NAME%.key -subj "%CERT_SUBJECT%" -out /work/%CERT_NAME%.csr || exit /b 1
call :openssl x509 -req -in /work/%CERT_NAME%.csr -CA /work/ca.pem -CAkey /work/ca.key -CAcreateserial -out /work/%CERT_NAME%.pem -days 3650 -sha256 -extfile /work/%CERT_NAME%.ext || exit /b 1
exit /b 0

:write_ext_header
set "EXT_FILE=%~1"
> "%EXT_FILE%" (
    echo basicConstraints=CA:FALSE
    echo keyUsage = digitalSignature, keyEncipherment
    echo extendedKeyUsage = serverAuth, clientAuth
    echo subjectAltName = @alt_names
    echo.
    echo [alt_names]
)
exit /b 0

:write_compute_ext
call :write_ext_header "%CERT_OUT%\compute.ext"
>> "%CERT_OUT%\compute.ext" echo DNS.1 = rust-compute
>> "%CERT_OUT%\compute.ext" echo DNS.2 = compute
>> "%CERT_OUT%\compute.ext" echo DNS.3 = localhost
exit /b 0

:write_frontend_ext
call :get_env_value FRONTEND_TLS_HOSTS FRONTEND_TLS_HOSTS
if not defined FRONTEND_TLS_HOSTS set "FRONTEND_TLS_HOSTS=localhost,127.0.0.1,ts-frontend"
set "CERT_OUT=%CERT_OUT%"
powershell -NoProfile -ExecutionPolicy Bypass -Command "$out=Join-Path $env:CERT_OUT 'frontend.ext'; $hosts=($env:FRONTEND_TLS_HOSTS -split ',') | ForEach-Object { $_.Trim() } | Where-Object { $_ }; $lines=New-Object System.Collections.Generic.List[string]; $lines.Add('basicConstraints=CA:FALSE'); $lines.Add('keyUsage = digitalSignature, keyEncipherment'); $lines.Add('extendedKeyUsage = serverAuth, clientAuth'); $lines.Add('subjectAltName = @alt_names'); $lines.Add(''); $lines.Add('[alt_names]'); $dns=1; $ip=1; foreach($h in $hosts){ if($h -match '^([0-9]{1,3}\.){3}[0-9]{1,3}$'){ $lines.Add(('IP.{0} = {1}' -f $ip,$h)); $ip++ } else { $lines.Add(('DNS.{0} = {1}' -f $dns,$h)); $dns++ } }; [IO.File]::WriteAllLines($out,[string[]]$lines,[Text.Encoding]::ASCII)"
exit /b %ERRORLEVEL%

:write_db_ext
call :write_ext_header "%CERT_OUT%\db.ext"
>> "%CERT_OUT%\db.ext" echo DNS.1 = db
>> "%CERT_OUT%\db.ext" echo DNS.2 = postgres-db
>> "%CERT_OUT%\db.ext" echo DNS.3 = localhost
>> "%CERT_OUT%\db.ext" echo IP.1 = 127.0.0.1
exit /b 0

:write_redis_ext
call :write_ext_header "%CERT_OUT%\redis.ext"
>> "%CERT_OUT%\redis.ext" echo DNS.1 = cache
>> "%CERT_OUT%\redis.ext" echo DNS.2 = redis-cache
>> "%CERT_OUT%\redis.ext" echo DNS.3 = localhost
>> "%CERT_OUT%\redis.ext" echo IP.1 = 127.0.0.1
exit /b 0

:write_mongo_ext
call :write_ext_header "%CERT_OUT%\mongo.ext"
>> "%CERT_OUT%\mongo.ext" echo DNS.1 = mongo
>> "%CERT_OUT%\mongo.ext" echo DNS.2 = mongo-db
>> "%CERT_OUT%\mongo.ext" echo DNS.3 = mongo-secondary
>> "%CERT_OUT%\mongo.ext" echo DNS.4 = mongo-db-secondary
>> "%CERT_OUT%\mongo.ext" echo DNS.5 = localhost
>> "%CERT_OUT%\mongo.ext" echo IP.1 = 127.0.0.1
exit /b 0

:write_kafka_ext
call :write_ext_header "%CERT_OUT%\kafka.ext"
>> "%CERT_OUT%\kafka.ext" echo DNS.1 = kafka
>> "%CERT_OUT%\kafka.ext" echo DNS.2 = localhost
>> "%CERT_OUT%\kafka.ext" echo IP.1 = 127.0.0.1
exit /b 0

:compose_up
echo.
echo == build and start stack ==

set "COMPOSE_CMD=docker compose --profile prod"
if "%WITH_DEBUG%"=="1" set "COMPOSE_CMD=%COMPOSE_CMD% --profile debug"

if "%SKIP_BUILD%"=="1" (
    %COMPOSE_CMD% up -d --remove-orphans
) else (
    %COMPOSE_CMD% up -d --build --remove-orphans
)

if errorlevel 1 (
    echo WARN: docker compose up failed, cleaning stale containers and networks before one retry.
    call :remove_known_containers
    call :remove_known_networks

    if "%SKIP_BUILD%"=="1" (
        %COMPOSE_CMD% up -d --remove-orphans
    ) else (
        %COMPOSE_CMD% up -d --build --remove-orphans
    )

    if errorlevel 1 exit /b 1
)

exit /b 0

:wait_stack
echo.
echo == wait for services ==

call :wait_container postgres-db || exit /b 1
call :wait_container redis-cache || exit /b 1
call :wait_container mongo-db || exit /b 1
call :wait_container kafka || exit /b 1
call :wait_kafka_init || exit /b 1
call :wait_container rust-compute || exit /b 1
call :wait_container go-backend || exit /b 1
call :wait_container go-worker || exit /b 1
call :wait_container go-compute-runner || exit /b 1
call :wait_container ts-frontend || exit /b 1

exit /b 0

:wait_container
set "WAIT_NAME=%~1"
set /a WAIT_ATTEMPTS=(WAIT_TIMEOUT_SECONDS + 1) / 2

for /L %%A in (1,1,%WAIT_ATTEMPTS%) do (
    set "STATUS="
    set "HEALTH="

    for /f "usebackq delims=" %%S in (`docker inspect -f "{{.State.Status}}" %WAIT_NAME% 2^>nul`) do set "STATUS=%%S"
    for /f "usebackq delims=" %%H in (`docker inspect -f "{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}" %WAIT_NAME% 2^>nul`) do set "HEALTH=%%H"

    if "!HEALTH!"=="healthy" (
        echo OK: %WAIT_NAME% is healthy
        exit /b 0
    )

    if "!HEALTH!"=="none" if "!STATUS!"=="running" (
        echo OK: %WAIT_NAME% is running
        exit /b 0
    )

    if "!STATUS!"=="exited" (
        echo.
        echo --- logs: %WAIT_NAME% ---
        docker logs --tail=120 %WAIT_NAME%
        echo ERROR: %WAIT_NAME% exited before becoming ready.
        exit /b 1
    )

    if "!STATUS!"=="dead" (
        echo.
        echo --- logs: %WAIT_NAME% ---
        docker logs --tail=120 %WAIT_NAME%
        echo ERROR: %WAIT_NAME% is dead.
        exit /b 1
    )

    timeout /t 2 /nobreak >nul
)

echo.
echo --- logs: %WAIT_NAME% ---
docker logs --tail=120 %WAIT_NAME% 2>nul
echo ERROR: timeout waiting for container: %WAIT_NAME%
exit /b 1

:wait_kafka_init
set /a WAIT_ATTEMPTS=(WAIT_TIMEOUT_SECONDS + 1) / 2

for /L %%A in (1,1,%WAIT_ATTEMPTS%) do (
    set "STATUS="
    set "EXIT_CODE="

    for /f "usebackq delims=" %%S in (`docker inspect -f "{{.State.Status}}" kafka-init 2^>nul`) do set "STATUS=%%S"
    for /f "usebackq delims=" %%E in (`docker inspect -f "{{.State.ExitCode}}" kafka-init 2^>nul`) do set "EXIT_CODE=%%E"

    if "!STATUS!"=="exited" if "!EXIT_CODE!"=="0" (
        echo OK: kafka-init completed
        exit /b 0
    )

    if "!STATUS!"=="exited" if not "!EXIT_CODE!"=="0" (
        echo.
        echo --- logs: kafka-init ---
        docker logs --tail=120 kafka-init
        echo ERROR: kafka-init failed.
        exit /b 1
    )

    timeout /t 2 /nobreak >nul
)

echo.
echo --- logs: kafka-init ---
docker logs --tail=120 kafka-init 2>nul
echo ERROR: timeout waiting for kafka-init
exit /b 1

:print_summary
echo.
echo == installation completed ==
docker compose --profile prod --profile debug ps

call :get_env_value BOOTSTRAP_ADMIN_EMAIL ADMIN_EMAIL
call :get_env_value BOOTSTRAP_RESEARCHER_EMAIL RESEARCHER_EMAIL

echo.
echo Frontend: https://127.0.0.1:8080
echo Backend health: http://127.0.0.1:3001/health
echo Admin email: %ADMIN_EMAIL%
echo Researcher email: %RESEARCHER_EMAIL%
echo.
echo Passwords are stored only in local .env. Keep this file private.
exit /b 0

:print_help
echo Usage:
echo   install.bat
echo   install.bat --with-debug
echo   install.bat --keep-existing
echo   install.bat --prune-build-cache
echo   install.bat --skip-build
echo   install.bat --wait-timeout 600
echo.
echo Default mode:
echo   install.bat performs fresh install and recreates .env.
echo.
echo Requirements:
echo   Windows CMD, PowerShell, Docker Desktop, Docker Compose v2.
echo   Git Bash and WSL are not used.
exit /b 0
