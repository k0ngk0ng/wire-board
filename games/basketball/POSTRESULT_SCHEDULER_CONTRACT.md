# Post-result resumable scheduler contract

## Failure inventory before scheduler implementation

This document records the source investigation and test oracles before implementation. The subsequent standalone implementation is verified in `POSTRESULT_VIDEO_CONTROLLER_VERIFICATION.md`; neither stage modifies runtime/server behavior.

- Add fixed waits to a fresh three-frame delay instead of retaining original4C resets and current-scene CAE9 threshold.
- Treat4C as monotonic elapsed video time during fades or background uploads.
- Return a phase's final actor state before its blocking resources finish, or run actor clocks on every wait frame.
- Resume by calling the entire phase again, repeating inventory insertion, RNG, text update, controller sampling or entry writes.
- Lose ordered message28/30/29 waits or use controller state sampled after the waits instead of FF80's dispatch sample.
- Consume queue headers when queued rather than at NMI; miss same-value queue writes or consume newly queued post-NMI data in the same interrupt.
- Generalize the existing eight-frame fade to delay2 or delay0 incorrectly; lose per-level global counter reset.
- Apply Scene1 outer RNG to Scene7, or omit the single Scene1 draw before a suspended reward call.
- Treat background upload duration as a captured per-phase timeline, constant frame count, or instantaneous resource swap.
- Confuse F831 special blanking handoffs with normal NMI counter increments.
- Ignore current destination Scene after reward exit when CAE9 chooses its threshold.
- Duplicate video wait/counter/continuation authority; restore a snapshot by replaying requests already issued.
- Conflate necessary gameplay/animation ordering with unresolved exact PPU fetch/audio/CPU scheduling.

## Proven wait rules

The oracle is `scripts/basketball/investigate_postresult_scheduler.py`. It executes bounded unchanged-ROM source functions through the existing Python source-probe tools and checks retained whole-console event logs. No interpreter is added to the product. The generated instructions and detailed oracle are `.local/postresult-scheduler/{instructions.txt,oracle.json}`.

### Outer dispatch: CA54, CAA4/CAE3, CAE9

CA54 clears global NMI counter4C, increments logical byte0300, copies pending camera to displayed camera, finalizes the previous draw list and samples controllers through FF80. The scene is then dispatched. Scene1 CAA4 makes one F9C3 draw before F049 and the phase handler. Scene7 CAE3 runs FD0A text before FC83 and makes no unconditional RNG draw.

CAE9 selects its minimum using the **current scene after the handler returns**. CB06 is `[0,2,0,0,0,0,0,0]`. It waits until unsigned4C reaches that threshold, snapshots4C, then waits for a further change. Under the observed ordinary NMI path:

```
remaining NMIs = max(0, threshold(current scene) - current4C) + 1
```

This is not a fresh three-frame countdown after every handler. It matched every one of113 reward and13,369 ending dispatch intervals. Scene changes and counter resets inside blocking helpers are part of the calculation.

| Observed call | Blocking video frames | Counter at CAE9 | Destination threshold | Remaining NMIs | Total dispatch interval |
| --- | ---: | ---: | ---: | ---: | ---: |
| Reward entry | 4 background +8 fade | 1 | 2 | 2 | 14 |
| Ordinary reward | 0 | 0 | 2 | 3 | 3 |
| Reward timer40 | 3 messages | 3 | 2 | 1 | 4 |
| Reward exit | 8 fade | 1 | 0 for Scene2 | 1 | 9 |
| Ordinary ending | 0 | 0 | 0 | 1 | 1 |
| Ending delay1 fade | 8 | 1 | 0 | 1 | 9 |
| Ending delay2 fade | 12 | 2 | 0 | 1 | 13 |

The ending has additional intervals15/16/19/20/22 where backgrounds and fades occur in one call. All measured intervals are retained, not just these examples.

### Palette fade: C021 → FCEC → bank4 94DF

The delay byte0302 is not an arbitrary hold after the complete fade.

