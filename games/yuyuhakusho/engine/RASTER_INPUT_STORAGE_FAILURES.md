# Raster/input storage ownership: test-first scope

The original `6000..63ff` byte stream and the two raster row views
`6000..61bf` / `6200..63bf` share physical storage. The existing world stores
both independently. Original source7 special world frame 4054 first exposes
this: the arena updates PanA/PanB after the preceding callback sets feedback,
but the world's Stream retains its previous bytes.

Before changing production code, cover these failure cases:

1. The world retains mutable PanA/PanB arrays or another independent owner.
2. Recording four bytes changes Stream but does not immediately change either
   raster view; views use the wrong endian order, plane offset or final row.
3. Arena feedback changes rows but playback reads old stream bytes.
4. Arena writes the two 64-byte gaps, ignores signed row values, or changes
   storage when feedback is absent, paused or slowed.
5. A later callback failure leaks the arena's writes to the caller's Stream.
6. Invalid/truncated storage panics or partially commits an active feedback
   update; zero/nil worlds cannot still be serialized.
7. JSON changes the old key order, omitted optional fields or ordinary world
   bytes, disrupting historical trace digests and schema1 saved sessions.
8. Restoring an old conflicting Pan projection recreates a second state owner,
   or a copied returned view can mutate the world.
9. Strict original CPU Stream/Pan checks are weakened or original failing
   evidence is overwritten to obtain a green result.

The focused tests exercise ownership and cross-reading in isolation; the
existing source7 special full-world tests remain the E2E authority. Preserve
the original failing log and fixture identities, then run those tests, the
ordinary world, native race/Wasm checks and an exact historical session report.
