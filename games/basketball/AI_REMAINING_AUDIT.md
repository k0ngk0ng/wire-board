# Remaining original AI coverage audit

## Recovery after this audit

The subsequent command recovery implements personal modes 3/10/11/13/14/30/36/37/38/41 and all team strategies4–7. Current entry counts are **41/42 personal modes** (mode15 remains unsupported), **14/14 team strategies**, **13/16 direct shot plans**, and **7/14 evasion plans**. The normal A/B command consumer now applies original request/status/reply/window writes and auxiliary bindings through canonical world state. Entry counts still do not prove full gameplay coverage.

See [command integration](AI_TEAM_COMMAND_VERIFICATION.md) for source comparisons, ordinary-controller paths, full-match results and browser evidence. Remaining guards and unknown source domains below are not automatically resolved by the added entries.

Next work is the remaining direct shot/evasion entries and their real callers, mode15/alias boundaries, and campaign/equipment semantics. Platform tactical choices and complete network acceptance remain separate from these engine entry counts.

## Historical pre-recovery audit

Audit date: 2026-10-18. This is a read-only implementation/source audit, not a new gameplay acceptance result. Only this report was written. No AI behavior, product configuration, source fixtures, or coverage headline was changed, and no unrelated game tests were run.

The current implementation has **31 of the original 42 personal mode entries**, **10 of 14 team strategy entries**, **13 of 16 direct shot-plan entries**, and **7 of 14 direct evasion-plan entries**. These are dispatch counts, not percentages of gameplay completed: implemented entries retain important guards, and some missing entries are original aliases or have no demonstrated ordinary caller.

## Evidence boundary

The original ROM is `.local/nes-basket/basketball.nes`, SHA-256 `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`. All addresses below refer to its PRG bank 5. This audit read the actual pointer bytes and compared every index with current Go dispatch; it did not infer completeness from TODO strings or historical reports.

| Original source | Existing evidence |
|---|---|
| Personal dispatch `A05D`, 42 pointers at `A09C`, handlers and helper entries | `.local/basketball/rea/ai-batch/batch.json`, `ev_eef21fb30302a3ec59b8b3dc097be9a7fa2a559339d598963a8489a9d646759d` |
| Team dispatch `9AF6`, 14 pointers at `9B08`, command handlers | `.local/basketball/rea/world-schedule/global/batch.json`, `ev_07eaf15f099c93a5a0f6613a4e0d132dbdd15765b5a764f002e85ad934bec0fa` |
| Team schedule `99F0` and role assignment | `.local/basketball/rea/team-ai/reused-analysis.json`, including `ev_9c8ecd00722363df65731629837cf49a46ae7226853abf7fd56d941dde66660c` and `ev_d2d269c7f23f7ce3ec46c51ff7d4bc6f86433d921ec1b889d3b678bf784f6030` |
| Shot dispatch `B383`, 16 pointers at `B38B` | `.local/basketball/rea/shot-planning/dispatch/batch.json`, `ev_8bac8418e07eaa8a98c2ac6472d25618bfc7cb159ee632963fe08b31581cafda` |
| Evasion dispatch `B7CD`, 14 pointers at `B7D3` | `.local/basketball/rea/dribble-evasion/batch.json`, `ev_58b2c56658b4321b847a48fb0a53b4d307c599eecc82b6aff7938aefa72b9dde`; actual ROM pointer bytes establish the bounded table |
| Human tactical profiles | Six pointers at `9467`, normal and command subrows at `9473` through `94AE`; `engine/ai_team_profiles.go`, `engine/ai_command_resume_profiles.go` and their source metadata |

REA pseudocode is static evidence, not proof of a natural match. Existing natural/isolated validation reports remain separately scoped. Historical `AI_SHOT_VERIFICATION.md` and `AI_TEAM_VERIFICATION.md` describe earlier milestones; their old limitations are not the authoritative current dispatch list.

