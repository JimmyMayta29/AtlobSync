@echo off
echo ==================================================
echo   Compilando ATLOB Sync - SUNAT (ATLOB Consultores)
echo   Modo: Ejecutable de Doble Clic (Windows GUI)
echo ==================================================
go build -ldflags "-H=windowsgui -s -w" -o sire_api.exe .
if %ERRORLEVEL% EQU 0 (
    echo.
    echo [EXITO] Compilacion completada: sire_api.exe generado.
    echo Puede hacer doble clic sobre 'sire_api.exe' para iniciar.
) else (
    echo.
    echo [ERROR] Fallo la compilacion. Revise los errores anteriores.
)
pause
