# Legal human-input dynamic pan verification

All seven recovered arenas pass this separate source-7 pan scenario in native
Go and Node/Wasm. The ordinary complete-match matrix's genuine zero-pan
failure remains retained. Its AI cases do not substitute for these inputs.

The verifier creates a new regular match with source 7 in slot 0/channel 0 and
source 9 in slot 3/channel 1, both explicitly human. The second controller moves
and punches normally until source 7 reaches 97 health. After recovery, source 7
uses the original Down+B+A command at 223 energy. No reference state is loaded;
no health, energy, request, action, child, or pan field is assigned by the
controller. It observes the public player view and submits only legal Held bits.

The command and thresholds are established by the retained original source-7
discovery: at original frame 4038, PC `19798` executed with health 96 and energy
201; callback `15094` first executed at 4053. Those original files are evidence
for the command, not input state for the independent sessions.

Each independent case has the same observed milestones:

| Result | Value |
| --- | --- |
| Total ticks | 1,416 |
| Health gate | tick 943, health 97 |
| Down+B+A | tick 1,039, health 97, energy 223 |
| First `15094` callback | tick 1,055 |
| First nonzero PanOffset / save | tick 1,056 |
| Nonzero pan frames | 232 |
| Active arena-feedback frames | 224 |
| Paused nonzero-pan frames | 8 |
| Restored nonzero-pan frames | 231 |
| Full restored rule/save/RGBA frames | 360 |

The first saved PanOffset is 7, with both scroll sources selecting the actual
Stream pan rows. Feedback leaves the Stream gap bytes intact. Physical Start
edges pause and resume the match. The pan lifecycle finishes with zero offset,
zero amplitude, and the `15094` child cleared. Both human mappings remain
unchanged. Arena A emits 8,496 immediate raw scene sprites across the case.

An independent newly constructed rules session runs beside the public
runtimebridge session with the same Held samples. Every rule-owned field and
byte must agree each tick. Only the established renderer-owned GraphicCache
has a comparison-only projection; neither live world is changed. Saving during
nonzero pan and restoring both sessions must preserve subsequent complete rule
state/events, bridge saves/views, and every 320x224 opaque RGBA byte. The HUD
window and normal sprite output remain complete throughout.

Native discovery records a Held-only fixture. A second fresh native process
replays that fixed fixture and must produce the identical full report and all
artifact digests. Wasm uses the same native fixture. Each native/Wasm report is
identical after removing only `environment`, including the complete rules,
save-state, and pixel trajectory hashes. The saved nonzero-pan PNGs for arenas
2 and A were also inspected visually.

Results are in `additional-arena-pan/{native,wasm}/arena-<hex>.json`, with each
environment's seven-case `validation.json`. Each case directory retains the
Held fixture, per-frame digest trace, nonzero-pan save, and complete PNG. Artifact
map keys are basenames, allowing strict cross-environment comparison without
discarding any path or digest. `additional-arena-pan/verification.json` binds
the reports, source, assets, original command evidence, and test-first failures.

The first legal damage controller failed at health 145 after a knockback
cross-over: at close distance it supplied no direction, so source 9 kept facing
away from its target. The original 6,000-tick failure, a read-only 1,800-tick
diagnostic trace/save, and the frozen pre-fix verifier are retained. The fix
changes only the test controller's legal directional input. No production
engine, runtimebridge, presentation, existing verifier, or root test changed.

Reproduce from the worktree root using the existing worktree-local Go caches:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -B scripts/yuyuhakusho/verify_arena_pan.py native --workers 2
PYTHONDONTWRITEBYTECODE=1 python3 -B scripts/yuyuhakusho/verify_arena_pan.py wasm --workers 2
```

For one scenario:

```sh
go run ./games/yuyuhakusho/cmd/verify-arena-pan \
  -arena 2 -output .local/md-yuyuhakusho/arena-pan-recheck
go run ./games/yuyuhakusho/cmd/verify-arena-pan -arena 2 \
  -inputs games/yuyuhakusho/verification/additional-arena-pan/native/arena-2/held-inputs.json \
  -output .local/md-yuyuhakusho/arena-pan-fixed-recheck
```

The scenarios finish the pan lifecycle with both players alive. The root
complete-match matrix separately owns defeat/result/clear acceptance.
