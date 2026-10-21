# Original selection admission audit

The old platform limits—courts0–6, teams exactly0/1, and character slots0/1—were
**development gates, not original rules**. Final single-match selection must
expose all eight courts, all eight teams with different opposing teams, and all
four character slots per team with different teammates. Original versus matches
have no equipment. Campaign mode retains its separately recovered home team0,
seven opponents, opponent roster selection and equipment rules.

## Existing original evidence re-audited

`audit_selection_admission_source.py` rereads the retained source artifacts and
checks their SHA-256 bindings. No new emulator session, RAM mutation, seed change
or reverse-engineering installation was needed.

- Eight controller-only versus routes select all courts while retaining teams0/4.
- Eight other routes select all teams and display their four-member roster.
- All56 distinct ordered team pairs were selected through the original menu;
  the second-team cursor skips the already selected team.
- Eight original member-collision routes, one per team, reject choosing an already
  occupied teammate; a following directional input is a positive movement control.
- Twenty-four bound menu screenshots and the named source records match their
  recorded hashes. Original member layout places slots0/1 in the left column and
  slots2/3 in the right column. These are character choices, not platform seats.

Source detail remains in `selection/NAMES_VERIFICATION.md`. Its earlier UI gate
paragraph describes a historical implementation, not original content limits.
The source audit does not claim every multiplayer-menu permutation or a complete
match from each original menu route.

## Explicit complete-match matrix

`TestRuntimeOriginalSelectionAdmissionMatrix` is opt-in and contains56 ordered
team-pair configurations, with deterministic representative court/roster choices,
plus32 Court7 configurations: each of eight teams in the first position,
characters2/3, and solo/cooperative/versus/all-computer control arrangements.
`TestRuntimeAllOriginalCharactersAdmission` adds eight all-computer configurations
using character slots0/1. Together they cover all32 characters and all courts in
**96 configurations, each repeated twice**.

Every run starts cold with seed12345678, uses normal three-minute halves and
source AI, checks periodic JSON restore equivalence and trajectory determinism,
and requires the actual score/result presentation. No clock shortening,
precomputed outcome or seed search is used. These are representative integration
cases, not the enormous Cartesian product of all selections and human inputs.

The first88-configuration run retained81 passing cases and seven failures. Five
were a missing shared-target dunk domain; two were genuine tied untimed overtime.
A follow-up after the dunk fix exposed a separate high-receiver pass guard. The
original failed trajectories remain distinct evidence; they were not rewritten
as passing runs.

## Source fixes found by the matrix

### Entered dunk and shared target byte

The original056E target is shared scratch. Another actor can overwrite it with
0/1 while a previously entered dunk is still animating. Bank6 8E9D only requires
hoop mount geometry during action entry; rejecting a non-hoop target on every
later update incorrectly aborted valid source execution.

The restored handler preserves the live target action writes and0577+target
aliases. For targets0..2 those bytes are actors1..3 JumpPhase; targets3..6 are
actors0..3 AttackVariant. They remain their existing semantic owners rather than
an additional fake durability array. Invalid unentered mounts remain rejected.

Tests and source fixtures preceded the implementation:420 complete original
E165..E168 calls and48 continuous entered-dunk chains containing720 calls. Only
the existing FBCC sound sink is intercepted; source action/motion instructions
are unchanged. The entered target corpus covers0..6, all four actors, all three
dunk actions and every animation cursor. Existing hoop-target fixtures remain
unchanged and passing.

### Elevated pass receiver

Teams4/7 on Court3, original fixed seed, reached video22310 with a receiver at
626px. The prior511px guard rejected an original-valid launch. Before widening
it,19,950 complete original8C23 calls checked source/receiver integer heights
through03FF, all integer heights512..1023, representative lower heights and
arcs0/24/255. All source calls terminate without skipped instructions or RNG
changes and match Go launch velocity/gravity.

The change only expands the proven positive height guard to1023px. It does not
clamp or change actor positions, pass physics, AI, RNG or scores. Existing FF
negative receiver behavior and zero-source/nonprogressing normalization rejection
remain. Values outside the proven positive domain remain rejected.

### Idle human seats in untimed overtime

Court7 teams2/3 and3/4 with characters2/3 in versus mode reached54:54 at the fixed
48,000-video diagnostic bound. Both had the original second-period overtime
flag128 and zero clock; one ball was held normally, the other was in flight.
This is the original untimed tie rule, not the historical suspended-ball bug.

The acceptance now explicitly retains the complete48,000-frame no-input prefix,
then invokes the existing `SetRuntimeAutomation` platform API. The original AI
finishes at videos48,800 (54:58) and48,923 (61:54); both repeats have identical
trajectory hashes. This is declared seat takeover, not a new strategy, injected
score or claimed automatic timeout inside the game engine. Platform timeout
behavior is verified separately by server tests.

## Reproduction and retained evidence

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/audit_selection_admission_source.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_dunk_shared_target.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_high_pass_height.py
BASKETBALL_SELECTION_ADMISSION=1 go test ./games/basketball/engine -run '^TestRuntime(OriginalSelectionAdmissionMatrix|AllOriginalCharactersAdmission)$' -count=1 -parallel=4 -v
go test -race ./games/basketball/engine -run '^(TestDunkSharedTarget|TestDunkVariants|TestHighPassHeight|TestNegativeReceiverHeight|TestPassLaunchAgainstOriginal|TestDiagonalLaunchAgainstOriginalFunction|TestPassFloorAgainstOriginalFunction)' -count=1 -v
GOOS=js GOARCH=wasm go test ./games/basketball/engine -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^(TestDunkSharedTarget|TestDunkVariants|TestHighPassHeight|TestNegativeReceiverHeight|TestPassLaunchAgainstOriginal|TestDiagonalLaunchAgainstOriginalFunction|TestPassFloorAgainstOriginalFunction)' -count=1 -v
```

Evidence directory: `.local/selection-admission/`. Original source hashes and
scope are in `source-audit.json` and committed source fixture metadata. Initial
bounded/failure runs, exact pre-failure RuntimeStates, targeted follow-ups, final
matrix, source Native/race/Wasm logs are separately named. Server/UI admission,
actual player input, browser layout and multiplayer networking remain separate
platform acceptance responsibilities; this audit changes no server or UI files.

Final result: all96 configurations /192 complete-match repetitions passed in
198.639 seconds. New source-domain Native/race/Wasm checks passed; race6.002s
and Wasm1.687s. The machine-readable index, per-case names and artifact SHA-256
bindings are in `.local/selection-admission/result.json`.
