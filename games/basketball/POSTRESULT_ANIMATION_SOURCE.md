# Post-result animation source map

Source-only investigation, 2026-10-18. No production implementation or test fixture was changed. Original ROM SHA-256: `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`.

The next recoverable unit is the original reward presentation plus the existing result-actor interpreter's remaining commands. The ending has a different scheduler: Scene 7 normally advances once per video frame and does **not** make Scene 1's unconditional RNG draw. Its phase order is **0 → 1 → 2 → 3 → 4 → 5 → 6 → 8 → 9 → 10 → 7 → reset**. Treating phases 0–10 as numerical playback order, or reusing the runtime's unconditional three-frame cadence, is incorrect.

## Evidence and scope

| Evidence | Proven scope |
| --- | --- |
| `ev_f0ce6b1dbcabba1133dc39a0adf0773ec895a3f8489b27b31893fe9bf045a4e8` | New REA bank 3 ending dispatch, every phase entry, actor loop, animation/motion interpreter. Complete bundle: `.local/postresult-source-map/analysis/evidence.json`. |
| `ev_97bb53ec5fef3eea40b717f67d611c01fb89cf1eeeda1cc2a6a73e8ff23ab744` | New REA common scheduler, Scene 1 and Scene 7 dispatch, text trampoline, controller dispatch. Bundle: `.local/postresult-source-map/scheduler-analysis/evidence.json`. |
| `ev_7945a86e38a813fa472f9a63045cf673ae9f0c3702f1a79b6c8a17e20eebeefb` | Retained bank 4 BA40 result and BD8E reward evidence, `.local/basketball/rea/match-transition/`. |
| `ev_93cedbb127d764b97aff2143de421dba36068fe4f8fb1f7be5cfbe0251cbfe78` | Retained original actor entry/animation/motion evidence, `.local/basketball/rea/result-actors/`. |
| `ev_37af32e9b034716efaee6f2adaafab2a2553752baa05f1e2889edf15118919bd` | Retained campaign/ending/defeat evidence, `.local/basketball/rea/campaign-audit/followup-analysis/`. |

New byte-level evidence and reproducible controlled probes are exclusively under `.local/postresult-source-map/`. `bounded-disassembly.txt` separates executable ranges from `source-ranges.json` pointer/bytecode/data ranges. `decoded-actor-scripts.json` decodes relevant scripts by actual command width; command operands must not be interpreted as instructions or new script commands. REA pseudocode's indirect dispatch and unreachable-block limitations remain; instruction execution and exact dispatch bytes constrain the conclusions.

`probe.py` starts from declared **controlled** cases in the existing settlement oracle, at its BC96 boundary, then executes the unchanged original four-actor confirmation tail before entering reward/ending. It does not use a naturally earned victory as its seed. It retains every call's before/after state, ordered side-effect requests, RNG counts, actor indexes and complete 2 KiB RAM snapshots, and stops before executing reset at F79D. Initial source case and exact RAM are in `summary.json`.

Explicit omitted call boundaries are C000 (audio driver), C006 (CHR commit), C00C (projection/draw-list update), C021 (palette/NMI waits), C024 (background upload), C036 (message/NMI upload). **C033 is executed**, including initialization, script commands and motion. The ending kernel probe also omits the outer FD0A text processor and outer video/input scheduler. It therefore proves the phase/action kernel under those declared boundaries, not final PPU pixels, complete RAM parity with a whole console, or real video timing. The separately named reward `outer-rng` probe additionally executes the original outer F9C3 call, while retaining the other declared omissions.

Existing natural evidence remains useful but bounded: `match-result.ram.gz` contains 145 natural result calls / 580 actor calls, SHA-256 `4f61bfa317dbebe742710673017e0693784f2a28ac78baa7fe15e48360963d1a`. The retained ordinary loss/Continue capture has full RAM SHA-256 `c89d7deef5254893d10e815edaa2e14625198fbb9fde804a85bfb5d70728b7bc`. Neither proves reward playback, final victory, or an earned fourteen-win ending movie. No such movie was created or claimed here.

