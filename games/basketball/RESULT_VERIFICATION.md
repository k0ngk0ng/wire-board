# Result recovery verification

Failure cases, recorded before implementation:

- Phase 6 must run four original celebration actors, not stop after score/winner selection.
- Actor entry resets cursor/countdown but retains the separate repeat byte; countdown differs from gameplay animation elapsed time.
- Winner/loser scripts preserve mirror, fractions and gravity; presentation height integration changes only its low 16 bits.
- Sound 8 occurs on script commands; random crowd sound remains in the existing result core.
- Actor projection and draw order update after each actor, including the final confirmation call.
- Initial result entry is not completion. A/Start or timer 144 confirms; equal scores still follow normal overtime upstream.
- Full-result RAM and sounds must match passive original captures; captures must preserve reference RAM and screenshots.
- Snapshot restore retains result timer, actor counters, visible/submitted images and outcome; no replayed completion.
- Original destination scene/phase remains explicit. Unrestored reward/menu handling must fail, not silently restart.
- Result display uses cached decimal score digits, original result background and kind-17 sprites.

Evidence: REA result-actors batch `ev_93cedbb127d764b97aff2143de421dba36068fe4f8fb1f7be5cfbe0251cbfe78`, bank 3 9598/95B4/9769; bank 4 BA40 through BCA0.

## Evidence and results

`scripts/basketball/capture_match_result.py` passively observes BA40 through BCA0 in the existing natural 24,000-video-frame reference. All final RAM bytes and three PNG captures equal the uninstrumented reference. It records 145 complete calls, including 580 actor animation/motion/projection updates. Full `OpeningState` fields and ordered sounds match every record.

Fixture: `engine/testdata/match-result.ram.gz`, SHA-256 `4f61bfa317dbebe742710673017e0693784f2a28ac78baa7fe15e48360963d1a`; metadata and per-call frame/tick/sound index are adjacent. REA analysis is in `.local/basketball/rea/result-actors/`.

Targeted validation covers campaign/versus, both winners and automatic/A/Start confirmation. Every video update is compared with JSON-restored execution. Completed result state preserves the original next scene/phase and rejects unsupported continuation atomically. The original reward-entry actor poses (98/99/100) are restored only for BA40's final call; the phase 7 reward stage remains explicitly unsupported.

Native race passes all three result tests plus the seven existing Runtime tests. Wasm passes the three result tests. Logs: `.local/basketball/engine/result-before.log`, `result-after.log`, `result-runtime-before.log`, `result-runtime-after.log`, `result-race.log`, `result-wasm.log`.

Reproduce targeted tests:

```sh
source scripts/env.sh
go test -race ./games/basketball/engine -run '^(TestOriginalFullResultPresentation|TestResultActorScriptsAndConfirmation|TestRuntimeResultOutcomeRestoreAndBoundary)$' -count=1 -v
GOOS=js GOARCH=wasm go test ./games/basketball/engine -exec="$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^(TestOriginalFullResultPresentation|TestResultActorScriptsAndConfirmation|TestRuntimeResultOutcomeRestoreAndBoundary)$' -count=1 -v
```

This isolated result verification does not claim a cold complete match, full campaign, or browser pixel validation. Those remain separate integration/renderer checks.

## Result sprite CHR initialization

The initial BA40 call contains BCC1 `LDA #$0D` / BCC3 `STA $54`: result initialization installs sprite CHR page 13. F049's raw `0x86` branch then retains that installed page; it does not retain the previous court's ball page. The first natural complete-result record changes `$54` from 32 to 13 while raw phase changes `0x06` to `0x86`.

`initializeRuntimeResultDisplay` applies this write only on the first successful result initialization. It preserves court-ring columns, serial and epoch. `TestRuntimeResultInstallsOriginalCHRBank` was recorded failing first (`result-chr-before.log`), then passing (`result-chr-after.log`); it covers previous pages 32–35, JSON restoration and publishing the installed page to Visible.

The pre-initialization record has all 13 poses hidden and all 12 drawable slots empty. Because the DMA commit applies the current CHR page to an already submitted image, a phase-6 image with no Result and an empty draw order may legitimately expose page 13 before the result layout is submitted. This does not authorize page 13 for ordinary playing sprites.
