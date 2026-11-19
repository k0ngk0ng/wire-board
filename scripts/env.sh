if [ -f "$PWD/.local/business.json" ]; then
  python3 -B "$PWD/scripts/resource_store.py" export-env || return 1
  set -a
  . "$PWD/.local/business.env"
  set +a
fi
export GOCACHE="$PWD/.local/go-build"
export GOMODCACHE="$PWD/.local/go-mod"
export GOTMPDIR="$PWD/.local/tmp"
export TMPDIR="$PWD/.local/tmp"
export npm_config_cache="$PWD/.local/npm"
export TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry"
export GOTOOLCHAIN=local
mkdir -p "$GOCACHE" "$GOMODCACHE" "$GOTMPDIR" "$npm_config_cache" "$TEST_TELEMETRY_DIR"
