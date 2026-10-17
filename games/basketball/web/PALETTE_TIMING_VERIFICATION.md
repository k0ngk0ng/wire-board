# Court 4 palette queue, boundaries and result graphics

The original Court 4 cycle selects palettes 5/9/10, queues the built colors,
and applies them during the next NMI palette transfer. Result initialization
preserves the selected palette across temporary background loading. The renderer
now supports Court 4 result palettes 5/9/10; eight original result frames retaining
palette 9 compare exactly. The four-level fade implementation is now independently verified against original
frames; see `FADE_VERIFICATION.md` and `../DISPLAY_FADE_VERIFICATION.md`.

## Sources and reusable observations

REA evidence `ev_60bb8c335abec4b399b8e7f0ee297b2c8b137d4a7038cf6272e0b6508d30a53d`
for `$94E0,$9526,$CCE4,$D231,$D420,$BA40` is retained under
`.local/basketball/rea/palette-timing/`. The existing court-animation evidence is
`ev_3fb4a9e82c28e78326cecbf7989d21c6afda7465ddd29c906610caa566273c7a`.
Source instructions and original execution are both used; decompiler output is
not treated as the original source language.

`capture_palette_timing.py` reuses the configured Court 4 opening and records
selection, build, queue, NMI transfer, phase and background-load boundaries.
It adds normal Start pause/resume inputs and a 25,000-frame ordinary-input match
through intermission and result. The only configuration injection is the already
declared Court 4 selection at first `$D1B5`; scores, timers, result state and colors
are not injected. Every observer run's complete RAM and screenshots match its
corresponding reference without palette hooks.

`analyze_palette_timing.py` verifies **3,719 normal palette commands**. Each leaves
PPU colors unchanged after queuing, then commits at `queue_nmi + 1`; committed
colors equal the queued bytes. NMI serials are observed at the original vector
`$FFF1`. Some queue and commit events share an emulator frame label, so frame-label
`+1` is not a valid substitute for the actual video boundary.

`.local/basketball/palette-timing/samples.json` contains 12 representative normal
ID transitions, queue/commit cycles and NMI ordinals, colors before/after commit,
pause evidence, intermission selection changes and complete fade buffers. The
normal full-match observations include both directions: 5→9→10→5 and 5→10→9→5.
Source SHA-256 identities and RAM/PNG equality are retained alongside the samples.

## Exact selection and commit semantics

- Bank 4 `$9FBA` advances the raw direction/frame state. `$9FEA` contains 5/9/10.
  `$9FDB` writes the selected ID to `$0304`; `$9FDE..9FE3` clears `$0301/$0302`;
  `$9FE6 → C021 → FCEC → bank4 $8006 → 94E0` builds colors.
- With delay `$0302=0`, `$94E9` uses threshold `$40` (full colors). `$9526` writes
  palette bytes to `$0377...`, count to `$0376`, and marks the fourth video queue
  at `$0374=3F`. In scene 1/phase 1, `$9612..9617` returns without waiting.
- `$CCE4` consumes that queue during NMI, and `$CD1A` writes the colors to PPU.
  `$CD23` then clears the pending queue. Selection alone must not change `Visible`.
  Staging a sprite submission must not read the new pending ID as already visible.
- Palette transfer is independent of whether there is a new sprite submission.
  Apply pending colors at video commit and then associate the committed ID with
  the visible display. The pending queue also survives pause entry until consumed;
  the palette consumer has no special cancellation for phase 2.
- Replacing a pending selection before transfer uses the latest prepared queue.
  A saved state must retain both current and pending selections; replaying an old
  submission must not restore an older palette after the new queue commits.

Example source sample: queue at frame 3511/NMI 1406 selects 9 while the committed
palette is 5; transfer at frame 3512/NMI 1407 changes the PPU to 9. All original
color bytes are available in the sample, not just the ID assertion.

## Pause, intermission, opening and result

Pause/resume `$D3EB` changes the phase only. `$9B95` returns in phase 2, so no new
animation palette is queued while paused. In the observed pause, frame 3549 enters
pause with palette 9, frame 3579 resumes, and the next cycle selects 10. There are
no palette-build/queue/commit events between those boundaries; the retained
paused screenshots have identical palette bytes.

`$D420` does not replace `$0304`: it fades the current selection out. In the full
Court 4 observation the current selection is 9 at both period resets (frames 14362
and 25508). The original intermission then loads actual menu palettes: background
presets 4/5/6 select palette 19, background 77 selects 31, and the new court's
background 0 temporarily selects 1. These are menu/background-loader states,
not Court 4 variants to send to the court renderer.

