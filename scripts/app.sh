#!/bin/bash
# A simple process manager script for the CPU Monitor application.

# Find the directory where the script is located
# Get the absolute path to the script's directory
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
# Navigate to the project root (one level up)
cd "$SCRIPT_DIR/.." || exit

# Configuration
EXECUTABLE="cpu-monitor"
PID_FILE="/tmp/cpu-monitor.pid"
SOURCE_DIR="./cmd/cpu-monitor"

# Function to check if the process is running
is_running() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p "$PID" > /dev/null; then
            return 0 # Process is running
        fi
    fi
    return 1 # Process is not running
}

# Function to start the application
start() {
    if is_running; then
        echo "CPU Monitor is already running."
        exit 1
    fi

    # Build the executable if it doesn't exist
    if [ ! -f "$EXECUTABLE" ]; then
        echo "Executable not found. Building the application..."
        go build -o "$EXECUTABLE" "$SOURCE_DIR"
        if [ $? -ne 0 ]; then
            echo "Build failed. Please ensure Go is installed and configured correctly."
            exit 1
        fi
    fi

    echo "Starting CPU Monitor..."
    # Run the application in the background and store its PID
    nohup ./"$EXECUTABLE" > /dev/null 2>&1 &
    echo $! > "$PID_FILE"
    echo "CPU Monitor started with PID $(cat "$PID_FILE")."
}

# Function to stop the application
stop() {
    if ! is_running; then
        echo "CPU Monitor is not running."
        exit 1
    fi

    echo "Stopping CPU Monitor..."
    PID=$(cat "$PID_FILE")
    kill "$PID"
    rm "$PID_FILE"
    echo "CPU Monitor stopped."
}

# Function to check the status of the application
status() {
    if is_running; then
        echo "CPU Monitor is running with PID $(cat "$PID_FILE")."
    else
        echo "CPU Monitor is not running."
    fi
}

# Main script logic
case "$1" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    status)
        status
        ;;
    *)
        echo "Usage: $0 {start|stop|status}"
        exit 1
        ;;
esac

exit 0