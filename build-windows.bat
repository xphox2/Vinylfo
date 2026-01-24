@echo off
REM Vinylfo Windows Build Script

echo Building Vinylfo...

REM Extract version from CHANGELOG.md using PowerShell
for /f %%v in ('powershell -Command "$line = Get-Content CHANGELOG.md | Select-String '## \[0' | Where-Object { $_ -NotMatch 'Unreleased' } | Select-Object -First 1 -ExpandProperty Line; if ($line -match '\[([^\]]+)\]') { $matches[1] }"') do set VERSION=%%v

if not defined VERSION (
    echo Could not extract version from CHANGELOG.md
    set VERSION=0.0.0-alpha
)

REM Extract numeric version for file version (remove -alpha, -beta, etc.)
for /f "tokens=1 delims=-" %%n in ("%VERSION%") do set FILE_VERSION=%%n

echo Version: %VERSION% (file: %FILE_VERSION%)

REM Copy icon for resource compiler
set ICON_FILE=
if exist icons\vinyl-icon.ico (
    copy icons\vinyl-icon.ico winres\icon.ico >nul
    set ICON_FILE=icon.ico
) else if exist icons\vinyl-icon.png (
    copy icons\vinyl-icon.png winres\icon.png >nul
    set ICON_FILE=icon.png
)

REM Generate JSON with version and icon
(
    echo {
    echo   "RT_VERSION": {
    echo     "#1": {
    echo       "0409": {
    echo         "fixed": {
    echo           "file_version": "%FILE_VERSION%.0",
    echo           "product_version": "%FILE_VERSION%.0"
    echo         },
    echo         "info": {
    echo           "0409": {
    echo             "FileDescription": "Vinylfo Album Manager",
    echo             "FileVersion": "%FILE_VERSION%",
    echo             "InternalName": "vinylfo",
    echo             "LegalCopyright": "Copyright 2026",
    echo             "OriginalFilename": "vinylfo.exe",
    echo             "ProductName": "Vinylfo",
    echo             "ProductVersion": "%VERSION%"
    echo           }
    echo         }
    echo       }
    echo     }
    echo   }
) > winres\winres.json

if defined ICON_FILE (
    echo Adding icon: %ICON_FILE%
    (
        echo ,
        echo   "RT_GROUP_ICON": {
        echo     "APP": {
        echo       "0409": "%ICON_FILE%"
        echo     }
        echo   }
        echo }
    ) >> winres\winres.json
) else (
    echo } >> winres\winres.json
)

REM Generate version resource
cd winres
go-winres make --in winres.json
cd ..

REM Move syso files to root for go build
if exist winres\rsrc_windows_amd64.syso move winres\rsrc_windows_amd64.syso .
if exist winres\rsrc_windows_386.syso move winres\rsrc_windows_386.syso .

REM Build executable with version embedded
go build -ldflags "-H=windowsgui -X vinylfo/routes.Version=%VERSION%" -o vinylfo.exe .

if %errorlevel% equ 0 (
    echo Build successful! Output: vinylfo.exe
    echo Version metadata embedded: %VERSION%
) else (
    echo Build failed!
)
pause
