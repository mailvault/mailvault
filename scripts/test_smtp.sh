#!/bin/bash

# PrivateMail SMTP Server Test Script
# Tests SMTP server functionality

set -e

# Configuration
SMTP_HOST="${SMTP_HOST:-localhost}"
SMTP_PORT="${SMTP_PORT:-2525}"
FROM_EMAIL="${FROM_EMAIL:-test@example.com}"
TO_EMAIL="${TO_EMAIL:-recipient@privatemail.local}"
TEST_DOMAIN="${TEST_DOMAIN:-privatemail.local}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

# Test SMTP server connectivity
test_smtp_connection() {
    log_info "Testing SMTP server connection..."
    
    if timeout 5 bash -c "echo > /dev/tcp/$SMTP_HOST/$SMTP_PORT" 2>/dev/null; then
        log_success "SMTP server is reachable at $SMTP_HOST:$SMTP_PORT"
        return 0
    else
        log_error "SMTP server is not reachable at $SMTP_HOST:$SMTP_PORT"
        return 1
    fi
}

# Test SMTP banner
test_smtp_banner() {
    log_info "Testing SMTP banner..."
    
    response=$(timeout 10 bash -c "exec 3<>/dev/tcp/$SMTP_HOST/$SMTP_PORT; cat <&3" 2>/dev/null | head -n1)
    
    if [[ $response == 220* ]]; then
        log_success "SMTP banner received: $response"
        return 0
    else
        log_error "Invalid SMTP banner: $response"
        return 1
    fi
}

# Test SMTP EHLO command
test_smtp_ehlo() {
    log_info "Testing SMTP EHLO command..."
    
    {
        echo "EHLO $TEST_DOMAIN"
        sleep 1
        echo "QUIT"
    } | timeout 10 nc $SMTP_HOST $SMTP_PORT > /tmp/smtp_ehlo_response.txt 2>/dev/null
    
    if grep -q "250" /tmp/smtp_ehlo_response.txt; then
        log_success "EHLO command successful"
        log_info "Server capabilities:"
        grep "250-" /tmp/smtp_ehlo_response.txt | sed 's/^/  /'
        return 0
    else
        log_error "EHLO command failed"
        cat /tmp/smtp_ehlo_response.txt
        return 1
    fi
}

# Send a test email via SMTP
test_send_email() {
    log_info "Testing email sending via SMTP..."
    
    local timestamp=$(date '+%Y%m%d-%H%M%S')
    local message_id="<test-$timestamp@$TEST_DOMAIN>"
    
    # Create email content
    cat > /tmp/test_email.txt << EOF
From: $FROM_EMAIL
To: $TO_EMAIL
Subject: SMTP Test Email - $timestamp
Message-ID: $message_id
Date: $(date -R)
Content-Type: text/plain; charset=utf-8

This is a test email sent via SMTP to verify the PrivateMail SMTP server functionality.

Test Details:
- Timestamp: $timestamp
- From: $FROM_EMAIL
- To: $TO_EMAIL
- Message ID: $message_id

If you receive this email, the SMTP server is working correctly!
EOF

    # Convert email content to proper CRLF line endings for SMTP
    sed -i 's/$/\r/' /tmp/test_email.txt
    
    # Send email using SMTP commands with proper CRLF line endings
    {
        sleep 1
        echo -e "EHLO $TEST_DOMAIN\r"
        sleep 1
        echo -e "MAIL FROM:<$FROM_EMAIL>\r"
        sleep 1
        echo -e "RCPT TO:<$TO_EMAIL>\r"
        sleep 1
        echo -e "DATA\r"
        sleep 1
        cat /tmp/test_email.txt
        echo -e "\r\n.\r"
        sleep 1
        echo -e "QUIT\r"
    } | timeout 30 nc $SMTP_HOST $SMTP_PORT > /tmp/smtp_send_response.txt 2>/dev/null
    
    if grep -q "250.*OK\|250.*queued" /tmp/smtp_send_response.txt; then
        log_success "Email sent successfully via SMTP"
        log_info "Message ID: $message_id"
        return 0
    else
        log_error "Email sending failed"
        cat /tmp/smtp_send_response.txt
        return 1
    fi
}

# Test SMTP with telnet-style interaction
test_smtp_interactive() {
    log_info "Testing SMTP server interactively..."
    
    # Create a simple SMTP session script
    cat > /tmp/smtp_session.txt << EOF
EHLO $TEST_DOMAIN
MAIL FROM:<$FROM_EMAIL>
RCPT TO:<$TO_EMAIL>
DATA
Subject: Interactive SMTP Test
From: $FROM_EMAIL
To: $TO_EMAIL

This is an interactive SMTP test.
.
QUIT
EOF

    # Execute SMTP session
    if command -v telnet &> /dev/null; then
        log_info "Using telnet for interactive test..."
        timeout 20 bash -c "
            (
                sleep 2
                cat /tmp/smtp_session.txt
                sleep 2
            ) | telnet $SMTP_HOST $SMTP_PORT
        " > /tmp/smtp_interactive_response.txt 2>&1
        
        if grep -q "250" /tmp/smtp_interactive_response.txt; then
            log_success "Interactive SMTP test completed"
            return 0
        else
            log_warning "Interactive SMTP test may have issues"
            cat /tmp/smtp_interactive_response.txt
            return 0  # Don't fail for this
        fi
    else
        log_warning "telnet not available, skipping interactive test"
        return 0
    fi
}

