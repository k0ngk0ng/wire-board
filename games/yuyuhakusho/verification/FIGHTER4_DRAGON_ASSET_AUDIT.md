# Fighter4 dragon asset audit

This audit checks saved asset coverage only. It does not implement game behavior,
export new tiles, update shared catalogs, or validate a complete rendered frame.
The ROM is a read-only development source and is never copied to runtime assets.

Before implementation, the audit's failure cases are:

1. The selected ROM, saved catalog, or oracle no longer matches its declared
   SHA-256 identity; compressed input is corrupt or its schema is incompatible.
2. A required animation key is absent, duplicated, or has a different graphic
   parameter; the eight-entry body-angle table does not select direct values
   86, 88, and 90.
3. Any required exact GraphicsSet/TileBank/Parameter has no catalog frame, uses
   `PreloadedOnly`, or has no transfer or sprite piece.
4. The source graphics-directory offset, bounded descriptor/piece payload, or
   saved oracle payload differs from the selected ROM or decoded catalog.
5. A transfer has an invalid tile count or tile ID, any saved tile differs from
   its exact source ROM bytes, or a sprite piece uses a tile slot not assigned
   by that frame. Opaque source pixels cannot be replaced by transparent data.
6. A captured `draw=true` has no exact draw request, a silent callback has one,
   or draw-time actor fields differ from the fixture snapshot.
7. A newly generated draw uses an unexpected graphics key or tile bank. Dirty
   preexisting parameters in explicitly paused stage-two boundary inputs must
   be retained exactly and reported separately, never claimed as asset coverage.
8. A required head, body, boost, or initialization key is never exercised by the
   saved original CPU callback requests.

Run from the worktree root:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/audit_fighter4_dragon_assets.py \
  --out games/yuyuhakusho/verification/fighter4-dragon-assets.json
```

The deterministic JSON report binds the script and all inputs by SHA-256, lists
required graphic keys and exact tile-source checks, and distinguishes callback
requests from the later presentation consumer. Rerunning the command must
produce the same report bytes for unchanged inputs.

Observed result: direct reuse passes for 13 exact keys (five head, three body,
four boost, and initialization parameter 0), backed by 292 deduplicated indexed
tiles. All source descriptors, sprite pieces, assigned slots, and tile bytes
match the selected ROM. The existing catalog remains byte-identical at SHA-256
`200e28e775a809de89c19df0dfef314d5697224fcb6a145853beb7344b5833ef`.

The three original fixture sets contain 12,795 callbacks and 12,145 draw
requests. Of these requests, 12,091 use covered keys. The remaining 54 are
deliberately dirty stage-two paused boundary inputs, whose entire draw request
is preserved from the pre-call state; they are enumerated with
`asset_coverage: false`. Initialization parameter 0 matches its actual
transparent source tile. None of these conclusions uses renderer fallback.

The report was generated twice consecutively and compared byte-for-byte.
Final report SHA-256:
`16cb8694208768e314fd06beaee7a1c82a15409ab94adb2d79bfba011545006d`.

This establishes saved asset coverage. It does not establish presentation cache
persistence, palette timing, or full-frame natural rendering. No production,
shared data, tests, or UI files were modified, no commit was made, and this
audit created no temporary cache or build files.

The final natural oracle is bound at 489573 compressed bytes and SHA-256
`b8a48d382472e39a158916821c6a9eef7f58e274d84eb2002e14572979e87612`. It contains 1,592 callbacks
(head 113, trail 777, boost 702) and 1,582 draw requests (head 111, trail 770,
boost 701). All twelve non-initialization graphic parameters occur in these
natural requests, including every body-angle frame and every boost frame.
The final oracle, boundaries, and sequences were included in both identical
report executions; the audit script and saved catalog were unchanged.
