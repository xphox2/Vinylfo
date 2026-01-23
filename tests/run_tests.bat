@echo off
REM Vinylfo Test Runner Script
REM Run all tests before alpha release

echo ============================================
echo Vinylfo Test Suite
echo ============================================
echo.

set LOG_DIR=tests\test_results
if not exist %LOG_DIR% mkdir %LOG_DIR%

echo [1/4] Running Syntax Validation...
echo -------------------------------------------
go test ./tests/syntax/... -v > %LOG_DIR%\syntax_validation.log 2>&1
if %ERRORLEVEL% EQU 0 (
    echo PASSED: Syntax validation
) else (
    echo FAILED: Syntax validation - see %LOG_DIR%\syntax_validation.log
)
echo.

echo [2/4] Running All Unit Tests...
echo -------------------------------------------
go test ./... -v > %LOG_DIR%\unit_tests.log 2>&1
if %ERRORLEVEL% EQU 0 (
    echo PASSED: All unit tests
) else (
    echo FAILED: Some unit tests failed - see %LOG_DIR%\unit_tests.log
)
echo.

echo [3/4] Running Integration Tests...
echo -------------------------------------------
go test ./tests/integration/... -v > %LOG_DIR%\integration_tests.log 2>&1
if %ERRORLEVEL% EQU 0 (
    echo PASSED: Integration tests
) else (
    echo FAILED: Integration tests - see %LOG_DIR%\integration_tests.log
)
echo.

echo [4/4] Generating Coverage Report...
echo -------------------------------------------
go test ./... -coverprofile=%LOG_DIR%\coverage.out > nul 2>&1
go tool cover -func=%LOG_DIR%\coverage.out > %LOG_DIR%\coverage_report.txt
echo Coverage report generated: %LOG_DIR%\coverage_report.txt
echo.

echo ============================================
echo Test Summary
echo ============================================
echo.
echo Log files in %LOG_DIR%:
dir %LOG_DIR%\*.log /b
echo.
echo Coverage summary:
type %LOG_DIR%\coverage_report.txt | find "total:"
echo.
echo ============================================
echo Test run complete!
echo ============================================
