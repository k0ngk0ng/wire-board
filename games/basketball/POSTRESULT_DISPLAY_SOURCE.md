# Logical post-result display program

## Proposed boundary (before implementation)

The renderer already accepts pure source tile-row fragments. This work derives the original display program from semantic state, rather than replaying the captured register timeline. Own new source-program and row-projection helpers only; runtime/server, renderer, resumable waits and upload queue ownership remain with their assigned owners. Root approved this API/ownership boundary before implementation.

Inputs: committed physical nametables, palette/CHR pages and display controls; scene/phase; displayed camera; ending stage offset06D9 and credits offsets06DA..06DD. Resources are committed by the scheduler at their proven source boundaries. No helper may consume queues, shorten waits or overwrite gameplay.

Outputs: source NMI setup plus the named IRQ stages, each stage's ordered semantic changes and cycle offsets relative to its handler entry; separately a pure tile-row projection from an already-timed logical raster plan. IRQ arrival time is an explicit caller dependency. Logical scores/actors/phase alone do not identify the currently interrupted original CPU instruction or its completion latency. Captured cycle timestamps must not be hardcoded into production.

## Failure inventory before source probes / Go tests

- Decode entered phase high bit as another phase; choose wrong initial IRQ selector/latch or wrong chain order.
- Apply mirroring for an adjacent phase, particularly phase5 and defeat phase11.
- Confuse displayed camera with pending next-camera state.
- Drop the NMI initial control/CHR/scroll writes or restore sprites unnecessarily in Scene7 mode80.
- Use06D9 as a full scrolling offset in the stage-bank toggle, where only bit0 is consumed; lose its full byte in airplane lower scrolling.
- Use the credits fractional accumulator06DE as a displayed offset instead of high byte06DC.
- Collapse address and fine-X effects, change source operation order or omit the palette-independent font CHR switch.
- Omit source branches/delay loops while claiming exact handler-relative timing.
- Pretend that the source kernel determines an absolute IRQ entry clock without scheduler evidence.
- Let a pure display helper alter source actors, RNG, audio, upload queues or waits.
- Feed recorded timeline/OAM/test fixture data to the production helper as its program.

## Initial source map

Scene7 NMI uses fixed F0F2→F18F; fixed F1DA has twelve(selector,latch) pairs. In phase order0..11 the pairs are `(14,8),(14,8),(14,8),(2,190),(2,190),(3,10),(3,10),(3,10),(15,8),(1,192),(2,190),(2,190)`.

Named chains are phases0–2 `14→2→1`;3–4 `2→1`;5–7 `3→4→2→1`;8 `15→16→17→18→2→1`;9 `1`;10–11 `2→1`. Selector1 disables IRQ then runs the original sound driver. The render helper must leave sound-driver execution to the appropriate audio/scheduler owner.

Bounded source disassembly is retained in `.local/postresult-display-source/fixed-raster.asm`. Data tables were printed for inspection but must not be treated as executable instructions. The completed controlled probes execute the original NMI and IRQ handlers with declared inactive queues/silent audio state, retain complete writes and relative cycles, and compare helpers only against their owned rendering effects.

## Implemented API and ownership

`OriginalPostResultRasterSource` contains scene/phase (including its original entry bit), machine-control4D, displayed camera0397–039A, stage offset06D9, four visible credits offsets06DA–06DD, background CHR4F/50, sprite CHR51–54, control56 and mask57. The credits fractional accumulator06DE is intentionally absent; the existing logical ending state retains it and advances the high visible byte06DC.

- `BuildOriginalPostResultRasterNMI(source)` returns the original initial selector/latch, control and ordered display effects. Ending NMI relative cycles begin atF18F; reward cycles begin atF206. Both origins follow DMA/upload work, which belongs to the scheduler.
- `BuildOriginalPostResultRasterIRQ(stage, control, liveSource)` returns the selected original handler's changes and next selector/latch. Its cycle origin is instruction-start atF2D4. It samples **live logical source values**, not a frozen NMI snapshot. The caller commits persistent control56 changes.
- `BuildOriginalPostResultRasterProgram(source)` is the convenience composition when source values remain unchanged throughout the chain. It is not a license to freeze animation offsets during live playback.
- `ProjectOriginalPostResultRasterRows(surface, plan)` converts an already-timed semantic raster plan into the browser's source-tile row-fragment contract. It reads no ROM, source fixture, screenshot, CPU program, global clock or randomness. It does not determine IRQ arrival or advance game state.

`OriginalRasterSurface` holds two physical1024-byte nametables,32 palette indexes and mirroring. `OriginalRasterFramePlan` holds the initial logical source, odd-frame flag and ordered semantic effects positioned relative to the frame's first visible tile-fetch origin. It requires `TimingMode` (`source-observed` or `canonical`) and a nonempty `TimingSource`. Unknown effects, invalid resources and ambiguous/unordered timing fail explicitly. The helper preserves its input resources; pending queues, background transfer waits, fades, actor submission/OAM construction, audio and runtime/server lifecycle remain outside its ownership.

