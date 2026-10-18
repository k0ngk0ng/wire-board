# Command-selected attack modes 36–38 and opponent selector 41

## Failure scenarios specified before implementation

- Re-selecting the opponent or consuming RNG on an already-entered mode; failing to re-select on initial entry.
- Picking the nearest Euclidean opponent rather than the original low-byte-of-half-absolute-X comparison; losing the second-seat tie break, 16-bit wrap, or exclusion of actions 49/4A.
- Mode 41 incorrectly sets its entered flag, changes modes/actions, or consumes RNG: the original direct entry only changes TrackedObject.
- Modes 36/37/38 use masks 1/3/7 and offsets 0/2/0 with the shared eight-choice table; alias mode 15 uses an anomalous source index and must remain outside this recovered domain.
- B05C heading uses signed-byte depth and signed-16-bit X. Its Destination-based carry result differs from A63D's own strict X<32/no-depth-separation decision; do not replace the latter with the helper's reached result.
- First entry writes Destination=16 but preserves prior Plan/Counter unless it changes to mode7/Plan22. Continuation must preserve the previously chosen Parameter, target and RNG.
- Run action requests preserve the entered bit if already running; mode7 handoff does not immediately execute an attack. Subsequent calls must consume the emitted mode against the same live state.
- Invalid actor, wrong component mode, or unsupported tracked object fails atomically; do not fabricate alias coordinates.
- Complete world-input state comparisons must preserve ownership, positions, command state, other players and RNG. Isolated original calls and short AI chains are not natural full-match coverage.

## Source and implementation

The unchanged original bank5 dispatch uses `A09C[36..38] -> A63D` and `A09C[41] -> AD17`. Existing REA evidence `ev_eef21fb30302a3ec59b8b3dc097be9a7fa2a559339d598963a8489a9d646759d` (`.local/basketball/rea/ai-batch/batch.json`) contains both complete entries. This recovery reused that analysis and checked the original instructions and selector bytes at `A691..A69E`; it did not re-import identical source into a new REA session.

`ai_command_attack.go` adds two independent typed helpers, with no ROM interpretation at runtime:

- `selectOriginalCommandOpponent`: source `AD17`, used by direct mode 41 and initial command-attack entry. Both excluded opponents score 255; equal quantized distances select the second opponent. The stored field is the canonical `Control.TrackedObject`.
- `advanceOriginalCommandAttack`: source `A63D`, only modes 36–38. Entry selects the opponent, sets destination 16, consumes one shared RNG draw and selects from `{0,4,1,5,3,7,2,6}` using the original masks/offsets. Continuation reuses target/parameter. It preserves `B05C` heading via the recovered original helper, then independently checks X distance <32 and depth distance <8 before scheduling mode 7 with plan 22. It otherwise requests Run only when the current action differs.

All data comes from existing `AIControlView` fields. No shared context/type additions are needed. Shared dispatcher integration belongs to the parent task. Mode 15 remains rejected: its same-address alias derives selector index 11, outside the ordinary three-row mode36–38 domain, and has no established ordinary caller.

## Oracle and acceptance scope

`verify_ai_command_attack.py` executes the original `A05D` dispatch without editing the original instructions. Its 6,552 calls comprise 504 direct-selector cases and 6,048 command-attack cases. Cases cover all four actors, entered/unentered state, excluded opponent actions, second-seat ties, signed 16-bit X wrapping, 31/32/33-pixel attack boundaries, narrow-X heading and signed-byte depth boundaries. Three fixed RNG seeds are used; observed entry parameters are mode36 `{0,4}`, mode37 `{1,5,7}`, and mode38 `{0,4,6}`. Other selector values are established by the exact source table, not falsely claimed as dynamically visited.

Another 36 two-call chains retain the original state between command attack and actual mode7 consumption, with JSON restore checks on the Go path. These are AI-only handoff chains: actor action/animation/physics and complete A/B command-consumer gameplay are outside this component's acceptance scope.

The test compares the full `WorldInputState`, including action entered bits, heading, control fields, shared random state and all unchanged actors/objects/command fields. Tests and failure scenarios preceded implementation. The initial undefined-helper failure and the next unsupported shared-dispatch failure remain in `.local/basketball/rea/ai-command-attack/`.

Reproduction:

```sh
source scripts/env.sh
.local/monopoly/venv/bin/python scripts/basketball/verify_ai_command_attack.py
go test ./games/basketball/engine -run '^TestOriginalAICommandAttack' -count=1
go test -race ./games/basketball/engine -run '^TestOriginalAICommandAttack' -count=1
GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run '^TestOriginalAICommandAttack' -count=1
```

Native、race and Wasm targeted acceptance all passed after shared dispatcher integration. The fixture regenerated identically. Exact source/table/log checksums and result index: `.local/basketball/rea/ai-command-attack/result.json` and `table-equality.json`.
