# Shoes23 complete original jump handlers

## Failure inventory fixed before implementation

- Missing any of five EC31 callers or aliases04/05/0D,14/15,24/25,34/35,44/45.
- Applying shoes by actor rather than character, to opponent teams or Mode128; missing enabled/disabled controls or unequipped/other-shoe controls.
- Setting gravity every frame rather than at original takeoff; skipping windup, losing negative JumpPhase behavior or0xE0's shallow ordinary jump.
- Mutating gravity before the original motion call when takeoff follows animation/motion; dropping entry sounds, attachments, support writes or motion side effects.
- Treating hoop jumps34/35 as ordinary jumps with landing06 instead of36.
- Treating44/45 as free jumps: they retain attachment, change support action/facing, copy motion to support, move support then rider, copy motion back and clear support motion.
- Changing14/15 depth heading rules or ball action; changing24/25's immediate initialization and auxiliary-owner fallback.
- Ignoring entered continuations, fractional coordinates, full-world mutations, or JSON restoration.
- Calling isolated original execution natural gameplay or claiming full campaign/flight acceptance.

## Source, red result and implementation

ROM SHA-256 remains `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`. Retained REA/source pointers are indexed in `EQUIPMENT_EFFECTS_SOURCE.md`. `verify_shoes23_jump.py` executes complete original `E165..E168` player calls, with the original MMC3 mapping and only FBCC's sound sink intercepted. It does not run a substitute physics model.

The red run preceded `world_jump.go`: ordinary shoe23 jumps returned gravity256 instead of128; aliases0D/34/35/44/45 were rejected. `.local/equipment-effects-catalog/shoes23-red.log` preserves this result. The final fixture contains **2,112 complete isolated calls and132 once-initialized chains with2,352 further calls**. Inputs cover four actors/characters, human/computer rotation, enabled/disabled mode and opponent controls, empty versus23 slots, windup/entered/takeoff/continuation stages, JumpPhase0/-1/-32, and court0/1/5 chains. Shoe27 scoring work is a separate report.

The authoritative world dispatcher now selects the new helper for all11 jump aliases. The helper restores the four previously separate ordinary/ball/hoop/paired-body paths and delegates24/25 to the already restored auxiliary handler. It changes gravity only at each original EC31 call; does not repeatedly apply half-gravity during flight; preserves ordinary0xE0 shallow initial velocity, all animation timing, sounds, attachment handoff and support writes. Paired44/45 copies motion to the support, moves support before rider, copies the result back, stops support and preserves source landing actions40/04. These actions are no longer approximated by free-player motion.

The root worker owns the `world_actions.go` integration: `originalWorldJumpAction` is checked before legacy carrier/free/auxiliary branches and exempted from the early attachment rejection. It calls `advanceWorldJump` and retains final motion JumpPhase reset handling. No AI choices, scores or RNG draws were added.

## Verification and reproducibility

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_shoes23_jump.py
go test ./games/basketball/engine -run '^TestShoes23CompleteOriginalJumpHandlers$' -count=1
go test -race ./games/basketball/engine -run '^TestShoes23CompleteOriginalJumpHandlers$' -count=1
GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run '^TestShoes23CompleteOriginalJumpHandlers$' -count=1
```

All source calls compare the complete decoded `WorldActionState` and ordered sounds; chains continue directly from the previous result and also replay from JSON. Fixture SHA-256: `4bdffb6da8e118177aabb779874b96e8195dcb060bd8ee60f46a813a61ed492b`.

Native regressions passed for retained rider/free/carrier/auxiliary/hoop observations, every captured world action, continuous world trajectories and active-input rider trajectories. The combined shoe23/shoe27 source set, original rider jumps, captured world corpus and continuous world trajectory also passed race and Wasm checks. Result logs are `.local/equipment-effects-catalog/equipment-{native,race,wasm}.log`, with individual earlier jump runs retained. Temporary compilation failures while other workers' test-first actor/ending files were incomplete remain separate logs; those tests were not removed or altered.

## Scope and compatibility

These are controlled original complete-handler/continuous-chain comparisons, not naturally recorded shoe23 controller play or a complete equipped campaign. Existing standalone `StepFreePlayerAction`, `StepCarrierAction` and `StepHoopCarrierAction` compatibility APIs retain their former limited contexts; production world dispatch uses the complete source helper. A nil `WorldActionContext.JumpEnhancements` does not invent a shoe override in the new ordinary/ball/hoop/paired helper; the actual `WorldLoopContext` supplies its explicit character-derived four-entry shoe array on each action phase. Existing auxiliary24/25 retains its explicit missing-source error in enabled team0 contexts.

The independent all-computer campaign stall remains unresolved. Root reproduced the same final snapshots with the prior dispatcher through a baseline overlay, so this report does not treat the new jump comparisons as a resolution or silently skip those stalled trajectories.
