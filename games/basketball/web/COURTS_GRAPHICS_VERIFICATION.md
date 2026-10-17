# Eight-court graphics and ordered display updates

This restores eight separate original court asset packages and the renderer
selection/patch interface. It does not assert that the Go runtime already runs
all courts, that the original menus were restored, or that CPU/PPU IRQ timing
is emulated. All exported manifests remain `accepted_for_release: false`.

## Source and contract

Original `$058B` is the court identity (0–7); `$0588` is the projection/scene
layout. All configured match observations here retain layout 1. Bank 1
`$BC74` selects tile-row tables through `$800C[court*2]`; `$BCFF` selects
attributes through `$801C[court*2]`; `$BE2B` selects CHR through `$BEC5[court]`.
The CHR start pages are `[76,68,72,92,80,84,88,64]`. Original `$D231` loads
palette preset `court+1`. The existing REA evidence for these functions is
`ev_d65d9412a7dbb3fdcf8f43042defe8e327d525d1e7283954897456d63f8bbf12`.

Dynamic court graphics are separate from streamed columns: bank 4 `$9B95`
dispatches to `$9BBF,$A32C,$9D28,$9E62,$9FBA,$9FEF,$A124,$A32C` for courts
0–7. Original video queues `$030B/$032E/$0351` commit in that order through
`$CC54/$CC96/$CCD8`. The REA evidence is
`ev_3fb4a9e82c28e78326cecbf7989d21c6afda7465ddd29c906610caa566273c7a`.
The independent Go logic is documented in `../COURT_ANIMATION_VERIFICATION.md`.

The renderer contract is:

- `snapshot.court` is required and independent of the result metadata/layout.
- `snapshot.paletteID` is optional; omitted means `court+1`. Court 4 additionally
  supports presets 9/10 and court 6 presets 11–15. Selection is explicit display
  state, never inferred from a current animation pose or a renderer clock.
- `snapshot.hudFetchTiming` optionally supplies `addressResetFetch` (5 or 6)
  and `fineScrollResetFetch` (8–10). Omission uses the stable 5/9 profile;
  the Go runtime is not required to emulate original CPU clocks.
- Call `resetCourt(court)` before replaying that court's committed display state.
  Rendering a snapshot from a different court fails rather than resetting silently.
- `commitCourtEvents` accepts ordered `{column,phase}` and
  `{patch:{plane,row,column,tiles}}` events. Plane 0 excludes reserved HUD rows;
  tile rows are 0–29 and columns 0–31. A patch cannot cross a row boundary.
  The whole batch is validated before mutation. `commitCourtColumns` remains
  compatible and delegates to the same ordered implementation.
- A later column write overwrites earlier patches in that column. Restoring two
  unordered independent histories would be incorrect. The Go display integration
  must retain order or discard superseded tile patches before compaction.

Assets under `.local/basketball/courts/` contain 8 independent manifests, 60 PNGs
(including palette variants), and 16 initial-ring/column-packet JSONs. All
images come from each court's original maps, tiles and palettes. Original
screenshots are reference evidence only. No ROM, captured RAM, OAM or PPU
framebuffer enters the browser renderer API.

## Reproducible observations and current result

`capture_courts_graphics.py` starts from the original menu timeline. At the first
`$D1B5` court initialization it explicitly selects `$058B` once; this is an
isolated configuration probe, not natural-menu coverage. All subsequent loader,
streamer, animation and gameplay execution is original. For each of eight courts,
1,750 frames of complete RAM and six screenshots match the identically configured
reference without graphics hooks. The 48 observed frames include ordered column
and animation-patch commits. They do not derive game decisions from screenshots.

The before-implementation failures are retained in `courts-before.log`,
`courts-browser-before.log` and `.local/basketball/irq-raster/`. The former
four mismatches (Court 3 frames 1600/1680/1750 and Court 4 frame 1750) are now
resolved by the original font/scroll split projection. The HUD font is selected
before the old court-address tile prefetch drains; treating this as a single
full-line transition lost those pixels.