The court base selection is restored specifically at `$D231`: `$0304=court+1`.
This occurs at frame 14548 for the second opening, selecting 5. It is separate
from the earlier court-ring clear. The following opening fade builds palette 5
at frame 14554, then queues its four brightness stages. A modern host that omits
the original menus must still avoid treating a ring reset as a visible color
commit.

Result initialization saves current `$0304` at `$BA67..BA6A`. It loads backgrounds
`court+101` and 99, then restores the saved ID at `$BB36..BB37`, clears fade direction
at `$BB98..BB9A`, and rebuilds it through `$BB9D → FCEC`. For Court 4 the observed
result entry is palette 9 at frame 25518, the temporary backgrounds select 5 and
1, and frame 25528 restores 9. The result fade-in queues palette 9 at frames
25530/25532/25534/25536. Resetting the result to base 5 is incorrect.

## Result assets and pixel verification

The original result exporter had only the court base palette. Its first Court 4
comparison failed with 2,500 differing pixels. Tests were added before changing
the palette-aware exporter and renderer; failure logs are
`result-assets-before.log` and `result-browser-before.log`.

Result assets now contain the original 32 base PNGs plus eight Court 4 palette
9/10 variants. Background, HUD template, big digits and retained HUD tiles are all
selected by the committed `snapshot.paletteID`; there is no fallback from an
unrestored result palette to the base colors. All 40 assets pass hash/size checks.

`capture_result_palette.py` observes stable original result frames 23460, 23500,
23550, 23600, 23650, 23700, 23750 and 23800 after the configured full match. Complete
RAM equals the original full-run prefix, and screenshots equal the observer-free
reference. The result remains palette 9 with the original 024–035 scores.
All **491,520 pixels** of these eight frames match the browser renderer. A separate
selection check verifies that palette 10 does not silently reuse 9 or base 5.
Palette 10 has static original-ROM decoding/asset validation, not a claimed
naturally reached result screenshot.

The stable-frame renderer baseline covers 188 frames and 11,550,720 pixels,
including the existing New York, eight-court and IRQ-boundary cases. Strict
renderer/client TypeScript checks pass. The eight-frame PNG proof is
`.local/basketball/renderer/browser/result-palette-canvas.png`; complete comparison
records are in the same directory's `result.json`.

## Four-level fade boundary now verified

The committed `paletteLevel` and its pending queue now preserve the original
four-stage NES color clamp independently from sprite submissions. Fade-out uses
thresholds `30,20,10,00`; fade-in uses `10,20,30,40`. The renderer transforms the
composed original colors once through an exported RGB lookup table; it does not
use CSS opacity. Source probes prove that all 64 NES indices, including duplicate
RGB classes, preserve the index-clamp result at all five levels.

Four retained original runs provide 112 observed transition frames. Their RAM
and PNGs equal corresponding runs without the observation hooks. Five frames
still carry the preceding menu palette and are explicitly excluded from the
court renderer contract. The remaining **107 frames / 6,574,080 pixels** compare
exactly, covering New York palette 2, Court4 base 5, palette 9 court/result fades,
and a naturally reached palette 10 period exit. Palette 10 result imagery still
has static coverage only; its period-exit observation does not establish natural
palette 10 result coverage. Selective palette-group flags remain outside scope.

Combined with the stable baseline, the fade renderer gate passes
**295 frames / 18,124,800 pixels**, with zero mismatches. Subsequent Court6 result
coverage adds eight frames for 303 / 18,616,320 pixels; see
`COURT_SIX_RESULT_VERIFICATION.md`. The source provenance,
PNG hashes, exact coverage partition, generated lookup-table identity and atlas
hash are checked by `verify_fade_evidence.py`. Rebased-run logs are retained at
`.local/basketball/fade/{capture-revalidated,renderer-rebased,evidence-rebased}.log`.
The detailed evidence index is `.local/basketball/fade/evidence-verification.json`.

Reproduce after `source scripts/env.sh`:

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/analyze_palette_timing.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_result_assets.py .local/basketball/results
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/accept_renderer.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_fade_evidence.py
```

All source captures, logs and sample data remain in the workspace. No Go display
or runtime implementation was changed by this analysis/result-graphics task.
Assets remain `accepted_for_release: false`; no OSS upload or release occurred.
