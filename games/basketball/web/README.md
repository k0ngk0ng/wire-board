# Basketball web client and original-art Canvas renderer

`client.ts` connects the real Go/Wasm runtime, semantic Canvas renderer,
per-user controller mapping and platform `GameAudioPort`. The host supplies
video ticks through `advanceVideo`; it must not also call `StepLogic`.
`checkpoint`/`restore` preserve the runtime display and do not replay old
sounds. `controllerSink` connects the shared FC controller component.

The protocol in `runtimebridge` owns one match per session. A confirmed
`state.outcome` parks that session on its last submitted result image, retaining
the original next-scene metadata without entering unimplemented campaign/menu
logic. Calling `advanceVideo` again preserves the result. This is a match host
boundary, not an implementation of the remaining campaign.

Run `python3 scripts/basketball/accept_live.py` after `source scripts/env.sh`
for the actual Go/Wasm browser segment; evidence is retained in
`.local/basketball/live-browser/`. Add `--full-match` to run the cold default
New York match through both halves, native Start confirmation and the final
result; its artifacts are in `.local/basketball/full-match-browser/`. That
scenario passes at 22,383 video ticks with score 12:45. This remains a
development client: other teams/courts and platform multiplayer acceptance are
not covered by that result. Add `--all-computer` alongside `--full-match` for
the separate four-computer case; it passes at 22,512 video ticks with score
45:49, with evidence in `.local/basketball/all-computer-full-match-browser/`.

## Renderer

`renderer.ts` is a presentation module for the recovered basketball game. It
loads exported PNG/JSON assets and accepts semantic display snapshots. It does
not execute a ROM, decide actions, advance physics, or represent a completed
playable game. The public API does not accept RAM, OAM or PPU snapshots.

```ts
import { loadBasketballAssets, BasketballRenderer } from './renderer';

const assets = await loadBasketballAssets('/basketball-assets');
const renderer = new BasketballRenderer(canvas, assets);
renderer.commitCourtColumns(displayEvents.courtColumns);
renderer.render(displaySnapshot);
```

The asset directory contains `static-atlas/`, `static-objects/`, `court/` and
`raster/`. The raster package supplies original 8×8 fragments so the renderer
can reproduce the 64-fragment capacity and eight-fragments-per-scanline limit;
whole-player PNG composition alone cannot do this. Asset versions must match.
The renderer sets a 256×240 intrinsic canvas and disables interpolation.
The caller owns responsive layout and platform integration.

## Display boundaries

- Pass `ballCHR` from the display state. It is not computed from the current
  logical ball pose. The original selector and draw list operate at different
  points in the update sequence; scene phase 6 also retains the old selection.
- `projected`, `pose`, `position.height`, `drawOrder`, `reverseSprites` and the
  camera must describe the same submitted display. The image-sheet anchor is
  `projected + [0, 24]`. Inputs use integer projected pixels and 1/256-pixel
  physical positions; the renderer does not reproject current logic positions.
- `commitCourtColumns` applies actual committed column events, not merely camera
  intentions. Calling `resetCourt` restores the original New York initial ring.
  The source packets preserve the original partially refreshed edge columns.
- `hud.clock` and `hud.scores` are displayed values. Each panel supplies three
  nine-glyph label lines, three edge indicators and nine header glyphs for the
  original Japanese name accents. The old fixture-only `turnMarker` fallback
  is not a controller marker. Unicode name conversion is not
  implemented here. Numeric scores and time are laid out from numbers; `clockLabel` preserves
  the original period/overtime label. The Go bridge in `engine/display.go`
  generates names and team labels from original tables and supplies separate
  controller-assignment metadata.
- Phase 6's alternate HUD and behind-background sprite priority are explicit
  errors. Current assets and verification cover gameplay layout 1 / New York.

## Reproducible browser evidence

First generate the offline assets and observations in the authorized workspace:

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/export_static_atlas.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/export_static_objects.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/test_static_court.py > .local/basketball/rea/static-background/tests-after.log 2>&1
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/export_static_court.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/export_raster_assets.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_renderer.py
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/accept_renderer.py
```

The last command owns an isolated headless agent-browser profile and local
fixture server, and closes both. It compares 83 original screenshots, all
5,099,520 pixels, without exclusions or mismatch tolerance. Forty-seven original
frames exercise scanline overflow. Separate browser cases change scores/time,
select a different display CHR page without changing the pose, exercise the
64-fragment cutoff, reject invalid poses, and check a 320 px viewport.

The six selected first frames begin at a loaded-save boundary where the visible
ball precedes the logical ball. The offline fixture adapter uniquely recovers
that previous display from its original fragment sequence and shadow. This is
recorded separately in `initial-display-recovery.json`; it does not recover or
claim the old fractional physics state. All later selected frames use observed
original draw-stage semantic inputs directly.

Artifacts are in `.local/basketball/renderer/browser/`: `result.json`,
`requests.json`, `responsive.json`, `desktop.png`, `mobile-320.png`, and the
unscaled `canvas.png`. This proves the renderer and its original assets, not a
Go-driven full game, input responsiveness, audio, networking or release
readiness. Exported manifests retain `accepted_for_release: false`.
