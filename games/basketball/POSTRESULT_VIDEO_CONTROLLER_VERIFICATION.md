# Standalone original post-result video controller

The standalone controller now preserves reward and ending resource waits, phase continuations, actor timing, shared NMI counter behavior and request ordering. It runs no CPU/PPU interpreter and does not load captured state in production. Runtime/server integration remains a separate task.

The source contract and pre-implementation failure inventory are in `POSTRESULT_SCHEDULER_CONTRACT.md`. Tests were written before the controller/assets/constructor implementations; missing-API RED logs are retained under `.local/postresult-scheduler/`.

## APIs and ownership

- `BeginOriginalPostResultDispatch` starts at an original dispatch boundary without advancing video. It copies camera, submits the preceding actor display, clears the draw list and samples input once. Scene1 performs its one outer RNG draw/F049 selection; Scene7 performs FD0A text first.
- `StepOriginalPostResultVideo` advances one video boundary, services the applicable normal NMI, special blanking handoff or disabled upload work, then resumes until the next source wait.
- `WaitAfterOriginalPostResultDispatch` enters CAE9 after an already completed actor tail. It retains the current4C counter and chooses the current destination scene's threshold.
- `NewOriginalPostResultVideoAfterResult` adapts a completed semantic result tail after a committed black fade; its exact contract is below.

`PostResultVideoState.Scene` is the sole live `OpeningState`. `Context` retains existing ending/text state and physical queues0/1; `TailUploads` owns0351/0374 once. Source field CHR, palette-builder byte, PPU control/mask, logical byte0300 and counter4C remain explicit. The clock's typed stage, phase-relative continuation and active resource describe semantic source operations, not CPU instruction/register state. `VideoTicks` counts calls for diagnostics; it does not select phase/actor cadence.

The immutable confirmed match outcome is outside this state. Reward inventory is inserted once on first BD8E entry. Current scene period/wins and the original RNG continue through the movie; no fresh match or newly seeded state replaces them.

Blocked ticks do not rerun phase entry, actor animation, text, item insertion or RNG. Their input argument does not replace the current dispatch's retained sample. The next dispatch samples once. Host buffering of a tap that begins/ends between logical samples is an external input policy; the controller does not silently introduce one.

## Resource and actor ordering

Messages queue their0351/0374 payloads and wait for one counter change. Fades queue one palette level, await NMI acknowledgment, reset4C and wait for0302 before proceeding. Backgrounds retain the normal-entry wait, special blanking entry, source-derived disabled work budget, blanking exit and normal-exit wait. Normal NMI commits all pending queues before incrementing4C; blanking handoffs and disabled work do neither.

Blocking ending branches are split at each source callsite. Nonblocking branches reuse `StepOriginalEndingPhase`. The reward uses the existing source-tested award transaction before its first background wait. No full logical handler result is computed and parked while its original blocking calls are still pending. Actors run after the final source wait, including reward's scene-change call; reset bypasses the actor tail.

The decoded backgrounds, palettes and work costs are generated from the pinned ROM into `postresult_video_assets_generated.go`. They are static resources, not recordings. Work budgets reproduce the nine supported background spans; they are not a universal model of arbitrary CPU entry fractions or other resources. The supported post-result data lies safely away from the coarse frame-budget boundary, as recorded in the scheduler source oracle.

## Commit effects and renderer boundary

Effects preserve source ordering:

| Effect | Meaning |
| --- | --- |
| `actors-submit` | CE09-equivalent input from the preceding actors/projection before draw-list reset. The renderer stages this for a later normal NMI. |
| `upload-queued` | A pending source write; it is not a completed PPU transfer. |
| `upload-commit` | Normal-NMI acknowledgment, with slot, PPU address, length, data and0307..030A control. Palette commits use the same physical queue semantics. |
| `normal-nmi` | NMI-time raster source after upload handling, before resumed mainline. |
| `oam-reverse` | The C0 NMI path's conditional F002 reversal of working OAM after DMA; visible OAM remains unchanged until a later commit. |
| `blanking-handoff` | F831's special NMI return; no normal queue/DMA/counter work is implied. |
| `background-chr`, `background-commit`, `sprite-chr`, `render-mask`, `mirroring` | Ordered resource/register commits. Background data carries its original raw PPU target. |
| `sound` | Original request value, including FD/FF audio-control requests around background work. |
| `reset` | The jump to F79D before executing machine reset. |

