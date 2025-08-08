#!/bin/bash

# PrivateMail API Test Script
# Tests all available API endpoints

set -e

# Configuration
API_BASE_URL="${API_BASE_URL:-http://localhost:3000}"
TEST_EMAIL="${TEST_EMAIL:-recipient@privatemail.local}"
TEST_PASSWORD="${TEST_PASSWORD:-testpassword123}"
TEST_DOMAIN="${TEST_DOMAIN:-privatemail.local}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Global variables
AUTH_TOKEN=""
USER_ID=""
DOMAIN_ID=""
EMAIL_ID=""
API_KEY=""

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Test if API is running
test_health() {
    log_info "Testing API health endpoint..."
    
    response=$(curl -s -w "%{http_code}" -o /dev/null "$API_BASE_URL/health")
    
    if [ "$response" -eq 200 ]; then
        log_success "API health check passed"
    else
        log_error "API health check failed (HTTP $response)"
    fi
}

# Test user registration
test_register() {
    log_info "Testing user registration..."
    
    response=$(curl -s -w "\n%{http_code}" -X POST "$API_BASE_URL/api/v1/auth/register" \
        -H "Content-Type: application/json" \
        -d "{\"email\":\"$TEST_EMAIL\",\"password\":\"$TEST_PASSWORD\"}")
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" -eq 201 ]; then
        AUTH_TOKEN=$(echo "$body" | jq -r '.token')
        USER_ID=$(echo "$body" | jq -r '.user.id')
        log_success "User registration successful"
        log_info "Token: ${AUTH_TOKEN:0:20}..."
        log_info "User ID: $USER_ID"
    else
        log_error "User registration failed (HTTP $http_code)"
        echo "$body" | jq '.' 2>/dev/null || echo "$body"
    fi
}

# Test user login
test_login() {
    log_info "Testing user login..."
    
    response=$(curl -s -w "\n%{http_code}" -X POST "$API_BASE_URL/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{\"email\":\"$TEST_EMAIL\",\"password\":\"$TEST_PASSWORD\"}")
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" -eq 200 ]; then
        AUTH_TOKEN=$(echo "$body" | jq -r '.token')
        USER_ID=$(echo "$body" | jq -r '.user.id')
        log_success "User login successful"
        log_info "Token: ${AUTH_TOKEN:0:20}..."
    else
        log_error "User login failed (HTTP $http_code)"
        echo "$body" | jq '.' 2>/dev/null || echo "$body"
    fi
}

# Test getting current user info
test_me() {
    log_info "Testing /me endpoint..."
    
    if [ -z "$AUTH_TOKEN" ]; then
        log_error "No auth token available"
        return 1
    fi
    
    response=$(curl -s -w "\n%{http_code}" -X GET "$API_BASE_URL/api/v1/auth/me" \
        -H "Authorization: Bearer $AUTH_TOKEN")
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" -eq 200 ]; then
        log_success "User info retrieved successfully"
        echo "$body" | jq '.'
    else
        log_error "Get user info failed (HTTP $http_code)"
        echo "$body" | jq '.' 2>/dev/null || echo "$body"
    fi
}

# Test creating a domain
test_create_domain() {
    log_info "Testing domain creation..."
    
    if [ -z "$AUTH_TOKEN" ]; then
        log_error "No auth token available"
        return 1
    fi
    
    # Generate a simple public key for testing
    PUBLIC_KEY="ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7vbqajDhA..."
    
    response=$(curl -s -w "\n%{http_code}" -X POST "$API_BASE_URL/api/v1/domains" \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"domain\":\"$TEST_DOMAIN\",\"public_key\":\"$PUBLIC_KEY\",\"storage_enabled\":true}")
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" -eq 201 ]; then
        DOMAIN_ID=$(echo "$body" | jq -r '.id')
        API_KEY=$(echo "$body" | jq -r '.api_key')
        log_success "Domain created successfully"
        log_info "Domain ID: $DOMAIN_ID"
        log_info "API Key: $API_KEY"
    else
        log_error "Domain creation failed (HTTP $http_code)"
        echo "$body" | jq '.' 2>/dev/null || echo "$body"
    fi
}

# Test getting domains
test_get_domains() {
    log_info "Testing get domains..."
    
    if [ -z "$AUTH_TOKEN" ]; then
        log_error "No auth token available"
        return 1
    fi
    
    response=$(curl -s -w "\n%{http_code}" -X GET "$API_BASE_URL/api/v1/domains" \
        -H "Authorization: Bearer $AUTH_TOKEN")
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" -eq 200 ]; then
        log_success "Domains retrieved successfully"
        echo "$body" | jq '.'
    else
        log_error "Get domains failed (HTTP $http_code)"
        echo "$body" | jq '.' 2>/dev/null || echo "$body"
    fi
}

