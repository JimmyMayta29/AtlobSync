@echo off
setlocal EnableExtensions
cd /d "%~dp0"

set "NO_PAUSE=0"
set "DEV_NO_LICENSE=0"

:parse_args
if "%~1"=="" goto :args_done
if /I "%~1"=="--no-pause" set "NO_PAUSE=1"
if /I "%~1"=="--dev-no-license" set "DEV_NO_LICENSE=1"
shift
goto :parse_args
:args_done

set "BUILD_DIR=%CD%\build"
set "PAYLOAD_DIR=%CD%\cmd\installer\payload"
set "OUT_INSTALLER=%CD%\..\ATLOB_Sync_Instalador.exe"
set "LICENSE_ENV=%CD%\.env.license"
set "SUPABASE_ENV=%CD%\.env.supabase"

if "%DEV_NO_LICENSE%"=="0" (
    if exist "%LICENSE_ENV%" (
        for /f "usebackq eol=# tokens=1,* delims==" %%A in ("%LICENSE_ENV%") do (
            set "K=%%~A"
            set "V=%%~B"
            call :assign_license_var
        )
    )
    if exist "%SUPABASE_ENV%" (
        for /f "usebackq eol=# tokens=1,* delims==" %%A in ("%SUPABASE_ENV%") do (
            set "K=%%~A"
            set "V=%%~B"
            call :assign_license_var
        )
    )
)

set "LIC_FINAL_URL=%LICENSE_API_URL%"
if "%LIC_FINAL_URL%"=="" set "LIC_FINAL_URL=%SUPABASE_URL%"
set "LIC_FINAL_KEY=%LICENSE_API_KEY%"
if "%LIC_FINAL_KEY%"=="" set "LIC_FINAL_KEY=%SUPABASE_ANON_KEY%"

set "LIC_LDFLAGS="
if "%DEV_NO_LICENSE%"=="1" (
    echo [AVISO] Compilando en modo desarrollo SIN proteccion comercial de licencias ^(--dev-no-license^).
) else (
    if "%LIC_FINAL_URL%"=="" goto :missing_license
    if "%LIC_FINAL_KEY%"=="" goto :missing_license
    set "LIC_LDFLAGS=-X github.com/SwodLore/sire-api/internal/license.BuildSupabaseURL=%LIC_FINAL_URL% -X github.com/SwodLore/sire-api/internal/license.BuildSupabaseAnonKey=%LIC_FINAL_KEY% -X github.com/SwodLore/sire-api/internal/license.BuildRequireLicense=true"
)

echo ==============================================================
echo   Compilando ATLOB Sync - SUNAT (ATLOB Consultores) v2.0
if "%DEV_NO_LICENSE%"=="1" (
    echo   Modo: Desarrollo Local ^(Sin Licenciamiento^)
) else (
    echo   Modo: Comercial con Licenciamiento Cloud + Instalador
)
echo ==============================================================

if exist "%BUILD_DIR%" rmdir /s /q "%BUILD_DIR%"
if exist "%PAYLOAD_DIR%" rmdir /s /q "%PAYLOAD_DIR%"
mkdir "%BUILD_DIR%" || goto :fail
mkdir "%PAYLOAD_DIR%" || goto :fail

echo.
echo [1/6] Generando recursos de Windows (icono ATLOB, version y DPI)...
go run ./scripts/buildtool -winres
if errorlevel 1 goto :fail

echo.
echo [2/6] Compilando aplicacion de escritorio (ATLOB_Sync.exe)...
set "CGO_ENABLED=0"
go build -ldflags "-H=windowsgui -s -w %LIC_LDFLAGS%" -o "%BUILD_DIR%\ATLOB_Sync.exe" .
if errorlevel 1 goto :fail

echo.
echo [3/6] Compilando desinstalador nativo (Desinstalar_ATLOB_Sync.exe)...
go build -ldflags "-H=windowsgui -s -w" -o "%BUILD_DIR%\Desinstalar_ATLOB_Sync.exe" ./cmd/uninstaller
if errorlevel 1 goto :fail

echo.
echo [4/6] Empaquetando binarios para el instalador...
go run ./scripts/buildtool -gzip-in "%BUILD_DIR%\ATLOB_Sync.exe" -gzip-out "%PAYLOAD_DIR%\ATLOB_Sync.exe.gz"
if errorlevel 1 goto :fail
go run ./scripts/buildtool -gzip-in "%BUILD_DIR%\Desinstalar_ATLOB_Sync.exe" -gzip-out "%PAYLOAD_DIR%\Desinstalar_ATLOB_Sync.exe.gz"
if errorlevel 1 goto :fail

echo.
echo [5/6] Compilando instalador principal (ATLOB_Sync_Instalador.exe)...
go build -tags installer_bundle -ldflags "-H=windowsgui -s -w" -o "%OUT_INSTALLER%" ./cmd/installer
if errorlevel 1 goto :fail

echo.
echo [6/6] Limpiando archivos intermedios para evitar ejecutables duplicados...
call :cleanup

echo.
echo ==============================================================
echo   [EXITO] Instalador principal generado en la carpeta raiz:
echo   %OUT_INSTALLER%
echo   Haga doble clic para instalar ATLOB Sync en modo comercial.
echo ==============================================================
if "%NO_PAUSE%"=="0" pause
exit /b 0

:assign_license_var
if /I "%K%"=="LICENSE_API_URL" if "%LICENSE_API_URL%"=="" set "LICENSE_API_URL=%V%"
if /I "%K%"=="LICENSE_API_KEY" if "%LICENSE_API_KEY%"=="" set "LICENSE_API_KEY=%V%"
if /I "%K%"=="SUPABASE_URL" if "%SUPABASE_URL%"=="" set "SUPABASE_URL=%V%"
if /I "%K%"=="SUPABASE_ANON_KEY" if "%SUPABASE_ANON_KEY%"=="" set "SUPABASE_ANON_KEY=%V%"
exit /b 0

:missing_license
echo.
echo [CONFIGURACION PENDIENTE] No se encontro LICENSE_API_URL / LICENSE_API_KEY ^(ni SUPABASE_*^).
echo Para compilar el instalador comercial con licencias de 7 dias / pago:
echo   1. Configura .env.license ^(Cloudflare Workers^) o .env.supabase ^(Supabase^)
echo   2. Revisa cloudflare\GUIA_CLOUDFLARE.md o supabase\GUIA_SUPABASE.md.
echo.
echo Si solo deseas compilar internamente sin licencias, ejecuta:
echo   compilar.bat --dev-no-license
echo.
if "%NO_PAUSE%"=="0" pause
exit /b 1

:fail
echo.
echo [ERROR] Fallo la compilacion. Revise los mensajes anteriores.
call :cleanup
if "%NO_PAUSE%"=="0" pause
exit /b 1

:cleanup
if exist "%BUILD_DIR%" rmdir /s /q "%BUILD_DIR%"
if exist "%PAYLOAD_DIR%" rmdir /s /q "%PAYLOAD_DIR%"
if exist "%CD%\rsrc_windows_amd64.syso" del /q "%CD%\rsrc_windows_amd64.syso"
if exist "%CD%\cmd\installer\rsrc_windows_amd64.syso" del /q "%CD%\cmd\installer\rsrc_windows_amd64.syso"
if exist "%CD%\cmd\uninstaller\rsrc_windows_amd64.syso" del /q "%CD%\cmd\uninstaller\rsrc_windows_amd64.syso"
exit /b 0
