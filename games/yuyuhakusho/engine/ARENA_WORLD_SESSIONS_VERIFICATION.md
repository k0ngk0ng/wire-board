# Arena world/session integration verification

The independent world now dispatches arenas `2/4/6/8/a/c/e`, with the original
default arena unchanged. Sessions select an even arena ID through
`ChuSessionOptions.Arena`; zero remains the default. Setup runs the ordinary
match manager and never imports a captured world.

`ChuSessionResourceForArena(id, library)` returns the selected resource list and
16 CRAM palette words. New arenas complete readiness through
`CompleteChuSessionResource(state, resource, library)`. The historical
two-argument default completion and `DefaultChuSessionResource` remain valid.
Invalid IDs, unavailable rules/resources, wrong palettes/sources, duplicates,
and incorrect manager stages fail atomically.

The schema-3 rules bundle has an optional paired `arena_scenes` /
`arena_resources` extension. Old schemas 1/2/3 without that extension still
decode and enable the default arena. Present extensions must be complete and
agree with the recovered MatchStart bindings. Wave values share the existing
immutable waveform; default tails share slices of the single full linear-row
table after validation. No ROM image, executable bytes, CPU snapshots, VRAM,
textures, or other client media enter the rule bundle.

Scene scratch remains in `WorldObject`. Rows are views of `Cells`, pan rows
remain views of `Stream`, and only the three arena-4 primary palette words are
merged. Scene C reads the low byte of `Clock.LogicFrame`. `BackdropColor` owns
the VDP register-7 byte and is omitted when zero. The separate serialization
owner handles the existing explicit JSON projection.

Arena A creates/updates the actual `9e00` child. Each child callback emits six
ordered `ChuWorldEvent{Kind: "arena-sprite", RawArenaSprite: ...}` operations.
Every event preserves its immediate descriptor; no SAT, physical sprite count,
or output cursor is stored in the world.

## Evidence

`testdata/arena-world-test-first.json` binds the failure checklist, tests,
pre-edit production sources, CPU captures, and genuine missing-callback and
missing-symbol red logs. The existing strict rule-bundle comparison also
failed before exporting the extended bundle.

The capture producer replays each existing unique observer-free 3602-frame
control in a fresh hook-core process. Every RAM/video frame hash agrees with
that original control. It creates no new original control, performs no RAM
writes or private calls, and starts collection at the real practice-ready
boundary at frame 2700. Each arena has 901 complete whole-object passes through
frame 3600; two additional video frames drain pending execution.

The seven fixtures contain 6,307 passes and 61,461 ordered callbacks. Arena A
adds 5,406 immediate raw descriptors. The same fixtures pass per-pass and
single-initial continuous comparisons, including save/restore, every owned
world field/byte, actor/collision/HUD state, camera, palette, rows, and Stream.

Existing complete-CPU boundary fixtures also pass through the dispatcher:
364 cases each for arenas 2/6/8, 1,359 for arena 4, 1,551 each for C/E, and
2,448 A-parent plus 1,128 A-child cases. These cover pause/slow, dirty storage,
feedback, Stream gaps, and scene-C frame parity. CPU rows already marked
`reference_only` because they read beyond the supplied row storage must fail
atomically. The initial integration adapter omitted this existing negative
classification; its corrected assertion and retained failure log have explicit
SHA lineage in the test-first manifest. Production boundaries were not widened.

Native, Node/Wasm, and native `-race` runs pass. Default resource/startup/input
and source-0/source-4 schema compatibility checks pass. Machine-readable results
and source/artifact digests are in `testdata/arena-world-verification.json`.
The root task owns the final historical 280-test regression and product/runtime
acceptance; this report does not claim that broader delivery.

## Reproduction

Run from the worktree root with caches and temporary files inside the worktree:

```sh
export GOCACHE="$PWD/.local/go-cache"
export GOMODCACHE="$PWD/.local/go-mod"
export GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp"
export TMPDIR="$PWD/.local/md-yuyuhakusho/tmp"
export GOTELEMETRY=off

go test ./games/yuyuhakusho/engine -count=1 -json \
  -run '^(TestArenaWorldRules|TestArenaSession|TestArenaWorldGating|TestChuWorldRuleBundle)'
go test ./games/yuyuhakusho/engine -count=1 -json -run '^TestArenaWorldCPU'
go test ./games/yuyuhakusho/engine -count=1 -json \
  -run '^TestArenaWorld(OriginalWholePasses|SingleInitialContinuous)$'
go test ./games/yuyuhakusho/engine -race -count=1 -json \
  -run '^(TestArenaWorld|TestArenaSession)'
```

For Wasm, use the same three short regex groups with `GOOS=js GOARCH=wasm`
and the installed Go `lib/wasm/go_js_wasm_exec` wrapper. The retained Wasm runs
use a minimal environment and Node 24.15.0; their exact command groups are in
the verification JSON.

With the pinned research-only core and bridge already available, recapture an
arena against its existing unique control without replacing that control:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -B \
  scripts/yuyuhakusho/capture_arena_world_sessions.py \
  --arena 2 --out .local/md-yuyuhakusho/arena-world-recapture
```

The producer accepts `2/4/6/8/a/c/e`. Compare the resulting fixture SHA-256 to
the retained arena manifest. Export rules using
`python3 -B scripts/yuyuhakusho/export_chu_world_rules.py`.