## Scheduler and shared state

Fixed-bank CA54 increments `$0300`, copies pending camera `$039B..039E` to displayed `$0397..039A`, resets the draw list through CE09, samples controllers through FF80, then dispatches by Scene `$0588`.

- Scene 1 CAA4: F9C3 RNG → F049 CHR selection → D19D match-phase dispatch.
- Scene 7 CAE3: FD0A → FC83. FD0A maps bank 4 and calls `800C → A496`, the text processor. FC83 maps bank 3 and calls `8015 → B995`, ending dispatch. There is no unconditional F9C3 in this Scene 7 branch.
- CAE9 waits until NMI counter `$4C >= CB06[scene]`, then waits for the next counter change. CB06 bytes are `00 02 00 00 00 00 00 00`. Hence the ordinary minimum is three video frames for Scene 1, one for Scene 7, before accounting for blocking uploads/fades and execution time. These are source scheduling facts, not measured movie durations.
- Each phase uses `$0589 & 0x7F` plus its high-bit entry latch. Timer `$06D7:$06D8` is little-endian; reward timer is `$03C3`. `$06CA..06CD` are actor repeat flags; `$0437..043A` countdowns; `$0444..0447` two-byte-slot cursors.
- FD0A is not passive rendering: A496 resets `$0307..030A`, updates `$06AC` text-state flags/counters, consumes `$06AB` requests and `$06AD` text IDs, and advances text pointers. `text-update.asm` retains the bounded A496–A577 observation; complete text stream/control and upload integration remains a dependency.

## Reward: Scene 1 phase 7, bank 4 BD8E–BE6D

The preceding BA40 confirmation has already assigned actions `[0x62,0x63,0x64,0]`, kinds 18, source positions/poses, saved original characters in `$03CB/$03CD`, substituted characters 0/opponent, and run all four C033 actors once. Do not start the first reward frame from their pre-tail state.

| Branch | State / output / ordering |
| --- | --- |
| First entry BD96 | Set entry bit. Select reward from FF99 pointer indexed by `$06D4`; item index `(opponent-1)*2 + ((period>>1)&1)`. Insert in first negative inventory slot, scanning exactly 16 slots. No item RNG draw. Full inventory still shows offered item. |
| Entry presentation BDD4 | Preserve `$0304/$4F/$50` around background 26 upload; timer = 0; CHR `$52=$53=0x3B`; fade in via `$0301=0` (existing delay retained). Then update actors 0,1,2 through C033. |
| Entered BE0A | Increment 8-bit reward timer. If it equals 112, go directly to exit before messages/RNG/input. |
| Timer 40 | Request messages 28,30,29 in that order through C036, then continue RNG/input handling. |
| Other entered calls | C018 → F9C3 once; if returned byte `&31 == 0`, request sound 50. Then test controller-0 pressed bits `$04 & 0x90` (A or Start). Early input exit still consumes this RNG draw. |
| Exit BE45 | Fade out (`$0301=0x80`); restore `$03E3/$03E5` from saved characters; Scene=2, phase=0. **Still update actors 0,1,2 after changing destination**, then return. |

Controlled auto-exit runs 113 calls: entry plus timer 1–112. There are 111 **internal** reward RNG calls. The source-scheduled RNG variant additionally performs 113 outer Scene 1 draws, total 224; these two traces intentionally have different random/crowd outcomes. An A exit on tick 1 has one internal draw; A on tick 40 emits all three messages before its internal draw and fade. With the declared controlled seed and executed confirmation tail, actor sound 8 occurs on reward tick 51; that ordinal is a probe observation, not a natural-console frame number.

