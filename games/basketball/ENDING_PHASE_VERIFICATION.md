# Ending phase driver verification

## Failure inventory recorded before implementation

- Confuse entry latch with continuation; use numerical phase order instead of 0,1,2,3,4,5,6,8,9,10,7/reset.
- Increment timers on entry or compare before increment; use >= rather than exact16-bit equality; miss carry/wrap.
- Phase0 period guard incorrectly plays ending for non28 periods; lose original camera/control/mirroring writes.
- Lose actor kind/pose/action/repeat resets or clear fields the source retains; accidentally execute the BF35 actor tail in the phase-only API.
- Phase5 resets actor2 on every call instead of low-byte zero, fails to mask entered bit, changes action5E, uses wrong table or subtraction/wrap order, or drops cursor/countdown resets.
- Phase8 credits use wrong low/high byte pair or borrow; omit text-active/reset flag difference; wrong entry/event ordering.
- Phase9 accepts held buttons instead of controller0 pressed A/Start, accepts B/directions, skips first-entry wait or invents an automatic exit.
- Phase6 incorrectly waits for latch; phase10 goes to reset directly; phase7 resets on entry, runs actor tail after reset, or changes RNG.
- Asset requests lose source ordering or the state at each omitted call; confuse request boundaries with rendered images/video waits.
- Duplicate authoritative timer/credits in obsolete gameplay Passage aliases.
- JSON restore changes phase, timer, actor or presentation context.
- Unsupported scene/phase/table-index context partially mutates state instead of returning an explicit error.

Evidence is bounded unchanged-ROM phase calls ending at BF35 before actors or F79D before machine reset. C000/C006/C021/C024 are typed request boundaries, not implemented audio/CHR/fade/background effects. No outer text update, input/video scheduler, PPU rendering or natural earned ending is claimed.

## Implemented component

`engine/ending_phase.go` exposes `StepOriginalEndingPhase(OpeningState, EndingPhaseContext, Buttons)`. It handles Scene 7 phases 0 through 10. The returned `Reset` marks the original jump to F79D; reset instructions are not executed. Every successful non-reset call still needs the BF35 actor tail, including phase-change calls. Phase0 with Period other than28 returns the original phase11 destination; the separate defeat/Continue driver is outside this API.

The source order is 0 → 1 → 2 → 3 → 4 → 5 → 6 → 8 → 9 → 10 → 7 → reset. Timers remain16-bit and use the original exact-equality gates. Phase6 has no entry delay. Phase9 waits indefinitely for controller0 pressed A/Start after its entry call. Phase5 preserves action5E, entered-bit interpretation, source table lookup timing, cursor/countdown resets and byte subtraction. Phase8 preserves the unusual credits pair (low06DE/high06DC), including borrow and wrap.

`EndingPhaseContext.Actor` reuses `ResultActorContext` for text requests, palette IDs and display mask. The ending context adds text-active state, CHR pages, background/fade request registers, machine control and pending upload buffer. `EndingTimer` and `EndingCredits` give semantic names to the existing `Transition.Passages` storage at06D7..06DF. There is one authoritative copy; no RAM differences are normalized away in tests.

Requests preserve source order and values at mirroring, sound, CHR, fade and background boundaries. The phase4 pending upload buffer is retained as typed address/length/data state. These outputs describe requests, not completed rendering, audio playback or waits. The implementation performs no actor motion, projection, text processing, RNG draw, clock shortening or reseeding.

## Source evidence and reproduction

Source map: `POSTRESULT_ANIMATION_SOURCE.md`. REA ending evidence: `ev_f0ce6b1dbcabba1133dc39a0adf0773ec895a3f8489b27b31893fe9bf045a4e8`. Executed ROM SHA-256: `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`.

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_ending_phase.py
GOCACHE="$PWD/.local/go-build" GOMODCACHE="$PWD/.local/go-mod" GOTMPDIR="$PWD/.local/tmp" TMPDIR="$PWD/.local/tmp" GOTOOLCHAIN=local go test ./games/basketball/engine -run '^TestOriginalEndingPhase' -count=1 -v
```

The fixture retains1,798 unchanged-ROM calls, each with complete initial/final RAM, executed PC path and ordered requests. Of these,896 are samples from the retained controlled ending trajectory: phase/latch changes, timer-byte boundaries and actor2 action changes. Additional declared states cover every phase's entry/continuation, exact timer boundaries and wrap, Period guard, pressed-button masks, actor2 subtraction/table transitions and credits borrow. Table indexes0/2/3 are not separately exercised by a successful lookup; their source constants remain statically read as12. The fixture is a bounded component matrix, not an exhaustive combination space.

All calls stop at BF35 before actor execution or F79D before machine reset. C000/C006/C021/C024 are intercepted as request boundaries. The outer FD0A text processor, controller/video loop, NMI waits and rendering are omitted explicitly. The retained source trajectory starts from a declared controlled settlement case; it is not an earned final victory or a whole-console movie.

Tests were written before implementation; `.local/ending-phase-restore/red.txt` records missing API types/functions. After implementation, all1,798 full state/context/request comparisons passed; JSON round-trip replay passed for every row; unsupported scene/phase or unrecovered table contexts return errors without partial changes. The targeted suite passed in0.827s. No source case was skipped after failure. Zero original and semantic RNG calls occur in this component.

A second source run reproduced fixture SHA-256 `7e9d1d8013ec7f56d0f513d27ad57967ff57a00bf18de3a4bb90ed5416ad654d`. `engine/testdata/ending-phase-source.json` retains exact sampled ordinals, source RAM digest and omissions. Local execution logs are in `.local/ending-phase-restore/`.

## Integration boundary and remaining work

The caller must execute FD0A text processing before phase dispatch, then run actors0..3 in ascending order after a successful non-reset phase call. Pass `Context.Actor` into `StepOriginalResultActor`, carry its returned context/state forward after each actor, and retain actor sound ordering after phase requests. The Scene7 scheduler has no unconditional Scene1 RNG draw. Requests need a separately validated rendering/audio/wait implementation before claiming original presentation timing.

This component does not implement phase11 defeat/Continue, runtime/server lifecycle, immutable-outcome integration, final graphics, music76, text streams, natural input E2E or the complete fourteen-win campaign. Unsupported phase5 table addresses outside the documented16-entry action table or12-entry offset table fail explicitly rather than reading executable bytes as invented semantic data. The subsequent phase-plus-actor kernel is separately verified in `ENDING_PRESENTATION_VERIFICATION.md`; it retains the same text/render/video limitations. Overall game reconstruction remains incomplete.
