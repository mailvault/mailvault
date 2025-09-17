#!/bin/bash

# MailVault Prometheus Monitoring Setup Script
# This script sets up the complete monitoring infrastructure for MailVault

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to check if service is running
check_service() {
    local service=$1
    local port=$2

    if curl -s --connect-timeout 5 "http://localhost:$port" > /dev/null 2>&1; then
        print_status $GREEN "✅ $service is running on port $port"
        return 0
    else
        print_status $RED "❌ $service is not accessible on port $port"
        return 1
    fi
}

# Function to wait for service to be ready
wait_for_service() {
    local service=$1
    local port=$2
    local timeout=${3:-60}

    print_status $YELLOW "⏳ Waiting for $service to be ready on port $port..."

    local count=0
    while [ $count -lt $timeout ]; do
        if check_service "$service" "$port"; then
            return 0
        fi
        sleep 2
        count=$((count + 2))
    done

    print_status $RED "❌ Timeout waiting for $service to be ready"
    return 1
}

# Main setup function
main() {
    print_status $BLUE "🚀 Setting up MailVault Prometheus Monitoring Infrastructure"

    # Check prerequisites
    print_status $BLUE "\n📋 Checking prerequisites..."

    if ! command_exists docker; then
        print_status $RED "❌ Docker is not installed. Please install Docker first."
        exit 1
    fi

    if ! command_exists docker-compose; then
        print_status $RED "❌ Docker Compose is not installed. Please install Docker Compose first."
        exit 1
    fi

    print_status $GREEN "✅ Docker and Docker Compose are available"

    # Check if MailVault services are running
    print_status $BLUE "\n🔍 Checking MailVault services..."

    API_RUNNING=false
    SMTP_RUNNING=false

    if check_service "MailVault API" "8080"; then
        API_RUNNING=true
    fi

    if check_service "MailVault SMTP" "8081"; then
        SMTP_RUNNING=true
    fi

    if [ "$API_RUNNING" = false ] && [ "$SMTP_RUNNING" = false ]; then
        print_status $YELLOW "⚠️  MailVault services are not running. You can start monitoring infrastructure and start services later."
        read -p "Continue with monitoring setup? [y/N]: " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi

    # Create network if it doesn't exist
    print_status $BLUE "\n🌐 Setting up Docker network..."
    docker network create mailvault-monitoring 2>/dev/null || print_status $YELLOW "Network mailvault-monitoring already exists"

    # Start monitoring infrastructure
    print_status $BLUE "\n🐳 Starting monitoring infrastructure..."
    docker-compose -f docker-compose.prometheus.yml up -d

    # Wait for services to be ready
    print_status $BLUE "\n⏳ Waiting for services to be ready..."

    wait_for_service "Prometheus" "9090"
    wait_for_service "Grafana" "3000"
    wait_for_service "AlertManager" "9093"
    wait_for_service "Node Exporter" "9100"

    # Check optional services
    if wait_for_service "PostgreSQL Exporter" "9187" 10; then
        print_status $GREEN "✅ PostgreSQL Exporter is ready"
    else
        print_status $YELLOW "⚠️  PostgreSQL Exporter failed to start (check DATABASE_URL configuration)"
    fi

    if wait_for_service "Redis Exporter" "9121" 10; then
        print_status $GREEN "✅ Redis Exporter is ready"
    else
        print_status $YELLOW "⚠️  Redis Exporter failed to start (Redis may not be running)"
    fi

    # Display access information
    print_status $GREEN "\n🎉 Monitoring infrastructure is ready!"

    cat << EOF

📊 Access URLs:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔍 Prometheus (Metrics Database)
   URL: http://localhost:9090
   Status: http://localhost:9090/targets

📈 Grafana (Dashboards)
   URL: http://localhost:3000
   Username: admin
   Password: admin123

🚨 AlertManager (Alert Management)
   URL: http://localhost:9093

📊 Node Exporter (System Metrics)
   URL: http://localhost:9100/metrics

📊 PostgreSQL Exporter (Database Metrics)
   URL: http://localhost:9187/metrics

📊 Redis Exporter (Cache Metrics)
   URL: http://localhost:9121/metrics

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📋 Pre-configured Dashboards:
   • MailVault Overview
   • MailVault Database Metrics

🔔 Alert Rules Configured:
   • API Service Health
   • SMTP Service Health
   • Database Performance
   • Security Events
   • System Resources

EOF

    # Provide next steps
    print_status $BLUE "\n📝 Next Steps:"
    cat << EOF

1. 🚀 Start MailVault Services (if not already running):
   ./build/service    # API service with metrics on :8080/metrics
   ./build/smtpd      # SMTP service with metrics on :8081/metrics

2. 📊 Access Grafana:
   - Navigate to http://localhost:3000
   - Login with admin/admin123
   - View pre-configured MailVault dashboards

3. 🔍 Check Prometheus Targets:
   - Navigate to http://localhost:9090/targets
   - Ensure all targets are "UP"

4. 🚨 Configure AlertManager:
   - Edit ./monitoring/alertmanager/alertmanager.yml
   - Configure email/Slack notification channels
   - Restart: docker-compose -f docker-compose.prometheus.yml restart alertmanager

5. 📈 Monitor Performance:
   - Use ./scripts/db-monitor.sh for database monitoring
   - View metrics in Grafana dashboards
   - Set up custom alerts as needed

EOF

    print_status $GREEN "✅ Monitoring setup complete!"
}