The ordinary action gate in `originalAIActionAvailable` was also compared with every one of the 128 original `9321` flag bytes: no difference in bit `04`. The exceptional action `3E` gate matches original `95EF` bit `10` for precisely modes **1, 29, 31, 34**. A denied action returning unchanged state therefore does not prove that its personal mode is implemented.

## Every personal mode

`Implemented` means the Go switch has the original entry or alias. It does not mean every dependency or natural route is accepted. `Missing` means the ordinary dispatch reaches the explicit unsupported-mode error when its action gate allows execution.

| Mode | Original entry | Current Go route | Coverage / remaining issue |
|---:|---|---|---|
| 0 | A0F0 | recover in `ai_control.go` | Implemented; restores stored Plan, which may itself be missing. |
| 1 | A11E | possession wait | Implemented, including team 7 handoff to 33 and action 3E handoff to 31. |
| 2 | A1D1 | `advanceOriginalPursuit` | Implemented; canonical alias reads remain bounded, see guards below. |
| 3 | A187 | none | Missing; waits for ownership of auxiliary object 4/5 and action 20, otherwise returns to recover/plan 8. Reachable from missing mode 13. |
| 4 | A4D4 | `advanceOriginalMarking` | Implemented. |
| 5 | A229 | `advanceOriginalHomeGuard` | Implemented; original tactic-menu-to-mode-5 evidence exists. Holder 4/5 still fails shared position lookup. |
| 6 | A2AB | `advanceOriginalShotPlan` | Implemented entry, incomplete direct plan domain. |
| 7 | A2E9 | `advanceOriginalEngagement` | Implemented; all 12 engagement choices present, target-source guards remain. |
| 8 | A323 | forward positioning | Implemented. |
| 9 | A3C2 | defence | Implemented. |
| 10 | A461 | none | Missing; entry requests standing pass action 17 and switches to mode 1. Implemented mode 28 can select it when the support actor holds the ball. |
| 11 | A476 | none | Missing; action 10–13 requests pass, mode 1 and continuation 22. Original human command tables select it for profiles 2–5. |
| 12 | A4D4 | marking alias | Implemented. |
| 13 | A557 | none | Missing; auxiliary approach/pickup (`AD78`, `ADA4`), then action 4E, mode 3, plan 14; failure writes command/status fields. Profile 3 command table selects it. |
| 14 | A599 | none | Missing; auxiliary carrying/partner attachment and throw handling. Mode 13→3→14 is an original chain. |
| 15 | A63D | none | Missing alias. Unlike modes 36–38, `(mode & 15)-4` gives 11 and reads beyond the intended three-entry selection tables. Normal caller not established; do not silently alias to mode 36. |
| 16 | A69F | dribble attack | Implemented shared handler. |
| 17 | A69F | dribble attack | Implemented, ordinary team possession strategy. |
| 18 | A7A3 | jump-shot setup | Implemented. |
| 19 | A851 | approach delay | Implemented; computer difficulty 28–31 is explicitly rejected by its seven-entry delay table. |
| 20 | A1B7 | wait idle | Implemented. |
| 21 | A8C1 | command resume | Implemented exact static lookup; its returned modes are not all implemented. |
| 22 | A8C1 | command resume alias | Implemented exact static lookup, same limitation. |
| 23 | A49E | forced shot plan 4 | Implemented. |
| 24 | A49E | forced shot plan 5 | Implemented. |
| 25 | A49E | forced shot plan 7 | Implemented. |
| 26 | A8DE | dribble evasion | Implemented entry, incomplete direct plan domain. |
| 27 | A944 | pause | Implemented. |
| 28 | A95F | team support | Implemented; selects missing mode 10 for one possession case. |
| 29 | AA2C | rider approach | Implemented; needs actual tracked player and holder position. |
| 30 | A208 | none | Missing; shared ball reaction then clears command status on the original carry/holder condition. Profile 2 command table selects it. |
| 31 | AB87 | launch-shot plans 8–10 | Implemented; original difficulty 28–31 adjacent-byte thresholds retained. |
| 32 | ABCE | score strategy | Implemented. |
| 33 | ABEF | team mode 33 | Implemented; source selection verified for team 7. |
| 34 | AAC7 | team mode 34 | Implemented; writes teammate AI mode 28/recovery and needs canonical teammate state. |
| 35 | AC3A | team mode 35 | Implemented; original engagement parameter 7 continuation. |
| 36 | A63D | none | Missing; opponent selection, original random attack choice and approach, then mode 7/plan 22. Selected by every ordinary human tactic's command-A subrow. |
| 37 | A63D | none | Missing same handler with distinct selection subtable; command-B profiles 0/1/5. |
| 38 | A63D | none | Missing same handler with distinct selection subtable; command-B profile 4. |
| 39 | A851 | approach delay alias | Implemented; same internal guards as 19. |
| 40 | AC87 | collect loose hoop | Implemented. |
| 41 | AD17 | none | Missing direct dispatch; selects an opponent using action 49/4A exclusions and quantized X distance. Also a real helper of missing modes 15/36–38. Normal direct-mode caller not established. |

