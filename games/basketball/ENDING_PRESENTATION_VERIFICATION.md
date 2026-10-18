# Ending phase and actor choreography

## Failure inventory before implementation

- Start from pre-confirmation actor state; reload expected RAM each frame instead of continuing the previous semantic result.
- Omit or reorder actors0..3, including on phase-change calls; execute actors after reset.
- Lose contextual actor text/palette/display-mask changes or place actor sound requests before phase requests.
- Omit original C00C projection/motion; accumulate draw-list entries without the declared renderer-finalization boundary.
- Add Scene1 outer RNG to Scene7, shorten timers, reseed, or alter input to force an early exit.
- JSON continuation diverges; combined API error returns partial phase/actor writes.
- Present isolated CPU kernel calls as console video timing, rendered ending visuals or naturally earned victory.

The complete kernel oracle will execute unchanged ROM from the existing declared settlement confirmation boundary, including its four-actor tail, then all ending phase/actor calls until reset. FD0A text processing, rendering/NMI/resource uploads/audio remain explicitly outside this component.

## Result and source identity

`StepOriginalEndingPresentation` now runs the existing ending phase component followed by actors 0, 1, 2, 3. It carries each actor's returned state and `ResultActorContext` into the next actor and appends actor sounds after phase requests. Reset exits before the actor tail. Any phase or actor error returns the original state/context with no partial requests.

The source comparison passed all 12,745 consecutive ending calls and 50,976 actor calls, each retaining original motion and C00C projection. The test first executes the four confirmation actors semantically and compares that complete state with the original confirmation tail. It then advances from each previous semantic result; it never replaces state with an expected snapshot. It checks complete `OpeningState`, complete ending context, ordered requests, reset, zero RNG changes and original phase order. JSON round-trip replay is checked every 64 calls and at reset. The atomic actor-failure check also passed. Targeted suite: 0.750 s.

The observed phase entry ordinals are:

| Phase | Kernel call ordinal |
| --- | ---: |
| 0 | 0 |
| 1 | 2 |
| 2 | 883 |
| 3 | 885 |
| 4 | 4478 |
| 5 | 4480 |
| 6 | 8449 |
| 8 | 8450 |
| 9 | 12739 |
| 10 | 12741 |
| 7 | 12743 |

Start is pressed on the first entered phase9 call, matching the declared original probe. Timers are unshortened and RNG is never reseeded or advanced by an outer Scene1-style draw. These ordinals are kernel calls, not measured video-frame times.

ROM SHA-256: `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`. The source starts at settlement case `0-0-1-26-empty-True`, BC96, and executes the original confirmation actor tail with C00C retained. That settlement is a declared controlled case, not a naturally earned fourteen-win campaign. Its source digest and all full-chain requests/call counts are retained in `engine/testdata/ending-presentation-source.json`.

Fixture SHA-256: `37717009bce742b4f2a4c4c8112f6030d61843dbe6fa0311ab7a351e08a51c72`. The compressed fixture includes the confirmation input and initial state, followed by complete RAM after every kernel call, inputs, ordered requests, actor indexes, projection/RNG counts and instruction counts. No failed calls are omitted.

## Reproduction

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_ending_presentation.py --check
GOCACHE="$PWD/.local/go-build" GOMODCACHE="$PWD/.local/go-mod" GOTMPDIR="$PWD/.local/tmp" TMPDIR="$PWD/.local/tmp" GOTOOLCHAIN=local go test ./games/basketball/engine -run '^TestOriginalEndingPresentation' -count=1 -v
```

Failure inventory and comparison tests were written before the wrapper implementation. `.local/ending-phase-restore/full-red.txt` records the missing wrapper failure. `full-tests.txt` records the passing test output; `full-source.json` and `full-source-check.json` retain generation/check summaries.

## Explicit component boundary

Before each kernel call, the source harness copies pending camera bytes to displayed camera bytes and performs the exact D063 renderer-finalization draw-list reset: count 041C is zeroed and slots 041D..0428 become FF; slot13 remains unchanged. The semantic harness applies the same existing reset helper. This is a declared boundary operation. The complete CE09 renderer is not executed or claimed.

C033 actor animation/motion and C00C projection execute unchanged original instructions. Audio C000, CHR C006, fade C021 and background C024 remain request boundaries; C036 is declared but any occurrence in this ending kernel fails the generator. FD0A text processing, uploads, NMI waits, controller/video cadence, final pixels and actual music playback remain outside the oracle. Root integration must preserve those dependencies; this passing component test does not establish final ending visuals or video timing.

Runtime/server lifecycle, immutable result integration, natural final-win input E2E, reset presentation and the complete earned campaign remain outstanding. No runtime, server, actor-interpreter or gameplay files were modified by this wrapper task.
