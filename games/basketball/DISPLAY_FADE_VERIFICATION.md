# Original four-level court fades

The Go display now preserves a committed palette level and a pending palette
level independently from sprite submissions. A missing level means full color
for older checkpoints; explicit zero is the original all-`0F` palette and remains
present in JSON. The browser field is `paletteLevel` in the range 0–4.

## Original evidence

`export_palette_fade_timing.py` derives four complete sequences from the retained
Court 4 original run: first-period exit, second opening, second-period exit, and
result entry. Sixteen queued color buffers are checked against the observed PPU
colors and original threshold clamp. The first queue commits at the next NMI;
subsequent levels commit two video interrupts apart. Source identities and all
observed queue/commit ordinals are in `engine/testdata/palette-fade-video*.json`.

The source palette builder `$9526` clamps all 32 court colors. A nonzero level
sets threshold `level*16`; colors at or above that threshold become
`(color&15)+threshold-16`. Level zero becomes `0F`. This is not alpha blending.
Other palette-builder flags that restrict affected groups are outside this court
fade contract and must not be silently approximated with the same field.

## Runtime behavior

`DisplayPaletteFade` records the suspended builder's elapsed video commits.
Fade-in commits levels `1,1,2,2,3,3,4,4`; fade-out commits
`3,3,2,2,1,1,0,0`. Each initial queue is invisible until the next video boundary.
Old submitted sprite images cannot reset a newer committed palette. Checkpoints
retain pending colors and the exact sequence cursor; malformed levels and
inconsistent pending/committed cursors are rejected.

Cold opening remains black before the original opening fade. D231 selects the
court base palette while retaining black; the referee opening queues the fade.
Period exit keeps the preceding court submission while fading it to black.
Result entry preserves the selected court palette, loads the result background,
and fades in before the first normal celebration sprite submission. Its original
eight-video wait was missing previously and is now explicit. Result confirmation
continues to retain the final picture for the platform; it does not visually
leave for the original menu.

## Completed engine validation

Tests and source fixture extraction preceded implementation. The initial API
failure, malformed-state failure, and subsequent runs remain in
`.local/basketball/palette-timing/runtime-fade-*.log`. Existing display regression
found one stale Court4-rejection assertion; Court4 already has independent full
integration evidence, so the assertion now rejects Courts5–7.

- Four source-derived sequences cover video commits, black JSON encoding,
  sprite-independent updates, stale submissions and restore at every boundary.
- Cold opening on all five currently offered courts and period/result boundaries
  are exercised through the actual Go runtime.
- Scoped display/runtime regression passes Native, race and Node Wasm.
- Six complete configurations (Court1 cold/all-computer and Court4
  cold/all-computer/cooperative/versus) each complete twice with normal clocks,
  fixed seed and periodic JSON recovery. Result fade adds eight video ticks;
  logic update counts and scores remain unchanged from their prior runs.

The separate keyboard/touch carrying acceptance now also verifies nondirectional
B set-down and picking up the same prop again: 41 checks pass in
`.local/basketball/engine/carry-setdown-browser.log`.

## Rendering validation status

The real Go/Wasm/Canvas test `accept_live.py --court 4 --fade` was added before
renderer integration and correctly fails because level zero still displays full
colors. Its retained record is `live-fade-before.log`. After renderer integration, the
opening test passes33 checks. The complete Court4 all-computer run passes41
checks, including all20 committed levels from two openings, two period exits
and result entry. Each four-step sequence has distinct actual Canvas pixel
hashes at two-video intervals; both exits are visibly black and result entry
fades the result background. Mid-fade and final-picture restoration pass.
Evidence is `live-fade-full-timeline.log` and
`.local/basketball/all-computer-full-match-browser-court-4/natural-fades.json`.
Original screenshot pixel regression remains a separate in-progress gate;
passing timing and browser flow alone does not prove all original pixels.

```sh
source scripts/env.sh
python3 scripts/basketball/export_palette_fade_timing.py
go test ./games/basketball/engine -run '^(TestDisplayFade|TestRuntimeOpeningUsesOriginalFade|TestRuntimePeriodAndResultFade)' -count=1 -v
python3 scripts/basketball/accept_live.py --court 4 --fade
```

No production registration, OSS upload or release is included.
