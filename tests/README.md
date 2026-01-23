# Vinylfo Test Plans

## Overview
This document outlines the comprehensive test plans for the Vinylfo application before alpha release.

## Test Directory Structure
```
tests/
├── README.md                          # This file
├── syntax/                            # Syntax validation tests
│   └── syntax_validation_test.go     # Automated syntax checks
├── integration/                       # Integration test plans
│   ├── api_integration_test.go       # API endpoint tests
│   └── database_integration_test.go  # Database tests
├── test_plans/                        # Manual test plans
│   ├── critical_paths.md             # Critical user paths
│   └── smoke_tests.md                # Smoke test procedures
└── test_results/                      # Test output logs
    └── .gitkeep
```

## Test Categories

### 1. Syntax Validation (tests/syntax/)
**Purpose**: Catch syntax errors before compilation
**Tools**: Go parser, go fmt, go vet
**Test Files**:
- `syntax_validation_test.go` - Validates all Go files parse correctly

### 2. Unit Tests (existing in packages)
**Purpose**: Test individual functions in isolation
**Coverage**:
- `services/youtube_web_search_test.go` - YouTube URL parsing
- `services/youtube_matcher_test.go` - Matching algorithms
- `discogs/rate_limiter_test.go` - Rate limiting
- `controllers/youtube_test.go` - OAuth flow
- `controllers/playback_test.go` - Playback management
- `controllers/discogs_test.go` - Discogs sync

### 3. Integration Tests (tests/integration/)
**Purpose**: Test component interactions
**Test Areas**:
- API endpoints
- Database operations
- OAuth flows
- External API calls (Discogs, YouTube)

### 4. Critical Path Tests (tests/test_plans/)
**Purpose**: Validate user-facing workflows
**Paths**:
1. Application startup
2. Discogs OAuth connection
3. Album sync process
4. Playback control
5. Settings management

## Running Tests

### Syntax Validation
```bash
go run tests/syntax/syntax_validation_test.go
```

### All Unit Tests
```bash
go test ./... -v
```

### Specific Package
```bash
go test ./services/... -v
go test ./controllers/... -v
go test ./discogs/... -v
```

### With Coverage
```bash
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

## Success Criteria

### Before Alpha Release
- [ ] All syntax validation tests pass
- [ ] All existing unit tests pass (no regressions)
- [ ] Critical paths documented and tested
- [ ] Integration tests created for key workflows
- [ ] Test logs captured in `tests/test_results/`

### Syntax Requirements
- No parser errors in any .go file
- No `go vet` warnings
- Code properly formatted (`go fmt`)
- All dependencies resolvable

### Test Coverage Goals
- Core matching algorithms: >90%
- Playback management: >85%
- OAuth flows: >80%
- API endpoints: >75%

## Known Test Areas

### Already Tested
1. YouTube URL parsing and validation
2. YouTube matching score calculations
3. Rate limiting logic
4. Playback state management
5. OAuth security headers
6. Discogs API authentication

### Needs Coverage
1. Database migrations
2. Error handling paths
3. Configuration loading
4. Template rendering
5. Static file serving
6. Session management

## Notes
- Tests use table-driven patterns for clarity
- Mock external APIs where possible
- Tests should be idempotent (can run multiple times)
- Each test should be independent (no shared state)
