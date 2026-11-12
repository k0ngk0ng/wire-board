# Instrument alignment failures (before CSS changes)

- Desktop instruments sit 60 px above the lower-left map instead of sharing its bottom edge (actual `gauge-layout-before-01` browser red).
- A narrow mouse viewport inherits the mobile gauge bottom offset without moving the map.
- Touch instruments and the upper-left map use independent top offsets and drift when F7 changes the mirror.
- Landscape touch instruments stay bottom anchored while the map is top anchored; aligning them can overlap the HUD, mirror or pedals unless those regions are checked together.
- F5, viewport changes or fullscreen leave an invisible box covering a required control.
- Lowering desktop instruments lets the platform chat button cover the tachometer rim; the first aligned-layout capture measured 1165 px² of outer-box overlap.
- Landscape mirror layout wraps the rank value onto a second line or clips the track title.
- Small analog gear text without a defined background loses contrast against bright road textures.

Use actual browser rectangles and full compositor PNGs for desktop, 320 px portrait and 844×390 touch landscape, with F7 off/on. Desktop bottom and mobile top differences must be at most 2 px; preserve visible HUD, mirror and touch controls. No new unit tests for CSS declarations.
