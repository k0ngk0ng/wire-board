# Negative receiver-height pass recovery

## Failure inventory (before tests and implementation)

- Silently clamp a negative receiver to ground, changing original flight timing.
- Widen to arbitrary negative launch heights or unproven adjacent memory/table domains.
- Use Go division toward zero for fractional negative height when the original reads signed integer bytes.
- Enter the original nonterminating arc-normalization loop when the launch height is zero and the required rise is negative.
- Change RNG, holder, target position, source fractions or unrelated state while calculating flight velocity.
- Treat a source component call as full-world/campaign completion, or hide the retained failed runtime continuation.

The retained legacy Court1 recovery continuation fails at video15047 because receiver3 is at integer height−256 while the source ball is at30. From that exact complete world, one declared original-machine alignment produces the next original8C23 call with target height `FF00` (−256), sourceheight30, targetX241/depth77, arc0. It returns velocity571/−86/1280 and gravity128 without changing RNG. The existing Go guard rejects this source-valid target.

Scope of restoration: receiver heights whose signed integer high byte isFF (−256..−1), plus the existing positive domain. Launch heights remain nonnegative and retain the existing maximum; no arbitrary negative source or wider signed target range is admitted. Zero-source/negative-rise inputs remain explicitly rejected because the original loop cannot progress.

## Implemented behavior and acceptance

Tests preceded the implementation and failed on the actual source launch. `LaunchPass` now permits only the established FF receiver-height byte domain down to −256px, preserves the source signed integer-byte read for negative fractions, and explicitly rejects the nonprogressing zero-source/negative-rise normalization. It does not change valid positive-height behavior, gravity, AI, target coordinates or source fractions. No central runtime/world hook was needed.

The independent matrix contains **4,125 complete original8C23 executions**: the retained producer, all256 negative integer targets with source heights0/1/30/62/255/511 and arcs0/24/255 wherever the source loop progresses, plus five negative fractional-target cases. All match semantic Go body results. The489 zero-source/negative-rise combinations are explicitly excluded from the finite matrix; three bounded original probes retain101 repetitions of the unchanged zero-add normalization loop in2,000 instructions. These are rejected inputs, not silently accepted source calls. The ROM itself is unchanged.

The original21 ordinary pass-launch records, existing diagonal/floor source checks, targeted race and Wasm runs, and deterministic fixture regeneration all pass. The previously failing legacy Court1 continuation now finishes naturally atvideo22415, score47:51, with65 JSON restore comparisons. The same retained Court0 andCourt7 recovery continuations still finish. No score, clock, seed, position or roster field was changed to achieve this result.

Evidence:

- ROM SHA-256: `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`.
- Fixture SHA-256: `fddd1b3e14d50d168cf4bae2e95e96b337f96408990160c163fc6112ce09ebe6`.
- Actual controlled-world pass trace SHA-256: `381712b2ab41f5adcaa946fbfdffc95a7e768895898fe35e6cb1c9af3ebf8ee6`.
- `.local/basketball/rea/negative-pass-height/{red,after,race,wasm,reproduce}.log` retains test and reproduction results. The first race attempt captured an unrelated worker's temporary missing postresult symbol; that attempt is retained separately once rerun.
- `capture_negative_pass_height.py` reproduces the declared-origin original-machine observation. It does not assert a natural menu path or use repeated state injection.

```sh
source scripts/env.sh
.local/board-art-env/bin/python -B scripts/basketball/verify_negative_pass_height.py --check
go test -race ./games/basketball/engine -run '^(TestNegativeReceiverHeight.*|TestPassLaunchAgainstOriginal|TestDiagonalLaunchAgainstOriginalFunction|TestPassFloorAgainstOriginalFunction|TestSuspendedBallRecoveryRetainedRuntime)$' -count=1 -v
```

This resolves the receiver-height launch boundary, not every possible source alias or the origin of every negative actor height. Wider negative receiver/source-height domains remain unsupported until their original producers and arithmetic are recovered.
