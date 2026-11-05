# Pixel-free shared render state: failures before implementation

- Server builds must embed only computational rules/metadata. No sprite tile
  bytes, HUD tile pixels, arena textures, audio, client Wasm, ROM, or framebuffer.
- Metadata must bind all 1,500 actual sprite frames, 34,520 tile identities,
  320 preload bindings, transfer stamps/IDs, piece counts, HUD key domains, eight
  resource lists, and exact rule/client-catalog SHA-256 values. Reject malformed,
  missing, conflicting, out-of-range, and duplicate metadata atomically.
- Preserve original culling, unsigned coordinate wrap, cache hits/misses,
  SkipTiles, preloaded-only inheritance, transfer order, slot wrap, clearing of
  unused cache words, 80-piece Flags termination, and raw arena-A signed budget.
- GraphicCache remains only in WorldObject. Sprite slots and HUD refs have one
  persistent owner in renderstate.State; no second pixel Slots/HUD owner.
- Apply every callback-time draw even if its slot was later cleared/reused, but
  update WorldObject.GraphicCache only when its current DrawRequest still equals
  that exact callback-time request. Never overwrite later rule-owned fields.
- HUD refs track ordered base/bar/glyph/portrait assignments directly. Do not
  invert pixels to infer refs: the source catalog has four duplicate-pixel
  aliases. Preserve cell ownership, name cursor, reset/resource semantics, and
  all overlapping name/portrait/bar writes.
- Failures after earlier draw/HUD work must return both original inputs and no
  partial cache, slot, cell/ref-map, or allocation-cursor mutation.
- RulesDigest excludes only GraphicCache. Controller history, Stream, feedback,
  RNG, effects, automation and all canonical rule fields remain covered.
  Presentation Digest separately covers world caches, slots and HUD refs.
- Restore validates exact asset/metadata identities and bounded slot/ref state.
  An arbitrary current rules world cannot reconstruct inherited slots/names.
  Schema-1 research pixel saves must not be migrated by guessing semantic refs.
- Freeze current unmodified runtimebridge reference trajectories before adding
  the shared implementation. Reuse the fourteen existing legacy render configs,
  twenty-eight nondefault-arena render configs and seven fixed legal-pan inputs;
  record sources, deduplicate configurations, and never execute an original CPU.
- Tests must compare all 1,500 frames plus culling/budget/skip/alias/atomic cases,
  and complete fresh world/cache/slot/HUD/pixel trajectories with midstream
  JSON restores against that frozen bridge. Save a genuine missing-API red.
- Native/race and Node/Wasm use the same typed data and fixtures. New metadata
  arithmetic must not depend on client pixels, host floats, network timing or IO.
