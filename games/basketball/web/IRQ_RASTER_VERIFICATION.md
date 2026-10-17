# Original HUD raster boundary

The four known Court 3/4 pixel differences were a projection error: the original
font selection, background-address reset and fine-scroll reset happen at separate
points. The renderer now preserves that order and the two prefetched tile rows.
No reference image pixels are copied into runtime assets, and no CPU interpreter
was added. Game rules, runtime court animation, Court 3 logic and controller layout
were not changed.

## Evidence and diagnosis

The pinned ROM SHA-256 is
`0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`.
REA evidence `ev_0b7775d88a48b030637b0d428eb13f069041ea9e81832a2c4b0b555dea6357ad`
resolves fixed-bank `$F298`, `$F2D4` and `$F317`; the complete batch, snapshot and
source identity are retained under `.local/basketball/rea/irq-raster/`.

The original `$F317` handler performs the fixed delay, selects font CHR pages at
`$F331/$F33B`, writes `$2000` to the background address at `$F343/$F348`, and
clears fine X with the first `$2005` write at `$F350`. The observer's write
callback reports the next PC (`F334/F33E/F34B/F353` respectively). Those are
observation locations, not different source instructions.

`capture_irq_raster.py` reads CPU cycle timestamps and those hardware writes;
it does not alter original execution beyond the previously declared isolated
court selection at first initialization. Complete RAM and selected PNGs equal
the configured references. The original graphics/reference captures were not
edited. `capture_irq_holdout.py` records 40 further sample positions: six repeat
existing references and the other 34 form additional image-validation cases.
All recorded RAM and PNGs equal matching runs without the graphics observer.

The installed reference is FCEUX 2.6.6 with `SDL.NewPPU=0` and `SDL.PAL=0`.
Pinned upstream `ppu.cpp`, `pputile.inc`, `x6502.cpp` and `state.cpp` explain
`ResetRL`, `GETLASTPIXEL`, the two-row prefetch and saved cycle remainder.
Their exact URLs/hashes and the actual emulator binary hash are retained in
`.local/basketball/irq-raster/source.json`. They are research evidence, not
browser dependencies.

## Clock-derived boundaries

The original slot-2 state contains `TSBS=62509272`, `ICou=-96`, `KOOK=1`.
The observer independently reports the same initial CPU cycle count. For capture
frame `n` (starting at 1), the visible scanline-193 origin in PPU dot units is:

```text
base_dot = TSBS * 3 + ICou / 16
prior = n - 1
prior_dots = prior * 89342 - (floor(prior / 2) + (prior % 2) * initial_KOOK)
current_KOOK = initial_KOOK XOR (prior % 2)
line193_dot = base_dot + prior_dots + (22 + 193) * 341 - 16 - current_KOOK
write_dot = original_CPU_write_cycles * 3 - line193_dot
fetch_boundary = floor(write_dot / 8)
```

This follows the pinned NTSC frame budget, skipped odd-frame dot, pre-render
prefetch and CPU instruction remainder. `derive_irq_timing.py` reads the save
state and cycle logs only; it never opens reference images. Font writes must
precede the first scanline-193 fetch. The two resulting semantic phases are the
background-address reset and fine-scroll reset:

| Court/frame | Address-reset dot/fetch | Fine-X-reset dot/fetch |
| --- | --- | --- |
| 3/1600 | 44 / 5 | 74 / 9 |
| 3/1580 | 53 / 6 | 83 / 10 |
| 3/1590 | 53 / 6 | 83 / 10 |
| 4/1750 | 41 / 5 | 71 / 8 |

Forty-six unique timed frames are retained in `timing.json`. The derived pairs
are 5/8, 5/9 and 6/10. This explains why the original four regressions accept the
stable 5/9 projection while two additional frames require 6/10.

## Renderer behavior and verification

At the first boundary line the renderer reads the old court's tile/attribute IDs
through the HUD font atlas. It drains two prefetched rows before selecting each
new row. Address reset changes the source to the semantic HUD; fine-scroll reset
separately changes pixel selection within those rows. The first eight background
pixels remain masked. Ordinary HUD rows continue at their original coordinates.
The original result-scene rendering remains separate.

`hudFetchTiming` is an optional display parameter containing only
`addressResetFetch` (5 or 6) and `fineScrollResetFetch` (8–10). Missing timing uses
5/9. The old `hudStartLine` observation and every court/frame-specific raster
branch are removed. A test changes the timing while keeping camera, objects and
HUD identical and requires the predicted output to change; invalid timing is
rejected. The original four regressions are also checked with timing omitted.

Verification was extended before the implementation. Retained failures include
`before-renderer-result.json`, `before-pixel-failure.png`, `semantic-before.log`
and `timing-before.log`. Final browser comparison covers 180 distinct frames:

- 83 existing New York gameplay frames: 5,099,520 pixels.
- 15 original result frames: 921,600 pixels.
- 48 eight-court frames: 2,949,120 pixels.
- 34 additional Court 3/4 frames: 2,088,960 pixels.

All 11,059,200 pixels match, with no ignored region or tolerance. The independent
Python background/HUD checker passes 48 frames and 2,869,662 non-sprite pixels.
Strict renderer/client TypeScript checks pass. Original before/after image
comparisons and the new default-profile regression remain reproducible.

After `source scripts/env.sh`:

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/derive_irq_timing.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/test_courts_art.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/accept_renderer.py
```

To recreate the additional source observations, run `capture_irq_holdout.py`,
then `capture_irq_raster.py` with and without `--holdout`. Logs and source
manifests are under `.local/basketball/irq-raster/`; the full browser result is
`.local/basketball/renderer/browser/result.json`.

## Remaining boundary

These results establish exact projection from original semantic display inputs.
The Go engine does not model original CPU/APU instruction stalls or emit their
per-frame cycle phase; it uses the stable default. Thus the original four defects
are fixed directly in the normal renderer, while exact reproduction of the rare
6/10 hardware phase requires the optional measured display timing. This work does
not claim complete Go-game or physical-console timing equivalence. No game was
registered, uploaded, released or marked accepted for release by this change.
