# Original court terrain recovery

## Failure scenarios recorded before implementation

- Court selector $058B is confused with Scene/layout $0588; all on-court projections use Scene 1.
- All eight courts accidentally reuse New York's two platforms or rectangular floor approximation.
- Raw cell travel bits are discarded, or treated as height rather than low-three-bit attributes.
- Current/candidate X pages or depth rows are mixed at cell/page boundaries.
- Query coordinates outside the extracted original row-table extent silently wrap into unrelated ROM data.
- The old floor cache uses the candidate point rather than the old position.
- Court 5/7 platform-edge flag 0x40, low-byte-only height comparisons, depth-byte increments and action-14 depth-velocity writes are omitted.
- Travel bounds lose fractional coordinates or reset vertical velocity/gravity unnecessarily.
- Player-only human boundary action cancellation leaks to computers/ball/props.
- Original boundary equality and right-page handling are replaced by symmetric clamp assumptions.
- Isolated original instruction runs are mislabeled as natural gameplay captures.
- New profiles are wired to runtime before special floor responses, auxiliary mechanisms and rendering are restored.

## Scope and integration plan

Add independent semantic court profiles, full raw-cell queries, the 89FB terrain-probe side effects, and the 8021..80C6 travel-boundary block. Keep existing New York production constructors and all live court gates unchanged.

`FlatCourt` currently represents the New York platform geometry and passage response; replacing only its floor constants cannot restore other courts. Future integration must preserve object-specific player/ball responses, original travel attributes, ownership/rider side effects, and the court-specific 8458 dispatch before widening Runtime configuration.

Current hardcoded entry points: `opening.go`, `opening_world.go`, `runtime.go`, `world_preparation.go`, `world_actions.go`, `auxiliary_rest.go`, `player_terrain.go`, `shot.go`, `display_sources.go`, and the compatibility entry points in `ball.go` / `possession_step.go`. `Projection.LayoutID` is Scene, not Court, and must remain independent.

Original bank 4 `89FB` selects `8B1F[court]`, then an X-page table, a depth/8 row and X/8 cell. High five bits encode height; low three bits encode travel attributes. Special probes for courts 5/7 operate on original position bytes and retain the original byte wrapping rules.

The `8458` dispatch targets for courts 0..7 are `84D6,84D6,846E,85B5,84E4,863C,84D6,84D6`; courts 2/3/4/5 therefore require additional independent device/prop responses beyond static terrain. These are not claimed restored by the lookup/bounds work.

## Restored independent interfaces

- `OriginalCourtProfile(court)` returns the original selector, decoded row count and exclusive playable depth limit.
- `OriginalCourtSurface(court, x, depth)` returns the raw `CourtSurface`; `Height()` and `Travel()` preserve the separate interpretations.
- `ProbeOriginalCourtTerrain(state, court)` returns old/new/cross-axis samples plus the exact 89FB edge flag, depth and velocity side effects. Samples are captured before the special depth mutation, matching the original row pointers.
- `ClampOriginalCourtTravel(state, court, object)` applies the original pre-terrain boundary block and only its human-player action cancellation. It does not apply terrain response or integrate motion.

Decoded source row counts for courts 0..7 are `[10,12,10,11,10,11,9,11]`; playable exclusive depth limits are `[73,88,73,88,80,81,72,81]`. Table extent is not the playable boundary. Ball/auxiliary X limits differ from player/hoop limits. All public queries reject unknown court/object IDs and positions outside the decoded original table rather than silently falling back to New York.

## Evidence and validation

Existing REA terrain evidence `ev_b618a1d77454e328308befc87f536eeead9325fd771af4f40edcab7c95578f5e` covers bank-4 8021/8249/89FB; plateau evidence `ev_d1a2ac6742dc81078ff4fda47d598dcbb0492134a4a44086dfa918b553789b8a` covers surface/side response. The dispatch table and byte comparisons were additionally checked against original decoded instructions, because Ghidra's linear analysis crosses inline tables in 802F and 845E.

