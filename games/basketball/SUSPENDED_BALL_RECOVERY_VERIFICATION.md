# Approved unsupported-ball recovery

The user explicitly authorized fixing the original roll-off stall. New runtime configurations enable the persisted `recoverUnsupportedBall` policy; missing/false stays original for old snapshots and source fixtures. The runtime validates the stored action-context copy and gates the feature to active playing updates. Opening, pause, period ending and result sequences do not run this repair.

After a successful original ball dispatch, `recoverUnsupportedLooseBall` checks unowned holder6, no attached support, final action0/5, no pending request, airborne, zero vertical velocity/gravity, and integer height strictly above the actual source terrain. It changes only action10 and clears its entry flag. The next original dispatch supplies the existing release/fall parameters, animation, physics and bounce. It does not modify scores, clocks, RNG, position, horizontal velocity, AI or original source handlers, and never performs an extra physics integration.

## Test-first and source compatibility

The failure inventory preceded code in `SUSPENDED_BALL_REPAIR_PROPOSAL.md`. Missing-helper and missing-dispatch-hook failures are preserved in `.local/basketball/rea/suspended-repair-proposal/repair-red.log` and `repair-dispatch-red.log`.

Six retained actual source states cover Court0/1/7 roll-off triggers and already suspended saves. `prepare_suspended_ball_recovery.py` independently executes their unchanged original complete ball handler, retains its original results and checks deterministic regeneration. Tests distinguish strict source output from the two-field repaired output, validate idempotence/JSON, and assert exactly one source motion event per dispatch. Thirty-six negative scenarios exclude intentional actions, possession/attachments, pending requests, vertical motion and actual support. Unknown terrain errors atomically.

The original 221 rest/rolling dispatches and 260 natural action10 calls still match unchanged source fixtures. Native, race and Wasm runs of these checks, the repair matrix, campaign runtime flow and Court7 default full-match gate passed. No source fixture expectation was changed to pretend the repair existed in the ROM.

## Full-match evidence

Unchanged fixed seeds, original match clocks, 48,000-video limit, existing intermission inputs and two deterministic repeats:

| Cold run | Video frames | Score | JSON restores per repeat |
| --- | ---: | --- | ---: |
| Campaign solo | 22,385 | 24:23 | 84 |
| Campaign solo automated | 22,457 | 39:20 | 84 |
| Campaign cooperative automated | 22,457 | 39:20 | 84 |
| Court1, teams0/5, four original computers | 22,373 | 48:39 | 84 |
| Court7 default four-computer gate | 22,373 | 41:29 | 84 |

Do not attribute the Court7 default green run specifically to this repair: a follow-up current-code policy-off/on comparison finds identical complete worlds and outcomes. That current cold trajectory no longer reaches the retained historical Court7 trigger; its original failure and source-proven trigger remain covered separately. The browser `--observe-rolloff` attempt correctly failed its event requirement and is retained, rather than claiming an event occurred.

Court1/team5 has direct causal evidence: the first policy-on/off world difference is video6300, with identical coordinates/motion/RNG and only original rolling5 versus repaired release10. The browser starts that same cold configuration and naturally records the event, then completes the real match.

## Browser acceptance

`accept_live.py --full-match --all-computer --court 1 --teams 0 5 --characters 0 1 0 1 --observe-rolloff` passed 40 checks using actual Go/Wasm, Canvas, platform audio and native controls. It fetched no ROM or playback fixture. The full match uses no world injection, altered score/clock or reseeding.

The event's authentic checkpoint is separately restored for desktop and 320px viewport checks. Both produce the same 48-frame continuation: source half-pixel gravity appears, the visible ball falls from height64 to ground, and rendered captures are retained. No fields in that checkpoint are edited. Screenshots were visually inspected.

