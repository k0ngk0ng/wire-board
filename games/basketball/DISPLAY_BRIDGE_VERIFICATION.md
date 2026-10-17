# Semantic display bridge

The Go bridge keeps the submitted drawing, visible drawing, selected ball
resource page, HUD values, terrain shadow cache and committed court columns
separate. Runtime functions read only semantic engine state and generated
original data tables; no fixture, RAM, OAM, savestate or ROM is read at runtime.

## Original evidence

`capture_display.py` observes the original opening and the existing 6,000-frame
match without changing input or memory. Complete RAM and screenshots remain
identical to their uninstrumented references.

- 2,083 `CE09` draw inputs match object kinds, characters, packed poses,
  projections, positions, airborne state, original action, draw order and
  terrain-derived shadow suppression.
- 2,083 `F049` calls match ball CHR selection, including the original hidden-pose
  mapping. The raw `0x86` retention branch is preserved from static source; these natural records do not enter that special phase.
- Four original character-name conversions and 1,020 tactic-label conversions
  match the original upper/lower glyph rows.
- All 14,845 actual terrain-cache writes in the long and rider traces match
  `ObserveDisplayMotion`. They use the physical step's old horizontal point;
  attached following, a jump revisit without motion and passage bypasses do not
  emit a write.

`extract_display_profiles.py` generates eight teams × four character names
from bank 4 `B24E`, thirteen tactic labels from `B33F`, and eight 13-glyph team
labels from bank 3 `B6A7`. Original `C13F/C03F` glyph conversion preserves
voiced/semi-voiced marks. Team labels are retained as metadata: the ordinary
court HUD displays player names, not a newly invented team-name layout.
Controller assignments are separate metadata and are never confused with the
Japanese glyph marks along the upper border.

## Runtime order

1. At a main-loop display boundary, convert the previous completed world with
   `DisplaySourceFromOpening` or `DisplaySourceFromWorld`, then call
   `StageDisplay`. This submits the original draw order and pose/projection
   values, resets sprite reversal and runs the explicit ball-page selection
   before the next gameplay update.
2. Consume real `MotionSurfaceWrites` with `ObserveDisplayMotion`. Do not infer
   shadow suppression from the final position or emit a write for every actor
   visit indiscriminately.
3. Call `ObserveDisplayHUD` only when the original HUD routine executes through
   its even-logic-frame gate. It updates only the selected clock/side slot and
   never advances the clock or recomputes gameplay scores itself.
4. Queue camera column requests with `QueueDisplayColumn`. At the video commit,
   `CommitDisplayVideo` publishes submitted values and the pending column,
   then applies the crowding/no-detached-hoop reversal for the next display.
5. A real court background reload calls `ResetDisplayCourt`; it changes the
   epoch and clears the ring replay state without clearing terrain state that
   the original loader retained. Original `B39A` initialization uses
   `InitializeOriginalDisplayHUD` to rebuild name/period labels, the 3:00
   template and retained second-half decimal scores.

`DisplayState` is JSON-restorable. Its chronological `CourtColumns` list retains
only the most recent write per physical column, at most 32 entries. Starting
from the exported initial ring and replaying that list restores the final tiles
and attribute groups. A reconnect is not a real court reset; replay display
columns only, never replay old sounds.

All bridge functions copy fixed arrays and allocate replacement slice storage
when it changes. A failed step must not mutate the input state's backing data.

## Validation and limits

Native race and Wasm targeted checks pass. The Canvas browser regression still
matches all 83 original frames / 5,099,520 pixels and additionally checks multiple
name accents and the second-half clock label. Source and result records are in
`.local/basketball/rea/display/`.

These are original phase tests plus renderer tests. They do not prove the entire
Go/video/display schedule against every original frame. Scene phase 6's special
HUD remains an explicit limitation, as do unrecovered gameplay stages. All
exported visual assets remain `accepted_for_release: false` pending complete
playable-game visual acceptance.
