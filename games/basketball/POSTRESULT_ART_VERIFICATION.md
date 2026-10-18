# Post-result source artwork verification

## Failure inventory (before implementation)

- Decode a background with the wrong alias descriptor, CHR source window, RLE operand order, PPU destination or output length.
- Mistake background preset bytes for palette IDs: descriptor byte 5 selects background palettes; scene registers 0305/0306 select sprite palettes independently.
- Treat partial writes (including reward preset 26) as a complete screen, or discard retained nametable/attribute bytes.
- Apply the reward's descriptor palette instead of the caller-restored court palette.
- Assume ending kind 19 lives in bank 2; its pose table is fixed-bank C410, unlike kind 17/18.
- Include a non-pose pointer as another pose; omit reversed piece order, signed offsets or original raw attribute bytes.
- Bake one sprite palette/CHR configuration into art that later animation changes.
- Present an offline static preview as a captured console frame or claim IRQ/CHR splits, dynamic text, fading and animation timing are already verified.
- Accept a different ROM/RGB table, stale output, missing files or changed hashes.

## Acceptance contract

`scripts/basketball/verify_postresult_assets.py` is written before the exporter. It runs unchanged original decompression instructions with a bounded PPU read/write adapter, original D079/D0C6 piece extraction, and original 9526 palette construction. It compares exported logical resources and every index-atlas pixel; static previews are derived from those resources but are not independent full-scene pixel comparisons. Export is reproducible and fails closed on unknown bounds or source identity. Controlled instruction calls prove resource decoding, not whole-console video timing or natural campaign completion.

## Source and results

Source ROM SHA-256: `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`. Retained NES RGB lookup SHA-256: `da30843b331e2e4474395866bc9871fcd60a7d716b3c8ee5ce3c89ee46e8c91e`. Both are pinned by the exporter. Palette RGB is the existing capture tool's 64-color lookup, not a claim about a specific physical display.

New REA evidence `ev_b5a7b0814ff69b7d44f8d5248f3503031ab3bbcc1f85c3d1371e645ebb2a4a79` covers descriptor loading, all four decompressor branches and the original sprite piece reader. Retained bundle: `.local/postresult-art-source/analysis/evidence.json`. Its imported `bank3-visible.bin` SHA-256 is `38f5141f8b77e75504ddd5e23586c54ea1d61d4835071937c5a8d809322e9877`; fixed-bank resource procedures are unchanged in that mapped image. The CLI session was closed after export. Pseudocode is supporting evidence; isolated unchanged 6502 instructions provide the comparisons.

| Preset | PPU destination | Bytes | Two 2 KiB CHR starts | Background palette | Meaning |
| --- | --- | --- | --- | --- | --- |
| 26 | 2000 | 192 | 96, 98 | descriptor 1; reward restores previous palette | Reward top six tile rows only |
| 95 | 2000 | 1024 | 60, 62 | 33 | New York stage first nametable |
| 96 | 2400 | 1024 | 60, 62 | 33 | New York stage second nametable |
| 97 | 2000 | 1024 | 84, 86 | 6 | Seaside |
| 33 | 2000 | 1024 | 50, 52 | 21 | Airplane first nametable |
| 34 | 2400 | 1024 | 50, 52 | 21 | Airplane second nametable |
| 25 | 2800 | 1024 | 96, 98 | 1 | Cleared third nametable for credits preparation |
| 98 | 2000 | 1024 | 12, 12 | 34 | Sunset/credits scenery |
| 23 | 2000 | 1024 | 20, 22 | 36 | Final group picture |

All addresses in the table are hexadecimal; lengths, page and palette IDs are decimal. The two CHR starts each map 2 KiB. Repeated page 12 for preset 98 is intentional. Compressed data lives in the descriptor's temporary CHR window (112 or 124); it is not the final rendering window.

Actor profile table bounds are kind 17 `863A..872A` (120 poses), kind 18 `872A..877E` (42), and kind 19 **fixed-bank `C410..C44E`** (31). Profiles retain original OAM ordering, attributes, signed X and signed Y minus 23. A total of 193 poses / 2,198 fragments matches the original `D079`/`D0C6` calls. This exporter deliberately does not modify `static_sprites.SpriteROM.pose_count`, existing result assets, gameplay assets or any renderer.

All nine background writes match original `DC15..DC96` execution, including the real `DD34` buffered PPU read and `DD61` upload instructions. The adapter substitutes only `FA0B`'s CHR commit with a four-page CHR view; it does not simulate an NMI or claim upload duration. Original palette function `9526` was compared for 19 background palettes × 15 sprite palette-pair IDs = **285 calls**, stopping at `9619` before its NMI handshake. The validation includes the universal-background normalization of every fourth palette entry. It preserves two independent eight-color sprite selections.

