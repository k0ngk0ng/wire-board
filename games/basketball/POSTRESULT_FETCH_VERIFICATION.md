# Post-result fetch derivation

## Failure inventory (before implementation)

Wrong video-clock origin or integer CPU remainder; confusing a PPU write's callback PC with the instruction start; ignoring NMI initial scroll/mask/CHR; using after-frame font CHR for scenery; wrong nametable mirroring; applying a scroll write to an entire scanline when the tile pipeline changes midline; conflating nametable-address reset and fine-X reset; discarding preloaded tiles; mishandling pattern low/high plane fetch around CHR changes; row29/31 vertical wrap; restoring horizontal bits at the wrong boundary; treating sprite/background priority or eight-sprite limits as irrelevant; silently masking or excluding mismatching pixels; deriving offsets by searching reference screenshot pixels.

Tests precede the parser. `test_postresult_fetch.py` asks for all eight source-frame reconstructions and compares **all 61440 pixels per frame**, including sprites, text and boundaries. It saves each result/difference and mismatch coordinates. The parser receives recorded register/PPU/OAM/frame-state observations and static ROM-derived art only. Expected screenshots are read exclusively by the test; unexplained differences remain failures. Offline reconstruction is an evidence tool; production must consume semantic tile/scroll state, not execute a CPU/PPU or replay RAM/OAM captures.

## Result

All **eight primary frames / 491520 pixels** match the original console capture exactly. An additional **eight holdout frames / 491520 pixels** also match exactly. No pixel is excluded, repainted from the expected screenshot, cropped away or accepted with a tolerance. Coverage includes background, sprites, text, left-edge masks, sprite priority, and midline changes.

| Family | Primary local frames | Holdout local frames |
| --- | --- | --- |
| Reward | 540,660 | 600,720 |
| Ending stage | 600 | 900,1200 |
| Ending seaside | 1800 | 3000,4800 |
| Ending airplane | 6000 | 9000 |
| Ending credits | 10500,12000 | — |
| Ending final picture | 13500 | 14000 |

The initial missing-module failure was recorded in `.local/postresult-fetch/expected-red.log` before `postresult_fetch.py` existed. Passing results, reconstructed images, difference images and semantic frame JSON are in the same directory. Holdout results are separately retained as `holdout-result.json`. Expected images are read only in the verification calls; `derive_frame` and `render_frame` never open them.

## Source model and timing

The image reference is the existing FCEUX 2.6.6 original-ROM capture, not a physical NTSC capture. The offline derivation uses its documented old-PPU fetch grouping. Supporting source was read from the upstream `v2.6.6` tag and retained only in the workspace:

- `https://raw.githubusercontent.com/TASEmulators/fceux/v2.6.6/src/ppu.cpp` → `.local/postresult-fetch/fceux-ppu.cpp`.
- `https://raw.githubusercontent.com/TASEmulators/fceux/v2.6.6/src/pputile.inc` → `.local/postresult-fetch/fceux-pputile.inc`.

Exact downloaded-file digests are in `.local/postresult-fetch/manifest.json`. No FCEUX code is bundled into production. Its frame schedule, `ResetRL`, `RefreshLine`, `Fixit1/2` and ordinary tile-fetch branch explain the recorded reference pixels. Original game register writes, CHR pages, nametable bytes and OAM remain the authoritative content.

For the saved state immediately before each selected frame:

```text
baseDot = TSBS * 3 + ICou / 16
line0ResetDot = baseDot + 22 * 341 - 16
lineYResetDot = line0ResetDot + Y * 341 - (Y > 0 ? KOOK : 0)
writeDot = observedCpuCycle * 3 - lineYResetDot
```

`ICou` is signed and must be divisible by16; ppudead must be zero. These are derived source timing constants, not fitted offsets. Each register write first retires complete fetch groups through `floor(writeDot / 8)`, capped at34. A group fetches one source tile row; the group two steps behind becomes output. The current fine-X selects across those two retained rows. A later address write changes future fetched rows, while a fine-X write immediately changes which already-fetched columns are selected. Collapsing them into a single scroll change loses real pixels.

The offline adapter preserves `v`, `t`, fine-X, the write toggle, bank selectors, control and mask. Pre-render copies both scroll axes. Each visible line advances fine/coarse Y with the original row29/31 behavior, then restores horizontal scroll after the visible fetches. Background index0 remains transparent for sprite-priority decisions. Sprite ordering and the original eight-sprite limit are applied per line.

This is a **bounded offline observation adapter**, not a general NES emulator and not platform gameplay. It accepts these captured stable scene frames in 8×8 sprite mode. It does not emulate CPU instructions, mapper IRQ generation, NMI uploads, audio, arbitrary games, CHR-RAM mutation, emphasis colors or unobserved midline nametable/palette writes. The frame's original mapped nametables and palette have already been captured after its visible-frame NMI updates. New capture types outside these bounds require their own prior failure inventory and evidence.

## Semantic renderer contract

Each generated frame JSON contains `palette[32]`, `oam[256]` and `rows[240]`. Each row contains:

```text
{ y, mask, spriteCHR: [page0,page1,page2,page3], segments: [...] }
segment = { x, length, page, tile, row, column, palette, mask }
```

A segment references `length` adjacent pixels from one **original static tile row**. `page` is the original1KiB CHR page; `tile` is local0–63; `row` and `column` are0–7; `palette` is a background group0–3. Source X advances from `column` through `column + length - 1`. Segments are ordered, non-overlapping and cover the entire256px row. A segment does not contain RGB output or screenshot data. It can begin or end within an eight-pixel output group because fine-X selects across retained tile rows.

Apply segment mask to background enable/left-edge visibility. Index0 selects universal background color and remains non-opaque for priority. Apply row mask to sprite visibility/left-edge clipping and final grayscale behavior. Sprites use the row's four CHR pages, the raw original OAM layout for this **test fixture**, first non-transparent OAM pixel wins, and behind-background priority tests original nonzero background indices. Production must instead obtain the equivalent display objects from recovered scene state; recorded OAM is only the independent reference observation.

The atlas includes all required static pages, including actual reward sprite page32 and every court's retained field pages64–95. Court page starts are read from original `bank1:BEC5`: `[76,68,72,92,80,84,88,64]`. The L-mode index texture now has67 pages; all274432 pixel indices match direct source bitplane extraction. No precolored court texture was duplicated.

Whole-line bands remain useful for ordinary areas, but the verified browser input path must also accept these row fragments for boundary lines. The runtime engine can produce compact semantic bands plus their boundary fragments later. It must not execute this offline mapper/write adapter in the browser or replay test snapshots as gameplay.

## Reproduction

Capture prerequisites and commands are in `POSTRESULT_VIDEO_VERIFICATION.md`. Additionally save the reward before-frame states:

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_postresult_video.py reward --raster-states
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/test_postresult_fetch.py
```

The second command compares every pixel and recreates all primary frame metadata and images. The eight holdouts are reproducible using the same `derive_frame(kind, frame)` and `render_frame(metadata)` calls, selecting the holdout frame numbers above and comparing the complete result to the corresponding original `frame-NNNNN.png`; the retained holdout report records exact selections and zero-difference results.

These frames prove the resource/fetch adapter under the declared captured conditions. They do not prove every animation frame, all reward courts, a complete browser renderer, engine-generated display packets, refresh/resume or natural campaign completion. Browser verification is owned separately by the root's `postresult-raster.ts` work; no browser renderer file was changed here.
