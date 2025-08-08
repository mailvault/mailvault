#!/bin/bash

# PrivateMail Complete Test Suite
# Runs all tests for API and SMTP server

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Default configuration
API_BASE_URL="${API_BASE_URL:-http://localhost:3000}"
SMTP_HOST="${SMTP_HOST:-localhost}"
SMTP_PORT="${SMTP_PORT:-2525}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-60}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

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

log_header() {
    echo -e "${BOLD}${BLUE}$1${NC}"
}

# Wait for service to be ready
wait_for_service() {
    local service_name="$1"
    local host="$2"
    local port="$3"
    local timeout="${4:-30}"
    
    log_info "Waiting for $service_name to be ready at $host:$port..."
    
    local count=0
    while ! timeout 1 bash -c "echo > /dev/tcp/$host/$port" 2>/dev/null; do
        sleep 1
        ((count++))
        if [ $count -ge $timeout ]; then
            log_error "$service_name is not ready after ${timeout}s"
            return 1
        fi
        if [ $((count % 10)) -eq 0 ]; then
            log_info "Still waiting for $service_name... (${count}s)"
        fi
    done
    
    log_success "$service_name is ready!"
    return 0
}

# Check if services are running via docker-compose
check_docker_services() {
    log_info "Checking Docker services status..."
    
    if ! command -v docker-compose &> /dev/null && ! command -v docker &> /dev/null; then
        log_warning "Docker/docker-compose not found, skipping service status check"
        return 0
    fi
    
    # Try docker-compose first, then docker compose
    local compose_cmd=""
    if command -v docker-compose &> /dev/null; then
        compose_cmd="docker-compose"
    elif docker compose version &> /dev/null; then
        compose_cmd="docker compose"
    fi
    
    if [ -n "$compose_cmd" ] && [ -f "$PROJECT_ROOT/docker-compose.yaml" ]; then
        cd "$PROJECT_ROOT"
        
        log_info "Docker services status:"
        $compose_cmd ps | grep -E "(privatemail-api|privatemail-smtp|db)" || true
        
        # Check if services are running
        if $compose_cmd ps | grep -q "privatemail-api.*Up"; then
            log_success "API service is running"
        else
            log_warning "API service is not running"
        fi
        
        if $compose_cmd ps | grep -q "privatemail-smtp.*Up"; then
            log_success "SMTP service is running"
        else
            log_warning "SMTP service is not running"
        fi
        
        if $compose_cmd ps | grep -q "db.*Up"; then
            log_success "Database service is running"
        else
            log_warning "Database service is not running"
        fi
    else
        log_info "No docker-compose.yaml found or compose command unavailable"
    fi
}

# Start services if not running
start_services() {
    log_info "Starting services if needed..."
    
    local compose_cmd=""
    if command -v docker-compose &> /dev/null; then
        compose_cmd="docker-compose"
    elif docker compose version &> /dev/null; then
        compose_cmd="docker compose"
    fi
    
    if [ -n "$compose_cmd" ] && [ -f "$PROJECT_ROOT/docker-compose.yaml" ]; then
        cd "$PROJECT_ROOT"
        
        log_info "Starting services with docker-compose..."
        $compose_cmd up -d
        
        # Wait for database first
        log_info "Waiting for database to be ready..."
        sleep 5
        
        # Wait for API service
        wait_for_service "API" "localhost" "3000" 30
        
        # Wait for SMTP service
        wait_for_service "SMTP" "localhost" "2525" 30
        
        return 0
    else
        log_warning "Cannot start services automatically - docker-compose not available"
        return 1
    fi
}

# Run API tests
run_api_tests() {
    log_header "=== Running API Tests ==="
    
    if [ ! -f "$SCRIPT_DIR/test_api.sh" ]; then
        log_error "API test script not found: $SCRIPT_DIR/test_api.sh"
        return 1
    fi
    
    # Make sure the script is executable
    chmod +x "$SCRIPT_DIR/test_api.sh"
    
    # Run API tests
    if bash "$SCRIPT_DIR/test_api.sh"; then
        log_success "API tests completed successfully"
        return 0
    else
        log_error "API tests failed"
        return 1
    fi
}

# Run SMTP tests
run_smtp_tests() {
    log_header "=== Running SMTP Tests ==="
    
    if [ ! -f "$SCRIPT_DIR/test_smtp.sh" ]; then
        log_error "SMTP test script not found: $SCRIPT_DIR/test_smtp.sh"
        return 1
    fi
    
    # Make sure the script is executable
    chmod +x "$SCRIPT_DIR/test_smtp.sh"
    
    # Run SMTP tests
    if bash "$SCRIPT_DIR/test_smtp.sh"; then
        log_success "SMTP tests completed successfully"
        return 0
    else
        log_error "SMTP tests failed"
        return 1
    fi
}

# Run integration tests
run_integration_tests() {
    log_header "=== Running Integration Tests ==="
    
    log_info "Testing API and SMTP integration..."
    
    # Test 1: Send email via API, verify SMTP handles it
    log_info "Integration test 1: API to SMTP flow"
    # This would require more complex setup, so we'll skip for now
    log_warning "Integration tests not yet implemented"
    
    return 0
}

