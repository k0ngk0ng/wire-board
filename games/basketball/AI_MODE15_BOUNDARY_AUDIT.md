# Original mode 15 boundary audit

Mode 15 shares the original bank5 `A63D` handler with command modes36–38, but it does not use their three intended selector rows. `(mode & 15) - 4` is 11, so the original reads mask7 at `A69C` and offset189 at `A69F`. Its eight choices read adjacent instruction bytes: **17,157,118,6,96,189,106,6**.

The reproducible probe executes the unmodified `A05D` dispatcher for each source-selected index, then executes the next original AI call without changing semantic state. All eight sampled first and next calls return. This is not evidence of a complete game or valid normal menu route. Several parameters are outside the currently restored engagement-plan domain; rejecting them cannot be replaced by silently treating mode15 as mode36.

The bounded caller audit found no mode15 in the six ordinary/command tactic tables, the eight-team/seven-score-band computer role table, or the retained original command-resume matrix. A retained default original trace contains 3,594 AI calls and no mode15 calls. Exact adjacent `LDA #15/#143; STA $0672..$0675` byte-pattern searches across the eight PRG banks found no candidates. This search does **not** cover all indirect writes, computed values, instruction/data distinctions, or all original gameplay. It proves neither a normal caller nor unreachability.

No Go alias handler was added. The remaining source domain stays explicit in the AI coverage ledger. The next useful step is to trace a real producer or establish the relevant complete source dataflow, not manufacture a selectable platform option.

```sh
source scripts/env.sh
.local/board-art-env/bin/python -B scripts/basketball/audit_ai_mode15.py
```

Result: `.local/basketball/rea/ai-mode15-audit/result.json`, including ROM SHA, exact samples, table sets, trace identity and limitations. The probe never stubs an unknown call or skips an original device write; any unsupported execution is recorded as such. This is a retained-evidence/source-instruction audit; it does not replace original-menu/campaign acceptance.
