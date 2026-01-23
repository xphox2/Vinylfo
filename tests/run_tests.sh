#!/bin/bash
# Vinylfo Test Runner Script
# Run all tests before alpha release

echo "============================================"
echo "Vinylfo Test Suite"
echo "============================================"
echo ""

LOG_DIR="tests/test_results"
mkdir -p "$LOG_DIR"

echo "[1/4] Running Syntax Validation..."
echo "-------------------------------------------"
go test ./tests/syntax/... -v > "$LOG_DIR/syntax_validation.log" 2>&1
if [ $? -eq 0 ]; then
    echo "PASSED: Syntax validation"
else
    echo "FAILED: Syntax validation - see $LOG_DIR/syntax_validation.log"
fi
echo ""

echo "[2/4] Running All Unit Tests..."
echo "-------------------------------------------"
go test ./... -v > "$LOG_DIR/unit_tests.log" 2>&1
if [ $? -eq 0 ]; then
    echo "PASSED: All unit tests"
else
    echo "FAILED: Some unit tests failed - see $LOG_DIR/unit_tests.log"
fi
echo ""

echo "[3/4] Running Integration Tests..."
echo "-------------------------------------------"
go test ./tests/integration/... -v > "$LOG_DIR/integration_tests.log" 2>&1
if [ $? -eq 0 ]; then
    echo "PASSED: Integration tests"
else
    echo "FAILED: Integration tests - see $LOG_DIR/integration_tests.log"
fi
echo ""

echo "[4/4] Generating Coverage Report..."
echo "-------------------------------------------"
go test ./... -coverprofile="$LOG_DIR/coverage.out" > /dev/null 2>&1
go tool cover -func="$LOG_DIR/coverage.out" > "$LOG_DIR/coverage_report.txt"
echo "Coverage report generated: $LOG_DIR/coverage_report.txt"
echo ""

echo "============================================"
echo "Test Summary"
echo "============================================"
echo ""
echo "Log files in $LOG_DIR:"
ls -la "$LOG_DIR"/*.log 2>/dev/null || echo "No log files yet"
echo ""
echo "Coverage summary:"
grep "total:" "$LOG_DIR/coverage_report.txt" 2>/dev/null || echo "No coverage data yet"
echo ""
echo "============================================"
echo "Test run complete!"
echo "============================================"
