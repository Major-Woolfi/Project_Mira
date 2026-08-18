#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIBS_DIR="$(dirname "$SCRIPT_DIR")"
SRC_DIR="$LIBS_DIR/src"
BUILD_DIR="$LIBS_DIR/build"

if [ ! -d "$BUILD_DIR" ]; then
    mkdir -p "$BUILD_DIR"
fi

GO_PATH="${GO_PATH:-/usr/local/go/bin}"
PYTHON="${PYTHON:-python3}"

echo "Scanning \"$SRC_DIR\" for packages..."

for pkg_dir in "$SRC_DIR"/*/; do
    if [ ! -d "$pkg_dir" ]; then
        continue
    fi

    PKG_NAME=$(basename "$pkg_dir")
    if [ -f "$pkg_dir/go.mod" ]; then
        echo
        echo "========================================"
        echo "Processing Go: $PKG_NAME"
        echo "========================================"
        "$SCRIPT_DIR/go_build.sh" "$pkg_dir" "$BUILD_DIR" "$GO_PATH"
    fi
done

for pkg_dir in "$SRC_DIR"/*/; do
    if [ ! -d "$pkg_dir" ]; then
        continue
    fi

    PKG_NAME=$(basename "$pkg_dir")
    if [ -f "$pkg_dir/setup.py" ]; then
        echo
        echo "========================================"
        echo "Processing Cython: $PKG_NAME"
        echo "========================================"
        "$SCRIPT_DIR/cython_build.sh" "$pkg_dir" "$BUILD_DIR" "$PYTHON"
    elif [ -f "$pkg_dir/pyproject.toml" ]; then
        echo
        echo "========================================"
        echo "Processing Python: $PKG_NAME"
        echo "========================================"
        "$SCRIPT_DIR/python_build.sh" "$pkg_dir" "$BUILD_DIR" "$PYTHON"
    fi
done

echo
echo "========================================"
echo "Build complete. Artifacts in \"$BUILD_DIR\""
echo "========================================"