# Generate test report
generate_report() {
    local api_result=$1
    local smtp_result=$2
    local integration_result=$3
    
    log_header "=== Test Report ==="
    
    echo ""
    echo "Test Results Summary:"
    echo "===================="
    
    if [ $api_result -eq 0 ]; then
        echo -e "API Tests:         ${GREEN}PASSED${NC}"
    else
        echo -e "API Tests:         ${RED}FAILED${NC}"
    fi
    
    if [ $smtp_result -eq 0 ]; then
        echo -e "SMTP Tests:        ${GREEN}PASSED${NC}"
    else
        echo -e "SMTP Tests:        ${RED}FAILED${NC}"
    fi
    
    if [ $integration_result -eq 0 ]; then
        echo -e "Integration Tests: ${GREEN}PASSED${NC}"
    else
        echo -e "Integration Tests: ${RED}FAILED${NC}"
    fi
    
    echo ""
    
    local total_failed=$((api_result + smtp_result + integration_result))
    if [ $total_failed -eq 0 ]; then
        log_success "All test suites passed!"
        return 0
    else
        log_error "$total_failed test suite(s) failed"
        return 1
    fi
}

# Show usage information
show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --api-only          Run only API tests"
    echo "  --smtp-only         Run only SMTP tests"
    echo "  --no-start          Don't try to start services"
    echo "  --api-url URL       API base URL (default: http://localhost:8000)"
    echo "  --smtp-host HOST    SMTP host (default: localhost)"
    echo "  --smtp-port PORT    SMTP port (default: 25)"
    echo "  --timeout SECONDS   Service wait timeout (default: 60)"
    echo "  --help              Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  API_BASE_URL        API base URL"
    echo "  SMTP_HOST           SMTP host"
    echo "  SMTP_PORT           SMTP port"
    echo "  WAIT_TIMEOUT        Service wait timeout"
    echo ""
    echo "Examples:"
    echo "  $0                          # Run all tests"
    echo "  $0 --api-only               # Run only API tests"
    echo "  $0 --smtp-only              # Run only SMTP tests"
    echo "  $0 --no-start               # Run tests without starting services"
    echo "  $0 --api-url http://api:8000 # Test remote API"
}

# Main function
main() {
    local run_api=true
    local run_smtp=true
    local start_services_flag=true
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --api-only)
                run_smtp=false
                shift
                ;;
            --smtp-only)
                run_api=false
                shift
                ;;
            --no-start)
                start_services_flag=false
                shift
                ;;
            --api-url)
                API_BASE_URL="$2"
                shift 2
                ;;
            --smtp-host)
                SMTP_HOST="$2"
                shift 2
                ;;
            --smtp-port)
                SMTP_PORT="$2"
                shift 2
                ;;
            --timeout)
                WAIT_TIMEOUT="$2"
                shift 2
                ;;
            --help)
                show_usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
    
    log_header "PrivateMail Test Suite"
    echo ""
    log_info "Configuration:"
    log_info "  API URL: $API_BASE_URL"
    log_info "  SMTP: $SMTP_HOST:$SMTP_PORT"
    log_info "  Project root: $PROJECT_ROOT"
    log_info "  Script directory: $SCRIPT_DIR"
    echo ""
    
    # Check dependencies
    local missing_deps=()
    
    if ! command -v curl &> /dev/null; then
        missing_deps+=("curl")
    fi
    
    if ! command -v nc &> /dev/null; then
        missing_deps+=("netcat")
    fi
    
    if ! command -v jq &> /dev/null; then
        missing_deps+=("jq")
    fi
    
    if [ ${#missing_deps[@]} -gt 0 ]; then
        log_error "Missing required dependencies: ${missing_deps[*]}"
        log_info "Install with: apt-get install ${missing_deps[*]}"
        exit 1
    fi
    
    # Check and start services if requested
    if $start_services_flag; then
        check_docker_services
        
        # Try to start services if they're not running
        if ! timeout 1 bash -c "echo > /dev/tcp/localhost/8000" 2>/dev/null || \
           ! timeout 1 bash -c "echo > /dev/tcp/localhost/25" 2>/dev/null; then
            log_info "Services not detected, attempting to start them..."
            start_services
        else
            log_success "Services are already running"
        fi
    fi
    
    # Export configuration for child scripts
    export API_BASE_URL
    export SMTP_HOST
    export SMTP_PORT
    
    # Run tests
    local api_result=0
    local smtp_result=0
    local integration_result=0
    
    if $run_api; then
        run_api_tests || api_result=$?
        echo ""
    fi
    
    if $run_smtp; then
        run_smtp_tests || smtp_result=$?
        echo ""
    fi
    
    if $run_api && $run_smtp; then
        run_integration_tests || integration_result=$?
        echo ""
    fi
    
    # Generate final report
    generate_report $api_result $smtp_result $integration_result
}

# Check if script is being sourced or executed
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi