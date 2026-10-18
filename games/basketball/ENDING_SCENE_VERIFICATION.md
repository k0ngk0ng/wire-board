# Logical Scene7 text and presentation composition

## Failure inventory before implementation

- Execute FD0A after phase/actors, consuming a newly issued request in the same call.
- Duplicate authoritative06AB/06AC/06AD or first upload buffer between text and ending contexts.
- Lose second upload buffer, text cursors/clock/repeat/pause, or palette override clears.
- Consume pending NMI queues implicitly, discard queued writes, or hide external RAM reloads between continuous calls.
- Replace observed whole-console resource effects with pure-kernel claims; conceal calls crossing real fade/background/NMI boundaries.
- Introduce Scene1 outer RNG in Scene7, shorten timers, replay phase requests on restore, or execute actors after reset.
- Reorder text queue writes versus phase/actor resource requests; lose text progress after phase clears active flags.
- Fail atomically after text succeeds but phase/actor fails.
- Treat controlled final-score/progress capture as naturally earned gameplay or logical calls as video timing.

Source integration will use retained whole-console boundaries where the logical component can be compared exactly, with explicit resource/NMI boundaries. An uninterrupted unchanged-ROM logical oracle will additionally declare its external queue-consumption policy and omit real video waits/rendering. No expected RAM will be reloaded to force that continuous trajectory.

## Implemented ownership and order

`StepOriginalEndingScene(OpeningState, EndingSceneContext, Buttons)` runs `StepOriginalSceneText` first, commits its returned text state and pending buffers, then runs `StepOriginalEndingPresentation`. Actor-issued text requests therefore remain pending until the next logical call. Text upload events precede phase and actor presentation events. Phase writes may replace the first pending buffer after a text request; event snapshots preserve that ordering without adding another persistent queue.

`EndingSceneContext` owns:

- `Ending`: existing `EndingPhaseContext`, including the sole actor request/ID, text-active byte and first upload buffer.
- `Text`: `SceneTextProgress`, containing only 06AE..06B7.
- `SecondUpload`: the sole second upload buffer beginning at032E.
- `UploadControl`: 0307..030A, cleared by the text call as in the original.

All state is returned by value. Text, phase and actor failures return the original state/context and no requests. Queue consumption, camera/draw finalization, resource waits, input sampling and video scheduling remain explicit caller responsibilities. The component makes no outer RNG draw and does not consume pending uploads.

## Source-first comparison results

The failure inventory and Go comparisons were written before the implementation. `.local/ending-scene-audit/red.txt` records the undefined API failure. The targeted suite then passed in 2.549 s:

- 12,745 consecutive unchanged-ROM FD0A→FC83 calls match complete semantic state, context and ordered requests. The test starts from the first retained Scene7 text boundary and continues from its previous semantic result. No expected RAM is reloaded. JSON restoration is checked every64 calls and at reset.
- 13,358 ordinary calls from the real whole-console capture match complete `OpeningState`, complete context and ordered text/audio requests between FD0A entry and BF3F return.
- All 13,370 captured Scene7 starts are retained. The other11 calls are labelled by observed fade/background entry events before comparison; their external resource/NMI effects are outside this logical component. One final reset has no after-RAM snapshot. Text progress/order, reset and no-RNG checks still cover all captured starts. No row was discarded in response to a mismatch.
- Errors in text, phase and actor processing return the original state/context atomically.

The whole-console comparisons are independent before/after call comparisons, each using its captured input. They are not a continuous replay of video, NMI or resource timing. The separate 12,745-call oracle is continuous under the explicit external contract below. These are different evidence scopes and must not be combined into a claim of full video reproduction.

## Exact source boundaries

Whole-console input: `.local/nes-basket/runs/postresult-ending-observed/events.jsonl` and `boundaries.ram`. Scene7 `textBegin` is FD0A before instructions; `textEnd` is FD16 after A496; `endingBegin` is B995 after text returns; `endingEnd` is BF3F after all actors. The capture executes real rendering, waits and NMI. Its ending was triggered by one declared score/Period28/win-bit injection at the first BA40; it is not a naturally earned final victory.

The logical source oracle executes complete original FD0A and FC83, including phase, actors, motion and C00C projection. Before each call, the harness explicitly sets only the two pending upload-header high bytes030B/032E toFF, copies pending to displayed camera, and clears draw count/slots0..11 at the documented D063 boundary. Address low bytes, length and data remain untouched. No expected snapshot, timer, phase, score or RNG value is loaded between calls.

C000 audio, C006 CHR, C021 fade and C024 background operations remain typed request boundaries in the logical oracle. It does not execute real resource waits, NMI, rendering or the video scheduler. The declared once-per-call queue consumption is a test boundary contract, not a claim that every real upload completes within one video frame. The11 captured calls crossing resource boundaries remain available in the fixture for subsequent resource/wait integration.

ROM SHA-256: `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`.

Fixture SHA-256: `8c35a366be2ec2c385ffe4e9ab342b78387815a5dd8392efcdbef34925e682cb`.

Capture events SHA-256: `cdc38da2e8af2ee23c1828ffea160257e320f3c5a85f1e1f50d2e369e6b60c79`.

Capture RAM SHA-256: `387c1947eb5767e969fd6c8bb451059639edac9e60f245d486554c522f0ad418`.

## Reproduction and remaining work

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_ending_scene.py --check
GOCACHE="$PWD/.local/go-build" GOMODCACHE="$PWD/.local/go-mod" GOTMPDIR="$PWD/.local/tmp" TMPDIR="$PWD/.local/tmp" GOTOOLCHAIN=local go test ./games/basketball/engine -run '^TestOriginalEndingScene' -count=1 -v
```

`engine/testdata/ending-scene-source.json` preserves source digests, counts and external boundaries. The compressed fixture retains all observed text/phase entry/exit snapshots, resource-boundary labels and request observations, plus every continuous logical output. Local generation/check/test logs are in `.local/ending-scene-audit/`.

No runtime, video scheduler, renderer or server integration is included. Real fade/background state effects, queue completion timing, pixels/audio and natural full-campaign acceptance remain outstanding. The complete game reconstruction is not finished.
