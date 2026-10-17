# Court 3 engine integration

Court 3 now enters the real opening, preparation and action pipeline through the existing complete original-motion binding. Its independent profile has 24/40-pixel raised cells, raw depth-travel attributes and exclusive depth limit 88. It does not reuse New York's platform/passage geometry. Runtime selectors 4–7 remain rejected. This workstream does not change production registration, server or web code.

## Canonical gate state and original device

The gate flags at 06D7 remain `Actors[0].Player.NewYorkPassage`; the timer at 06D8 remains `Actors[1].Player.NewYorkPassage`. These are historical field names for shared original slots, not a selector for New York passage logic. Root owns the Court 3 animation adapter and its complete-match gates.

The already verified bank-4 85B5 device branch is used unchanged. At old X144..159 and depth80 or greater, it selects left facing/mirrored pose below X152 and right facing/pose from X152 onward, ORs 128 into the gate flags without destroying their low bits, and applies the original ownership/action effects. If the object already has its original device action, it does not replay sound21 or clear the jump phase again. A free ball enters original Action4, not Action9. The separate Action9 path comes from rim handling and is not reclassified as a gate effect.

Sources: the full 8249/8458 configured instruction oracle in `engine/testdata/court-floor-machines.*`, and the focused REA device evidence `ev_4f740cca747dd4aad8894e0f9f027a601b02c4f9dfd1fa5454b37b39f44e693a` at `.local/basketball/rea/court-floor-devices/` (85B5 decompiles successfully). The existing source-state and sound comparisons remain in force.

## Tests written before gate changes

`court_three_binding_test.go` first failed at the closed runtime gate. After wiring, it verifies X144/151/152/159 facing, preserved gate low bits and independent timer, no repeated launch on entered Action3E, and no trigger at X143/X160/depth79. The tests use the actual world motion binding and Court 3 terrain. The existing unsupported-court negative tests now use still-unsupported Court4, rather than incorrectly rejecting the newly enabled Court3 selector.

The changed production files are `runtime.go`, `opening.go`, `opening_world.go`, `world_preparation.go`, `world_actions.go`, `court_motion_binding.go`, and the binding-only admission in `auxiliary_rest.go`. The floor/device implementation itself was not replaced. `runtime_court_animation.go` and `runtime_court_three_test.go` are owned and edited by root, not this workstream.

Short native/race/Wasm logs are `.local/basketball/engine/court-three-binding-{before,after,race,wasm}.log`. Source regression includes all 68,094 configured original floor/device cases, 13,229 complete natural EEFF calls, and 1,998 original world-action stages.

## Complete-match status and exact boundaries

Root runs the four unchanged fixed-seed/normal-clock/JSON-restoration configurations centrally, avoiding duplicate long runs. Initial runs stopped at video116/logic36 on Kind14 Action0 Frame0; snapshots are `.local/basketball/rea/court-three/failure-0.json` through `failure-3.json`.

After the contact workstream restored the independently recovered static Kind14 domain, root reported complete repeated all-computer and two-human cooperative configurations. The solo configuration then exposed a legitimate Action4 ball launch with holder6 at video2081/logic691; versus exposed Action9 at video2339/logic777. Exact snapshots are `.local/basketball/rea/court-three/contact-failure-2.json` and `contact-failure-14.json`. Those distinct original ball handlers were subsequently recovered and connected; their source and world-level evidence are in BALL_DEVICE_VERIFICATION.md. No owner substitution, action fallback, seed change or shortened clock is used to pass them.

All four complete configurations now pass twice, with seed 12345678, normal clocks, the unchanged 48,000-video bound and periodic JSON restoration. Native, Wasm and race report identical hashes:

| Configuration | Video frames | Logic updates | Restores | Score | Trajectory SHA-256 |
| --- | ---: | ---: | ---: | --- | --- |
| Original cold, one human | 22383 | 7435 | 84 | 6–54 | bec7661883840202a8cd0accfd09c5f88b912a413b6af8a1e8752b8310c8ff1a |
| All original AI | 22395 | 7439 | 84 | 35–29 | 462b309033e8ddf5d2b8abf4de712f7721bd07f7c52bcda2d315bfecf498a391 |
| Platform cooperative | 22365 | 7429 | 84 | 0–45 | e61e97cf9d6d7f4bdb4a4f9b40dacca48eb8a7712582754ac0ee172431e6e130 |
| Platform versus | 22365 | 7429 | 84 | 51–15 | 0b27f312fb6c34cdbfbc3bc0ee1648a008f6a68fa0859b872dd49e5409c595bc |

The human actors remain idle in these complete-match gates; this is not complete coverage of all human techniques or all original teams. Logs: `.local/basketball/engine/court-three-runtime-device-integration.log`, `court-three-runtime-wasm.log`, and `court-three-runtime-race.log`.

The runtime animation adapter matches 1,138 retained original calls/probes, including 258 trigger cases, with canonical gate bytes and JSON display commit recovery. Browser acceptance `scripts/basketball/accept_live.py --court 3 --gate --full-match --all-computer` passes 34 checks, including a complete all-AI match and a separate native-input gate demonstration. The latter uses only cold configuration and keyboard input: walk to the gate, move rearward, jump and perform the original second jump to clear its 40-pixel ledge. The failed single-jump approach is retained separately; no actor position was injected. Its physical-input scenario has three idle human controllers and does not replace the AI gates.

The platform preparation allowlist now includes court-3. Production server registration, OSS publication and the remaining original-content work are still incomplete.