The browser receives tile-row references and palette. Recorded OAM remains an independent test observation; production actor display comes from the recovered submission/projection path. No PPU/CPU execution core was added to the game.

## Complete original-function comparison

`scripts/basketball/verify_postresult_display_program.py` executes **78 complete NMI calls and228 complete IRQ calls**, covering reward phase7 with/without its entry bit, ending phases0–11 with/without entry bits, three camera/control/offset patterns, and all selected IRQ chains. It executes every original instruction, including terminal selector1's silent audio-driver call. No JSR is omitted. All original device writes and visited-instruction digests remain in `postresult-display-program.json.gz`.

The declared I/O adapter records PPU/MMC3/APU writes and supplies neutral status reads. OAM DMA hardware stalls are not emulated; NMI raster cycle origins are explicitly **after** DMA/queue work. The fixture's full instruction count is not a claim about a full live audio driver's cost or physical video duration. The Go helper returns only owned rendering effects and their source-relative cycles.

The first Go test failed on missing helper symbols before implementation; see `.local/postresult-display-source/expected-red-go.log`. Source comparisons, rejection tests and JSON restoration now pass. Original phase6's special font page14 and phase7's page98 are separately covered for live selector0; unsupported table indexes are rejected.

The source-controlled cases keep source fields constant between handler calls. The live-source test additionally reproduces the observed credits ordering: at local10500, early IRQs read184/92; mainline then changes the credits; later IRQs read45/91. At airplane local9000, the lower strip reads updated06D9=199 while the NMI snapshot had204. Source observations prove why a whole-frame frozen NMI snapshot is incorrect. This does not establish every possible IRQ/mainline interleaving.

## Timing provenance and the three-cycle callback boundary

The helper uses original **instruction-start** cycles. FCEUX's execution callback has a different timestamp convention: `src/x6502.cpp` calls `ADDCYC(CycTable[b1])` before `CallRegisteredLuaMemHook(...EXEC)`. Consequently the recorded `irqBefore` callback atF2D4 already includes its first `PHA` instruction's three cycles. A source-observed IRQ anchor is therefore `irqBefore.cycles - 3` when positioning these handler effects. This is a source-proven instrumentation boundary, not a screenshot-fitted offset.

The upstream `v2.6.6/src/x6502.cpp` evidence and digest are retained in `.local/postresult-display-source/`. NMI anchors must likewise use their declared instruction-start boundary rather than blindly using an execution callback's timestamp.

Logical phase/actors alone do not identify which original CPU instruction was interrupted, its remaining cycles, or all mid-frame mainline changes. Production may use an explicitly documented deterministic **canonical visual sampling** policy while keeping original gameplay/phase/animation semantics. Such sampling must remain distinct from exact source-observed pixel timing. No captured IRQ times or captured interleavings are embedded into production code, and no CPU interpreter is introduced to conceal this remaining timing boundary.

## Go row projection and source pixels

The row fixture is independently derived from the original controlled frame observations, with timing explicitly labeled `source-observed`. It is **test data only**. `prepare_postresult_rows_fixture.py` converts the recorded device writes to typed address, horizontal/vertical scroll, CHR, control, mask and mirror changes. Its expected rows were already verified against983040 original pixels across16 frames, including8 holdouts.

`TestOriginalPostResultRasterRowsSourcePixels` compares the Go helper's complete rows/palette to those references, checks committed-surface purity and repeats from JSON-restored surface/plan state. All16 frames match, without exempting text, sprites' background-priority inputs or boundary fragments. This proves the Go row projection for the declared timed inputs; it does not claim that a canonical live scheduler generates the captured clocks.

The projection retains two fetched tile rows, separately applies address and fine-X changes, preserves horizontal restore and row29/31 vertical wrapping, and maps physical nametables according to current mirroring. Each output row covers exactly256 pixels with static source tile references. It emits no RGB screenshot patches. The renderer applies source OAM ordering, priority and palette under its own separately verified contract.

## Reproduction and limits

From the worktree:

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_postresult_display_program.py --check
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/prepare_postresult_rows_fixture.py --check
source scripts/env.sh
go test ./games/basketball/engine -run '^TestOriginalPostResultRaster' -count=1
```

Both generated fixtures reproduce byte-for-byte. Their tracked `*-source.json` files contain hashes, counts and scope. Test logs, initial failures, source disassembly, REA output, source-file hashes and repeated-generation results are in `.local/postresult-display-source/`.

Remaining owner integration: source-timed resource commits; actor submission rather than post-tail instant display; explicit live/canonical IRQ sampling; runtime/server/Wasm/browser wiring; whole playback/reconnect/restoration E2E; and the independently outstanding earned campaign acceptance. These helpers do not complete or release the game.

New REA evidence: `ev_b1ac981cf9f7603a7fe81c3a78d8cceaa67e066ea9db3354588d71129f8ad775`, retained at `.local/postresult-display-source/analysis/evidence.json`. All15 requested fixed-bank functions were analyzed and the session was closed. Instruction-executed fixtures constrain decompiler limitations. `.local/postresult-display-source/manifest.json` records source/evidence/log hashes.
