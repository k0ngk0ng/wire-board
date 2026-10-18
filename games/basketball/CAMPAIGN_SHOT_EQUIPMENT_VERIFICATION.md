# Campaign shirt1 release/source verification

## Failure scenarios specified before source tests

- Shirt1 is read from the actor's current character equipment slot, not actor number, global character0 or a platform seat. Rotation must preserve the correct source owner.
- Mode128 disables equipment replacement; Mode0/1 enables it only for team0. Opposing teams must retain their own original character special ball.
- Ground and air special release are different: shirt1 maps to ball13 only when the original SpecialShot flag allows it. Ordinary/high/charged cycles and zone gates must remain distinct.
- First entry issues ball action and launch metadata exactly once; already-entered continuation must not repeat equipment selection or RNG consumption.
- Shirt1 itself consumes no random value. Original shirt7 is a positive control for the existing RNG branch: preserve both selected-ball and fallback outcomes, including air behavior.
- The actual world adapter must commit actor flag/cursor, ball action/owner, facing, LastShooter, ShotOriginX and RNG in source order before ball motion. Complete original scheduler visits and output sounds must not be filtered.
- Retained normal campaign menu inventory proves ownership and character binding, not a naturally observed item1 shot. Modified initial states are explicitly controlled calls, with no intermediate-state patching.
- Do not alter production code if the source comparison fails; retain and report the first exact difference.

## Sources and actual results

The unchanged ROM is SHA-256 `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`. Existing REA release/target evidence `ev_22e089841ea9f9c13f2d325c435946a52e8700cd2ee6a265d6caa7c02c223f6e` was reused, especially bank6 `9EFC..9F47`. Original instruction inspection confirms `9F26` calls fixed `F9C3` directly for the random equipment branch; it does not pass through `C018`.

The source normal ownership/character binding is retained `campaign-audit-equipped-home-match`, video4345, full-trace SHA-256 `c7c6cffa803c4d996d86cd7b8968c5788fc953f1545f3d9d070dce96e42b7173`. That normal frame contains Mode0, Period2, Court0 and character0 shirt1. It is used as the actual home-court RAM basis. Separate New York calls reuse the previously recorded whole-action RAM basis. The subsequent release inputs are explicitly controlled modifications to the initial state, not claimed as a naturally played shot.

`verify_campaign_shot_equipment.py` now produces:

- **3,184 complete original player-action calls**, from E165 through E168. They cover character0..3, actor/character rotation, ground and air, Mode0/1/128, source cycle -1..8, valid and excluded zone/depth boundaries, entered continuation, opposite-team controls, and shirt7 as a positive RNG control.
- **96 complete original D345..D348 world-action phases**, on Court0 and New York. These compare the complete `WorldActionState`, every original scheduler visit and sound request, and replay identically after JSON restore. The source ball phase consumes the just-issued release within the real world scheduler; tests do not manually apply a desired ball action after the player call.

All state comparisons match the existing production implementation. No production change was made.

Specific observed distinctions:

| Source path | Observed result |
|---|---|
| Enabled team0 shirt1, ground charged special and allowed zone | Ball13; 64 matrix calls cover this path. |
| Enabled team0 shirt1, air special and allowed zone | Original character special ball23/21/17/16, not an unconditional shirt1 override; 64 calls cover it. |
| Mode128, or another team | Source equipment replacement is skipped; character/cycle logic remains. |
| Shirt1, including character rotation | No F9C3 call and unchanged shared RNG during the player handler. |
| Shirt7 original random mapping | 64 actual F9C3 calls, with both fallback and mapped ball outcomes. Seed0 is included: a random call can leave its zero state unchanged, so observed call count is checked separately from state inequality. |
| Already-entered release animation | No repeated first-entry shot selection. |

The first test run matched all player/world states but failed its coverage assertion because the observer counted C018 instead of F9C3. That instrumentation failure is preserved in `.local/campaign-shot-equipment/first-run.log`. The probe address was corrected; no expected gameplay state or production rule was changed.

## Reproduction and limits

Run from the isolated basketball worktree:

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_campaign_shot_equipment.py
go test -v ./games/basketball/engine -run '^TestCampaignShotEquipment' -count=1
go test -race ./games/basketball/engine -run '^TestCampaignShotEquipment' -count=1
GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run '^TestCampaignShotEquipment' -count=1
```

Source fixtures and hashes use the new `engine/testdata/campaign-shot-*` prefix. The only new test is `campaign_shot_equipment_test.go`; runtime and the parent campaign-runtime tests were not edited.

These checks establish source-correct release decisions and immediate complete action-phase wiring. They do not establish a naturally recorded shirt1 shot from human input, a complete special-ball flight/contact/score chain, or resolution of the separately investigated all-computer campaign stall. A matching release cannot rule out a later motion, collision, hoop or possession problem.

Native, race and Wasm targeted checks passed; hashes and result index are in the worktree-local `.local/campaign-shot-equipment/result.json`.