`scripts/basketball/verify_court_profiles.py` produces an independent bounded py65 oracle from original bank-4 code. The inputs are deliberately synthetic cell/platform/boundary cases, not natural gameplay. It does not mutate a running emulator. Production Go contains only semantic logic and extracted immutable terrain cells; it does not load ROM, fixtures or a CPU interpreter.

All 29,960 terrain probes and 19,968 object boundary cases match original instruction execution, including 2,979 platform-edge effects. The fixture is `engine/testdata/court-profiles.json.gz`, SHA-256 `877401a8ad26132f62c6765faabe09b6b4193debe767000935a3b6a0d7884e6c`; its provenance explicitly sets `natural_gameplay: false`.

New York's 12×64 old-floor cache matches the existing display surface table cell-for-cell. Its existing natural plateau-motion fixture still passes. The unchanged cold New York complete match repeats identically twice: 22,383 video ticks, 7,435 logic updates, 84 save/restore comparisons, score 12–45 and trajectory SHA-256 `3aff241fb5090fc176c9ea579929e9113d89f3797145e7d76e28b662ce65629a`.

Native race and Wasm profile tests pass. Logs are `.local/basketball/engine/court-profile-{before,after,race,wasm}.log`. Reproduce:

```sh
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH="$PWD/.local/basketball/python" python3 scripts/basketball/verify_court_profiles.py
source scripts/env.sh
go test -race ./games/basketball/engine -run '^TestOriginal(EightCourt|CourtProfile)' -count=1 -v
GOOS=js GOARCH=wasm go test ./games/basketball/engine -exec="$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^TestOriginal(EightCourt|CourtProfile)' -count=1 -v
```

The lookup-only increment did not enable non-New-York courts. The following floor-phase section records the subsequent 8249/8458 work; complete motion/runtime integration and natural multi-court matches remain separate prerequisites.

## Floor/side phase failure inventory (before implementation)

The next independent block restores 8249 through its own returns, not a replacement of current New York motion. Its working state explicitly distinguishes the current floor point, pending XY travel, integrated height and previous integer height.

Failure cases: selecting floor before the 89FB edge mutation; reloading row pointers after mutation; treating negative/high-page height as ordinary signed comparisons where original code checks only a byte; losing edge/air bits; snapping old versus candidate fractions incorrectly; applying depth travel to held/pass ball actions; leaving rolling ball velocity after a flagged edge; symmetric side clamps; missing ball deflection requests; New York passage entry failing to detach riders or release held ball; omitted side-collision bounce/action changes; court-device requests conflated with completed device animations; auxiliary wrap failing to preserve RNG order or detach riders; unrelated objects altered; errors partially mutating the input.

Natural evidence will observe 8249 entry/return and verify the full uninstrumented reference RAM/screenshots. Additional non-New-York coverage will use bounded original instructions with explicit synthetic configurations and will not be labeled as natural whole matches.

## Floor/side/device phase implementation

`ResolveOriginalCourtSurface(CourtFloorState, object, court)` now restores 8249 and the floor, side, passage-entry and device subroutines it calls. `CourtFloorObject.Body` contains the current floor point and already integrated height; `Travel` is pending XY; `PreviousHeight` is the saved integer height before integration. This is a deliberate phase boundary, not a second authoritative playing world. The resolver does not integrate motion or commit pending XY.

The resolver preserves all 13 objects, air/edge flags, requests, rider links, jump phases, shared court/passage flags, RNG and last-shooter state. It emits original sound requests plus the old floor cache values. Court flags are outputs consumed by the separate court-animation stage; setting a trigger flag does not execute that animation.

Restored behaviors include floor equality/landing, negative/high-page height branches, previous-floor fallback, raw depth travel attributes, rolling-ball edge stops, side/depth collisions, ball deflection requests, New York passage entry and rider detachments, generic side bounces, and all eight 8458 dispatch destinations. Device details remain distinct: courts 2/5 trigger separate switches, court 3 sets gate/facing state, and court 4 moves/wraps auxiliary objects with original RNG selection and rider detachments. Court 4 clears only the horizontal velocity's integer byte at wrap, preserving its fractional byte. Court 5 can release a ball owned by a non-player object; courts 2/3 restrict that particular release to player objects.

### Natural and configured evidence