# Test SMTP server with swaks (if available)
test_smtp_with_swaks() {
    log_info "Testing SMTP with swaks (if available)..."
    
    if command -v swaks &> /dev/null; then
        log_info "Using swaks for comprehensive SMTP test..."
        
        swaks_output=$(swaks \
            --to "$TO_EMAIL" \
            --from "$FROM_EMAIL" \
            --server "$SMTP_HOST:$SMTP_PORT" \
            --header "Subject: Swaks SMTP Test" \
            --body "This is a test email sent using swaks." \
            --timeout 30 2>&1)
        
        if echo "$swaks_output" | grep -q "250"; then
            log_success "Swaks SMTP test successful"
            echo "$swaks_output" | grep "250"
            return 0
        else
            log_warning "Swaks SMTP test had issues"
            echo "$swaks_output"
            return 0  # Don't fail for this
        fi
    else
        log_info "swaks not available (install with: apt-get install swaks)"
        return 0
    fi
}

# Test SMTP server with multiple recipients
test_smtp_multiple_recipients() {
    log_info "Testing SMTP with multiple recipients..."
    
    local recipients=("user1@$TEST_DOMAIN" "user2@$TEST_DOMAIN" "user3@$TEST_DOMAIN")
    
    {
        sleep 1
        echo "EHLO $TEST_DOMAIN"
        sleep 1
        echo "MAIL FROM:<$FROM_EMAIL>"
        for recipient in "${recipients[@]}"; do
            sleep 1
            echo "RCPT TO:<$recipient>"
        done
        sleep 1
        echo "DATA"
        sleep 1
        echo "From: $FROM_EMAIL"
        echo "To: ${recipients[0]}, ${recipients[1]}, ${recipients[2]}"
        echo "Subject: Multiple Recipients Test"
        echo ""
        echo "This is a test email sent to multiple recipients."
        echo ""
        echo -e "\r\n.\r"
        sleep 1
        echo "QUIT"
    } | timeout 30 nc $SMTP_HOST $SMTP_PORT > /tmp/smtp_multi_response.txt 2>/dev/null
    
    local success_count=$(grep -c "250.*OK" /tmp/smtp_multi_response.txt || echo "0")
    
    if [ "$success_count" -gt 0 ]; then
        log_success "Multiple recipients test completed ($success_count successful responses)"
        return 0
    else
        log_warning "Multiple recipients test had issues"
        cat /tmp/smtp_multi_response.txt
        return 0  # Don't fail for this
    fi
}

# Cleanup function
cleanup() {
    log_info "Cleaning up temporary files..."
    rm -f /tmp/smtp_*.txt /tmp/test_email.txt
}

# Main test function
run_tests() {
    log_info "Starting PrivateMail SMTP server tests..."
    log_info "SMTP Server: $SMTP_HOST:$SMTP_PORT"
    log_info "From Email: $FROM_EMAIL"
    log_info "To Email: $TO_EMAIL"
    log_info "Test Domain: $TEST_DOMAIN"
    echo ""
    
    # Check dependencies
    if ! command -v nc &> /dev/null; then
        log_error "netcat (nc) is required but not installed"
        exit 1
    fi
    
    # Test sequence
    local tests=(
        "test_smtp_connection"
        "test_smtp_banner"
        "test_smtp_ehlo"
        "test_send_email"
        "test_smtp_interactive"
        "test_smtp_with_swaks"
        "test_smtp_multiple_recipients"
    )
    
    local passed=0
    local total=${#tests[@]}
    
    # Set up cleanup trap
    trap cleanup EXIT
    
    for test in "${tests[@]}"; do
        echo ""
        "$test"
    done
    
    echo ""
    log_info "Test Results: $passed/$total tests passed"
    
    if [ $passed -eq $total ]; then
        log_success "All SMTP tests passed!"
        return 0
    elif [ $passed -ge 3 ]; then
        log_success "Core SMTP functionality is working ($passed/$total tests passed)"
        return 0
    else
        log_error "SMTP server tests failed"
        return 1
    fi
}

# Show usage information
show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --host HOST     SMTP server host (default: localhost)"
    echo "  -p, --port PORT     SMTP server port (default: 25)"
    echo "  -f, --from EMAIL    From email address (default: test@example.com)"
    echo "  -t, --to EMAIL      To email address (default: recipient@privatemail.local)"
    echo "  -d, --domain DOMAIN Test domain (default: privatemail.local)"
    echo "  --help              Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  SMTP_HOST           SMTP server host"
    echo "  SMTP_PORT           SMTP server port"
    echo "  FROM_EMAIL          From email address"
    echo "  TO_EMAIL            To email address"
    echo "  TEST_DOMAIN         Test domain"
    echo ""
    echo "Examples:"
    echo "  $0                                    # Test with defaults"
    echo "  $0 -h smtp.example.com -p 587        # Test remote SMTP server"
    echo "  $0 -f sender@test.com -t recv@test.com # Test with custom emails"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--host)
            SMTP_HOST="$2"
            shift 2
            ;;
        -p|--port)
            SMTP_PORT="$2"
            shift 2
            ;;
        -f|--from)
            FROM_EMAIL="$2"
            shift 2
            ;;
        -t|--to)
            TO_EMAIL="$2"
            shift 2
            ;;
        -d|--domain)
            TEST_DOMAIN="$2"
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

# Check if script is being sourced or executed
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    run_tests "$@"
fi