Artifacts: `.local/basketball/all-computer-full-match-browser-teams-0-5-characters-0-1-0-1-rolloff-recovery/`, including `natural-result.json`, `natural-rolloff-events.json`, `rolloff-rendered-continuation.json`, `rolloff-{desktop,mobile}-*.png` and final states. The harness now accepts Court7 without opening it in platform selection; existing source compatibility/whole-game admission is still required. Its socket directory resolves to the existing shorter workspace path because the worktree path exceeded agent-browser's Unix socket limit; the initial infrastructure failure log is retained.

## Explicit legacy-save opt-in and recovered follow-up boundary

`TestSuspendedBallRecoveryRetainedRuntime` retains three real old failed runtime snapshots. Each first proves strict mode stays suspended. The repair continuation changes only the persisted policy and its validated context copy, preserving every gameplay field, seed and clock.

- Court0 retained video3420 finishes at22604, score34:26, with76 restore checks.
- Court7 retained video14025 finishes at22535, score50:37, with41 restore checks.
- Court1 retained video6348 initially exposed a separate pass-height boundary atvideo15047. Its complete failure state/log remain `retained-newyork-failure.json/.log`. The subsequent source-first restoration in `NEGATIVE_PASS_HEIGHT_VERIFICATION.md` proves the original receiver-heightFF domain and now completes this continuation atvideo22415, score47:51, with65 restore comparisons. The cold repaired Court1 match above does not encounter this separate path.

These three retained original failure continuations pass, but this is not exhaustive legacy-save or fourteen-win campaign acceptance. No release claim follows from the narrowly completed repair.

## Repeatability

```sh
source scripts/env.sh
.local/board-art-env/bin/python -B scripts/basketball/prepare_suspended_ball_recovery.py --check
go test -race ./games/basketball/engine -run '^(TestSuspendedBallRecoveryOnlySelectsOriginalRelease|TestSuspendedBallRecoveryRejectsIntentionalOrSupportedStates|TestSuspendedBallRecoveryNormalNextDispatch|TestSuspendedBallRecoveryDispatchPolicy|TestRestRollingCompleteOriginalDispatches|TestOriginalDunkAndReleasedBallTrace|TestCampaignPasswordEquipmentRuntimeFlow|TestRuntimeCourtSevenAllComputerCompleteMatchAndRestore)$' -count=1 -v
BASKETBALL_RUNTIME_DIAGNOSTIC=1 BASKETBALL_RUNTIME_SELECTION='{"court":1,"teams":[0,5],"characters":[0,1,0,1]}' go test ./games/basketball/engine -run '^TestRuntimeTeamRosterDiagnostic$' -count=1 -v
.local/board-art-env/bin/python -B scripts/basketball/accept_live.py --full-match --all-computer --court 1 --teams 0 5 --characters 0 1 0 1 --observe-rolloff
go test ./games/basketball/engine -run '^TestSuspendedBallRecoveryRetainedRuntime$' -count=1 -v
```

The last command also exercises the separately recovered negative receiver-height boundary; its earlier red evidence remains retained. Logs are in `.local/basketball/rea/suspended-repair-proposal/`: `repair-after.log`, `repair-race.log`, `repair-wasm.log`, `fullgames-native.log`, `team5-native.log`, `team5-browser.log`, `retained-runtime-native.log` and `retained-newyork-failure.log`. Native/race/Wasm cold trajectory hashes match within each configuration; logs retain the exact values.


## Campaign policy inheritance

The separate test-first server check `campaign_recovery_policy_test.go` covers original(false)/repaired(true) policies across victory-next-opponent and defeat-Continue, JSON restoration and the next opening-to-playing handoff. The original false cases first failed because NewCampaignRuntime supplied the new-game default. `startNextCampaign` now inherits the preceding RuntimeConfig policy before automation/first tick. New campaigns and new password entries retain the approved true default. The central hook was integrated by the root task; red evidence is `campaign-policy-red.log`, and the targeted green run is `.local/campaign-recovery-policy-green.log`.