Reward script sources: action 0x62 at A01E, 0x63 at A032, 0x64 at A04C. Action 0x62 clears integer height, poses 9/9/8/7 with countdowns 8/16/8/16, emits sound 8, then repeats pose 7 through its FA command. Action 0x63 clears integer height, poses 10/11, sets X velocity `0xF400`, toggles the retained mirror for pose 10 for countdown 1, clears X velocity, then holds pose 10 before switching to idle. Action 0x64 clears integer height, poses 7/8/9 with countdowns 24/8/240, then idle. Exact slots/operands and wrap semantics are retained in the decoded script artifact.

## Ending: Scene 7, bank 3 B995

Dispatch table B99B has twelve little-endian entries; phase 11 is the separate defeat/Continue path. Every normal phase branch ends at BF35, updating actors **0,1,2,3** through C033 in ascending order, including phase-change calls. The reset branch jumps away without that final tail.

| Phase / entry | First entry and later transition | Graphics / text / audio |
| --- | --- | --- |
| 0 / B9B3 | Set entry bit and `$4D`; mirroring=0; zero timer and pending camera. If Period !=28, set phase11. For Period28 initialize actions 51/52/53/54, kinds 19/18/18/18, hidden mirrored poses, `$06CA..06D1=0`, actor0 X/depth velocity=0, `$06D9=0`. Next call fades in and sets phase1. | Backgrounds95,96; CHR pages `[25,26,0B,28]` (hex); palette `$0305=49`; sound75 on transition to1. |
| 1 / BA74 | Entry requests text ID1. Subsequent calls increment timer; exact `0x0370` (880) → phase2. Before the gate `$06D9=(timerLow>>3)&1`. | `$06AB=0x80`, `$06AD=1`; actors51–54 run throughout. |
| 2 / BABD | Entry only latches. Next call fades out, clears four repeat flags/text flags, assigns actions55/56/57/0 and kinds19/19/18/19, then phase3. | Poses `[FF,FF,7F,7F]`; fade delay1. |
| 3 / BB25 | Entry zeroes timer and `$06D9`; later exact `0x0E08` (3592) → phase4. | Background97; palettes50,51; fade in delay1. Scripts request text and can start actor3 action58. |
| 4 / BB83 | Entry latches. Next call fades out, clears four actions/repeat/X velocities, kinds17, hides poses except actor0 mirror; clears text flags; installs actions5C/5D/5E/0; phase5. | Backgrounds33,34. Zero 32-byte upload buffer at030E; upload length/low address32, high address24(hex). |
| 5 / BC04 | Entry sets timer=0. Later increment; if actor2 action masked !=5E and timerLow=0, use table BC9A[timerHigh] as its new action and reset its cursor/countdown. While !=5E, subtract BC8E[action-0C] from `$06D9`. Exact `0x0F80` (3968) → phase6. | Palettes13,14; fade in delay1. BC9A table = `[0C,0C,0C,0C,0D,0E,0F,10,10,10,10,10,10,10,10,10]`; BC8E = `[1,2,3,4,5,6,6,6,6,6,6,6]`. Actor1's F9 starts actor2 action0C before these later overrides. |
| 6 / BCAA | No entry-latch wait: fade out; clear four actions/repeats; install actor0 action5F and kind18, then **phase8**. | CHR `$51=38,$52=39` hex; actor0 poseFF; palettes29,30. |
| 8 / BD27 | Entry clears text-active flag, credits offsets `$06DA..06DE`, timer. Later each call subtracts2 from06DA,1 from06DB,0x0080 from pair(low06DE, high06DC),1 from06DD; increments timer. Exact `0x10C0` (4288): fade out delay2, set actor0 action0, phase9. | Background25 then98; fade in delay1; sound76; request text ID36 (`$06AD=0x24`). Actor0 action5F moves X at `0xFFE4`. |
| 9 / BDDF | Entry clears text flags. Entered waits for `$04 & 0x90`; no automatic timer exit. On A/Start fade out delay1 and phase10. | Background23; fade in delay2. |
| 10 / BE24 | Entry latches; next call sets **phase7**. | Four actors still run on both calls. |
| 7 / BCF3 | Entry latches; next call clears all four X/depth velocities, sets Period0, sound0, `$4D=0xC0`; jumps FFF4→F79D reset. | Original machine reset is proven; probe stops at F79D. This is not a next-opponent continuation. |

