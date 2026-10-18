# Result actor extension verification

Failure inventory recorded before fixture/tests/implementation:

- Losing the existing 145 natural result calls or changing BA40's final actor tail.
- Treating C033 as drawing, omitting 9598 entry initialization, 95B4 commands, 9769 motion or C00C projection.
- Clearing repeat flags on entry; off-by-one countdowns/cursor slots; losing mirror XOR or high-byte height retention.
- Losing original fractional position, signed velocity, X24 wrapping, depth16 wrapping, gravity subtraction or height16 integration.
- F0–FD contextual writes omitted: character, text request, palette IDs, display mask, phase/latch, camera and other actors.
- Cross-actor changes applied after all actors, or restarting the current script when only another actor changed.
- F8 incorrectly setting the entry latch immediately; F9 incorrectly preserving target cursor/countdown; FA loop wrap incorrect.
- FB vertical F0-page wrap flattened to ordinary vector arithmetic or high camera byte modified incorrectly.
- Misreading command operand bytes as opcodes, or source script fallthrough as a new action.
- Invalid actor/cursor/action/projection mutating input before returning an error.
- Fixture expectations copied from Go implementation, fabricated natural histories or skipped projection hidden in full-call claims.
- Snapshot restoration losing contextual writes or producing different ordered sounds/state.

Scope: source-specific actors for reward/ending and representative original commands. No runtime/server lifecycle or complete-ending acceptance is implied.

## Implementation and API

`StepOriginalResultActor(OpeningState, ResultActorContext, actor)` returns `ResultActorStep{State, Context, SoundIDs}`. The actor range is 0–3. Failed calls return both original inputs and no emitted sounds. The caller retains draw-list reset, ascending actor order and outer video/RNG scheduling. Existing BA40 uses its compatible private wrapper; no field was added to `OpeningState`, and immutable outcome/lifecycle handling was not changed.

`ResultActorContext` contains the authoritative script-written fields absent from `OpeningState`: `TextRequestFlags` (06AB), `TextID` (06AD), `PaletteIDs` (0305/0306), and `DisplayMask` (0304). Persist this context beside the active presentation state. Commands write characters, action/latch/cursor/countdown, repeat flags, camera and phase/latch directly into the existing `OpeningState` fields; these must not be duplicated elsewhere.

`result_actor_commands.go` implements typed semantic commands F0–FD. FE/FF source handler aliases decode to `resultOtherAction`; no runtime ROM or CPU emulator is used. `result_actor_tables.go` is reproducibly generated semantic data for 31 source programs, with original address and two-byte slot indexes retained. Tables include reward62/63/64, ending actors and their action closure, existing result scripts, and selected original scripts exercising additional command handlers. This is not a claim that every original menu has been integrated.

Source bytecode operands are parsed at their actual command widths. F8 immediately reloads the selected program without prematurely marking its action entered; F9 applies the other actor's action/cursor/countdown before a later actor runs; FA retains its separate wraparound repeat byte. FB preserves its unusual vertical retry: add delta, and if the result has high nibble F, add16 then reload and add the same delta again. Height motion retains the original high byte. Existing original projection is called after motion with the updated camera and phase.

The supported programs retain their original actor roles. An early probe assigned action57 to actor3, which makes its F9 command target itself and resume into the old program's operand slot. That artificial choreography is outside the supported phase domain; the fixture generator now declares the source actor assignments. Invalid/non-command cursors remain explicit errors. The original isolated command handler remains covered; no earned history or random seed was changed to obtain acceptance.

## Source fixtures and validation

Original source: bank3 9598/95B4/9769/C00C, REA `ev_f0ce6b1dbcabba1133dc39a0adf0773ec895a3f8489b27b31893fe9bf045a4e8` and retained `ev_93cedbb127d764b97aff2143de421dba36068fe4f8fb1f7be5cfbe0251cbfe78`. ROM SHA-256 remains `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`.

`testdata/result-actor-extension.json.gz` contains 2,168 complete unchanged original actor calls, including actual C00C/CD3C/D0D9 projection, and 64 explicitly isolated original command-handler cases. Only C000 audio-driver execution is replaced with ordered sound-ID recording. Handler cases use declared RAM0180 operand bytes without changing original instructions, and stop before the next command fetch. F7 palette and FE/FF alias cases prove those handlers under controlled operands, not a naturally executed source script. SHA-256: `8851dfb356fb67336c54e68ee9c3e906092cbb82f208b4942a07d19bffc59582`.

RED was captured before implementation: `action-62-slot-3-mirror-0: unrestored result animation action 98 cursor 3` in `.local/result-actor-extension/before.log`. The contextual/handler/invalid-input tests were also written before their implementation; `context-before.log` retains their failures. The passing source tests compare complete `OpeningState`, contextual fields, sounds and JSON restoration. The existing 145 natural BA40 calls / 580 actor updates still match.

Reproduction (from the worktree):

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_result_actor_extension.py --check
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_result_actor_extension.py --check --emit-tables
. scripts/env.sh
go test -race ./games/basketball/engine -run '^(TestOriginalResultActor.*|TestOriginalFullResultPresentation|TestResultActorScriptsAndConfirmation|TestRuntimeResultOutcomeRestoreAndBoundary|TestCampaignConfirmationOriginalOperations|TestOriginalCampaignRosterCommitAndTail|TestCampaignIntermissionStartCommitsOnce)$' -count=1 -v
GOOS=js GOARCH=wasm go test ./games/basketball/engine -exec="$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^(TestOriginalResultActor.*|TestOriginalFullResultPresentation|TestResultActorScriptsAndConfirmation|TestRuntimeResultOutcomeRestoreAndBoundary|TestCampaignConfirmationOriginalOperations|TestOriginalCampaignRosterCommitAndTail|TestCampaignIntermissionStartCommitsOnce)$' -count=1 -v
```

Logs are under `.local/result-actor-extension/`: `after.log`, `race.log`, `wasm.log`, `oracle-repeat.log`, `tables-repeat.log`. Native source/regression checks passed; final race/Wasm status is recorded below. Parent reward-driver integration separately passed 1,146 full calls across 15 chains with continuous caller RNG and JSON replay; those fixtures and lifecycle work belong to the parent.

Remaining boundaries: source text processing, uploads/fades, ending phase integration, scene resources/audio76, original scheduler and whole-platform E2E. This actor extension does not claim a completed ending movie or fourteen-win campaign.

Final targeted native/race/Wasm checks all passed, including the existing campaign confirmation, roster-commit actor tail and intermission commit tests. Both fixture and generated semantic table reproduction checks passed without changes.
