# PrivateMail Test Scripts

This directory contains comprehensive test scripts for the PrivateMail API and SMTP server.

## Scripts Overview

### 🧪 `test_all.sh` - Complete Test Suite
The main test runner that orchestrates all tests and can automatically start services.

**Usage:**
```bash
# Run all tests (starts services automatically)
./scripts/test_all.sh

# Run only API tests
./scripts/test_all.sh --api-only

# Run only SMTP tests
./scripts/test_all.sh --smtp-only

# Run tests without starting services
./scripts/test_all.sh --no-start

# Test against remote services
./scripts/test_all.sh --api-url http://api.example.com --smtp-host smtp.example.com
```

### 🌐 `test_api.sh` - API Test Suite
Comprehensive tests for all REST API endpoints.

**Tests include:**
- Health check endpoint
- User registration and login
- Authentication token validation
- Domain management (CRUD operations)
- Email address management
- Email sending endpoint

**Usage:**
```bash
# Test local API
./scripts/test_api.sh

# Test remote API
API_BASE_URL=http://api.example.com ./scripts/test_api.sh
```

### 📧 `test_smtp.sh` - SMTP Server Test Suite
Tests SMTP server functionality and email handling.

**Tests include:**
- SMTP server connectivity
- SMTP banner and EHLO commands
- Email sending via SMTP protocol
- Multiple recipients handling
- Interactive SMTP sessions
- Integration with `swaks` (if available)

**Usage:**
```bash
# Test local SMTP server
./scripts/test_smtp.sh

# Test remote SMTP server
./scripts/test_smtp.sh --host smtp.example.com --port 587

# Test with custom email addresses
./scripts/test_smtp.sh --from sender@test.com --to recipient@test.com
```

## Prerequisites

### Required Tools
- `curl` - For HTTP API testing
- `netcat` (nc) - For SMTP protocol testing
- `jq` - For JSON parsing
- `docker` and `docker-compose` - For service management (optional)

### Optional Tools
- `telnet` - For interactive SMTP testing
- `swaks` - For advanced SMTP testing
- `timeout` - For connection timeouts (usually available by default)

**Install on Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install curl netcat-openbsd jq telnet swaks
```

**Install on macOS:**
```bash
brew install curl netcat jq telnet swaks
```

## Configuration

### Environment Variables

**API Configuration:**
- `API_BASE_URL` - API server URL (default: `http://localhost:8000`)
- `TEST_EMAIL` - Email for registration/login tests (default: `test@example.com`)
- `TEST_PASSWORD` - Password for authentication tests (default: `testpassword123`)
- `TEST_DOMAIN` - Domain for testing (default: `example.com`)

**SMTP Configuration:**
- `SMTP_HOST` - SMTP server host (default: `localhost`)
- `SMTP_PORT` - SMTP server port (default: `25`)
- `FROM_EMAIL` - Sender email address (default: `test@example.com`)
- `TO_EMAIL` - Recipient email address (default: `recipient@privatemail.local`)

**Service Management:**
- `WAIT_TIMEOUT` - Service startup timeout (default: `60` seconds)

### Example Configuration
```bash
# .env file or export these variables
export API_BASE_URL="http://localhost:8000"
export SMTP_HOST="localhost"
export SMTP_PORT="25"
export TEST_EMAIL="testuser@example.com"
export TEST_PASSWORD="securepassword123"
```

## Running Tests

### Quick Start
```bash
# 1. Start services
docker-compose up -d

# 2. Run all tests
./scripts/test_all.sh
```

### Individual Test Suites
```bash
# Test API only
./scripts/test_api.sh

# Test SMTP only
./scripts/test_smtp.sh
```

### Advanced Usage
```bash
# Test against staging environment
API_BASE_URL=https://api-staging.privatemail.com \
SMTP_HOST=smtp-staging.privatemail.com \
./scripts/test_all.sh

# Run tests with custom configuration
./scripts/test_all.sh \
  --api-url http://localhost:3000 \
  --smtp-host localhost \
  --smtp-port 2525 \
  --timeout 120
```

## Test Output

The scripts provide colored output with different log levels:
- 🔵 **INFO** - General information
- 🟢 **SUCCESS** - Test passed
- 🟡 **WARNING** - Non-critical issues
- 🔴 **ERROR** - Test failed

### Sample Output
```
[INFO] Starting PrivateMail API tests...
[INFO] API Base URL: http://localhost:8000
[SUCCESS] API health check passed
[SUCCESS] User registration successful
[SUCCESS] User login successful
[INFO] Test Results: 8/8 tests passed
[SUCCESS] All tests passed!
```

## Troubleshooting

### Common Issues

**1. Services not running:**
```bash
# Check if services are up
docker-compose ps

# Start services
docker-compose up -d

# Check logs
docker-compose logs api
docker-compose logs smtp
```

**2. Connection refused:**
```bash
# Check if ports are open
netstat -tlnp | grep :8000  # API
netstat -tlnp | grep :25    # SMTP

# Test connectivity manually
curl http://localhost:8000/health
nc -zv localhost 25
```

**3. Missing dependencies:**
```bash
# Install required tools
sudo apt-get install curl netcat-openbsd jq

# Or on macOS
brew install curl netcat jq
```

**4. Permission denied:**
```bash
# Make scripts executable
chmod +x scripts/*.sh
```

### Debug Mode
Add `-x` flag to bash for verbose debugging:
```bash
bash -x ./scripts/test_all.sh
```

## Integration with CI/CD

These scripts are designed to work in CI/CD environments:

```yaml
# Example GitHub Actions workflow
- name: Run PrivateMail Tests
  run: |
    docker-compose up -d
    ./scripts/test_all.sh --timeout 120
    docker-compose down
```

## Contributing

When adding new tests:
1. Follow the existing pattern of log functions
2. Add proper error handling
3. Include cleanup functions
4. Update this README
5. Test both success and failure scenarios

## Support

For issues with the test scripts, please check:
1. Service logs: `docker-compose logs`
2. Network connectivity: `nc -zv localhost 8000`
3. Dependencies: Ensure all required tools are installed
4. Configuration: Verify environment variables are set correctly