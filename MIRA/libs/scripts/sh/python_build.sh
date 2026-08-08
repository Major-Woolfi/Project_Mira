#!/usr/bin/env bash
set -e

PKG_DIR="$1"
BUILD_DIR="$2"
PYTHON="${3:-python3}"

cd "$PKG_DIR"

PKG_NAME=$(basename "$PKG_DIR")
ARTIFACT_DIR="$BUILD_DIR/$PKG_NAME"
mkdir -p "$ARTIFACT_DIR"

echo "[PYTHON] Installing dependencies..."
if [ -f "requirements.txt" ]; then
    $PYTHON -m pip install -r requirements.txt
fi

echo "[PYTHON] Building $PKG_NAME..."
if [ -f "pyproject.toml" ]; then
    $PYTHON -m build --wheel
elif [ -f "setup.py" ]; then
    $PYTHON setup.py bdist_wheel
else
    echo "[PYTHON] No build system found, copying source..."
    mkdir -p "$ARTIFACT_DIR/src"
    cp -f *.py "$ARTIFACT_DIR/src/" 2>/dev/null || true
    exit 0
fi

if [ -d "dist" ]; then
    cp -f dist/*.whl "$ARTIFACT_DIR/" 2>/dev/null || true
fi
