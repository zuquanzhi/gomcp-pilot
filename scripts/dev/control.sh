#!/bin/bash

# gomcp-dev: Control script for GoMCP-Pilot development environment

GOMCP_BIN="./bin/gomcp"
WEB_DIR="./web"
PID_FILE=".gomcp-dev.pids"
LOG_DIR="logs"

function build() {
    echo "Building gomcp..."
    go build -o $GOMCP_BIN ./cmd/gomcp
}

function start() {
    build
    mkdir -p $LOG_DIR

    echo "Starting Backend (gomcp serve)..."
    nohup $GOMCP_BIN serve > $LOG_DIR/backend.log 2>&1 &
    BACKEND_PID=$!
    echo "Backend PID: $BACKEND_PID"

    echo "Starting Frontend (npm run dev)..."
    cd $WEB_DIR
    nohup npm run dev > ../$LOG_DIR/frontend.log 2>&1 &
    FRONTEND_PID=$!
    cd ..
    echo "Frontend PID: $FRONTEND_PID"

    echo "$BACKEND_PID $FRONTEND_PID" > $PID_FILE
    echo "Services started. Logs in $LOG_DIR/"
    echo "Backend: http://localhost:8080"
    echo "Frontend: http://localhost:5173"
}

function stop() {
    if [ -f $PID_FILE ]; then
        PIDS=$(cat $PID_FILE)
        echo "Stopping services (PIDs: $PIDS)..."
        kill $PIDS 2>/dev/null
        rm $PID_FILE
        echo "Services stopped."
    else
        echo "No PID file found. Are services running?"
    fi
}

function restart() {
    stop
    sleep 1
    start
}

function logs() {
    tail -f $LOG_DIR/*.log
}

function tui() {
    # Check if backend is running, if so, warn user
    if [ -f $PID_FILE ]; then
        echo "Warning: Backend service is running in background."
        echo "Stopping background backend to free port 8080..."
        stop
    fi
    build
    echo "Starting TUI..."
    $GOMCP_BIN start
}

case "$1" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    logs)
        logs
        ;;
    tui)
        tui
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|logs|tui}"
        exit 1
        ;;
esac