The controlled kernel executes 12,745 calls, with phase entry call ordinals `0:0, 1:2, 2:883, 3:885, 4:4478, 5:4480, 6:8449, 8:8450, 9:12739, 10:12741, 7:12743`; it presses Start on the first entered phase9 call. Timers are unshortened, RNG is not reseeded, and no intermediate RAM snapshots are used to force transitions. These ordinals exclude real upload/fade/text scheduling delays. No F9C3 call occurs in the measured ending kernel. The examined outer Scene7 branch also has no unconditional RNG; a claim covering every text/upload/NMI helper still requires their complete observation.

## C033 is stateful animation and motion

`C033 → FCB0 → bank3:801E → 9598`: initialize unentered action (set high bit, reset cursor/countdown), execute 95B4 script, integrate motion at9769, then C00C projection/draw update. Existing `engine/result.go` already implements this narrow source family, but reward actions currently stop after the first pose. It must not be replaced by the gameplay animation interpreter or a visual-only tween.

| Script opcode | Source handler / state effect |
| --- | --- |
| `<F0` | Pose XOR retained mirror; next byte countdown; one two-byte slot. A newly set countdown N is decremented on subsequent calls and advances on the following call. |
| F0 / 9614 | Set integer X high/low and depth integer, retain fractions. |
| F1 / 962B | Set signed X/depth velocities; six-byte command. |
| F2 / 964E | Set integer height high/low; retain fraction. |
| F3 / 9660 | Height velocity high/low and gravity low; gravity high=0. |
| F4 / 967F | Change actor character. |
| F5 / 968A | Request text (`06AB=80`, `06AD=ID`), processed next outer text update. |
| F6 / 969A | Ordered sound request. |
| F7 / 96A5 | Change two palette IDs. |
| F8 / 96B9 | Change this actor action, clear cursor and immediately continue script dispatch. Entry latch is resolved on next actor call. |
| F9 / 96C9 | Change another actor action and reset its cursor/countdown. Ascending actor order matters: a later actor can execute the new action in this same call. FE/FF alias this handler in the pointer table. |
| FA / 96E7 | Four-byte loop command: first visit installs sign bit OR initial repeat byte and jumps to target slot; later visits increment repeat byte until wrap to0, then skip the complete command. |
| FB / 971C | Add camera deltas with original byte/carry behavior and vertical F0-wrap handling; do not replace with normal signed vector addition. |
| FC / 9753 | Assign phase byte. |
| FD / 975E | Assign display mask0304. |

9769 sign-extends X velocity into 24-bit X, updates 16-bit depth, subtracts gravity from height velocity and integrates only the low16 bits of height, retaining its high byte. Relevant actions include 0, 0C–10, 51–58, 5C–5F, 62–65; some are activated by other actors rather than phase initialization. Do not deduce reachable command kinds by searching raw bytes: e.g. F8/FC can also be operand bytes inside FA.

## Maintainable integration boundary and acceptance order