`LiveRaster` is separate from the NMI-time source. Later IRQ handlers can read offsets changed by resumed mainline. The early stage-screen IRQ14 also persists RAM56 bit0: the controller uses the source-verified IRQ helper with the early source snapshot, excluding background wait-entry where F831/SEI blocks it. This repaired the first captured mismatch at absolute frame26086 without copying any expected control byte into production.

These snapshots do not provide an invented exact CPU interleaving for all raster changes. The renderer may use an explicitly documented canonical sampling policy; source-observed pixel mode remains separate validation input. Fine IRQ/mainline timing must not be replaced by a frozen NMI snapshot or final-state snapshot for the entire frame.

## Actual video source acceptance

The complete controlled captures are described in `POSTRESULT_VIDEO_VERIFICATION.md`. Both tests start from the real per-video state immediately after the original confirmation actor tail at absolute frame26052, then continue from the previous Go result. Expected RAM is used only for comparison, never to repair the running state.

| Chain | Consecutive video ticks | Compared dispatches |
| --- | ---: | ---: |
| Reward through its exit actor tail | 358 | 113 |
| Ending through F79D reset | 13509 | 13369 completed calls plus the reset dispatch |

Every non-reset video tick compares the complete semantic `OpeningState`, text/ending context, all four pending upload buffers, source4C/0303/4F/50/56/57/0300 values and dispatch cadence. Reward inventory/mode/period/win masks are checked against the captured bytes. Every captured FBCC sound request is checked on its exact video frame and in order, including FD/FF and terminal0. JSON round-trip replay covers every resource wait entry/intermediate/exit tick and periodic ordinary ticks.

The last captured ending row is retained, but its RAM is already affected by subsequent machine reset instructions. The controller checks the reset event/sound boundary there instead of claiming that it implements the following reset/intro. No preceding video row is omitted.

Final standalone validation passed on native Go (1.850 s), the race detector (19.804 s) and Go/Wasm (4.374 s), including the constructor and conditional OAM-reverse effect. Static resource/video generation repeated with identical digests.

Static acceptance additionally checks1900 original9526 palette calls and all nine background decoded byte sequences/source work costs. The background source runner verifies every PPU output address/value against the independent static resource decoder. Tests also reject unknown assets, double dispatch entry and replacement of the retained controller sample during a blocked call.

The wins in the source captures were triggered by explicit one-time score/progress interventions. These tests prove presentation semantics, not naturally earned wins or the fourteen-match gameplay campaign.

## Legacy semantic-runtime handoff

The constructor accepts:

```go
NewOriginalPostResultVideoAfterResult(
    confirmedActorTail OpeningState,
    committedDisplay DisplayState,
    beforeAward campaign.Progress,
    logicByte uint8,
) (PostResultVideoState, error)
```

Use the live pre-award inventory, normally `Config.Campaign`. `SettleCampaignRuntime(...).Progress` already includes the item and must not be passed as the live video inventory. The constructor copies that inventory/equipment, takes period/wins from the actual confirmed scene and leaves the first video reward entry to award once. The immutable settlement result can remain separate presentation/history data. Next preparation must take final progress and RNG from the completed video state.

Required boundary: unentered winning Scene1 phase7 or final Scene7 phase0, no pending display fade/palette-level commit, committed black level0 and source result BallCHR13. The legacy runtime must actually finish its outgoing fade before constructing the controller. Merely expiring an eight-frame wait while retaining a full-color result is insufficient. Existing legacy confirmed snapshots remain parked unless explicitly started by their owner.

