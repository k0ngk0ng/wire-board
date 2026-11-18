#!/bin/sh
# Source from repository root; all build caches remain inside this workspace.
export GOCACHE="$PWD/.local/go-build"
export GOMODCACHE="$PWD/.local/go-mod"
export GOTMPDIR="$PWD/.local/tmp"
export TMPDIR="$PWD/.local/tmp"
export npm_config_cache="$PWD/.local/npm"
export TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry"
export GOTOOLCHAIN=local
mkdir -p "$GOCACHE" "$GOMODCACHE" "$GOTMPDIR" "$npm_config_cache" "$TEST_TELEMETRY_DIR"