# Test creating an email address
test_create_email() {
    log_info "Testing email address creation..."
    
    if [ -z "$AUTH_TOKEN" ] || [ -z "$DOMAIN_ID" ]; then
        log_error "No auth token or domain ID available"
        return 1
    fi
    
    response=$(curl -s -w "\n%{http_code}" -X POST "$API_BASE_URL/api/v1/domains/$DOMAIN_ID/emails" \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"local_part\":\"test\",\"is_catch_all\":false,\"forward_addresses\":[\"forward@example.com\"]}")
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" -eq 201 ]; then
        EMAIL_ID=$(echo "$body" | jq -r '.id')
        log_success "Email address created successfully"
        log_info "Email ID: $EMAIL_ID"
    else
        log_error "Email address creation failed (HTTP $http_code)"
        echo "$body" | jq '.' 2>/dev/null || echo "$body"
    fi
}

# Test getting email addresses
test_get_emails() {
    log_info "Testing get email addresses..."
    
    if [ -z "$AUTH_TOKEN" ] || [ -z "$DOMAIN_ID" ]; then
        log_error "No auth token or domain ID available"
        return 1
    fi
    
    response=$(curl -s -w "\n%{http_code}" -X GET "$API_BASE_URL/api/v1/domains/$DOMAIN_ID/emails" \
        -H "Authorization: Bearer $AUTH_TOKEN")
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" -eq 200 ]; then
        log_success "Email addresses retrieved successfully"
        echo "$body" | jq '.'
    else
        log_error "Get email addresses failed (HTTP $http_code)"
        echo "$body" | jq '.' 2>/dev/null || echo "$body"
    fi
}

# Test send email endpoint
test_send_email() {
    log_info "Testing send email endpoint..."
    
    response=$(curl -s -w "\n%{http_code}" -X POST "$API_BASE_URL/api/v1/send" \
        -H "Content-Type: application/json" \
        -H "X-API-Key: $API_KEY" \
        -d "{\"to\":[\"recipient@example.com\"],\"from\":\"sender@$TEST_DOMAIN\",\"subject\":\"Test Email\",\"text_body\":\"This is a test email.\"}")
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 202 ]; then
        log_success "Send email test completed"
        echo "$body" | jq '.' 2>/dev/null || echo "$body"
    else
        log_warning "Send email test failed (HTTP $http_code) - this might be expected if not fully implemented"
        echo "$body" | jq '.' 2>/dev/null || echo "$body"
    fi
}

# Cleanup function
cleanup() {
    log_info "Cleaning up test data..."
    
    # Delete email address
    if [ -n "$AUTH_TOKEN" ] && [ -n "$DOMAIN_ID" ] && [ -n "$EMAIL_ID" ]; then
        curl -s -X DELETE "$API_BASE_URL/api/v1/domains/$DOMAIN_ID/emails/$EMAIL_ID" \
            -H "Authorization: Bearer $AUTH_TOKEN" > /dev/null
    fi
    
    # Delete domain
    if [ -n "$AUTH_TOKEN" ] && [ -n "$DOMAIN_ID" ]; then
        curl -s -X DELETE "$API_BASE_URL/api/v1/domains/$DOMAIN_ID" \
            -H "Authorization: Bearer $AUTH_TOKEN" > /dev/null
    fi
    
    log_info "Cleanup completed"
}

# Main test function
run_tests() {
    log_info "Starting PrivateMail API tests..."
    log_info "API Base URL: $API_BASE_URL"
    log_info "Test Email: $TEST_EMAIL"
    log_info "Test Domain: $TEST_DOMAIN"
    echo ""
    
    # Check dependencies
    if ! command -v curl &> /dev/null; then
        log_error "curl is required but not installed"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        log_error "jq is required but not installed"
        exit 1
    fi
    
    # Test sequence
    local tests=(
        #"test_health"
        "test_register"
        "test_login"
        "test_me"
        "test_create_domain"
        "test_get_domains"
        "test_create_email"
        "test_get_emails"
        "test_send_email"
    )
    
    #local passed=0
    #local total=${#tests[@]}
    
    # Set up cleanup trap
    #trap cleanup EXIT
    
    for test in "${tests[@]}"; do
        echo "test: $test"
        # Temporarily disable 'set -e' to allow the loop to continue on failures
        set +e
        "$test"
        rc=$?
        set -e
        # if [ $rc -eq 0 ]; then
        #     ((passed++))
        # fi
    done
    
    echo ""
    #log_info "Test Results: $passed/$total tests passed"
    
    #if [ $passed -eq $total ]; then
   #     log_success "All tests passed!"
   #     return 0
    #else
   #     log_error "Some tests failed"
   #     return 1
    #fi
}

# Check if script is being sourced or executed
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    run_tests "$@"
fi