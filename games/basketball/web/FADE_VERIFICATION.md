# Original palette fade verification

The implementation uses `snapshot.paletteLevel?: number`, default 4, with
explicit 0 preserved as black. Its current scope is original court fades with
flags `$0301=00/80`, which transform all background and sprite palette groups.
The selective-group flag `$40` and background-only transfer flag `$01` require
separate scope and must not silently use the global level.

Failure cases declared before implementation:

- Treating level 0 as an absent value and displaying level 4.
- Accepting fractional, negative or out-of-range levels.
- Replacing the original NES index clamp with opacity, alpha blending or an overlay.
- Darkening already-low color indices again, changing hues, or failing universal
  color/transparent sprite behavior.
- Transforming only the court while leaving HUD, actors, shadows or result digits
  at full brightness.
- Applying the transform twice across repeated snapshots, restore or palette-ID
  changes; retaining a previous fade after a full-color snapshot.
- Losing color-index information when duplicate full-bright RGB values have
  different lower-level results; the exporter must prove all equivalence classes.
- Showing a queued level before its original PPU commit (Go timing is owned by root).
- Substituting synthetic image filters for original-frame comparisons, or calling
  static palette10 checks natural frame coverage.

Required validation includes original palette-buffer output and complete original
images for fade-in levels 1/2/3/4 and fade-out 3/2/1/0, multiple courts, Court4
base/9/10, both court and result scenes, and all existing stable-frame regressions.
Reference screenshots remain immutable. Any isolated original-function probe must
be identified separately from ordinary-input match captures.

Evidence-package failure cases for the final audit:

- A successful log referring to a stale renderer, changed source PNG, different
  ROM, or stale generated palette table.
- A missing source frame counted as covered, or an excluded menu frame silently
  counted among the court frames.
- A modified atlas without an updated digest, a shortened fade case list, or a
  missing palette10 period-exit sequence hidden by an aggregate pass count.
- Asset export accidentally marking these development resources release-ready.


## Completed pixel and asset verification

The implementation renders the complete frame at full original color, then
applies one exported 54-entry RGB lookup table for levels 0–4. All 64 original
NES color indices are represented; duplicate RGB values have identical outputs
at every level. This proves that the RGB mapping preserves the original index
clamp without requiring runtime ROM bytes. HUD, sprites and result digits share
the same transform. Omitted levels restore full color; repeated frames do not
accumulate darkening. Invalid levels are rejected.

The isolated `$9526` oracle contains 80 original calls: palettes 2/5/9/10, two
team pairs, both fade directions and five levels. Its three focused LUT tests
pass. This isolated function evidence is separate from the original full-run
screenshots below.

| Original run | Observed frames | Court-renderer frames | Scope |
| --- | ---: | ---: | --- |
| New York opening | 8 | 7 | Palette 2 opening |
| New York full match | 40 | 39 | Palette 2 court and result transitions |
| Court4 full match | 48 | 46 | Base 5 openings, palette 9 court and result transitions |
| Court4 pause/resume match | 16 | 15 | Base 5 opening, naturally reached palette 10 period exit |
| Total | 112 | 107 | Five prior-menu black frames explicitly excluded |

Observation hooks leave all RAM and screenshots identical to their corresponding
reference runs. The Court4 selection is configured only at initial court setup;
normal inputs and normal clocks reach the observed transitions. The palette10
run uses ordinary pause/resume to change the later palette phase, without
injecting colors, scores, period state or remaining time. All 107 selected images
match Canvas byte-for-byte: **6,574,080 pixels**, zero residuals. No renderer
correction was required when revalidating the rebased checkout.

The fade-gate renderer baseline covers **295 frames / 18,124,800 pixels**:
83 original gameplay frames, 48 court frames, 34 IRQ holdouts, 15 New York result
frames, eight palette9 result frames and 107 fade frames. IRQ comparisons retain
the explicitly observed fetch timing; this is not Go CPU-cycle emulation.
The subsequent Court6 result gate adds eight natural frames, for a current total
of 303 frames / 18,616,320 pixels; see `COURT_SIX_RESULT_VERIFICATION.md`.

The final evidence audit checks source ROM/scenario identities, source PNG
hashes, exact inclusion/exclusion sets, palette/direction coverage, exported LUT
identity and sprite-atlas hash/size. Separate existing asset checks pass for all
40 original result images (subsequently expanded to 60 by the Court6 result gate)
and all 60 court images plus 16 JSON resources. Assets remain
`accepted_for_release: false`. Palette10 result frames, other court fades,
selective-group fades and complete Go-world/original-state equivalence are not
claimed by this gate.

After `source scripts/env.sh`, reproduce with:

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_fade_graphics.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/test_palette_fade.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_result_assets.py .local/basketball/results
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_courts_assets.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/accept_renderer.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_fade_evidence.py
```

`capture_fade_graphics.py` reuses terminal captures and rechecks reference equality;
it does not rerun completed original observations. Actual passing logs are in
`.local/basketball/fade/`: `capture-revalidated.log`, `lut-rebased.log`,
`result-assets-rebased.log`, `court-assets-rebased.log`, `renderer-rebased.log`,
and `evidence-rebased.log`. `evidence-verification.json` binds the coverage report
to current inputs. Full per-frame comparisons and PC/320px screenshots are in
`.local/basketball/renderer/browser/`. No OSS upload or release was performed.