The renderer now uses the court tile/attribute ring, font atlas, camera fine
scroll, semantic HUD and two bounded fetch phases. `hudStartLine` and all
court/frame-specific hints have been removed. The stable default fetch phases
5/9 reproduce all four original regressions without additional snapshot fields.
An optional `hudFetchTiming` carries measured original display-clock phases;
`derive_irq_timing.py` derives these from CPU timestamps and the initial saved
clock remainder without reading any reference image.

All 48 court frames, 34 additional original frames not used in the initial
projection investigation, 83 New York frames and 15 result frames now compare
all **11,059,200 pixels** exactly. The 34 additional frames include two cases
that require measured fetch phases 6/10 instead of the stable default. The Go
engine does not execute original CPU/APU cycles, so this is exact rendering
from original semantic display inputs, not proof that Go reproduces every
hardware timing phase. See `IRQ_RASTER_VERIFICATION.md` for source addresses,
clock derivation, retained failures and this remaining boundary.

The independent Python background/HUD check passes 48 frames and 2,869,662
non-sprite pixels. Ordered patch overwrite, invalid-batch rejection, explicit
fetch-timing validation, original sprite limits and 320 px rendering still pass.
Renderer/client strict TypeScript checks pass. All original screenshots remain
unchanged and all exported manifests remain `accepted_for_release: false`.

## Reproduce

After `source scripts/env.sh`, run:

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_courts_assets.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/accept_renderer.py --legacy-only
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/accept_renderer.py
```

All three commands pass with the retained evidence present.
The default command retains complete results in
`.local/basketball/renderer/browser/result.json`; the scoped regression writes
`legacy-result.json`. Eight individual court screenshots and
`courts-contact-sheet.png` show the separate graphics. Source capture identities
are in `.local/basketball/courts/capture.json`. No OSS upload or release occurred.

## Actual local client routing and patch-only redraw

The real local Go/Wasm acceptance server now allows `/assets/courts/` alongside
its existing named asset directories. It serves only JSON/PNG there and verifies
the resolved path remains inside the selected allowed directory. Direct research
fixture requests and encoded parent-directory traversal both return 404.
`asset-routes.json` retains the four actual HTTP responses.

`BasketballClient` restores compacted `CourtColumns` followed by surviving
`CourtPatches`; it invalidates the image cache whenever the court display serial
changes, even if `Visible` is identical. This depends on the Go display layer
removing patches overwritten by later columns and increasing `CourtSerial` on
patch-only commits. The client must not concatenate two unfiltered histories.

The actual browser test first failed because the court package returned 404,
and the added patch-only test first failed with `changed:false`. After the
changes, default `accept_live.py` passes 26 checks, including native keyboard and
mobile touch input, pause/resume, unchanged-picture restore, 320 px/desktop
layout, the explicit result boundary, and patch-only redraw/replay/clear-on-restore.
The patch test deliberately supplies synthetic display state through the real
client; it does not claim that the Go runtime naturally emitted those commands.

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/accept_live.py
```

No `--full-match` flag was used. Evidence is under
`.local/basketball/live-browser/`: `result.json`, `asset-routes.json`,
`patch-contract.json`, `courts-route-before.log`, `courts-route-after.log`,
`patch-before.log`, `patch-after.log`, `desktop.png`, `mobile.png` and
`result-{desktop,mobile}.png`. This earlier local-client success is separate from the subsequent exact raster
projection verification described above.

## Court 4 retained result palette

The result renderer additionally supports Court 4 palettes 9/10. Eight stable
original palette-9 result frames add 491,520 exact pixels, bringing the combined
comparison to 188 frames / 11,550,720 pixels. See `PALETTE_TIMING_VERIFICATION.md`
for selection/queue/NMI evidence, source samples and the separate unimplemented
brightness-level scope.
