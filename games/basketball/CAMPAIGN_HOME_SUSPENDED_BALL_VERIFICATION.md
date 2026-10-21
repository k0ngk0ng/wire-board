# Campaign home court suspended-ball investigation

## Failure inventory

- Treat a matching final ball as proof that the preceding whole-world path matches the original.
- Fail to distinguish source bugs from an upstream Go action, AI, contact, or equipment divergence.
- Reset seed, shorten the match, move the ball, fake scores, change expected failures, or add a watchdog while original-bug policy remains unanswered.
- Reinject source state each frame and call that an independent continuation.
- Omit controller/mode/equipment state when creating a declared aligned source origin.
- Miss the exact support boundary because airborne is confused with nonzero absolute height.
- Claim an aligned segment is an original-menu natural match or a completed campaign.

Investigation uses the existing cold diagnostic without changing production code or acceptance expectations. Source component calls precede a bounded full-machine comparison.

## Result

Both `TestCampaignPasswordEquipmentRuntimeFlow/solo-automated` and `coop-automated` reproduce the original unsupported roll-off defect. No production repair or acceptance expectation was changed. Their 48,000-frame failures remain failures; the diagnostic recorder succeeding means only that it captured the failure.

The retained configuration is Court 0, teams 0/1, characters 0/1/0/3, campaign period 2, shirt item 1 on character 0, four automated original players and seed `651989288`. Solo and cooperative mode use their own configuration values and produce the same rule-world trajectory in this all-automated run. The starting equipment, score and seed were not changed.

### Exact trigger

| Go video | Original ball behavior | Actual terrain support |
| --- | --- | --- |
| 3342 | Last bounce lands at height 32; vertical velocity and gravity become 0. | 32 |
| 3345 | Action 6 consumes its final bounce, becomes rolling action 5. | 32 |
| 3381 | Rolling has X velocity 0 and depth velocity −53/256; depth is 72.140625. | 32 |
| 3384 | Action 5 switches to rest action 0, but its final movement changes depth to 71.93359375. Airborne becomes true; gravity stays 0. | 0 |
| 3387 | Rest action clears the remaining depth velocity. | 0 |
| 3420 | The unchanged diagnostic has seen 12 persistent suspended rest states. | 0 |

The final position is X `257.5390625`, depth `71.93359375`, height `32`. The ball is unowned, action 0, airborne, with all velocities and gravity zero. This is the same depth-friction boundary mechanism established for Court 7, at Court 0's depth=72 platform edge.

### Independent source comparisons

For each mode:

1. The existing cold diagnostic retains 50 complete before/after worlds surrounding the trigger. All 50 original `$E149..E14C` complete ball calls match Go; one continuously executed 50-call ball-only chain also matches. Original terrain bytes give support 32 before the edge and 0 after it.
2. A source whole-machine run is initialized once from Go video 3272/world 1053, before the final bounce and roll-off. It then runs 13,000 video frames without further state writes. All **3,864 complete compared rule worlds** match independently advanced Go. The original remains suspended for **3,825 worlds** and reaches clock 0, score 0:3, still playing.
3. To exclude an earlier gameplay divergence, a separate capture starts once from the first platform playing world, Go video 114/world 1. All **3,864 complete compared rule worlds** again match independently advanced Go, including the entire path to roll-off and **2,773 suspended worlds**. The original again reaches clock 0, score 0:3, still playing.
4. Each capture has a separate reference run without ongoing observations. Its entire 13,000-frame RAM trace and all five screenshots match the corresponding observed run byte for byte.

Each controlled origin is explicitly encoded and round-tripped against the source decoder before comparison. No subsequent state is injected. Whole-world comparison includes players, ball, AI and inputs. It explicitly excludes reused zero-page contact scratch and rendering projection; it does not claim all RAM equals Go, original-menu natural coverage, a completed campaign, or that initialization before the first playing world has been proven equivalent.

This closes the suspected upstream gameplay divergence for these two bounded cold configurations. Changing AI would not reproduce the source more faithfully. The original-bug handling policy remains unanswered; no watchdog, altered gravity, fake win or automatic abandonment was introduced.

## Artifacts and reproduction

All original and Go evidence is retained under `.local/basketball/rea/campaign-home-stall/`, with `solo/` and `coop/` directories. Each contains the cold diagnostic `first-entry/`, component source calls, declared origin/patch, observed/reference specifications, original result and full Go world JSONL. The `cold/` subdirectory contains the separate first-playing-world comparison. Full machine RAM and screenshots are in `.local/nes-basket/runs/campaign-home-stall-{solo,coop}[-cold]-{reference,observed}/`.

The input snapshots `equipment-runtime-regression-{0,1}.json` are extracted verbatim from the retained failing test's `before=` log lines, not edited runtime states. `.local/equipment-baseline-regression.log`, `.local/equipment-baseline-coop.log` and `.local/equipment-runtime-regression.log` retain the pre-change/current failure evidence.

For solo, using the retained failure snapshot:

```sh
source scripts/env.sh
BASKETBALL_REST_STALL_OUTPUT="$PWD/.local/basketball/rea/campaign-home-stall/solo/first-entry" BASKETBALL_REST_STALL_SOURCE="$PWD/.local/basketball/rea/campaign-home-stall/equipment-runtime-regression-0.json" go test ./games/basketball/engine -run '^TestBallRestStallDiagnostic$' -count=1 -v
.local/board-art-env/bin/python -B scripts/basketball/capture_campaign_home_stall.py --mode solo
go run ./scripts/basketball/replay_world_segment .local/basketball/rea/campaign-home-stall/solo/controlled-origin.json 3864 > .local/basketball/rea/campaign-home-stall/solo/go-worlds.jsonl
.local/board-art-env/bin/python -B scripts/basketball/compare_campaign_home_stall.py --mode solo

go run ./scripts/basketball/replay_world_segment --cold-origin .local/basketball/rea/campaign-home-stall/equipment-runtime-regression-0.json > .local/basketball/rea/campaign-home-stall/solo/cold-origin.json
.local/board-art-env/bin/python -B scripts/basketball/capture_campaign_home_stall.py --mode solo --cold
go run ./scripts/basketball/replay_world_segment .local/basketball/rea/campaign-home-stall/solo/cold-origin.json 3864 > .local/basketball/rea/campaign-home-stall/solo/cold/go-worlds.jsonl
.local/board-art-env/bin/python -B scripts/basketball/compare_campaign_home_stall.py --mode solo --cold
```

For cooperative mode use `coop` and failure snapshot `equipment-runtime-regression-1.json`. Both complete command sequences were executed. Source ROM identity, per-segment RAM/world hashes and comparison results are retained in `engine/testdata/campaign-home-stall-source.json`. The capture scripts use the existing workspace-contained FCEUX tooling; no new emulator or REA setup is required.


## 后续：已获授权的修复

用户随后明确选择修复原版卡死问题。新对局通过持久化策略启用原动作10下落路径；旧快照缺省仍保留原行为。本文中的原版缺陷与失败证据保持原样。实现、真实触发、浏览器动画、原源兼容与旧存档续跑结果见 [修复验收](SUSPENDED_BALL_RECOVERY_VERIFICATION.md)。这不等于完整战役或所有球场模式已通过最终发布验收。
