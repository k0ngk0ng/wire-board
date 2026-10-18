# AI late campaign difficulty verification

## Failure inventory (before source fixtures and tests)

- Treat period 28 (the ending boundary) as an active half, or omit the actual last playable periods 24–27.
- Reject a valid late-game difficulty, clamp the source delay, or fail to decrement the signed counter on entry.
- Consume zero or multiple random values during entry, consume another value on an entered call, or select pursuit options using the wrong character/team.
- Apply the computer difficulty delay to a team with a human; skip the human fixed delay or read invalid difficulty unnecessarily.
- Lose the source mode 19/39 alias, opponent target, retreat direction, depth-distance boundary, or mode 7 handoff.
- Compare only a selected counter and miss other actor, ball, command, or random-state changes.
- Call controlled AI-only continuations natural gameplay, silently update positions between them, or infer a complete campaign from them.

The acceptance scope is original complete A05D dispatcher calls for playable difficulty 0–27 and uninterrupted AI-only delay-to-engagement chains. It does not change or accept the existing whole-match stalls, and it does not establish a natural producer for mode 15.

## Findings

The existing semantic Go implementation matches all 5,376 source calls and 448 continuous AI-only chains (4,032 calls). No production AI change was required. Each comparison covers the complete `WorldInputState`, including shared random state, commands, other players and world objects. Each chain serializes/restores state between calls and then consumes the original mode 7 engagement handoff.

The matrix covers modes 19/39, all four actors, every difficulty 0–27, both computer-only and human-containing teams, six distance/depth boundary pairs and two initial random states. Entry consumes one original RNG call; entered delay calls consume none. The strict source runner does not skip sound calls or replace unknown functions, and unexpected device writes fail the run. Source decisions come from the ROM independently of Go.

The source period reset at `D420` increments `$058C` after each half. The retained original result confirmation records route period 28 to scene 7, before another playable half. `campaign-settlement-cases.json.gz` retains 32 controlled period-28 completion cases; these are not earned full-campaign wins. Thus the seven delay bands cover the current campaign's playable periods 0–27. Difficulty 28–31 remains an explicitly unsupported approach-delay input, not an established missing normal campaign branch. Other isolated AI routines accepting that band does not establish campaign reachability.

This resolves the late-period acceptance uncertainty only. Mode 15 still has no demonstrated normal producer; its prior bounded source audit remains valid. The prop-holder 4/5 team guards and remaining raw pursuit aliases still require a real producer or stronger source dataflow proof before expansion. No new handler was invented to clear those guards.

The retained Court 7 stall evidence already compares 3,864 complete worlds from an aligned controlled origin against the original machine, including AI and inputs. That source stall is not evidence of a missing AI handler. Its original-bug policy remains unanswered, and its failing whole-match test remains unchanged. This check neither reruns nor accepts that match.

## Evidence and repeatability

- ROM SHA-256: `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`.
- Fixture SHA-256: `445459afdd7f2ba14c71d40162498a7e5ea2ef60786da1c2d0c4dfc564eb1e2e`.
- Full original dispatcher source context reuses REA evidence `ev_eef21fb30302a3ec59b8b3dc097be9a7fa2a559339d598963a8489a9d646759d`; actual behavioral evidence is the newly retained unmodified ROM execution fixture and source manifest.
- Original result boundary reuses `campaign-settlement-source.json`, REA evidence `ev_7945a86e38a813fa472f9a63045cf673ae9f0c3702f1a79b6c8a17e20eebeefb` and `ev_37af32e9b034716efaee6f2adaafab2a2553752baa05f1e2889edf15118919bd`.
- Logs: `.local/basketball/rea/ai-mode15-audit/late-difficulty-{source,test,reproduce,race}.log`.

```sh
source scripts/env.sh
.local/board-art-env/bin/python -B scripts/basketball/verify_ai_late_difficulty.py --check
go test -race ./games/basketball/engine -run '^(TestOriginalAILateDifficultySource|TestOriginalAIForwardPositionAndApproachDelay|TestOriginalApproachDelayBoundaries|TestOriginalAICommandAttackGuards)$' -count=1 -v
```

The initial focused acceptance, deterministic source regeneration and all four targeted race tests passed. The race result is retained in the named log. No seed changes, source state corrections, hidden failures, shortened clocks or whole-match acceptance changes were made.
