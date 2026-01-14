# Tenant Service Tests

This directory contains comprehensive tests for the tenant onboarding service.

## Test Structure

```
tests/
├── integration/        # Integration tests (full API workflows)
│   └── onboarding_test.go
├── unit/              # Unit tests (individual functions/methods)
│   └── validation_test.go
└── README.md          # This file
```

## Prerequisites

1. **PostgreSQL Test Database**
   - Create a dedicated test database: `tesseract_hub_test`
   - Ensure PostgreSQL is running
   - Test database will be automatically migrated

2. **Environment Variables**
   Set the following environment variables for testing:
   ```bash
   export DB_HOST=localhost
   export DB_PORT=5432
   export DB_USER=dev
   export DB_PASSWORD=dev
   export DB_NAME=tesseract_hub_test
   export DB_SSL_MODE=disable
   export VERIFICATION_SERVICE_URL=http://localhost:8088
   ```

3. **Go Dependencies**
   ```bash
   go mod download
   ```

## Running Tests

### Run All Tests
```bash
go test ./tests/... -v
```

### Run Integration Tests Only
```bash
go test ./tests/integration/... -v
```

### Run Specific Test
```bash
go test ./tests/integration -run TestCompleteOnboardingFlow -v
```

### Run with Coverage
```bash
go test ./tests/... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### Run with Race Detection
```bash
go test ./tests/... -race -v
```

## Test Database Setup

### Create Test Database
```sql
CREATE DATABASE tesseract_hub_test;
GRANT ALL PRIVILEGES ON DATABASE tesseract_hub_test TO dev;
```

### Clean Test Database
```sql
DROP DATABASE IF EXISTS tesseract_hub_test;
CREATE DATABASE tesseract_hub_test;
```

## Unit Tests

### TestSubdomainValidation
Tests subdomain validation logic with mocked dependencies:
- ✅ Available subdomain returns false (not exists)
- ✅ Taken subdomain returns true (exists)
- ✅ Subdomain available for same session (updating own subdomain)

**What it validates:**
- Repository method behavior with mocks
- Business logic without database dependencies
- Different validation scenarios

### TestBusinessNameValidation
Tests business name validation logic:
- ✅ Available business name
- ✅ Taken business name

**What it validates:**
- Name uniqueness checking
- Mock expectations and assertions

### TestProgressCalculation
Tests progress percentage calculation:
- ✅ Zero percent with no completed steps
- ✅ Fifty percent with half completed
- ✅ One hundred percent with all completed
- ✅ Handles decimal percentages correctly

**What it validates:**
- Mathematical accuracy
- Edge cases (0%, 100%)
- Floating point precision

## Integration Tests

### TestCompleteOnboardingFlow
Tests the complete happy path onboarding workflow:
1. ✅ Start onboarding session
2. ✅ Retrieve onboarding session
3. ✅ Update business information
4. ✅ Update contact information
5. ✅ Start email verification
6. ✅ Update business address
7. ✅ Get progress tracking

**What it validates:**
- All API endpoints return correct HTTP status codes
- Response structure matches expected format
- Data persistence across requests
- Progress tracking calculates correctly

### TestValidationErrors
Tests various validation and error scenarios:
1. ✅ Invalid session ID format
2. ✅ Non-existent session ID
3. ✅ Missing required fields
4. ✅ Invalid email format

**What it validates:**
- Proper error handling
- Validation of input data
- Appropriate HTTP error codes (400, 404)

### TestSubdomainValidation
Tests subdomain availability checking:
1. ✅ Available subdomain returns true
2. ✅ Taken subdomain returns false

**What it validates:**
- Subdomain uniqueness validation
- Validation endpoint correctness

## Test Data Management

### Automatic Cleanup
Each test automatically cleans up its data after execution using `CleanupTestData()`.

### Manual Cleanup
If tests fail and leave orphaned data:
```bash
cd /path/to/tenant-service
psql -U dev -d tesseract_hub_test -c "
  DELETE FROM verification_records;
  DELETE FROM onboarding_tasks;
  DELETE FROM business_addresses;
  DELETE FROM contact_information;
  DELETE FROM business_information;
  DELETE FROM onboarding_sessions;
"
```

## Writing New Tests

### Integration Test Template
```go
func TestNewFeature(t *testing.T) {
	tc := SetupTestEnvironment(t)
	defer tc.CleanupTestData(t)

	tc.SeedDefaultTemplate(t, "ecommerce")

	t.Run("Test Case Name", func(t *testing.T) {
		// Arrange
		reqBody := map[string]interface{}{
			"field": "value",
		}
		body, _ := json.Marshal(reqBody)

		// Act
		req := httptest.NewRequest(http.MethodPost, "/api/v1/endpoint", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		tc.Router.ServeHTTP(w, req)

		// Assert
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.True(t, response["success"].(bool))
	})
}
```

## Common Issues and Solutions

### Issue: Database Connection Failed
**Solution:**
- Ensure PostgreSQL is running: `docker-compose up -d postgres`
- Check environment variables are set correctly
- Verify database exists: `psql -U dev -l | grep tesseract_hub_test`

### Issue: Tests Fail with "template not found"
**Solution:**
- Tests automatically seed templates via `SeedDefaultTemplate()`
- Ensure test cleanup isn't removing templates mid-test

### Issue: Verification service not available
**Solution:**
- Integration tests mock external service calls
- For full E2E tests, start verification service: `./bin/verification-service`

### Issue: Port Already in Use
**Solution:**
- Tests use `httptest` which doesn't bind to real ports
- If real services conflict, stop them: `lsof -ti:8086 | xargs kill`

## Continuous Integration

### GitHub Actions Example
```yaml
name: Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_USER: dev
          POSTGRES_PASSWORD: dev
          POSTGRES_DB: tesseract_hub_test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      - run: go test ./tests/... -v -cover
```

## Test Coverage Goals

- **Unit Tests**: 80% code coverage
- **Integration Tests**: All critical user flows
- **API Endpoints**: 100% endpoint coverage

## Future Enhancements

- [ ] Add unit tests for individual services and repositories
- [ ] Add E2E tests with real verification service
- [ ] Add performance/load tests
- [ ] Add contract tests for external APIs
- [ ] Add mutation testing

## Resources

- [Testing Guide](https://go.dev/doc/tutorial/add-a-test)
- [Testify Documentation](https://github.com/stretchr/testify)
- [Go httptest Package](https://pkg.go.dev/net/http/httptest)