`scripts/basketball/capture_court_floor.py` observes natural 8249 entries/returns without writing RAM. The 6,000-frame reference's complete final RAM and PNG remain identical. Its 14,458 full calls cover objects `[2000,1952,1967,1986,1998,1998,1178,639,0,740,0,0,0]`; all represented fields, other-object effects, old-floor cache values and sounds match. Fixture SHA-256: `2d60d976b3f672c42538d87fdf2c4da611e70674550a511368d821130f5f70e5`.

`scripts/basketball/verify_court_floor.py` independently executes original 8249 with bank 4 and fixed bank 7 under 68,094 configured synthetic states: 5,238 cases per object across eight courts. It intercepts only the C000 audio request sink. Its metadata explicitly records `natural_gameplay: false`; this is not eight natural complete matches. All cases match. Instrumented original-code coverage includes 840 previous-floor fallback entries at 83CC, 340 rider detach calls at 89AE and 5,557 device action calls at 89E6. Fixture SHA-256: `792e802a0ba0ece4a4d8cb213c1a6f0e845e4913683a1a5960fc88daf09b63f8`.

Both floor suites pass native race and Wasm. Existing New York plateau motion still passes unchanged. The cold complete New York match again repeats with 22,383 videos, 7,435 logic updates, 84 restoration comparisons and score 12–45. The serialized trajectory now includes explicit display Court metadata and has repeatable SHA-256 `32c8fa757593a3be5792020bc451e84b434671e7efea109e9d9c157429c8089c`.

Evidence logs: `.local/basketball/engine/court-floor-{before,after,race,wasm}.log`. Natural and configured fixtures retain separate provenance and must not be combined into a natural-gameplay coverage claim.

### Remaining integration boundary

Existing New York callers are not replaced. An eventual complete motion adapter must preserve this original order: attached-object follow bypass; integration and previous-height save; 8021 object boundary handling; the existing-active New York passage path where applicable; this 8249 surface resolver; final pending-XY commit. New York passage *entry* is restored here, while active passage *advance* remains outside this function. The adapter must preserve the full edge/air byte and court-animation flag aliases, not reduce them to `Player.Airborne` or copy two independently advancing state stores.

DisplaySource and RenderSnapshot now carry explicit `Court` (`court` in JSON), sourced from the transition or world action context. Eight-selector display transport/restore is tested. The subsequent Court 0 integration below opens that additional runtime selector; other non-New-York runtime selectors remain rejected. Wider rendering metadata alone does not open gameplay.

