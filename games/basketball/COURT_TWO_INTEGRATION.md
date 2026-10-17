# Court 2 runtime integration

Court selector 2 is wired into the existing opening, world preparation, action dispatcher and complete original motion pipeline. Its terrain remains the original 32/48-pixel profile with exclusive depth limit 73. Runtime selectors 3–7 remain rejected; this change does not add production registration or claim recovery of every remaining auxiliary action.

## State and ordering

No second persistent mechanism state was introduced. The original shared slots are authoritative:

| Original slot | Court 2 meaning | Existing world storage |
| --- | --- | --- |
| 06D7 | Alternating side | Actor 0 `Player.NewYorkPassage` |
| 06D8/06D9 | Switch flags | Actors 1/2 `Player.NewYorkPassage` |
| 06DA | First switch timer | Actor 3 `Player.NewYorkPassage` |
| 06DB | Second switch timer | Auxiliary 0 `NewYorkPassage` |

The historical field name does not select New York physics. Non-New-York actions enter the transactional original-motion binding before the legacy passage branch. The independent animation adapter consumes the same slots after projection, using the original logic counter and displayed camera.

Court devices can write an actor's jump phase and the ball action during physics. `CourtFloorStep.ResetJumpPhase` and `Ball` retain the actual writes even when the old value already equals the written value. The world applies the selected actor's reset after the action-local return; the later device-held-ball command supersedes an earlier dribble command. Entry attack sounds precede device sound 21. A rider's action-local attachment clearing reaches the motion call before attached-follow selection.

## Action 3E integration

The independently recovered `PrepareLaunchedPlayer` / `CompleteLaunchedPlayer` handler is connected through `world_launched.go`. The dispatcher accepts attached 3E entries and explicitly commits prepared player/attachment, RNG, pose source, launch target/arc and special-shot writes before motion. Sound 6 is emitted before the physics call. The completion stage never writes an old RNG value back over a physics result.

`WorldActionContext.ActorHeldButtons` receives `WorldLoopState.Input.Held` for each action phase. It represents original 04E0..04E3 and is indexed by actor directly; it is not remapped through the controller ID again. Source position/pose/facing can come from any of the thirteen world objects. Throw-selector validation occurs only on first entry when the original E003 plan is actually used; an entered continuation does not fail because an unused old selector is invalid.

Preparation chooses equipment from the current holder's action (1B/1C), not a newly invented release flag. The holder's special-shot byte is read/cleared only on the recovered original branch. Unknown source/selector/holder contexts still produce explicit errors.

## Test-first evidence

`court_two_binding_test.go` was added before the device writeback change. It first failed at the runtime gate, then reproduced jump phases 7 and -32 overwriting the device's zero. It also checks the later held-ball write and sound order. `launched_world_test.go` first failed before the raw actor input field/dispatcher existed; it verifies attached preparation, physical integration rather than stale following, raw actor/controller distinction, global launch metadata and continuation behavior.

Logs: `.local/basketball/engine/court-two-binding-{before,gate-after,after}.log` and `launched-world-{before,after}.log`.

The existing 14,458 natural floor calls, 68,094 configured original floor/device calls, 13,229 complete natural EEFF calls and 1,998 original world action stages still pass. The separate contact, launched-player and AI workstreams retain original-source evidence for Kind13 geometry, the 64 Action3E calls, A05D possession routing and mode31/shot-plan8/9/10. Configured instruction matrices are not labeled as natural whole matches.

## Complete runtime gates

The root-owned Court 2 tests retain seed 12345678, the normal match clock, the 48,000-video upper bound, periodic JSON restoration and two repeated runs. No restored match-state fixture or shortened clock initializes these matches.

| Configuration | Video ticks | Logic updates | Restore comparisons | Score | Native repeated trajectory SHA-256 |
| --- | ---: | ---: | ---: | --- | --- |
| One human + three computer seats | 22,503 | 7,475 | 85 | 30–26 | 0fc85503befa6af628798b4fdf0df6cb993e505224631ebae1adfe81a8f76664 |
| Four computer seats | 22,365 | 7,429 | 84 | 45–33 | 81764044de37a6b50072705b579f04f966b3d4124bce95178f961a00368e30b8 |

The old New York and Court 0 match configurations also repeat successfully with unchanged video/logic counts and scores. Their full serialized hashes changed because `ActorHeldButtons` is now an explicit action-context field; the original motion/action corpus comparisons still match. The new native hashes are recorded in `court-two-old-courts-regression.log`.

Reproduce the integrated gates:

```sh
source scripts/env.sh
go test ./games/basketball/engine -run '^TestRuntimeCourtTwo' -count=1 -v
go test -race ./games/basketball/engine -run '^TestRuntimeCourtTwo' -count=1 -v
GOOS=js GOARCH=wasm go test ./games/basketball/engine -exec="$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^TestRuntimeCourtTwo' -count=1 -v
```

Full reports are `.local/basketball/engine/runtime-court-two-{after,race,wasm}.log`. These prove integrated semantic runtime completion and restoration; they do not claim cycle-exact original complete-match timing, complete campaign/menu recovery, browser pixel acceptance or production registration.

Final validation passed: Court 2's two complete configurations plus the device/launch integration regressions pass Node Wasm in 49.488 seconds and native race in 167.800 seconds. Native, Wasm and race report the same two trajectory hashes above. Old-court/source-corpus native regression completed successfully in 19.638 seconds. No production configuration registration was changed by this workstream.
