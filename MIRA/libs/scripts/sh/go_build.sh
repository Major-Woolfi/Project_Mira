#!/usr/bin/env bash
set -e

PKG_DIR="$1"
BUILD_DIR="$2"
GO_PATH="${3:-/usr/local/go/bin}"

export PATH="$GO_PATH:$PATH"

cd "$PKG_DIR"

PKG_NAME=$(basename "$PKG_DIR")
ARTIFACT_DIR="$BUILD_DIR/$PKG_NAME"
mkdir -p "$ARTIFACT_DIR"

echo "[GO] Tidying modules..."
go mod tidy

echo "[GO] Building $PKG_NAME..."
go build ./...

echo "[GO] Checking for main packages..."
MAIN_FOUND=0

if [ -f "main.go" ]; then
    echo "[GO] Found main.go in root"
    go build -o "$ARTIFACT_DIR/$PKG_NAME" .
    MAIN_FOUND=1
fi

for d in cmd/*/; do
    if [ -f "${d}main.go" ]; then
        CMD_NAME=$(basename "$d")
        echo "[GO] Found main.go in $d"
        go build -o "$ARTIFACT_DIR/$CMD_NAME" "$d"
        MAIN_FOUND=1
    fi
done

if [ "$MAIN_FOUND" -eq 0 ]; then
    echo "[GO] No main package found, library only"
fi

if [ -f "go.mod" ]; then
    cp go.mod "$ARTIFACT_DIR/"
    [ -f go.sum ] && cp go.sum "$ARTIFACT_DIR/"
fi