Reproduce phase verification:

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_court_floor.py
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH="$PWD/.local/basketball/python" python3 scripts/basketball/verify_court_floor.py
go test -race ./games/basketball/engine -run '^TestOriginalCourtFloor' -count=1 -v
GOOS=js GOARCH=wasm go test ./games/basketball/engine -exec="$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^TestOriginalCourtFloor' -count=1 -v
```

Focused REA device batch `ev_4f740cca747dd4aad8894e0f9f027a601b02c4f9dfd1fa5454b37b39f44e693a` in `.local/basketball/rea/court-floor-devices/` independently decompiles 846E, 85B5, 84E4, 863C, 89AE and 89E6. Five addresses decompile successfully; 846E remains an explicit provider failure in this batch. Its exact instructions and original execution are covered by the configured oracle rather than a claimed recovered decompilation. The implementation preserves byte-level effects where decompiler prose loses detail.


## Complete motion wrapper and Court 0 runtime integration

`StepOriginalCourtMotion` restores the complete EEFF entry: attached integer following, fixed-point integration, object-specific travel bounds, active New York passage movement or the floor resolver, and final pending-XY commit. Its terrain event is emitted only on the actual floor-query path, with the original pre-integration floor point. All **13,229** existing natural EEFF calls match the full represented shared state; this includes **3,996 auxiliary calls** as well as players and ball, rather than excluding props from the comparison.

Court 0 is now wired through `NewRuntime`, original opening, world preparation and action dispatch. A `FlatCourt.originalMotion` binding exists only for the active transaction. Local action changes are supplied to the complete motion operation at the actual physics call; other-object effects return to the same transactional world. There is no late second integration pass. Players, ball, auxiliary objects and loose hoops use the binding. Existing New York handlers remain intact.

Court 0 uses its original 32-pixel rear ledge and exclusive depth limit 73. The integration test lands a player at X264/depth72 on height32; New York's geometry at that point would incorrectly remain height0. Display terrain caches now call `ObserveDisplayMotionForCourt`, so the same point yields Court0=32 and NewYork=0. Unsupported courts 2–7 remain explicit runtime errors; unknown character/prop actions remain errors rather than borrowing New York behavior.

The 9B95 court animation tail is connected after world projection/column preparation. Its counter is `World.LogicFrame` (original 0300), and its camera is `Camera.DisplayedX` (0397). Court 0's water frame shares the canonical 06D7 field, currently named `Actors[0].Player.NewYorkPassage`; the non-New-York motion binding intercepts before the legacy New York passage branch. No parallel stored animation counter is advanced. Pause/resume retains the original no-animation path; video commit publishes pending patches with bounded final-tile history.

The first integrated cold run exposed missing Kind10 contact geometry, restored by the contact workstream from original tables. Water animation then exposed a saved-display edge case at video600: deleting the last patch left an empty non-nil history that JSON omitted. The display workstream added a failing regression and canonicalized empty histories to nil; the restoration assertion remains enabled.

With the original fixed seed 12345678, normal clocks, controller input only and periodic JSON restoration:

| Runtime configuration | Video ticks | Logic updates | Restore comparisons | Score | Repeated trajectory SHA-256 |
| --- | ---: | ---: | ---: | --- | --- |
| Court 0, one human + three computer seats | 22,452 | 7,458 | 84 | 15–45 | fb6c7801f25f251e118a13e47bf69572189bdf23c44a068a50104af421dcfe04 |
| Court 0, four computer seats | 22,407 | 7,443 | 84 | 52–29 | 26eca49b43278bbfa82b2534d573cfb2c6f7779d3ff5ad026e60924296265f98 |
| New York, one human + three computer seats | 22,383 | 7,435 | 84 | 12–45 | 32c8fa757593a3be5792020bc451e84b434671e7efea109e9d9c157429c8089c |
| New York, four computer seats | 22,512 | 7,478 | 85 | 45–49 | 4d354ce00497799193095710ada5eb7d927c6b929a07134725238620aef042bd |

Each cold configuration repeats twice; these are integrated semantic-runtime checks, not a claim of cycle-exact original full-match trajectories. Separate original instruction/natural-call evidence supports the restored pieces. New York's 1,998 original complete action stages (26,093 actual object visits), 14,456 terrain writes, 389 rider terrain writes and transaction rollback checks also remain passing.

Logs: `.local/basketball/engine/court-motion-{before,after,ny-regression,race,wasm}.log`, `runtime-court-zero-{before,ledge,after,race}.log`, and the display workstream's `court-animation-display-*` / `court-animation-empty-before.log`. Tests use `runtime_court_zero_test.go` and the existing cold-runtime gate without shortening clocks or rewriting its expected control configuration.

Final cross-runtime validation is complete: all four cold-match configurations above passed Wasm with the same native digests (98.747 seconds total). Both complete Court 0 configurations also passed native race, including repeated runs and restoration checks (177.402 seconds). The independent original action/motion/event corpus and rollback race checks passed separately. No other game's tests were included.

## Court 2 integration failure inventory (before implementation)

Preserve the original 32/48-pixel terrain and exclusive depth73, shared 06D7..06DB alternating/switch/timer slots, and device dispatch. Explicitly test selected-actor JumpPhase writes surviving local action returns, a later device ball-held write surviving an earlier dribble command, sound order across the physics call, and auxiliary switch-timer aliases not becoming New York passages. Reuse the unchanged full cold-match seed/clock/restore gates; do not enable untested court selectors or substitute New York actions.

Court 2 has subsequently been connected to the actual runtime. Its canonical mechanism mapping, Action3E preparation ordering and complete-match evidence are recorded separately in [COURT_TWO_INTEGRATION.md](COURT_TWO_INTEGRATION.md). Earlier statements restricting the runtime to Court0/1 describe the preceding increment; current engine gates admit 0/1/2 and still reject 3–7.
