# Runtime reward and ending display integration

The runtime now owns committed post-result display memory and OAM alongside the
existing source-derived video scheduler. `Display.Visible.postResult` transports
only committed surface, an explicitly canonical raster plan, and visible OAM.
The existing Canvas renderer can consume the same snapshot during local play,
authoritative playback, reconnect and spectator replay.

The failure inventory was added in `runtime_presentation_display_test.go` before
implementation. The first run failed on the missing integration APIs. A separate
corrupt-restore test failed before display validation was added.

## State and source boundaries

- The earned match outcome stays immutable. Reward inventory is awarded by the
  original reward transaction once; final campaign continuation uses post-animation
  RNG. Existing tests run earned home victory with scores 24:23 through video
  frame 22752, including the result fade and reward handoff.
- `Presentation.Display` owns committed nametables, palette, working/visible OAM
  and CHR/NMI state. Pending resources and uploads remain exclusively in `Video`.
  Display effects are applied once in source order; actor submission does not
  replace visible OAM until the normal NMI/DMA effect.
- `Display.Visible.postResult` is a derived persisted render snapshot. Restore
  validation rejects missing display ownership, invalid palette/mirroring/CHR,
  and a render snapshot that differs from its committed display owner.
- Existing confirmed saves without `Presentation` remain parked. They are not
  retroactively awarded or animated. An initial video boundary with zero video
  ticks may construct its display; a progressed presentation cannot silently
  replace missing display history.
- No production code reads source RAM, executes a CPU, or loads a captured
  timeline. The static tables are decoded from the pinned original ROM. The seed
  retains the runtime's actual court-column and animation-patch history, overlays
  original result presets and score tiles, and starts at the committed black fade.
- Unavailable inactive OAM bytes are explicitly canonical: zero payload with Y248
  hidden slots. These bytes are not represented as recovered original history.

## Timing policy

`runtime-v1` is **canonical visual timing**, not source-observed pixel timing.
NMI handler effects precede the first visible fetch; the first IRQ starts on its
latch scanline, and each rearmed IRQ follows its source reload completion plus
latch+1 scanlines. Handler-relative source cycles are three picture dots each.
The NMI scene/phase selects the frame's chain; each IRQ samples live post-dispatch
credits/offsets, rather than freezing the NMI's old actor offsets for the frame.

Resource effects retain their source scheduling. A fully black palette or
render-disabled resource boundary yields a blank frame, including the reward's
Scene2 handoff and the ending's reset. No unsupported destination is passed to
an earlier scene's raster program. This declared canonical sampling does not
claim arbitrary original IRQ/mainline instruction interleaving or new exact-pixel
coverage beyond the independently verified source-observed row projector.

## Reproduction

From the basketball worktree:

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/generate_postresult_runtime_seed.py --generate-code
BASKETBALL_PRESENTATION_ARTIFACT_DIR="$PWD/.local/presentation-client" go test ./games/basketball/engine -run '^TestRuntimePresentationDisplay|^TestRuntimePresentationCanonical' -count=1 -v
go test ./games/basketball/engine -run '^TestRuntimePresentation|^TestOriginalPostResult|^TestRuntime.*Replay|^TestRuntime.*Result' -count=1 -v
go test -race ./games/basketball/engine -run '^TestRuntimePresentation|^TestOriginalPostResultDisplayCommitOwnership' -count=1 -v
```

The source generator executes the original DC15..DC96 decompressor for all nine
result presets and compares its writes before exporting production tables. Eight
court fixtures cover retained column history, attribute masks, dynamic patches,
result backgrounds, digit writes and the black palette. Raw fixture SHA-256:
`13cda400b9bb382124fa51df6deccbd7c54572a115f3b7733707a05397c427a0`.

The controlled lifecycle test traverses complete reward and final-ending video
schedulers, checks rendering, input immutability and JSON restoration, and emits
`ending.json` plus `ending-needs-confirm.json` when the artifact directory is set.
These are controlled result-confirmation boundaries, **not an earned fourteen-win
campaign**. The latter is used by server tests to verify the persistent confirmation
timeout. Browser/native/Wasm acceptance is maintained in the presentation client
harness and its own report.

Retained local evidence: `.local/runtime-presentation-source.log`,
`.local/runtime-presentation-display.log`, `.local/runtime-presentation-regression.log`,
`.local/runtime-presentation-race.log` and `.local/presentation-client/`.