The ordinary computer role tables presently output modes `{2,4,8,9,17,33,34,35}`, with loose-hoop override 40 and elevated defence 28/29. All these entries exist. This does not cover the later possession/support transition to missing 10 or the shared internal guards.

## Every team strategy

| Strategy | Original entry | Current status |
|---:|---|---|
| 0 | 9CF7 | Implemented no-op. |
| 1 | 9CF8 | Implemented computer assignment. |
| 2 | 9DE1 | Implemented human teammate assignment. |
| 3 | 9DE1 | Implemented alias. |
| 4 | 9B9E | Missing cooperation lifecycle. |
| 5 | 9B9E | Missing alias. |
| 6 | 9BC7 | Missing cooperation lifecycle. |
| 7 | 9BC7 | Missing alias. |
| 8 | 9DE1 | Implemented human teammate assignment. |
| 9 | 9E08 | Implemented human teammate assignment. |
| 10 | 9CF8 | Implemented computer possession role assignment. |
| 11 | 9CF8 | Implemented computer defence assignment, including later elevated defence 28/29. |
| 12 | 9CF8 | Implemented loose-ball assignment. |
| 13 | 9DA7 | Implemented human refresh with counter. |

There is an earlier blocker than the missing switch cases: `StepOriginalTeamAI` rejects **any nonzero HumanCommands or HumanStatus** when that team has a human. The original `9B9E/9BC7` lifecycle writes/clears command request, teammate status, button windows, messages and acknowledgement. `TeamAIStep` currently returns only TeamAIState, so this needs explicit world commands/state integration, not just four extra switch labels.

This is an ordinary-input gap. `advanceOriginalHumanCommands` already produces request 1 or 2 when an even-numbered human is not the holder, its teammate is computer-controlled, and A/B remains held through the original three-call window. `world_loop.go` passes that state into the next preparation phase, where the guard rejects it. No ROM editing or invalid configuration is required for this producer path. A freshly captured complete native-input command sequence is still needed to verify the restored consumer.

### Actual original command subrows

These are the 12 bytes at each `9467` profile pointer, grouped as ordinary / request 1 / request 2. They are distinct from the buggy actor-indexed command-resume lookup.

| Tactic | Ordinary four entries | Request 1 four entries | Request 2 four entries |
|---:|---|---|---|
| 0 | 2,2,17,8 | 36,36,24,8 | 37,37,23,8 |
| 1 | 2,2,17,8 | 36,36,24,8 | 37,37,23,8 |
| 2 | 12,12,17,12 | 36,36,11,12 | 30,30,23,12 |
| 3 | 2,2,17,8 | 36,36,11,8 | 13,13,25,8 |
| 4 | 2,2,17,32 | 36,36,11,8 | 38,38,23,8 |
| 5 | 5,5,17,5 | 36,36,11,5 | 37,37,23,5 |

