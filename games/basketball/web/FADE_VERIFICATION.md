# Original palette fade verification plan

The implementation will use `snapshot.paletteLevel?: number`, default 4, with
explicit 0 preserved as black. Its current scope is original court fades with
flags `$0301=00/80`, which transform all background and sprite palette groups.
The selective-group flag `$40` and background-only transfer flag `$01` require
separate scope and must not silently use the global level.

Failure cases declared before implementation:

- Treating level 0 as an absent value and displaying level 4.
- Accepting fractional, negative or out-of-range levels.
- Replacing the original NES index clamp with opacity, alpha blending or an overlay.
- Darkening already-low color indices again, changing hues, or failing universal
  color/transparent sprite behavior.
- Transforming only the court while leaving HUD, actors, shadows or result digits
  at full brightness.
- Applying the transform twice across repeated snapshots, restore or palette-ID
  changes; retaining a previous fade after a full-color snapshot.
- Losing color-index information when duplicate full-bright RGB values have
  different lower-level results; the exporter must prove all equivalence classes.
- Showing a queued level before its original PPU commit (Go timing is owned by root).
- Substituting synthetic image filters for original-frame comparisons, or calling
  static palette10 checks natural frame coverage.

Required validation includes original palette-buffer output and complete original
images for fade-in levels 1/2/3/4 and fade-out 3/2/1/0, multiple courts, Court4
base/9/10, both court and result scenes, and all existing stable-frame regressions.
Reference screenshots remain immutable. Any isolated original-function probe must
be identified separately from ordinary-input match captures.
