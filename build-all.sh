#!/bin/bash

# Set up common variables
NAME="G-itemViewer"
PROJECT_PATH=$(pwd)
BIN_DIR="${PROJECT_PATH}/bin"
DIST_DIR="${PROJECT_PATH}/dist"
VERSION=$(git describe --tags --always --dirty)

# Create necessary directories
mkdir -p "$BIN_DIR"
mkdir -p "$DIST_DIR/windows"
mkdir -p "$DIST_DIR/linux"
mkdir -p "$DIST_DIR/macos"

# Build for Windows
echo "Building for Windows..."
GOOS=windows GOARCH=amd64 go build -o "${BIN_DIR}/${NAME}-win.exe" -ldflags="-X main.Version=${VERSION}" .
cp "${BIN_DIR}/${NAME}-win.exe" "$DIST_DIR/windows/${NAME}.exe"

# Build for Linux
echo "Building for Linux..."
GOOS=linux GOARCH=amd64 go build -o "${BIN_DIR}/${NAME}-linux" -ldflags="-X main.Version=${VERSION}" .
cp "${BIN_DIR}/${NAME}-linux" "$DIST_DIR/linux/${NAME}"

# Build for macOS
echo "Building for macOS..."
export PATH=/mnt/c/osxcross/target/bin:$PATH
export CC=o64-clang
export CXX=o64-clang++
export CGO_ENABLED=1
export GOOS=darwin
export GOARCH=amd64  # or arm64 for Apple Silicon
export MACOSX_DEPLOYMENT_TARGET=10.14

go build -o "${BIN_DIR}/${NAME}-darwin" -ldflags="-X main.Version=${VERSION}" .
cp "${BIN_DIR}/${NAME}-darwin" "$DIST_DIR/macos/${NAME}"

# Optionally, create ZIP files for distribution
echo "Creating ZIP files for distribution..."
zip -r "${DIST_DIR}/windows/${NAME}_${VERSION}_windows.zip" "$DIST_DIR/windows/${NAME}.exe"
zip -r "${DIST_DIR}/linux/${NAME}_${VERSION}_linux.zip" "$DIST_DIR/linux/${NAME}"
zip -r "${DIST_DIR}/macos/${NAME}_${VERSION}_macos.zip" "$DIST_DIR/macos/${NAME}"

echo "Build and packaging complete."
