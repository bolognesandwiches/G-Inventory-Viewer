#!/bin/bash

# Set up environment variables for osxcross
export PATH=/mnt/c/osxcross/target/bin:$PATH
export CC=o64-clang
export CXX=o64-clang++
export CGO_ENABLED=1
export GOOS=darwin
export GOARCH=amd64  # or arm64 for Apple Silicon

# Define variables
NAME="G-itemViewer"
PROJECT_PATH=$(pwd)
BIN_DIR="${PROJECT_PATH}/bin"
DIST_DIR="${PROJECT_PATH}/dist/macos"
VERSION=$(git describe --tags --always --dirty)

# Ensure we're in the correct directory
cd "$PROJECT_PATH"

# Create bin directory if it doesn't exist
mkdir -p "$BIN_DIR"

echo "Building for macOS..."

# Build the executable
go build -o "${BIN_DIR}/${NAME}-darwin" -ldflags="-X main.Version=${VERSION}" .

echo "Build complete."

# Create distribution folder
mkdir -p "$DIST_DIR"

# Copy the executable
cp "${BIN_DIR}/${NAME}-darwin" "$DIST_DIR/${NAME}"

# Optionally, create a ZIP file for distribution
ZIP_PATH="${DIST_DIR}/${NAME}_${VERSION}_macos.zip"
zip -r "$ZIP_PATH" "$DIST_DIR/${NAME}"

echo "Created ZIP file for distribution: $ZIP_PATH"