| Source field | Production derivation |
| --- | --- |
| Actors, positions, animation state, phase, timers, RNG, scores | Copy actual confirmed actor-tail `OpeningState`; never reinitialize. |
| Logical0300 | Carry actual `Runtime.LogicFrame`. |
| Counter4C | 1 after the original delay1 fade, then CAE9: reward needs2 further NMIs; ending needs1. |
| Background4F/50 | Bank4 BBA3 court table `[76,68,72,92,80,84,88,64]`, second page=first+2. |
| Sprite51..54 | Source team setup `[0, team0+1, team1+1, 13]`; BCC3 supplies the last page. |
| Background palette0304 | Current `Display.PaletteID`; legacy0 resolves to Court+1. Animated court4/6 palette IDs are preserved. |
| Sprite palettes0305/0306 | Source team+2 setup. |
| Control56 / mask57 | Source base88 plus displayed-camera nametable bits; result mask18. |
| Machine4D / background03A0 | C0 /99 at this result boundary. |
| Fade0301/0302/0303 |80 /1 /F0 after four-level fade-out. |
| Final palette queue0374 | Consumed FF header, length32, all original black0F. |

Canonical initialization is explicit:06AB/06AC/06AD=0; `SceneTextProgress` is zero; queues0/1/2 use addressFF00, length0 and zero data; `UploadControl` is zero. Queue3 retains the derived consumed32-byte black palette.

The existing runtime never persisted original pre-match menu text progress or inactive upload contents. The constructor deliberately uses canonical inactive values: no request/active text, zero unused progress/data and FF queue headers. It does **not** claim to recover lost RAM history. Source-backed tests run both the canonical constructor state and the captured retained state through both complete chains. They match core actors/phase/RNG/progress, clocks/resource continuations, raster sources, sounds and every effective upload address/length/payload. Bytes beyond an upload's declared length are excluded from effective-write comparison; no active payload byte is normalized.

A caller that already owns an active original text/queue history must preserve that complete live `PostResultVideoState` instead of using this legacy constructor. JSON restoration resumes the existing continuation and must not reconstruct or award again. Defeat/Continue and versus destinations are outside this winning-result constructor.

The old runtime computes the confirmation actor tail before its wait. This adapter starts only after that wait and does not claim to repair the earlier internal actor-write timing. Root integration must also bypass the old additive `FramesUntilLogic` wait while this controller owns the presentation. At `Handoff`, CAE9 has released the next dispatch; the new scene owner can begin in the same video boundary without adding another frame.

## Files, digests and reproduction

- `postresult_video.go`, `_resources.go`, `_reward.go`, `_ending.go`: controller and source continuations.
- `_assets.go`, `_assets_generated.go`: static resource API/data.
- `_handoff.go`: explicit legacy-runtime constructor.
- Corresponding tests and `testdata/postresult-video*.json*`: source acceptance.
- `investigate_postresult_scheduler.py`, `verify_postresult_video_sources.py`: source rules, assets and video vectors.

Video fixture SHA-256: `5f6e8e0ac29698fd95433c8a141f362db6c2240754f16a65ea15709f1221f371`.

Asset fixture SHA-256: `95b8f2526df2750a4b0c0b17aea2b5d931018b011473b2e0a33e8d971a4ffab9`.

ROM SHA-256: `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`.

Source metadata retains capture hashes, generated-table digest and handoff byte ranges. Local RED/pass/repeat logs are under `.local/postresult-scheduler/`.

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/investigate_postresult_scheduler.py --check
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_postresult_video_sources.py --check
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_postresult_video_sources.py --emit-tables --check
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_postresult_video_sources.py --videos --check
GOCACHE="$PWD/.local/go-build" GOMODCACHE="$PWD/.local/go-mod" GOTMPDIR="$PWD/.local/tmp" TMPDIR="$PWD/.local/tmp" GOTOOLCHAIN=local go test ./games/basketball/engine -run '^TestOriginalPostResultVideo' -count=1 -v
```

Remaining work: runtime/server and committed-surface/OAM integration; full save/reconnect/hidden-page UI acceptance; audible playback/music76 distribution and FD/FF pause/resume integration; source-derived fine IRQ/mainline interleaving where pixel identity is claimed; defeat/Continue and reset/intro lifecycle; natural complete campaign acceptance. Overall reconstruction is not complete.
