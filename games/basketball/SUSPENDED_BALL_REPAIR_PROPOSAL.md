# Suspended roll-off repair proposal (approved)

The user explicitly selected repairing the original stall. This document preserves the design and pre-implementation failure inventory. Implementation and current acceptance are recorded in `SUSPENDED_BALL_RECOVERY_VERIFICATION.md`; old original-mode failure evidence remains retained.

## Recommended mechanical change if repair is authorized

Reuse **original ball action 10**, already implemented by `StepDunkReleasedBall` (`bank6 B8CB`). After a successful ordinary ball dispatch, detect the narrowly defined unsupported rest/rolling state and set only `Action=10, Entered=false`. Let the **next normal dispatch** initialize and execute the existing original falling path. Do not integrate a second time in the detecting update, invent a falling AI, teleport the ball, clear airborne manually, alter the score/clock, or wait for a timeout.

Original action 10 is the shared falling release reached both by dunk release and by the original stuck-rim countdown. It adds `16/256` to vertical velocity and `128/256` to gravity, resets its spin animation, releases holder to 6, and uses normal original court physics. From zero vertical velocity/gravity, the first integrated change is **−112/256 pixels**, so the ball immediately falls rather than hopping upward. On landing it enters the original bounce action 6. These values are the original handler's, not new tuning parameters.

Existing evidence: `DUNK_CHAIN_VERIFICATION.md` retains 260 natural action-10 calls, including eight release entries, under REA `ev_bea0f79ba87f478cf701e72e25cfa17b2a82dca749f06781243f52a48313e99b`. `BALL_DEVICE_VERIFICATION.md` documents the original action9→10 countdown under `ev_e334d6e67ec5f29ef0edf180d96d811d9eb87ed5f6ae515c56546d71b603e97b`.

Do not use action 7, which reverses X velocity on entry, or action 11, which adds a 5px upward toss. A gravity-only repair is mechanically smaller but leaves action 0/5, so it bypasses the existing falling animation, airborne pursuit and fall→bounce transition. The source-probed action-10 option better reuses a complete original falling path.

## Exact eligibility

Evaluate only when the selected persisted repair policy is enabled and an active **playing-world** ball dispatch has completed successfully, after normal request resolution, actor interactions and court motion.

All conditions must hold:

- `Holder == 6`: genuinely unowned, not a player or auxiliary owner.
- `BallLink & 128 == 0`: not attached to any supporting player/object. Never detach it as part of this repair.
- The completed action is ordinary rest 0 or rolling 5. This includes bounce6→rolling5 in the detecting update. Exclude held/dribble1/2, passes/shots, stuck-rim9, release10, dropped11 and all special actions.
- No pending ball request remains; a legitimate request must be consumed by the original dispatcher before reconsidering recovery.
- Vertical velocity and gravity are both exactly zero. Preserve X/depth velocity and every fractional coordinate.
- The source motion result is airborne, and the **integer** ball height is greater than the source terrain support height at its final X/depth. A fractional offset above the same integer floor is not enough.
- There is no active attached-motion or court-passage route. Original dynamic support is handled through attachment or dedicated actions; unknown support/terrain must return the existing validation failure rather than assume ground 0.

Use `OriginalCourtSurface(court, finalX, finalDepth).Height()` for the canonical static terrain lookup, with existing court-motion/attachment semantics. Do not run `ResolveOriginalCourtSurface` as a speculative query: it can change state, trigger devices or consume RNG. Do not replace the source's old-position/candidate-position sampling order.

The narrow action and ownership tests exclude intentional suspensions. A supported ball at height32/64/24 must keep the original raised-platform release behavior; merely testing nonzero height or airborne would be unsafe. Keep the repair outside generic `IntegrateMotion`, outside the original source handlers and outside result/opening animations.

At detection the only new writes are action10 and clearing its entry bit; no RNG, sound, contact, score or clock write occurs. Later original falling, catch, AI, bounce and scoring behavior may naturally diverge from the buggy stalled trajectory. That divergence is the explicit rule fix, not source equivalence.

