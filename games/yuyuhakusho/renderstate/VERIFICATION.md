# Pixel-free renderstate verification

The shared package maintains only bounded logical sprite slots, HUD source refs
and the existing WorldObject cache fields. Its sole nonstandard import is the
independent engine. It performs no IO, pixel decoding/rasterization, ROM access
or client asset loading. Server and client use the same Advance operation;
pixel consumers construct a temporary view and do not persist another Slots/HUD.

The computational bundle is `data/renderstate-metadata.json.gz`: 486,408 JSON
bytes / 111,445 gzip bytes. It contains all 1,500 frame keys, transfer stamps and
stable tile identities, piece counts, 320 default slot bindings, HUD domains and
eight resource bindings. It contains no tile pixels, sprite geometry or palettes.
Only this bundle and the rules payload are computational server data; selected
resource identities are separately described by the root-requested descriptor.

All tests preceded production implementation, with a genuine missing-package
API red and a SHA-bound failure checklist. Before implementation, a separate
producer froze the unchanged runtimebridge over the fourteen existing legacy
render configurations, twenty-eight selected-arena render cases, and seven
fixed legal-pan inputs. All 49 original complete pixel traces/tick counts matched
the existing reports. No original CPU was run or reference state loaded.

The package passes 12,000 comparisons covering all 1,500 actual sprite frames
under cold/warm caches, SkipTiles, culling, wrapping and reused slots. Additional
tests cover the 80-piece/Flags boundary, raw-A signed budget ordering, late
atomic failure, all 20 glyphs/11 portraits/32 bars, the four duplicate-pixel
aliases with exact assignment provenance, invalid metadata/checkpoints, and
complementary rule/presentation digests. Native, race and Node/Wasm pass.

All 49 frozen complete flows pass in native and Node/Wasm, each over 203,428
frames with 1,624 nonzero-pan frames. World/cache, sprite-slot, materialized HUD
and complete pixel trajectories exactly match the old bridge. JSON restore at
tick 500 and nonzero-pan tick 1056 retains the sole persistent owners and exact
subsequent results. Extra complete native race cases cover legacy four-player,
arena-A four-player, and arena-C active pan.

Machine-readable checks, source/data identities and log SHA-256 values are in
`testdata/verification.json`; first-order evidence is `testdata/test-first.json`
and `testdata/bridge-reference-manifest.json`. The frozen bridge-source lineage
is retained separately, so later client refactoring cannot rewrite its origin.

Reproduction from the worktree root (all caches/tmp/telemetry inside `.local`):

```sh
export GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod"
export GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp"
export TEST_TELEMETRY_DIR="$PWD/.local/md-yuyuhakusho/renderstate-reference/telemetry"
export GOTELEMETRY=off
go test ./games/yuyuhakusho/renderstate -count=1 -json \
  -run '^TestRender(Metadata|All1500|Budget|HUD|Digests|ArenaA)'
go test ./games/yuyuhakusho/renderstate -race -count=1 -json \
  -run '^TestRender(Metadata|All1500|Budget|HUD|Digests|ArenaA)'
RENDERSTATE_SHARDS=4 RENDERSTATE_SHARD=0 go test ./games/yuyuhakusho/renderstate \
  -count=1 -json -run '^TestRenderFrozenBridgeTrajectories$'
```

Repeat the flow shard for indices 1..3. For Wasm use the same short regexes and
shards with GOOS=js/GOARCH=wasm and the installed Go go_js_wasm_exec wrapper.
The retained runs use Node 24.15.0 and a minimal environment.

The metadata exporter is `scripts/yuyuhakusho/export_renderstate_metadata.py`.
The old-bridge capture script is `scripts/yuyuhakusho/capture_renderstate_reference.py`;
its output may be regenerated against the bound original bridge source, not
overwritten and called the original after a later implementation change.

Server registration, room input/auth/loading, transport prediction/recovery,
schema-1 research-save policy, client schema-2 integration, packaging and audio
remain separate root-owned acceptance. This package is not that completed game.