The menu path for tactic 5 has a retained original-selection probe in `AI_MODE5_PLAN11_VERIFICATION.md`. The full set of command chains from all menu choices has not been naturally recorded. Table membership demonstrates source selection, not completed gameplay acceptance. Platform preparation currently does not expose all six tactics just because RuntimeConfig accepts them.

## Every direct shot plan

The real `B383` dispatcher has 16 entries; `ai_shot_plan.go` handles 13 of them.

| Plan | Entry | Status |
|---:|---|---|
| 0 | B3AB | Implemented ordinary aim/cycle. |
| 1 | B3C1 | Implemented jump/air shot. |
| 2 | B413 | Implemented second-jump sequence. |
| 3 | B471 | Implemented mount-aware approach; needs current mount flags and valid jump grade. |
| 4 | B4B9 | Implemented forced standing aim, continuation 22. |
| 5 | B4D2 | Implemented delayed choice 2, continuation 22. |
| 6 | B4E3 | Implemented pass request; older reports saying this is missing are stale. |
| 7 | B4EE | Implemented high-jump route or existing low-jump B58E helper, with caller's plan-22 override. |
| 8 | B51D | Implemented launch direction/air aim. |
| 9 | B540 | Implemented second jump at zero vertical byte. |
| 10 | B557 | Implemented selection of plan 2. |
| 11 | B564 | Missing direct route. Original retreats when zone >4E, otherwise faces/aims, mode 1 and shot cycle 0. |
| 12 | B58E | Missing direct route despite existing `advanceOriginalLowJumpShot` helper. Direct invocation must not inherit plan 7's unconditional continuation-22 override. |
| 13 | B58E | Missing direct alias, same caveat. |
| 14 | B5F6 | Implemented court-specific platform route. |
| 15 | B619 | Implemented platform continuation alias. |

Ordinary weighted selection maps to `{0,1,2,14,6,3,7}`; forced modes 23–25 select `{4,5,7}`; mode 31 selects `{8,9,10}`. None of those selectors picks missing direct plans 11/12/13. No ordinary menu-to-plan-11/12/13 chain was established in this audit. They remain genuine original dispatch entries requiring caller investigation; the absence from the inspected selectors is not a global unreachable-code proof.

## Every direct evasion plan

| Plan | Entry | Status |
|---:|---|---|
| 0 | B7EF | Implemented original action 2E/side-step command. |
| 1 | B811 | Implemented diagonal dribble escape and pause. |
| 2 | B840 | Implemented teammate-distance pass/next-plan-5 choice. |
| 3 | B87F | Implemented deferred mode 6 selection. |
| 4 | B885 | Implemented straight running jump. |
| 5 | B8A4 | Implemented diagonal running jump. |
| 6 | B8D8 | Missing. |
| 7 | B92C | Missing. |
| 8 | B92C | Missing alias. |
| 9 | B92C | Missing alias. |
| 10 | B95D | Missing. |
| 11 | B97B | Implemented short vertical walking interval. |
| 12 | B9A7 | Missing. |
| 13 | B9CB | Missing. |

All eight original twelve-choice probability rows at `BA5C` select only `{0,1,2,3,4,5,11}`; the New York upper-depth override selects 1. Those ordinary selection outputs are implemented. No normal menu-derived caller for missing plans 6–10/12/13 was established. Keep their unknown reachability distinct from the clearly reachable human command gap. Further recovery should inspect their actual entry instructions and setters before adding guessed behavior.

## Internal guards and dependency holes

These matter even when a top-level table entry is implemented.

