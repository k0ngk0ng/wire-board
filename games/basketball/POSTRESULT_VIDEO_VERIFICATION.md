# Controlled original reward and ending video evidence

The original console now supplies complete **controlled** reward and ending observations with normal NMI, text, actor updates, uploads, fades and video timing. These are graphics/scheduler references, not earned victories or a naturally completed fourteen-match campaign. No platform gameplay, renderer, server or ROM instruction was changed.

## Initial state and explicit intervention

The initial state replays the previously recorded `match-clock-complete` controller timeline through local video frame 22050. Its complete RAM prefix matches the retained reference byte for byte. At that frame the game is in normal match-end phase 5, before the first `BA40` result entry. The dedicated workspace state is `.local/nes-basket/states/slot-91.fcs`, SHA-256 `407339935ae9bdfba99cbfbc23f502fa53a40fc17745b1d0ca0614da33785adc`.

Both controlled runs load that state and alter bytes **once, at original bank4 `BA40` entry**, absolute emulator frame 25593 / CPU cycle 762146744. This is a function boundary; execution continues at the unchanged entry instruction. The full 2 KiB before/after RAM and every requested override are retained as `injection-before.ram`, `injection-after.ram` and `injection.json`.

| Address | Reward before → after | Ending before → after |
| --- | --- | --- |
| 0166 | home score 12 → 20 | 12 → 20 |
| 0167 | away score 61 → 10 | 61 → 10 |
| 0168,0169 | high score bytes 0 → 0 | 0 → 0 |
| 058C | unchanged | Period 2 → 28 |
| 06D2,06D3 | unchanged | win bitsets 0 → 127 |

No other RAM byte changes at the intervention. No timer, actor action, animation cursor, input byte or random state is injected. The original result entry and automatic confirmation execute all four `C033` actors before reward/ending. These deliberately winning scores/progress **must never be treated as gameplay or AI acceptance**.

Reward uses no controller input for 1100 subsequent video frames. Ending uses no input until a normal controller Start on local frames 14000–14001, then continues to frame 15000 through the original reset and intro. Input lists are retained in each `scenario.json`. No phase timer is shortened.

## Observation invariance and repeatability

- Reward reference versus observed: all 1100 RAM frames and 13 PNGs are identical; RAM SHA-256 `e7dd0adc1ac1f46c19720b35dc125c9020d0b7c47fdd4a641f430a525558501c`.
- Ending reference versus observed: all 15000 RAM frames and 24 PNGs are identical; RAM SHA-256 `80a8f1d21f76bfe5430cb27355529156366289260d9cf6cc99151bdd5edd7b0c`.
- Separate ending runs saving selected before-frame console states and observing all mirroring changes also reproduce the same complete RAM and screenshots. The state-saving callback does not alter the observed execution.
- Each process writes a closed-file completion marker before the existing launcher reaps it. The launcher uses the existing workspace-only macOS write sandbox and workspace FCEUX configuration; no outside state/configuration was written.

The minimally instrumented reference contains only the declared entry override. Observed runs add read-only callbacks. This repeated comparison proves that those callbacks preserve this controlled execution; it does not transform the intervention into an earned victory.

Artifacts are under `.local/nes-basket/runs/postresult-{reward,ending}-{reference,observed}/`. Ending additionally has `postresult-ending-mirroring/` and `postresult-ending-raster-states/`. The compact summaries and all important raw-log hashes are `.local/postresult-video/{reward,ending}-summary.json`. Contact sheet: `.local/postresult-video/original-console-contact-sheet.png`; these are actual original console screenshots, distinct from the earlier static nametable previews.

## Actual video schedule

Local frame 1 starts immediately after loading the saved state. Absolute emulator frame = local frame + 25549. Rows below describe the first per-video RAM observation of each phase; exact function entry cycles remain in `events.jsonl`.

