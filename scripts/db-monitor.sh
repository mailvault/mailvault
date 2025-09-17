#!/bin/bash

# Database Optimization Monitoring Script
# This script helps monitor database performance metrics and connection pool health

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
API_URL="${API_URL:-http://localhost:8080}"
METRICS_URL="${METRICS_URL:-http://localhost:8080/metrics}"
SMTP_METRICS_URL="${SMTP_METRICS_URL:-http://localhost:8080/metrics}"

# Function to print colored output
print_status() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Function to check if a URL is accessible
check_url() {
    local url=$1
    if curl -s --connect-timeout 5 "$url" > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Function to get metric value from Prometheus endpoint
get_metric() {
    local metric_name=$1
    local url=${2:-$METRICS_URL}

    curl -s "$url" 2>/dev/null | grep "^$metric_name" | head -1 | awk '{print $2}' || echo "0"
}

# Function to display database connection pool status
show_pool_status() {
    print_status $BLUE "\n=== Database Connection Pool Status ==="

    local total_conns=$(get_metric "database_total_connections")
    local active_conns=$(get_metric "database_active_connections")
    local idle_conns=$(get_metric "database_idle_connections")
    local waiting_conns=$(get_metric "database_waiting_connections")

    echo "Total Connections:   $total_conns"
    echo "Active Connections:  $active_conns"
    echo "Idle Connections:    $idle_conns"
    echo "Waiting Connections: $waiting_conns"

    # Calculate connection pool utilization
    if [ "$total_conns" -gt 0 ]; then
        local utilization=$(( (active_conns * 100) / total_conns ))
        if [ "$utilization" -gt 80 ]; then
            print_status $RED "⚠️  High pool utilization: ${utilization}%"
        elif [ "$utilization" -gt 60 ]; then
            print_status $YELLOW "⚡ Moderate pool utilization: ${utilization}%"
        else
            print_status $GREEN "✅ Good pool utilization: ${utilization}%"
        fi
    fi
}

# Function to display database performance metrics
show_performance_metrics() {
    print_status $BLUE "\n=== Database Performance Metrics ==="

    local query_rate=$(get_metric "database_queries_total")
    local avg_query_duration=$(get_metric "database_query_duration_seconds")
    local health_checks=$(get_metric "database_health_checks_total")
    local health_failures=$(get_metric "database_health_check_failures_total")

    echo "Total Queries:           $query_rate"
    echo "Avg Query Duration:      ${avg_query_duration}s"
    echo "Health Checks:           $health_checks"
    echo "Health Check Failures:   $health_failures"

    # Health check status
    if [ "$health_failures" -gt 0 ] && [ "$health_checks" -gt 0 ]; then
        local failure_rate=$(( (health_failures * 100) / health_checks ))
        if [ "$failure_rate" -gt 5 ]; then
            print_status $RED "🚨 High health check failure rate: ${failure_rate}%"
        else
            print_status $YELLOW "⚠️  Some health check failures: ${failure_rate}%"
        fi
    else
        print_status $GREEN "✅ All health checks passing"
    fi
}

# Function to display connection lifecycle metrics
show_connection_lifecycle() {
    print_status $BLUE "\n=== Connection Lifecycle ==="

    local created=$(get_metric "database_connections_created_total")
    local destroyed=$(get_metric "database_connections_destroyed_total")
    local failed=$(get_metric "database_connections_failed_total")

    echo "Connections Created:   $created"
    echo "Connections Destroyed: $destroyed"
    echo "Connection Failures:   $failed"

    if [ "$failed" -gt 0 ]; then
        print_status $YELLOW "⚠️  Some connection failures detected"
    else
        print_status $GREEN "✅ No connection failures"
    fi
}

# Function to show top queries by duration
show_slow_queries() {
    print_status $BLUE "\n=== Query Performance by Operation ==="

    echo "Getting query performance metrics..."
    curl -s "$METRICS_URL" 2>/dev/null | grep "database_query_duration_seconds_bucket" | head -10 || echo "No query metrics available"
}

# Function to generate recommendations
generate_recommendations() {
    print_status $BLUE "\n=== Optimization Recommendations ==="

    local total_conns=$(get_metric "database_total_connections")
    local active_conns=$(get_metric "database_active_connections")
    local waiting_conns=$(get_metric "database_waiting_connections")

    if [ "$waiting_conns" -gt 0 ]; then
        print_status $YELLOW "💡 Consider increasing DATABASE_POOL_MAX_SIZE (currently high demand)"
    fi

    if [ "$total_conns" -gt 0 ] && [ "$active_conns" -gt 0 ]; then
        local utilization=$(( (active_conns * 100) / total_conns ))
        if [ "$utilization" -lt 20 ]; then
            print_status $YELLOW "💡 Consider decreasing DATABASE_POOL_MIN_SIZE (low utilization)"
        elif [ "$utilization" -gt 80 ]; then
            print_status $YELLOW "💡 Consider increasing DATABASE_POOL_MAX_SIZE (high utilization)"
        fi
    fi

    print_status $GREEN "✅ For detailed tuning, monitor these metrics over time"
}

# Function to export metrics to JSON
export_metrics() {
    local output_file=${1:-"db_metrics_$(date +%Y%m%d_%H%M%S).json"}

    print_status $BLUE "\n=== Exporting Metrics to $output_file ==="

    cat > "$output_file" << EOF
{
  "timestamp": "$(date -Iseconds)",
  "database_metrics": {
    "total_connections": $(get_metric "database_total_connections"),
    "active_connections": $(get_metric "database_active_connections"),
    "idle_connections": $(get_metric "database_idle_connections"),
    "waiting_connections": $(get_metric "database_waiting_connections"),
    "connections_created": $(get_metric "database_connections_created_total"),
    "connections_destroyed": $(get_metric "database_connections_destroyed_total"),
    "connections_failed": $(get_metric "database_connections_failed_total"),
    "queries_total": $(get_metric "database_queries_total"),
    "health_checks_total": $(get_metric "database_health_checks_total"),
    "health_check_failures": $(get_metric "database_health_check_failures_total")
  }
}
EOF

    print_status $GREEN "✅ Metrics exported to $output_file"
}

# Function to run continuous monitoring
continuous_monitor() {
    local interval=${1:-30}

    print_status $BLUE "=== Starting Continuous Monitoring (${interval}s intervals) ==="
    print_status $YELLOW "Press Ctrl+C to stop monitoring"

    while true; do
        clear
        print_status $GREEN "$(date): Database Monitoring Dashboard"
        show_pool_status
        show_performance_metrics
        sleep $interval
    done
}

# Main function
main() {
    print_status $GREEN "🚀 MailVault Database Performance Monitor"
    print_status $BLUE "Metrics URL: $METRICS_URL"

    # Check if metrics endpoint is accessible
    if ! check_url "$METRICS_URL"; then
        print_status $RED "❌ Cannot access metrics endpoint: $METRICS_URL"
        print_status $YELLOW "Make sure the service is running and metrics are enabled"
        exit 1
    fi

    case "${1:-status}" in
        "status"|"")
            show_pool_status
            show_performance_metrics
            show_connection_lifecycle
            generate_recommendations
            ;;
        "watch")
            continuous_monitor "${2:-30}"
            ;;
        "export")
            export_metrics "$2"
            ;;
        "queries")
            show_slow_queries
            ;;
        "help")
            cat << EOF
Usage: $0 [command] [options]

Commands:
  status          Show current database pool status and metrics (default)
  watch [interval] Start continuous monitoring (default: 30s intervals)
  export [file]   Export metrics to JSON file
  queries         Show query performance metrics
  help            Show this help message

Environment Variables:
  METRICS_URL     Prometheus metrics endpoint (default: http://localhost:8080/metrics)

Examples:
  $0                    # Show current status
  $0 watch              # Start continuous monitoring
  $0 watch 10          # Monitor every 10 seconds
  $0 export report.json # Export metrics to report.json
  $0 queries           # Show query performance
EOF
            ;;
        *)
            print_status $RED "Unknown command: $1"
            print_status $YELLOW "Use '$0 help' for usage information"
            exit 1
            ;;
    esac
}

# Run main function with all arguments
main "$@"