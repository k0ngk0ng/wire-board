# Court 6 result palette verification

Failure cases declared before export changes:

- Result entry resets the selected palette to base 7 instead of preserving
  the source `$BA67/$BB37` value across temporary background loading.
- One of palettes 11–15 is missing or silently uses base 7 or another variant.
- Background, score HUD, large digits or retained final HUD row use different
  palettes, or generated files no longer match their manifest hashes.
- Static palette-table decoding is described as a naturally reached result.
- New resources break the existing 295-frame original pixel regression or
  accidentally become release-ready.

The existing original Court6 match retains palette15 in the final result.
Its normal-input timeline and complete RAM are already present in
`court-six-slot-complete-reference`; additional screenshots and passive draw
observations will reuse that exact timeline. No score, timer or result injection
is allowed. Palettes 11–14 remain static-source coverage unless separately
observed in a natural result.


## Source confirmation

Retained REA evidence
`ev_60bb8c335abec4b399b8e7f0ee297b2c8b137d4a7038cf6272e0b6508d30a53d`
contains the original result entry `$BA40`. The original bytes at `$BA67` are
`AD 04 03 48` (`LDA $0304; PHA`); `$BB36` is `68 8D 04 03`
(`PLA; STA $0304`). Between them the loader selects result background
`court+101` and preset 99. There is no Court4-only condition around the saved
palette. Court6 therefore must also retain its live palette, including 11–15.
The palette pointer table is bank4 `$962F`; all variants are decoded from those
original pointers. This static evidence establishes the mechanism separately
from the natural result observation.

## Implementation and static-source coverage

The result exporter now emits 60 images: the previous 40 plus five Court6
variants, each with background, score HUD, large digits and retained HUD tiles.
The existing renderer already selects all four resources by committed
`snapshot.paletteID` and rejects missing variants. No renderer implementation
change or fallback was needed.

Before changing the exporter, the asset contract and browser contract both
failed for missing Court6 coverage. Logs `result-assets-before.log` and
`result-browser-before.log` preserve those failures. After export all 60 files
pass RGBA dimension and SHA-256 checks.

`probe_court_six_result_palettes.py` independently executes the original `$9526`
120 times: palettes 7/11/12/13/14/15, two team pairs, in/out and levels 0–4.
`verify_court_six_result_art.py` compares all output buffers to the original
clamp and compares six palettes' four image roles to separately decoded captured
PPU patterns/nametables and those original palette buffers. **24 images and
858,624 pixels** match exactly. These are static recolorings of original result
PPU data; they are not naturally reached screenshots for palettes 11–14.

## Natural palette15 result and browser regression

The existing 26,000-frame Court6 reference already contains the final result
with palette15 and score **009–026**. The new reference stops at frame 25,600,
using the same initial selection and exact controller timeline. Its full RAM
matches the prior run prefix. Eight snapshots at frames 25240, 25280, 25320,
25360, 25400, 25480, 25540 and 25600 are taken after the result fade finishes.
Passive rendering hooks leave all RAM and PNG bytes equal to the new reference.
Neither scores, palette, result state nor clocks were injected; only the original
Court6 selection at first setup is configured, as documented in the parent
Court6 investigation.

All **491,520 pixels of the eight natural result frames** match the browser.
The earlier 295-frame suite remains green, yielding **303 original frames /
18,616,320 pixels** with zero mismatches. Court6 result frames are reported in a
separate `resultCourtSixFramesDetail` field, preserving earlier category counts.
The browser also renders every supported Court6 result palette and requires the
exact manifest set `[7,11,12,13,14,15]`.

Actual logs and the source-backed image report are in
`.local/basketball/rea/court-six/`:

- `result-source.log` and `result-capture.json`: two completed original runs,
  complete RAM identity and eight observed frames.
- `result-palette-source.log` and `result-palette-oracle.json`: 120 original calls.
- `result-art-source-comparison.log` and `result-art-verification.json`: 24 images.
- `result-assets-after.log`: 60 exported result images.
- `result-browser-after.log`: the complete 303-frame browser regression.

Full per-frame results and `result-court-six-canvas.png` are in
`.local/basketball/renderer/browser/`. Reproduce after `source scripts/env.sh`:

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_court_six_result.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/probe_court_six_result_palettes.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/export_result_assets.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_result_assets.py .local/basketball/results
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_court_six_result_art.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/accept_renderer.py
```

The capture script reuses completed observations. No Go/runtime code, platform
selection gates, OSS assets or production registration are changed here. All
exported resources remain `accepted_for_release: false`.