## Source probes of candidate continuations

`probe_suspended_ball_recovery_paths.py` runs unmodified complete `$E149..E14C` ball dispatches, 96 consecutive calls per candidate, for each of the three retained failures. Each candidate has a declared one-time initial edit. There is no Go implementation or per-call reinjection.

| Source failure | Unchanged control after 96 calls | Original action10: first ground call | Gravity256-only: first ground call |
| --- | --- | ---: | ---: |
| Court0, height32 | Still suspended | 11 | 8 |
| Court1, height64 | Still suspended | 16 | 11 |
| Court7, height24 | Still suspended | 10 | 7 |

All action10 candidates reach ordinary rest after their normal bounce sequence within 96 calls. The probe does **not** establish full-world catch/scoring completion, and does not claim the original ever selects action10 from the buggy rest state. The new selection would be the explicitly authorized repair.

Artifacts: `.local/basketball/rea/suspended-repair-proposal/paths.json`. Reproduce:

```sh
source scripts/env.sh
.local/board-art-env/bin/python -B scripts/basketball/probe_suspended_ball_recovery_paths.py
```

## Persistence and source compatibility

Represent the behavior as a versioned, persisted game-rule policy. The zero/missing value must retain strict original behavior for existing snapshots and source fixtures. If repair is selected, new platform matches can choose the repaired policy as their explicit default; snapshots retain that decision through automation, reconnection, intermission, campaign progression and replay. Do not infer the policy from wall time, failure history, court, client version or current admin settings.

Keep `StepRestingBall`, `StepRollingBall`, `StepBouncingBall`, `StepDunkReleasedBall` and original court-motion source tests unchanged. Apply the optional dispatch-level transition in the canonical world boundary. Strict mode must preserve every source fixture and the already retained suspended-ball evidence. Repaired mode gets separate expectations. Do not silently alter fixture results or relabel the old red full-match evidence as a successful original match.

## Test-first failure and acceptance plan after authorization

1. Preserve the Court0 solo/coop, Court1 and Court7 original failures and their source hashes. Add the repair policy and failing acceptance before its behavior implementation.
2. Use the **actual retained trigger worlds**, not only fabricated rest states: Court0 and7 depth-friction rolloff; Court1 bounce6→rolling5 crossing the X edge. Assert exactly one policy transition, preserved coordinates/X-depth velocity/ownership/request/RNG and no extra motion event.
3. Existing suspended saves: opt into the persisted repaired policy explicitly and verify a normal action10 entry on the next dispatch. Zero/missing/original policy stays source-identical; do not silently upgrade old saves.
4. Negative matrix: real ground0 and raised surfaces24/32/64, fractional-height-only offsets, attached supports, held and dribbling balls, pending legitimate requests, rim9, special shots, nonzero vertical motion or gravity, opening/result phases, unknown terrain and New York passage. No correction or state mutation on errors.
5. Compare the reused action10 continuation against the declared source candidate, including its additive parameters, spin, source bounce6 handoff and ordinary landing. Verify request/animation and source MotionSurfaceWrites order; no second integration.
6. Native/race/Wasm deterministic parity and JSON restoration at detection, before action10 initialization, in flight and after landing. Multiplayer/spectator snapshots observe the same authoritative local animation sequence.
7. Re-run the original fixed-seed, normal-clock failed full matches with repair enabled: both campaign automated modes, Court1 team5 and Court7. No seed changes, score injection or timeout-limit expansion. Repeat completed trajectories and restore at the existing checkpoints; any newly exposed error remains red.
8. Browser E2E shows visible fall, natural pickup/bounce and normal period/result progression, with reproducible video/screenshots and desktop/mobile coverage. Court7 remains unavailable until its **whole** existing acceptance gate passes; fixing this one blocker does not authorize admission by itself.

Other games' rule matrices are outside this game-local change. Existing strict source-component and world comparison suites remain separate compatibility checks.
