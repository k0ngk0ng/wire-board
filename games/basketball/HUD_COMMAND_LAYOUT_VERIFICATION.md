# Original team-command HUD layout

## Failure scenarios recorded before the fix

- Treating each team's main message and reply as two player labels.
- Copying at offsets 0 and 3, overwriting the last half of the main six-glyph message.
- Omitting the original blank separator at index 6 or placing the two-glyph reply in the wrong cells.
- Updating the wrong team's panel or overwriting player-name rows.
- Assuming `TacticCache` must change without inspecting original writes, thereby changing original scheduler behavior.

These scenarios and the source test draft were written under `.local/basketball/rea/hud-command-layout/` while runtime verification had a source freeze. After unfreeze, `display_tactic_layout_test.go` and the original fixture were installed and a failing test was retained before editing the implementation.

## Original layout evidence

The ROM SHA-256 is `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`.

Existing REA/Ghidra evidence was reused:

- Display glyph builder `B31E`: `ev_e2cdc63188d762884208bfee004817ad3ec5e9ca3fdf0a43e558c2d6047d3ca2`, `.local/basketball/rea/display/batch.json`.
- HUD scheduler and upload builder `AFF3/B074`: `ev_e512234c54456fd3a167b904732643cc324552d487272a08277c6aa6c25f14b0`, `.local/basketball/rea/match-clock/batch.json`. The historical query for `B19D` failed; this is not represented as a successfully decompiled function. The unchanged ROM instructions provide the executed oracle.

Bank 4 `B17A` contains offsets **0, 7, 0, 7**. `B074` copies each decoded six-glyph label into its team's shared upper and lower upload buffers at those offsets. `B0A7..B0AC` clear index 6; `B0AF..B0B4` request only nine uploaded glyphs per row. The visible layout is therefore **six message glyphs + one blank + two reply glyphs**. The source writes all six response glyphs into scratch memory, but only its first two lie in the nine-glyph upload. Original response IDs 1 and 2 contain two characters; ID 0 is blank.

These are team messages, not character labels. The runtime already takes `Tactics[team*2]` and `[team*2+1]` from `Human.Messages[0]` and `[1]` before the current actor-input phase. The corrected display projection now uses offsets 0 and 7 and explicitly clears cell 6. Player names, scores, scheduler and simulation state remain outside the change.

`verify_display_tactic_layout.py` executes 78 unmodified original `B074` calls: both teams, all 13 main-message IDs, and replies 0/1/2. The fixture records exact upload addresses, lengths, glyphs, and memory writes. It is isolated original function execution, not a new original PPU or natural full-match run.

The independent read-only comparison found **72 mismatches out of 78 calls before the fix and zero afterward**. For main message 4 and reply 1, the lower row changed from the incorrect `[54,56,93,54,131,0,0,0,0]` to source `[54,56,93,121,56,0,0,54,131]`.

## Tactic cache: retain original behavior

No cache fix was made. All 78 panel calls leave `03C7..03CA` unchanged with zero writes. A separate uninterrupted sequence of 12 original `AFF3` scheduler calls also leaves `[255,0,255,0]` unchanged and selects alternating team slots 1/3 when the clock is not dirty.

Existing original video snapshots additionally retain the same cache for all 24,000 `match-clock-complete`, 300 `match-clock-pause`, and 6,000 `world-actions-cpu-long` frames. Those snapshots establish observed stability; they are not a complete indirect-memory-write audit. A bounded raw PRG scan found only opening initialization direct writes to `03C7/03C9`, at fixed-bank CPU `D216/D219`. Do not infer all-program absence of writes from that scan alone.

The caller's unchanged `TacticCache` therefore agrees with the inspected original HUD path. Updating it as an unsolicited cleanup could change scheduler timing and match-clock progression.

## Reproduction and scope

```sh
source scripts/env.sh
.local/monopoly/venv/bin/python scripts/basketball/verify_display_tactic_layout.py --write-fixture
go test ./games/basketball/engine -run 'TestOriginalDisplayTacticMessageLayout|TestOriginalDisplaySemanticStages|TestDisplay|TestMatch' -count=1
go test -race ./games/basketball/engine -run 'TestOriginalDisplayTacticMessageLayout|TestOriginalDisplaySemanticStages|TestDisplay|TestMatch' -count=1
GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run 'TestOriginalDisplayTacticMessageLayout|TestOriginalDisplaySemanticStages|TestDisplay|TestMatch' -count=1
```

The new source test checks both glyph rows against original upload bytes, blank cell 6, and preservation of names and the other team panel. Existing original display-stage and match-clock tests are included as targeted regression evidence. Only `ObserveDisplayHUD` presentation projection changed; no world inputs, tactics, RNG, match scheduler, timer, score calculation or cache mutation code was edited. The full 12-configuration gameplay matrix is therefore not repeated for this display-only correction. Actual visible browser command/reply checks belong to the parent task's updated PC/mobile E2E.

Logs, original per-address writes, retained-cache snapshot hashes, and comparison output are under `.local/basketball/rea/hud-command-layout/`; the source-bound fixture is `engine/testdata/display-tactic-layout.json.gz` with adjacent metadata.

Targeted Native, race and Wasm checks passed. Exact current-source and log hashes are recorded in `.local/basketball/rea/hud-command-layout/verification.json`.
