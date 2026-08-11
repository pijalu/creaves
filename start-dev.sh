#!/bin/bash

# Start script for running both Creaves app and Consolidation processor in development
# Usage: ./start-dev.sh [app|consolidation|both]
# Default: both

set -e

MODE="${1:-both}"
APP_PORT="${APP_PORT:-3000}"
CONSOLIDATION_PORT="${CONSOLIDATION_PORT:-3001}"
APP_ENV="${GO_ENV:-development}"

echo "=========================================="
echo "Creaves Development Environment Starter"
echo "=========================================="
echo "Mode: $MODE"
echo "App Port: $APP_PORT"
echo "Consolidation Port: $CONSOLIDATION_PORT"
echo "Environment: $APP_ENV"
echo "=========================================="
echo ""

# Function to cleanup processes on exit
cleanup() {
    echo ""
    echo "Shutting down services..."
    if [ -n "$APP_PID" ]; then
        echo "Stopping Creaves app (PID: $APP_PID)..."
        kill $APP_PID 2>/dev/null || true
    fi
    if [ -n "$CONSOLIDATION_PID" ]; then
        echo "Stopping Consolidation processor (PID: $CONSOLIDATION_PID)..."
        kill $CONSOLIDATION_PID 2>/dev/null || true
    fi
    echo "All services stopped."
    exit 0
}

# Set trap to cleanup on exit
trap cleanup SIGINT SIGTERM EXIT

# Check if dependencies are installed
check_dependencies() {
    echo "Checking dependencies..."
    
    if ! command -v buffalo &> /dev/null; then
        echo "ERROR: buffalo CLI not found. Please install it first:"
        echo "  go install github.com/gobuffalo/cli/cmd/buffalo@latest"
        exit 1
    fi
    
    if ! command -v go &> /dev/null; then
        echo "ERROR: Go not found. Please install Go 1.18+"
        exit 1
    fi
    
    echo "Dependencies OK"
    echo ""
}

# Start the main Creaves app
start_app() {
    echo "Starting Creaves app on port $APP_PORT..."
    
    # Set environment variables
    export GO_ENV="$APP_ENV"
    export PORT="$APP_PORT"
    
    # Start buffalo dev in background
    buffalo dev &
    APP_PID=$!
    
    echo "Creaves app started with PID: $APP_PID"
    echo "App URL: http://127.0.0.1:$APP_PORT"
    echo ""
}

# Start the consolidation processor
start_consolidation() {
    echo "Starting Consolidation processor on port $CONSOLIDATION_PORT..."
    
    # Set environment variables
    export GO_ENV="$APP_ENV"
    export PORT="$CONSOLIDATION_PORT"
    
    # Build the consolidation app first
    echo "Building consolidation app..."
    go build -o /tmp/consolidation ./cmd/consolidation
    if [ $? -ne 0 ]; then
        echo "ERROR: Failed to build consolidation app"
        exit 1
    fi
    
    # Run the compiled binary
    /tmp/consolidation &
    CONSOLIDATION_PID=$!
    
    echo "Consolidation processor started with PID: $CONSOLIDATION_PID"
    echo "Consolidation URL: http://127.0.0.1:$CONSOLIDATION_PORT"
    echo ""
}

# Wait for services to be ready
wait_for_services() {
    echo "Waiting for services to start..."
    
    if [ "$MODE" = "app" ] || [ "$MODE" = "both" ]; then
        echo -n "Waiting for Creaves app..."
        for i in {1..30}; do
            if curl -s http://127.0.0.1:$APP_PORT/health > /dev/null 2>&1 || curl -s http://127.0.0.1:$APP_PORT/ > /dev/null 2>&1; then
                echo " OK"
                break
            fi
            echo -n "."
            sleep 1
        done
        echo ""
    fi
    
    if [ "$MODE" = "consolidation" ] || [ "$MODE" = "both" ]; then
        echo -n "Waiting for Consolidation processor..."
        for i in {1..30}; do
            if curl -s http://127.0.0.1:$CONSOLIDATION_PORT/health > /dev/null 2>&1 || curl -s http://127.0.0.1:$CONSOLIDATION_PORT/ > /dev/null 2>&1; then
                echo " OK"
                break
            fi
            echo -n "."
            sleep 1
        done
        echo ""
    fi
    
    echo ""
    echo "=========================================="
    echo "All services are running!"
    echo "=========================================="
    
    if [ "$MODE" = "app" ] || [ "$MODE" = "both" ]; then
        echo "Creaves App:       http://127.0.0.1:$APP_PORT"
    fi
    
    if [ "$MODE" = "consolidation" ] || [ "$MODE" = "both" ]; then
        echo "Consolidation:     http://127.0.0.1:$CONSOLIDATION_PORT"
    fi
    
    echo ""
    echo "Press Ctrl+C to stop all services"
    echo "=========================================="
    echo ""
}

# Main execution
main() {
    check_dependencies
    
    case "$MODE" in
        app)
            start_app
            ;;
        consolidation)
            start_consolidation
            ;;
        both)
            start_app
            sleep 2  # Give app a head start
            start_consolidation
            ;;
        *)
            echo "Usage: $0 [app|consolidation|both]"
            echo ""
            echo "Options:"
            echo "  app            - Start only the Creaves app"
            echo "  consolidation  - Start only the consolidation processor"
            echo "  both           - Start both services (default)"
            echo ""
            echo "Environment Variables:"
            echo "  APP_PORT           - Port for Creaves app (default: 3000)"
            echo "  CONSOLIDATION_PORT - Port for consolidation (default: 3001)"
            echo "  GO_ENV             - Environment (default: development)"
            exit 1
            ;;
    esac
    
    wait_for_services
    
    # Keep script running
    wait
}

main