| Area | Current guard / missing domain | Evidence and next bounded work |
|---|---|---|
| Human team commands | Any Request !=0 or Status !=0 is rejected before team dispatch. | Prioritize original 99F0→9B9E/9BC7 with real A/B input, exact acknowledgements and cross-phase state. |
| Holder object 4/5 | `originalHumanTeamPlan`, entered computer refresh, and `originalBallHolderPosition` reject them. | Actual objects 4/5 exist; whether they can hold the ball on each route needs original source/runtime evidence. Do not assume every index admitted by outer holder<7 is supported. |
| Pursuit candidate sources | Action sources exist for players and indices 7,14,23,31,39,40,48,55,73,74,75,76,77,79,87,88. Some have no depth or X implementation. | 55 lacks depth; 76/77 lack depth; 79/87 lack both depth and X. Other indices fail at their first unsupported read. Preserve lazy rejection before requesting unused fields. |
| Pursuit aliases | 73/74 and 88 need explicit reserved/alias bytes; 88's depth/X bytes are optional and cannot be zero-filled when absent. | `AI_PURSUIT_ALIAS_VERIFICATION.md` has source initialization and bounded natural traces. It does not prove all possible aliases are restored. |
| Engagement | All parameters 0–11 dispatch; >=12 fails. Negative tracked object or unsupported alias fails when a facing branch consumes it. | Current 8×4×4 pursuit parameter tables fit 0–11. No need to invent parameters beyond that domain. |
| Modes 19/39 difficulty | Seven delay entries accept difficulty 0–27; 28–31 fail, while several other planners accept 0–31. | Original opening sets Difficulty from match period. Normal campaign reachability of the eighth band must be established; if reached, inspect adjacent original bytes rather than clamp. |
| Modes 29/34 tracking | Require real tracked player 0–3; 34 also requires current TeamAI and world airborne state when consumed. | Normal entries initialize actor^1, but restored/aliased state can leave a different target. Existing canonical world wiring should be preserved. |
| Shot-grade/profile validation | Grade requires jump velocity 7*256 + n*128 for n=0..7; shot skill <8, team <8, difficulty band <8. | These are documented source table domains, not automatically missing legitimate features. Equipment/campaign mutations require source-bound acceptance before widening. |
| Command resume 21/22 | Exact 6×4×7 static return table exists, but original actor-index quirk returns raw adjacent bytes. | Results include modes 3/10/11/36 and values outside 0..41, with Entered bit preserved. Do not correct it to team indexing or call its single-step acceptance complete continuation coverage. |
| Recovery/wait | Modes 0,1,20,27 and command resume can hand off to a missing mode stored in Plan/table. | An implemented wrapper or green short fixture does not establish that subsequent action completes. |

`advanceOriginalEngagementPlan` covers all twelve original attack choices, but it emits actions rather than executing attack physics. Likewise shot and evasion planner coverage must be combined with independently verified actor/ball handlers. This report does not relabel those downstream components as accepted merely because their requests are now generated.

## Next recovery units

1. **Human-command consumer and missing command-selected personal modes.** First capture the original ordinary A/B-held route with a computer teammate, write failure scenarios and source-driven tests, then restore team strategies 4–7 plus the canonical request/status/message writes. Modes 36–38 and helper AD17, mode 11, and mode 30 are direct dependencies. This route is supported by current normal input production and original command tables.
2. **Mode 10.** Small independent A461 handler with an existing mode-28 producer; obtain an original continuous support/possession handoff so it is not only an isolated dispatch test.
3. **Auxiliary command chain 13→3→14.** Restore actual auxiliary selection/ownership and action commands, preserving shared status writes. Profile 3's request-2 table provides a source caller, while a full menu/input-to-chain trace remains to be collected.
4. **Remaining direct plans and aliases.** Inspect source callers/setters for shot 11/12/13 and evasion 6–10/12/13. Keep a reachability ledger: original table entry, actual caller, ordinary-menu evidence, isolated oracle, then integrated chain. Do not unlock them by arbitrary state injection and call that normal gameplay.
5. **Residual alias and campaign domains.** Expand pursuit sources only when the exact original read is required and has a canonical source; audit the modes-19/39 eighth difficulty band against real period/campaign transitions. Mode 15 and direct mode 41 require a caller audit before claiming ordinary reachability.

No complete-AI claim follows from the current default-roster full matches or from all ordinary random choice outputs being implemented. The explicit ordinary human-command failure alone leaves the requested original multiplayer/control behavior incomplete.