1. With delay0, build one full-brightness level4 palette.
2. With nonzero delay, build four levels: fade-in1/2/3/4, fade-out3/2/1/0.
3. Queue each palette into physical upload slot3, header0374=3F.
4. Wait at961E until NMI marks that headerFF.
5. **Reset global4C to0 at9623..9625.** Wait until4C reaches delay0302.
6. Build the next level, or return after the final hold.

Thus delay1 takes8 ordinary NMIs; delay2 takes12. The fade returns with4C equal to its delay, irrespective of its total duration. Delay0 takes one palette-commit NMI and returns with4C zero. The separate Scene1 phase1 palette fast path at9612 does not block; it must not be generalized to reward/ending.

All32 source probes for scenes/phases used here, both directions and delays0..3 passed. The12 observed reward/ending fades also matched exact queue/consume order, first commit on the following video frame and level spacing of delay+1 frames.

### Message: C036 → FD64 → bank4 BE6E

The message builder writes physical upload slots2/3, then BEBC snapshots4C and waits until it differs. It does not reset4C. A counter255→0 wrap is a valid wakeup. The12 bounded source probes include that wrap. All three observed reward timer40 messages take one video frame each, in order28,30,29.

The reward's RNG/input test occurs only after the third wait. It still uses the controller sample retained from the dispatch's FF80 call. On an input-triggered exit, the fade follows that single internal RNG draw. Timer112 goes directly to the exit path before messages/RNG/input.

### Pending uploads and NMI

The normal NMI path calls CB0E/CC1A before incrementing4C. The four buffers begin030B,032E,0351,0374. CC1A processes them in slot order. A nonnegative header is pending; even length0 is acknowledged by writing headerFF. Low address, length and data remain retained. A negative header is skipped. The source probe covers all16 slot masks with lengths0/1/32.

0307..030A are per-queue PPUCTRL overrides, not palette bytes. The source ORs each into PPUCTRL and clears it when that queue is serviced. Current Go names `UploadControl` and `ClearUploadControl` reflect this. Text, messages and fades share these physical queues; a normal NMI can consume pending text and palette data together. Pending data queued by the resumed mainline **after** that NMI must wait for the next interrupt.

## Background upload work and blanking

C024 enters fixed DBF8. It enables normal NMI/audio handling, waits for a counter change, then calls F831. F831 sets4D=0 and parks until a special NMI path F099..F0AC restores the caller and disables NMI. This handoff does not service normal queues or increment4C. The resource is then decoded/uploaded while rendering/NMI are disabled. A second F831 handoff returns to normal execution; the function re-enables NMI/audio handling and waits for a normal counter change before returning.

Consequently a background call increments4C by2, not by all its elapsed video frames. The source capture confirms this for all nine uploads. NMI-disabled compressed-data work remains real elapsed time; it cannot be replaced by an instantaneous texture swap.

The original DC15..DCB0 work was executed for all nine supported presets. Every PPU output byte/address matched the independent static resource decoder. The resulting costs are properties of the resource and original decode loops, not timings copied from a movie:

| Preset | Compressed bytes | Output bytes | Source work CPU cycles | Rounded work frames | Observed complete call frames |
| --- | ---: | ---: | ---: | ---: | ---: |
| 26 | 4 | 192 | 10540 | 1 | 4 |
| 95 | 600 | 1024 | 101769 | 4 | 7 |
| 96 | 599 | 1024 | 101622 | 4 | 7 |
| 97 | 597 | 1024 | 98347 | 4 | 7 |
| 33 | 309 | 1024 | 78177 | 3 | 6 |
| 34 | 471 | 1024 | 91295 | 4 | 7 |
| 25 | 7 | 1024 | 52563 | 2 | 5 |
| 98 | 153 | 1024 | 72245 | 3 | 6 |
| 23 | 255 | 1024 | 72632 | 3 | 6 |

