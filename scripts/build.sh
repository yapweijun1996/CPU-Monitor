#!/bin/bash
# This script builds the CPU Monitor application for macOS.

# Get the absolute path to the script's directory
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
# Navigate to the project root (one level up)
PROJECT_ROOT="$SCRIPT_DIR/.."
cd "$PROJECT_ROOT" || exit

# Configuration
EXECUTABLE_NAME="cpu-monitor"
SOURCE_DIR="./cmd/cpu-monitor"
DIST_DIR="./dist"
APP_NAME="CPU Monitor.app"
APP_DIR="$DIST_DIR/$APP_NAME"

echo "Cleaning up previous build..."
rm -rf "$APP_DIR"

echo "Creating .app bundle structure..."
mkdir -p "$APP_DIR/Contents/MacOS"

echo "Building the application for macOS..."
go build -mod=vendor -o "$APP_DIR/Contents/MacOS/$EXECUTABLE_NAME" "$SOURCE_DIR"

if [ $? -ne 0 ]; then
    echo "Build failed. Please ensure Go is installed and configured correctly."
    exit 1
fi

echo "Copying Info.plist..."
cp "assets/macos/Info.plist" "$APP_DIR/Contents/Info.plist"

echo "Build successful!"
echo "The application bundle is located at: $APP_DIR"