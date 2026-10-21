# Committed post-result surface and actor OAM

## Failure inventory before implementation

- Reinitialize a full background when a committed write changes only part of a nametable; apply mirroring after rather than before the write; ignore palette mirroring or original increment32 mode.
- Consume pending queues in the renderer, commit an upload twice, or expose submitted actors before the original DMA boundary.
- Rebuild the reward field from a screenshot or serialized original RAM; discard retained upper nametable rows/attributes when installing result presets99/101..108 or reward26.
- Guess score digit tiles, current court palette variants, initial fade level or stale inactive data unavailable in the legacy display state.
- Treat kind17/18/19 like gameplay sprites: Scene7 bypasses character head substitution and has pose-specific priority flags; reward has the original character>=5 head adjustment.
- Clear all four bytes of unused OAM slots instead of source Y-only248 clearing; ignore partial writes by culled fragments; reverse only used fragments when sourceF002 reverses all64 slots.
- Apply the hardware eight-sprite-per-line rule inside CE09, which emits up to64 total pieces; change original order, clipping, mirroring, palette offsets or signed byte arithmetic.
- Treat reference capture state as production state or claim inaccessible historical inactive bytes are recoverable from current logical snapshots.

Root approved new pure display-state/emission/effect/seed helpers. Runtime/server/renderer and the existing video controller remain owned elsewhere. Tests and full-function source probes precede implementation. Production static profiles/tables must be generated from documented ROM sources, never copied from fixture snapshots. Exact-pixel/source-observed timing and canonical live visual sampling remain distinct.