For these resources at the captured dispatch boundaries, `3 + ceil(workCycles / 29780.5)` matches every complete upload span. The extra three boundaries account for the ordinary entry wait, blanking synchronization and ordinary exit wait around the quantized work/handoff. The recorded source PC counts retain repeated compressed-pattern reads; consumed byte count alone is not a sufficient cost model.

This establishes a usable work-budget contract for the nine resources. It is not a universal cycle-exact formula for arbitrary entry fractions, other compressed resources, audio interrupt costs or PPU odd-frame conditions. Production should consume a generated source-cost table and explicit synchronization stages. It must not embed captured frame ordinals or source CPU instructions. If precise fractional timing becomes observable, retain an integer NTSC clock phase and source-derived fixed overhead instead of rounding each arbitrary operation independently. Rendering-disabled NTSC timing and enabled odd-frame shortening must remain distinct.

## Recommended standalone persisted API

The next implementation should be separate from runtime/server until its per-video source vectors pass. Proposed public boundaries:

```go
// No video advance: enter at the source dispatch boundary and run until blocked.
BeginOriginalPostResultDispatch(state PostResultVideoState, sample [4]ControllerFrame) (PostResultVideoStep, error)

// Exactly one video boundary, then resume until blocked or dispatch completes.
StepOriginalPostResultVideo(state PostResultVideoState, input [4]ControllerFrame) (PostResultVideoStep, error)
```

`PostResultVideoState` should own the live `OpeningState`, one tagged reward or ending context, canonical physical upload buffers and a scheduler continuation. It must not own a mutable copy of the confirmed outcome. The existing immutable `RuntimeOutcome` stays with its caller. Existing actor/timer/RNG state is reused, not duplicated in scheduler fields.

Recommended scheduler fields:

| Field | Purpose |
| --- | --- |
| `Counter4C uint8` | Original wrapping counter, including explicit resets. |
| `DispatchStage` | Begin, scene prefix, phase continuation, actor tail, outer threshold, outer counter-change wait, terminal reset. |
| `LatchedInput [4]ControllerFrame` | One FF80-equivalent sample per dispatch; unchanged during resource waits. |
| `ResumePoint` | Typed semantic caller stage, such as reward-after-background or ending-phase4-after-fade; never an emulated PC/register set. |
| `WaitKind` and captured counter/threshold | Header consumed, counter reaches threshold, counter changes, blanking handoff or work budget. |
| `ActiveResource` | Tagged message, fade or background continuation; only one original blocking helper is active. |
| `Resource stage and temporary values` | Fade level/flags/delay, background preset/work cycles, reward saved0304/4F/50. |

The continuation stage itself records that an operation was issued. Restoring it must not reissue a queued palette, award another item or consume another random value. Four canonical queue slots are needed. Existing ending slot0/1 fields can remain their sole owners through slot adapters; add slots2/3 once for message/palette work. Renderer output may cache committed pixels/colors, but must not maintain another independently writable pending queue.

Resource continuations should be semantic state machines:

- Message: queue prepared message → capture4C → wait for counter change → return to caller.
- Fade: queue current palette → wait for headerFF → reset4C → wait for delay → advance level or return. Delay0 uses its separate one-level path.
- Background: ordinary NMI wait → blanking-entry handoff → source-derived work budget → blanking-exit handoff → ordinary NMI wait → return.

A video step services the applicable NMI mode first, commits ready queues, updates4C only on the normal path, then runs the mainline continuation until its next genuine wait. Work while NMI is disabled advances elapsed video time without queue service or4C increments. Newly queued mainline work cannot be acknowledged by the same already-completed NMI.

## Phase continuations must precede actor tails

The existing full logical APIs are validated atomic kernels; their final state cannot simply be applied and followed by a sleep. Source writes occur on both sides of blocking calls. Factor source semantics into shared resumable chunks, retaining synchronous adapters for existing kernel tests.

Reward entry:

```
award once + set entry latch
save0304/4F/50
background26 [yield]
restore saved values; timer0; CHR52/53=3B
fade-in [yield]
actors0,1,2
CAE9
```