# Function to stop monitoring infrastructure
stop_monitoring() {
    print_status $BLUE "🛑 Stopping monitoring infrastructure..."
    docker-compose -f docker-compose.prometheus.yml down
    print_status $GREEN "✅ Monitoring infrastructure stopped"
}

# Function to restart monitoring infrastructure
restart_monitoring() {
    print_status $BLUE "🔄 Restarting monitoring infrastructure..."
    docker-compose -f docker-compose.prometheus.yml restart
    print_status $GREEN "✅ Monitoring infrastructure restarted"
}

# Function to show status
show_status() {
    print_status $BLUE "📊 Monitoring Infrastructure Status"
    echo

    docker-compose -f docker-compose.prometheus.yml ps

    echo
    print_status $BLUE "🔍 Service Health Checks:"

    check_service "Prometheus" "9090" || true
    check_service "Grafana" "3000" || true
    check_service "AlertManager" "9093" || true
    check_service "Node Exporter" "9100" || true
    check_service "PostgreSQL Exporter" "9187" || true
    check_service "Redis Exporter" "9121" || true
}

# Function to show logs
show_logs() {
    local service=${1:-}
    if [ -n "$service" ]; then
        docker-compose -f docker-compose.prometheus.yml logs -f "$service"
    else
        docker-compose -f docker-compose.prometheus.yml logs -f
    fi
}

# Command line interface
case "${1:-setup}" in
    "setup"|"start")
        main
        ;;
    "stop")
        stop_monitoring
        ;;
    "restart")
        restart_monitoring
        ;;
    "status")
        show_status
        ;;
    "logs")
        show_logs "${2:-}"
        ;;
    "help")
        cat << EOF
MailVault Monitoring Setup Script

Usage: $0 [command]

Commands:
  setup, start    Set up and start monitoring infrastructure (default)
  stop            Stop monitoring infrastructure
  restart         Restart monitoring infrastructure
  status          Show status of monitoring services
  logs [service]  Show logs (optionally for specific service)
  help            Show this help message

Examples:
  $0 setup        # Setup monitoring infrastructure
  $0 status       # Check status of all services
  $0 logs grafana # Show Grafana logs
  $0 stop         # Stop all monitoring services

Services:
  prometheus, grafana, alertmanager, node-exporter,
  postgres-exporter, redis-exporter
EOF
        ;;
    *)
        print_status $RED "Unknown command: $1"
        print_status $YELLOW "Use '$0 help' for usage information"
        exit 1
        ;;
esac