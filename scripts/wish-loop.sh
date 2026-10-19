#!/usr/bin/env bash
# Start the persistent 5-minute wish-processing loop in Claude Code.
# Usage: ./scripts/wish-loop.sh [--dry-run]
set -euo pipefail
ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd -- "$ROOT"

PROMPT="$(cat scripts/wish-loop-prompt.txt)"
if [[ ${1:-} == "--dry-run" ]]; then
    printf '/loop %s\n' "$PROMPT"
    exit 0
fi

if [[ ! -s .local/wishes/agent-token ]]; then
    echo "缺少 .local/wishes/agent-token；请先从服务器配置中取得许愿代理令牌。" >&2
    exit 1
fi

CLAUDE_BIN=${CLAUDE_BIN:-claude}
command -v "$CLAUDE_BIN" >/dev/null || {
    echo "找不到 $CLAUDE_BIN；请安装 Claude Code 或设置 CLAUDE_BIN。" >&2
    exit 1
}

exec "$CLAUDE_BIN" "/loop $PROMPT"
