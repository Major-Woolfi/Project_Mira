#!/usr/bin/env bash
set -e

PKG_DIR="$1"
BUILD_DIR="$2"
PYTHON="${3:-python3}"

cd "$PKG_DIR"

PKG_NAME=$(basename "$PKG_DIR")
ARTIFACT_DIR="$BUILD_DIR/$PKG_NAME"
mkdir -p "$ARTIFACT_DIR"

echo "[CYTHON] Installing dependencies..."
if [ -f "requirements.txt" ]; then
    $PYTHON -m pip install -r requirements.txt
fi

$PYTHON -m pip install cython

echo "[CYTHON] Building extensions for $PKG_NAME..."
if [ -f "setup.py" ]; then
    $PYTHON setup.py build_ext --inplace
    $PYTHON setup.py bdist_wheel
else
    echo "[CYTHON] No setup.py found, compiling .pyx files directly..."
    for f in *.pyx; do
        if [ -f "$f" ]; then
            echo "[CYTHON] Compiling $f"
            $PYTHON -m cython "$f"
        fi
    done
fi

if [ -d "dist" ]; then
    cp -f dist/*.whl "$ARTIFACT_DIR/" 2>/dev/null || true
fi

cp -f *.so "$ARTIFACT_DIR/" 2>/dev/null || true

echo "[CYTHON] Copying Go shared library..."
cp -f ../build/engine-memory-go/libmira_memory.so "$ARTIFACT_DIR/" 2>/dev/null || true
cp -f ../build/engine-memory-go/libmira_memory.dylib "$ARTIFACT_DIR/" 2>/dev/null || true
cp -f ../build/engine-memory-go/mira_memory.h "$ARTIFACT_DIR/" 2>/dev/null || true
