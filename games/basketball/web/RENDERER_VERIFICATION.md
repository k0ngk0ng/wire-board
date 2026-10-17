# Independent Canvas renderer verification

This is a renderer test, not a playable Go game or complete reconstruction.
No ROM, CPU interpreter, captured OAM, or captured PPU image enters the renderer API.

Failure cases declared before implementation:

- Incorrect pose/facing/head/team palette or ball CHR selection.
- Incorrect origin, shadow height, object draw order or global sprite reversal.
- Ignoring the original 64-fragment capacity, eight fragments per scanline, clipped fragments or transparent pixels.
- Treating camera pages as 256 visible rows rather than 240, or dropping committed court-column timing.
- Rendering captured HUD screenshots rather than original glyphs driven by displayed numbers and semantic labels.
- Accepting invalid semantic states or missing assets silently.
- Desktop/mobile resizing changing intrinsic pixels, smoothing original sprites or overflowing a 320 px viewport.

The browser acceptance fixture receives original observed semantic draw snapshots,
committed court-column events and displayed HUD values. It compares all 83 frames
with original screenshots, with no pixel mismatch tolerance. Any unrecovered
raster behavior must remain a failing case or an explicit coverage limitation.
Independent changes to scores/time and a 320 px viewport verify reuse beyond
replaying recorded images. Full game execution, input/physics, audio and multiplayer
are outside this renderer's acceptance scope.

## Original result scene

Result graphics use the original compressed background descriptors for eight
courts, the six-row preset 99 HUD, large decimal glyphs, and 120 kind-17 poses.
They are decoded offline into 32 base PNG assets plus profiles, with eight
additional Court 4 palette 9/10 variants; no runtime ROM or
captured framebuffer is required. The sprite atlas includes page 13 selected
by original result initialization at bank 4 `$BCC1..BCC3`.

REA result-art evidence is
`ev_ab418396a3f1810276537e011a3796d55f09349aa4b344c76354cb16618ac289`.
Its decompiler has non-return/indirect-call limitations; compressed commands
and digit placement were also checked against source instructions and original
runtime graphics. `capture_result_graphics.py` records a naturally reached
result scene without RAM writes; all RAM and selected screenshots match the
uninstrumented reference.

Tests were added before implementation. Fifteen original New York result frames
(22120 through 22500) compare all 921,600 pixels exactly, including actors and
the last HUD scanline retained from the court ring. The previous 83 gameplay
frames still compare 5,099,520 pixels exactly. Background-only Python checks
compare 896,051 non-sprite pixels. Other courts have static asset/hash/size
coverage, not natural runtime pixel coverage.

Result metadata distinguishes initialized phase 6 from the previous court
submission during entry. Kind 17 without initialized result metadata, metadata
outside phase 6, an invalid result camera and a wrong initialized result CHR
page are rejected. The exact entry boundary with phase 6, no result metadata,
empty draw order and all poses hidden may already carry CHR 13 after BA40;
ordinary court submissions still require pages 32–35. The result HUD uses original decimal caches, not a score
reformatted from unrelated current state. Court-ring history survives entry.

Reproduce from the workspace after `source scripts/env.sh`:

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/test_result_art.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_result_assets.py .local/basketball/results
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/accept_renderer.py
```

Retained evidence is in `.local/basketball/rea/result-art/` and
`.local/basketball/renderer/browser/` (`result.json`, `result-canvas.png`,
desktop/mobile screenshots, requested asset paths and responsive measurements).
This proves the renderer against original semantic inputs, not a natural
end-to-end Go match. Assets remain `accepted_for_release: false`.

Court 4 retained-result palette coverage and the separate outstanding fade scope
are documented in `PALETTE_TIMING_VERIFICATION.md`.