1. **Preserve the current immutable confirmed match result.** Add a separate persisted campaign-presentation state/step result that owns current scene, phase/latch, timer, actors/cursors/repeats, text state, camera/credits offsets, palette/CHR/background requests, RNG and completion event. The outcome's `NextScene/NextPhase` is the initial destination, not a moving cursor. Current `validateRuntimeOutcome` requires an unentered parked destination; relax it only through an explicit presentation variant and validation contract.
2. **Recover reward first.** Extend the source-specific result actor command implementation/state interface to actions62–64 and their shared idle path, with exact movement/sound/projection and the final exit actor tail. Then implement BD8E's phase driver, using the already-proven award transaction once. Feed the resulting RNG into next preparation; `SettleCampaignRuntime` currently returns confirmation RNG, which is not the post-animation RNG. Never instantiate a newly seeded match to hide the missing sequence.
3. **Recover ending actors, text and presentation resources.** Keep typed declarative script/phase data generated from documented source ranges, or explicit semantic handlers, separate from gameplay actions. Cross-actor/phase/palette/text commands require scene context; the current `stepOriginalResultAnimation(o, repeat)` interface is too narrow. Reuse its correct motion and projection pipeline. Implement FD0A text state and graphics resources before claiming ending visuals. Actual image/background naming, text glyphs, IRQ/CHR layout and upload/fade duration need original captures.
4. **Integrate scheduler and platform lifecycle.** Runtime currently sets `FramesUntilLogic=3` unconditionally and the transition path draws RNG before dispatch. Add source-bound Scene7 cadence/no-draw behavior and video waits. Server `settleCampaign` currently enters `reward` with a platform 3000ms decision deadline or terminal `complete` immediately. Those are lifecycle conveniences, not source animation timing. Continue computing the immutable settlement once, but keep the animation advancing until its original handoff/terminal event. Define the reset event's platform behavior without exposing an emulator or falsely claiming an original title-screen frame.
5. **Test before implementation.** Start with the declared failure list in `.local/postresult-source-map/FAILURES.md`. Write narrowly scoped failing Go comparisons for first-entry/repeated/exit reward calls, RNG counts and final actor tail; every ending phase boundary; per-command actor writes; cross-actor order; Scene7 cadence; save/restore during countdown/fade/text/credits; immutable outcome and no repeated item insertion. Fixtures must keep source-call boundaries and declared omissions visible. These tests are justified isolated source comparisons, not a substitute for end-to-end acceptance.
6. **Record integrated E2E evidence after the components work.** Ordinary earned win→reward→next map with original inputs, natural loss→Continue, final-win→full ending→A/Start→reset event; desktop/320px images, video/frame/timer/RNG/audio event traces, JSON restoration, reconnect/hidden-page behavior and repeatable commands. Then perform the complete fourteen-win campaign using the actual recovered gameplay/AI, normal clocks and unchanged seed policy. Retain first failure if motion/AI stops progress; fix that source behavior rather than writing scores, replacing AI, reseeding or shortening clocks.

Dependencies remain explicit: earned victories depend on the remaining full gameplay motion/AI acceptance; final actor graphics depend on source projection and scene resources; text progression precedes phase dispatch; music76 needs a valid intro-once/loop mapping. Existing audio documentation already proves finite music75, but states76 has no runtime distribution mapping (`assets/COURT_AUDIO.md`, `web/AUDIO_SCENE_STATE.md`). This investigation does not change those acceptance statuses.

## Reproduction and unresolved work

From the basketball worktree:

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python .local/postresult-source-map/probe.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python .local/postresult-source-map/decode_scripts.py
node .local/postresult-source-map/rea_batch.mjs .local/postresult-source-map/spec.json
node .local/postresult-source-map/rea_batch.mjs .local/postresult-source-map/scheduler-spec.json
```

The REA runner uses the existing canonical launcher and existing Ghidra/Python assets. It neither installs tools nor clears caches. Probe repeat hashes and source/evidence file hashes are retained in `reproducibility.json` and `manifest.json`. Both REA sessions were closed after evidence export.

Answered: exact phase table/order, main timer gates, reward input/RNG ordering, actor interpreter side effects, source scheduler difference, sound/background/CHR/palette request IDs, bounded reset destination. Partially answered: full text processing, final graphics and music timeline. Outstanding: whole-console post-victory/ending traces with uploads and NMI timing, full pixel comparison, music76 integration, natural earned campaign, terminal reset presentation, integrated restoration and platform E2E. The game and overall reconstruction goal remain incomplete.
