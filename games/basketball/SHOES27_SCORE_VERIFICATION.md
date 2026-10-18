# Shoes27 original scoring bonus

## Failure inventory before implementation

- Opening world never binds the original character shoe slot to ScoringBonus.
- Indexing by actor instead of character, or granting bonus to actors2/3 because their team/character matches.
- Adding a Mode/team gate absent from bank4 91BD..91E1; abnormal supplied Mode128 loadouts must remain distinguished from actual platform versus, which has no equipment.
- Awarding points without a real original rim goal, changing chance/RNG/contact/cooldown, or applying bonus twice on repeat contact.
- Wrong three/four-point base, side, score carry, character rotation, missing/other shoe controls.
- Save/restore losing a bound bonus or accepting a tampered profile flag inconsistent with configured equipment.
- Treating an artificially equipped source call as naturally earned equipment; include legal password-derived owned27 → equip → runtime opening handoff separately.

## Source and test-first result

ROM SHA-256: `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`. The full source rim entry is bank4`9035`, returning to`9290`. The scoring consumer `91BD..91E1` checks LastShooter<2, uses its character and the pointer table`94D8=[06E4,06E7,06EA,06ED]`, reads slot2 and adds2 only for ID27. This consumer has no Mode/team check. Earlier direct-address equipment scanning missed it because the final read is indirect `LDA ($28),Y`.

Before production edits, `.local/equipment-effects-catalog/shoes27-red.log` recorded the source score34 versus engine32 and the actual owned27 opening profile remaining false. `opening_world.go` now binds this boolean from the eligible actor's selected character shoe slot. `runtime_validate.go` rejects a stored boolean that differs from this configured source. Rim collision, base points, sound timing and RNG algorithms were not changed.

`verify_shoes27_score.py` produces **6,912 complete original rim calls** across actors0..3, characters0..3, shoes27/26/empty, Modes0/1/128, teams0/3, both goal sides, heights80/176/272, and goal/score-lock/outside/underside paths. Initial source context includes the enclosing original contact routine's current-object scratch`1F=6`; original instructions then run without intermediate patches. The sound sink is intercepted; no random or score operation is substituted. Source outputs verify +2 only on actual goals for actors0/1, including low-byte score carry. Fixtures retain full before/after RAM; SHA-256 `794de725b2e7570df1fd8c89a3bdc2990aded025c7a583c71b9991ee0fadecef`.

The consumer test obtains the bonus through `WorldFromOriginalOpening` and supplies that actual profile to the original-derived rim resolver, comparing the full ball/hoop/RNG/score/sound-delay state with source RAM. A separate integration test encodes and restores the legal two-win opponent6 progress, finds its owned27 reward, equips character3, rotates that character to actor0, completes the real campaign opening, and validates JSON restore plus rejection of tampered bonus flags on each actor. Opposing actor2 with the same character receives no bonus. The normal empty-equipment versus opening retains no bonus. The controlled source Mode128 loadouts are deliberately abnormal domain tests and do not add a versus equipment UI.

## Reproduction and results

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_shoes27_score.py
go test ./games/basketball/engine -run '^TestShoes27' -count=1
go test -race ./games/basketball/engine -run '^TestShoes27' -count=1
GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run '^TestShoes27' -count=1
```

Native, race and Wasm checks passed, including original rim/backboard-contact corpus regressions. Combined result logs are `.local/equipment-effects-catalog/equipment-{native,race,wasm}.log`; the source identity/fixture hashes are also in `engine/testdata/shoes27-score-source.json`. This does not claim a naturally played shoe27 acquisition/score video or a complete equipped campaign.