| Scene / phase | First local video frame |
| --- | --- |
| Result, Scene1 phase6 | 42 |
| Reward, Scene1 phase7 | 503 |
| Reward exit, Scene2 phase0 | 861 |
| Preparation, Scene2 phase1 | 886 |
| Ending, Scene7 phase0 | 503 |
| Ending phase1 | 527 |
| Ending phase2 | 1408 |
| Ending phase3 | 1418 |
| Ending phase4 | 5026 |
| Ending phase5 | 5049 |
| Ending phase6 | 9026 |
| Ending phase8 | 9035 |
| Ending phase9, waiting for input | 13355 |
| Ending phase10 after Start | 14008 |
| Ending phase7 | 14010 |
| Scene0 after original reset | 14018 |

Scene1 normal entry cadence is three video frames. Reward makes exactly **113 `BD8E` calls**. Its entry background upload lasts four video frames, followed by eight-frame fade-in. Timer 40 emits three message calls, each blocking for one video frame. Exit fade-out lasts eight frames. A normal entered reward call does not itself cross a video boundary; the outer schedule provides its wait. After the timer-40 message call, the next dispatch interval is **four**, not a guessed three plus three. Original NMI counters and existing blocking waits determine the outer remainder.

Scene7 has 13370 dispatch/text calls in this run, including the intentionally longer wait at phase9 until Start. Of its 13369 measured successive entry intervals, 13358 are one frame; the others span 9, 13, 15, 16, 19, 20 or 22 frames during blocking presentation work. All 13370 `FD0A→FD16` text calls finish within the same video frame. The final reset jumps away without a normal ending-tail return; this is explicitly an unmatched `endingBegin`, not a lost log record.

During Scene7, fade delay 1 consistently blocks eight video frames; delay 2 blocks twelve. The eight background uploads, in source order, block **7, 7, 7, 6, 7, 5, 6, 6** frames. Music request 75 occurs at local527 and request76 at local9055. This records source audio requests with the console audio driver executing; it does not validate audible playback or the platform music mapping.

## Raw boundary contract for source comparisons

Each line of `events.jsonl` contains `kind`, CPU `pc`, absolute video `frame`, CPU `cycles`, scene/phase, counter4C, fade flags/delay, message ID and timer. `raw >= 0` is the zero-based index of its **2048-byte** snapshot in `boundaries.ram`; byte offset is `raw * 2048`. Raw indexes are contiguous and checked against the exact file length.

| Event | Source boundary |
| --- | --- |
| `schedule`, `scene1`, `scene7` | CA54, CAA4, CAE3 |
| `scheduleWait` | first CAE9 visit per dispatch; repeated busy-loop visits intentionally deduplicated |
| `fadeBegin`, `fadeEnd` | C021, FCF8 after bank4 fade routine, before bank restoration |
| `backgroundBegin`, `backgroundEnd` | C024, DCC2 after original upload/waits, before RTS |
| `messageBegin`, `messageEnd` | bank4 BE6E, BEC2 after its original NMI wait |
| `textBegin`, `textEnd` | FD0A, FD16 after A496 before bank restoration |
| `endingBegin`, `endingActorsBegin`, `endingEnd` | bank3 B995, BF35, BF3F |
| `resultActorsBegin`, `resultEnd` | bank4 BC96, BCA0 |
| `rewardBegin`, `rewardEnd` | bank4 BD8E, BE6D |
| `queueHeader` | writes to 030B,032E,0351,0374, including repeated values |
| `sound`, `reset` | FBCC with accumulator value, F79D before reset |

The seed can already be inside a fade begun before capture, and a capture can stop inside a preparation message wait. Summaries retain unmatched boundaries; they do not fabricate starts or ends. Rewards have 959 full boundary snapshots. Ending has 55369 (including later reset/intro text), with 13369 completed ending actor tails. See the generated count rather than relying on a phase-based estimate.

## Renderer resource contract

