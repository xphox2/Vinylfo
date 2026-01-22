@echo off
echo Building Vinylfo...
go build -ldflags "-H=windowsgui" -o vinylfo.exe
if %errorlevel% equ 0 (
    echo Build successful! Output: vinylfo.exe
) else (
    echo Build failed!
)
pause
