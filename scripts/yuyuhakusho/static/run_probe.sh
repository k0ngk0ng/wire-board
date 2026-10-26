#!/bin/sh
set -eu
WORKTREE=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
MD_STATIC_ROOT="$WORKTREE/.local/md-yuyuhakusho/static"
GHIDRA_INSTALL=${YU_GHIDRA_INSTALL:-/Users/jason/Sources/github.com/k0ngk0ng/wire-board/.local/nes-basket/tools/ghidra_12.1.4_PUBLIC}
JAVA_BIN=${YU_JAVA_BIN:-/opt/homebrew/Cellar/openjdk/26.0.1/libexec/openjdk.jdk/Contents/Home/bin}
mkdir -p "$MD_STATIC_ROOT/tmp/java-home" "$MD_STATIC_ROOT/tmp/config" "$MD_STATIC_ROOT/tmp/cache" "$WORKTREE/docs/research/md-yuyuhakusho/static-work" "$WORKTREE/docs/research/md-yuyuhakusho/static"
export JAVA_TOOL_OPTIONS="-Duser.home=$MD_STATIC_ROOT/tmp/java-home -Djava.io.tmpdir=$MD_STATIC_ROOT/tmp -Dapplication.settingsdir=$MD_STATIC_ROOT/tmp/config -Dapplication.cachedir=$MD_STATIC_ROOT/tmp/cache -Dapplication.tempdir=$MD_STATIC_ROOT/tmp"
export PATH="$JAVA_BIN:$PATH"
OUTPUT_NAME=$1
shift
"$GHIDRA_INSTALL/support/analyzeHeadless" "$WORKTREE/docs/research/md-yuyuhakusho/static-work" md-control \
  -import "$WORKTREE/.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin" \
  -loader BinaryLoader -loader-baseAddr 0 -processor 68000:BE:32:default -noanalysis \
  -scriptPath "$WORKTREE/scripts/yuyuhakusho/static" \
  -postScript MdControlProbe.java "$WORKTREE/docs/research/md-yuyuhakusho/static/$OUTPUT_NAME" "$WORKTREE" "$@" \
  -deleteProject -log "$MD_STATIC_ROOT/ghidra.log" -scriptlog "$MD_STATIC_ROOT/ghidra-script.log" \
  > "$MD_STATIC_ROOT/console.log" 2>&1