`prepare_postresult_raster_reference.py` produces `.local/postresult-video/raster-reference.json`: eight selected frames with hashed screenshot/PPU/OAM/control sources, palette, exact observed CHR pages, original register-write cycles, and all before/after IRQ PPU snapshots. It is a renderer **test observation**. Production rendering must use recovered semantic scene state and static art, never replay these recorded RAM/OAM frames.

The first exact full-band contract is final-picture Scene7 phase9, local13500 / emulator39049:

```json
{"y0":0,"y1":240,"nametable":8192,"scrollX":0,"scrollY":0,"backgroundCHR":[20,21,22,23],"spriteCHR":[56,57,11,40]}
```

Files are `postresult-ending-observed/frame-13500.png`, `ppu-39049-ppu.bin`, `ppu-39049-oam.bin`, `ppu-39049-ctrl.bin`. Existing independent offline tile/sprite readers reproduce **all 61440 pixels exactly**, retained as `.local/postresult-video/phase9-offline.png`. Mirroring register is1. This single full contract does not establish the other scenes' fetch boundaries.

| Sample local frame | Scene | First visible CHR pages | Relevant raster behavior |
| --- | --- | --- | --- |
| reward540 / reward660 | reward | 68,69,70,71 / 0,59,59,32 | Retained Court1 field switches to96–99 for text |
| ending600 | stage | 60,61,62,63 / 37,38,11,40 | Three IRQs; an early scroll reset, later text CHR switch, IRQ disable |
| ending1800 | seaside | 84,85,86,87 / 37,38,11,40 | Two IRQs: text switch and disable |
| ending6000 | airplane | 50,51,52,53 / 37,38,11,40 | Four IRQs; separate horizontal scroll regions and text |
| ending10500 | credits | 12,13,12,13 / 56,57,11,40 | Six IRQs; four scrolling scenery bands, text, disable |
| ending13500 | final picture | 20,21,22,23 / 56,57,11,40 | One disable IRQ; no text CHR switch |

`ppu-<frame>-ppu.bin` is the **after-frame** view and often contains text CHR96–99, not the field mapping. For stage600, `irq-26149-104-ppu.bin` retains the stage CHR; `irq-26149-107-ppu.bin` is after the font switch. For reward660, field snapshot is `irq-26209-108-ppu.bin` and post-switch snapshot `irq-26209-109-ppu.bin`. Every IRQ snapshot and its source cycle is indexed in the summary.

The separate all-frame mirroring observer records changes at local504 (value0, vertical), local5050 (value1, horizontal), and local14018 reset (value0). Initial saved-state mapper value is1. The main raster logs preserve all selected-window writes to 2000/2001/2005/2006, MMC3 CHR selectors and IRQ registers. `before-<absolute-frame>.fcs` snapshots retain each selected frame's `TSBS`, `ICou`, `KOOK`, mapper registers and PPU state. The parser verifies `TSBS` equals that frame's observed callback cycle.

**Unresolved for the seven complex sample contracts:** exact pixel-fetch changes at CHR/address/fine-scroll writes, including the two-tile fetch pipeline. Their `bands` field remains `null`; no guessed band edges are passed off as verified. Exact per-frame cycles and FCS state are available for deriving them. A single 194-pixel field crop cannot reproduce all ending phases. The older long-running court formula also must not be transplanted without checking its initial phase and blanked upload intervals.

## Reproduction

From the basketball worktree, using the existing pinned ROM, capture runner, FCEUX and RGB table:

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_postresult_video.py seed
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_postresult_video.py reward
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_postresult_video.py reward --observe
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_postresult_video.py ending --start-frame 14000
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_postresult_video.py ending --observe --start-frame 14000
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_postresult_video.py ending --mirroring --start-frame 14000
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_postresult_video.py ending --raster-states --start-frame 14000
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/summarize_postresult_video.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/prepare_postresult_raster_reference.py
```

Do not recreate slot91 while another capture is loading it. No new REA/FCEUX installation is involved. ROM SHA-256 remains `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`. All data, saved states, scripts, logs and configuration remain inside the authorized workspace. Publication and complete-game acceptance remain outstanding.
