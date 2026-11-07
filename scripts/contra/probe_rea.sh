#!/bin/sh
# Bounded REA/Ghidra probe. Run probe_roms.py first from the repository root.
set -eu
task_root=$(pwd)
task_out="$task_root/.local/nes-contra-research"
test -f "$task_out/contra-j-reset-f80d.bin"
mkdir -p "$task_root/dist/contra-research-tmp" "$task_out/swift-cache"
# Ghidra rejects dot-prefixed project paths; keep its temporary root under dist.
export TMPDIR="$task_root/dist/contra-research-tmp"
export REA_DARWIN_MODULE_CACHE_PATH="$task_out/swift-cache"
export GHIDRA_INSTALL_DIR="${GHIDRA_INSTALL_DIR:-$task_root/.local/nes-basket/tools/ghidra_12.1.4_PUBLIC}"
export JAVA_HOME="${JAVA_HOME:-/opt/homebrew/opt/openjdk@26/libexec/openjdk.jdk/Contents/Home}"
export REA_LOG_LEVEL=silent
rea function "$task_out/contra-j-reset-f80d.bin" 0xf80d \
  --target-format raw-binary --architecture 6502 --entry-point 0xf80d \
  --provider ghidra --format json > "$task_out/rea-reset-function.json"
python3 -B - <<'PY'
import json
from pathlib import Path
p = Path('.local/nes-contra-research/rea-reset-function.json')
data = json.loads(p.read_text())
assert 'evidence_id' in data, data
assert data['subject']['digest']['sha256'] == 'e35c3fe1bc1ca5fc624c7764e06bb619d2ae54ccbc30336633c0a2892c32368e'
print(json.dumps({'evidence_id': data['evidence_id'], 'output': str(p)}))
PY