Reward timer40:

```
increment timer once
message28 [yield] → message30 [yield] → message29 [yield]
internal RNG once; test retained controller sample
optional exit fade [yield]
on exit: restore characters; Scene2/phase0
actors0,1,2 even after destination changes
CAE9 using destinationScene2 when applicable
```

Ending uses the source phase-specific callsites. For example phase4 first fades out, then clears/assigns actor state, uploads backgrounds33/34, writes the pending text-clear buffer and selects phase5, then executes actors. Phase8 entry clears text/credits/timer, uploads backgrounds25/98, fades in, emits music76 and requests text36, then executes actors. FD0A runs once before entering those phase continuations. F79D reset bypasses the final tail.

All source phase writes and actor updates must remain on their proper side of the yield. Frame callbacks during a wait do not decrement actor countdowns or phase timers. They do not rerun text, resample the current dispatch input, redraw a different semantic state or consume RNG.

## Reuse and required changes outside this investigation

`DisplayPaletteFade` already offers immutable-copy state and committed/pending palette separation. Its current two-video spacing, Frame<8 validation and palette-ID1..15 restriction are court-specific. Generalization needs source flags/delay, shared palette queue acknowledgment and ending palette IDs; changing only the total wait from8 to12 is insufficient.

`StepVideo` currently commits display, advances a generic match video tick, decrements `WaitVideoFrames`, then decrements `FramesUntilLogic`. `StepLogic` resets the latter to3 and existing transition code applies complete handler results before adding waits. The standalone controller must replace that post-result path, not layer another counter on top. Also gate gameplay-clock updates by scene: Scene7 phase1 is not Scene1 `MatchPlaying`, despite sharing the numeric phase value.

`StepOriginalEndingScene`, the text/message builders, phase handlers, actor interpreter, source asset decoder and draw-list reset are reusable. Their existing tests remain kernel oracles. The async controller needs additional assertions at every yield/resume boundary, visible palette commit, message acknowledgment, scene change and reset. Controllers pressed/released during waits must be specified and tested at the source sampling boundary; do not accidentally sample halfway through an already-running reward call.

## Scope, results and reproduction

The source oracle passed32 controlled fade calls,48 queue-consumer calls and12 message calls. It matched all113 reward and13,369 ending outer intervals,12 whole-console fades,3 messages and9 background spans. Captured victory/progress interventions remain explicitly controlled, not gameplay acceptance. No captured call was dropped to make the wait equations pass.

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/investigate_postresult_scheduler.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/investigate_postresult_scheduler.py --check
```

Oracle SHA-256: `b2480fd67f05aa8ddfd53a92e2f8dd28a13e74822dbd3fb46fd59a6f230e5aa5`. ROM SHA-256: `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`. Event source hashes and exact instruction-window digest are included in the oracle. Repeated generation matched.

Still unproven here: a standalone Go resumable controller; per-video semantic-state equivalence during all resource helpers; arbitrary CPU entry fractions; exact interrupt/audio/PPU fetch timing; input edges during long blocking calls; platform restoration/reconnect/hidden-page behavior; natural earned campaign. These are next implementation/acceptance tasks, not reasons to invent shortened clocks, replay movie state or substitute a runtime emulator.

## Constructor failure inventory (before constructor implementation)

The adapter from an existing semantic runtime must not read RAM fixtures. It must reject an unfinished/nonblack result fade, unsupported destination or inconsistent campaign mode; preserve the real actor tail/RNG/logic byte; derive result CHR/palette values from source rules; retain animated court palettes; use pre-award inventory and scene-derived period/wins; arm the remaining CAE9 wait rather than add3 frames; initialize unavailable inactive menu/queue bytes explicitly without creating pending writes. Canonical inactive fields require behavioral-equivalence evidence against the captured retained bytes. This adapter does not repair the legacy pre-handoff result timing or claim arbitrary original menu-history RAM equality.
