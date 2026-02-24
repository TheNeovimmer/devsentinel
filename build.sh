#!/bin/bash
# Build script for multiple platforms

VERSION="1.0.0"
APP_NAME="devsentinel"

echo "Building $APP_NAME v$VERSION for multiple platforms..."

# Clean
rm -rf dist/

# Build for each platform
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}
    OUTPUT="dist/${APP_NAME}-${GOOS}-${GOARCH}"
    
    if [ "$GOOS" = "windows" ]; then
        OUTPUT="${OUTPUT}.exe"
    fi
    
    echo "Building for $GOOS/$GOARCH..."
    GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" -o "$OUTPUT" ./cmd/main.go
    
    # Compress
    if [ "$GOOS" = "linux" ] || [ "$GOOS" = "darwin" ]; then
        tar -czvf "${OUTPUT}.tar.gz" -C dist/ $(basename $OUTPUT)
        rm "$OUTPUT"
    fi
done

echo "Build complete! Files in dist/"
ls -la dist/
