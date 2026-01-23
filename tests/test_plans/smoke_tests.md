# Smoke Tests

Quick sanity checks to verify basic functionality before deeper testing.

## Run All Smoke Tests

```bash
go test ./... -run Smoke -v 2>&1 | tee tests/test_results/smoke_test.log
```

## Smoke Test Categories

### 1. Application Startup Smoke Test

**Purpose**: Verify application can start without crashing

```bash
# Start the application in background
vinylfo.exe &
sleep 3

# Check if process is running
if ps aux | grep -q "[v]inylfo"; then
    echo "PASS: Application started successfully"
else
    echo "FAIL: Application did not start"
fi

# Check if web server is responding
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080
if [ $? -eq 200 ]; then
    echo "PASS: Web server responding"
else
    echo "FAIL: Web server not responding"
fi

# Cleanup
pkill vinylfo.exe
```

### 2. Database Smoke Test

**Purpose**: Verify database operations work

```bash
go test ./database/... -v
```

**Expected**: All tests pass

### 3. API Endpoint Smoke Test

**Purpose**: Verify all endpoints return valid responses

```bash
# Start app first, then run:
endpoints=("/" "/player" "/playlist" "/settings" "/sync" "/search" "/youtube")

for endpoint in "${endpoints[@]}"; do
    response=$(curl -s -w "\n%{http_code}" "http://localhost:8080$endpoint")
    code=$(echo "$response" | tail -n1)
    if [ "$code" -eq 200 ]; then
        echo "PASS: $endpoint returns 200"
    else
        echo "FAIL: $endpoint returns $code"
    fi
done
```

### 4. OAuth Flow Smoke Test

**Purpose**: Verify OAuth endpoints are configured

```bash
go test ./controllers/... -run TestOAuth -v
```

### 5. YouTube Integration Smoke Test

**Purpose**: Verify YouTube URL handling works

```bash
go test ./services/... -run TestExtract -v
```

### 6. Playback System Smoke Test

**Purpose**: Verify playback controls respond

```bash
go test ./controllers/... -run TestPlayback -v
```

## Quick Smoke Test Script

```bash
#!/bin/bash
# smoke_test.sh - Quick smoke test for Vinylfo

echo "=== Vinylfo Smoke Test ==="
echo ""

echo "1. Syntax validation..."
go run ./tests/syntax/syntax_validation_test.go
if [ $? -ne 0 ]; then
    echo "FAIL: Syntax validation failed"
    exit 1
fi
echo "PASS: Syntax validation"
echo ""

echo "2. Running all tests..."
go test ./... -short 2>&1 | tee tests/test_results/smoke_test.log
if [ ${PIPESTATUS[0]} -ne 0 ]; then
    echo "WARN: Some tests failed"
fi
echo ""

echo "3. Checking go.mod..."
if grep -q "module vinylfo" go.mod; then
    echo "PASS: go.mod valid"
else
    echo "FAIL: go.mod invalid"
    exit 1
fi
echo ""

echo "=== Smoke Test Complete ==="
```

## Pass/Fail Criteria

### Must Pass (Blockers)
- [ ] All syntax validation tests pass
- [ ] go.mod is valid
- [ ] main.go compiles
- [ ] Database package compiles

### Should Pass (High Priority)
- [ ] 80%+ unit tests pass
- [ ] API endpoints return 200
- [ ] No panic in tests

## Logging

All smoke test output should be captured in:
- `tests/test_results/smoke_test.log`
- `tests/test_results/unit_test.log`
- `tests/test_results/integration_test.log`