The local fallback bundle `.local/postresult-assets/` contains nine PNGs and `manifest.json`: eight complete source-nametable previews and one 67-page index atlas. Every index-atlas pixel (274,432 pixels) is checked against independent direct bitplane extraction. PNG file hashes, modes and dimensions are verified. Reward 26 has no fabricated full preview because its attributes and lower screen are retained from prior state. `.local/postresult-art-source/contact-sheet.png` is an inspected contact sheet of the raw nametable previews, **not** a console screenshot.

## Consumption contract

- `decode_background(rom, preset)` returns target, ordered tile/attribute bytes, descriptor and final background CHR/palette selectors. Apply only its `data` range to persisted nametable state. Upload requests can change page mapping; do not concatenate targets into a guessed screen layout.
- `actor_profiles(rom, kind)` returns original source pieces for kinds 17/18/19. Follow the existing renderer's character/head substitution, facing, OAM order, priority and projection rules; these profiles alone are not a rendered frame.
- `palette_groups(rom, id, sprite)` returns four background groups or two sprite groups. Background pixel 0 uses universal color 0; sprite pixel 0 is transparent. The scene retains separate left/right sprite selections. Normalize shared background entries as original `9526` does.
- `chr-indices.png` is an L-mode **index texture**, with pixel values 0–3, 64 tiles per row/page and manifest page order. It is not a gray display image. Tile rectangle is `[localTile * 8, pages.indexOf(page) * 8, 8, 8]`. The RGB palette is present in the manifest. Current mapping selects one of four sprite pages for an 8×8 tile; no runtime ROM or CPU interpreter is required.
- `background-*-preview.png` uses the descriptor's palette and unscrolled source nametable. Use it for resource review; exact animation must use persisted nametables, scene palettes, scrolling, text and scanline mapping.

## Reproduction

From the basketball worktree:

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/export_postresult_assets.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_postresult_assets.py .local/postresult-assets
```

Initial expected-red result is retained in `.local/postresult-art-source/expected-red.log`; the missing `postresult_art` module failed before implementation. Passing source/resource comparisons are in `validation.log`. `reproducibility.json` retains all output hashes from two fresh exports into separate worktree-local directories and checks byte-for-byte equality. The exporter refuses an output path outside this worktree; it performs no upload or writes to the linked shared result-assets directory.

## Remaining integration boundaries

No natural reward or earned ending capture was available in the retained source-map evidence. Existing normal result/Continue captures establish different scenes and are not reused as proof of these resources. There is no claimed end-to-end pixel parity here.

Remaining: text stream/glyph upload processing, actual nametable mirroring and ending IRQ/CHR/scroll splits, fades and blocking video waits, timed actor projection/priority, reward retained background composition, and captured whole-scene verification. Music 75/76 and outer scheduler integration belong to separate work. These exported assets provide faithful source resources; they do not complete ending playback, fourteen-win campaign acceptance, OSS publication or game release. `accepted_for_release` remains false.

## Controlled whole-console capture failure inventory (before capture implementation)

The follow-up capture must distinguish an injected winning result from an earned victory. Failures include an injected state at an arbitrary CPU instruction; skipped BA40 confirmation actors; altered original clocks/ending gates; retained inputs silently held through phase changes; treating an inspection callback as passive without comparison; missing frames during blocking uploads/fades/text; reading only final CHR mapping when IRQ switches it mid-frame; emulator state created from a different input replay; incomplete or truncated Lua outputs; unclosed capture processes; and output/config writes outside the authorized workspace.

The capture starts from the unchanged `match-clock-complete` replay at local frame 22050, before the first BA40 result entry. A dedicated saved state must match that replay RAM. The controlled runs alter only explicitly listed score/period/win bytes at the first BA40 entry; record before/after RAM and the execution boundary. No timers, animation states, ROM instructions or later result states are overwritten. A second run adds read-only observations, and must match per-video RAM and screenshots from its minimally instrumented controlled reference. Final acceptance keeps the original source identity, input list, timing, injection report, hashes and full capture completion marker.

The later controlled whole-console reward capture established sprite page 32 after outer `F049`; that page is now included. The earlier isolated kernel retained page 13 because outer Scene1 work was explicitly omitted. Reward field CHR remains the current court resource (observed Court1 pages68–71), supplied by the existing court asset bundle. See `POSTRESULT_VIDEO_VERIFICATION.md` for the distinct full-console evidence.

The precise reward raster uses indexed Court CHR, rather than a precolored court texture, so `bank1:BEC5[court]` starts `[76,68,72,92,80,84,88,64]` now contribute all eight courts’ pages64–95 to the same L-mode index atlas. `retainedCourtCHR` records this table in the manifest. This is static full-court page coverage, not observed reward playback on every court. The existing bitplane validator covers every added page.
