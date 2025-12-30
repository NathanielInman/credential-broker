#!/bin/bash
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Build output directory
BUILD_DIR="./built"

echo -e "${YELLOW}Starting credential-broker build...${NC}"

# Clean previous builds
if [ -d "$BUILD_DIR" ]; then
  echo "Cleaning previous build directory..."
  rm -rf "$BUILD_DIR"
fi

mkdir -p "$BUILD_DIR"

# Run tests before building
echo -e "${YELLOW}Running tests...${NC}"
if ! pnpm test; then
  echo -e "${RED}Tests failed! Aborting build.${NC}"
  exit 1
fi
echo -e "${GREEN}Tests passed!${NC}"

# Run linter before building
echo -e "${YELLOW}Running linter...${NC}"
if ! pnpm lint; then
  echo -e "${RED}Linting failed! Aborting build.${NC}"
  exit 1
fi
echo -e "${GREEN}Linting passed!${NC}"

# Build for each platform
echo -e "${YELLOW}Building binaries...${NC}"

# macOS x64
echo "Building for macOS x64..."
if nexe ./index.js -t mac-x64-20.18.0 -o "$BUILD_DIR/broker-mac-x64"; then
  echo -e "${GREEN}✓ macOS x64 build complete${NC}"
else
  echo -e "${RED}✗ macOS x64 build failed${NC}"
  exit 1
fi

# macOS ARM64 (Apple Silicon)
echo "Building for macOS ARM64..."
if nexe ./index.js -t mac-arm64-20.18.0 -o "$BUILD_DIR/broker-mac-arm64"; then
  echo -e "${GREEN}✓ macOS ARM64 build complete${NC}"
else
  echo -e "${RED}✗ macOS ARM64 build failed${NC}"
  exit 1
fi

# Linux x64
echo "Building for Linux x64..."
if nexe ./index.js -t linux-x64-20.18.0 -o "$BUILD_DIR/broker-linux-x64"; then
  echo -e "${GREEN}✓ Linux x64 build complete${NC}"
else
  echo -e "${RED}✗ Linux x64 build failed${NC}"
  exit 1
fi

# Linux ARM64
echo "Building for Linux ARM64..."
if nexe ./index.js -t linux-arm64-20.18.0 -o "$BUILD_DIR/broker-linux-arm64"; then
  echo -e "${GREEN}✓ Linux ARM64 build complete${NC}"
else
  echo -e "${RED}✗ Linux ARM64 build failed${NC}"
  exit 1
fi

# Windows x64
echo "Building for Windows x64..."
if nexe ./index.js -t windows-x64-20.18.0 -o "$BUILD_DIR/broker-windows-x64.exe"; then
  echo -e "${GREEN}✓ Windows x64 build complete${NC}"
else
  echo -e "${RED}✗ Windows x64 build failed${NC}"
  exit 1
fi

# Verification step - check all binaries exist
echo -e "${YELLOW}Verifying build outputs...${NC}"
EXPECTED_FILES=(
  "$BUILD_DIR/broker-mac-x64"
  "$BUILD_DIR/broker-mac-arm64"
  "$BUILD_DIR/broker-linux-x64"
  "$BUILD_DIR/broker-linux-arm64"
  "$BUILD_DIR/broker-windows-x64.exe"
)

ALL_PRESENT=true
for file in "${EXPECTED_FILES[@]}"; do
  if [ -f "$file" ]; then
    SIZE=$(du -h "$file" | cut -f1)
    echo -e "${GREEN}✓ $file ($SIZE)${NC}"
  else
    echo -e "${RED}✗ Missing: $file${NC}"
    ALL_PRESENT=false
  fi
done

if [ "$ALL_PRESENT" = false ]; then
  echo -e "${RED}Build verification failed! Some binaries are missing.${NC}"
  exit 1
fi

echo ""
echo -e "${GREEN}Build complete! Binaries are in $BUILD_DIR/${NC}"
echo ""
echo "Built files:"
ls -lh "$BUILD_DIR"
