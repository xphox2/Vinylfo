@echo off
echo Starting VinylFO Server...
echo.

REM Check if the binary exists
if exist "vinylfo.exe" (
    echo Found vinylfo.exe - starting server...
    vinylfo.exe
) else (
    echo Error: vinylfo.exe not found!
    echo Please build the project first using: go build -o vinylfo main.go
    pause
)