@echo off
setlocal EnableExtensions
cd /d "%~dp0"
title ATLOB Sync - Gestor Comercial de Licencias

go run ./cmd/licadmin
if errorlevel 1 (
    echo.
    pause
    exit /b 1
)
exit /b 0
