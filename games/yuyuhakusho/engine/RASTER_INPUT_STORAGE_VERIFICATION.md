# Raster/input storage: one owner

`ChuWorldState.Stream` now owns the 1024 recording/playback bytes and both
raster row views. The world has no persistent PanA/PanB arrays. `PanRows`
decodes a copied 224-word big-endian view at offset 0 or 512. Default-arena
feedback updates those same bytes and preserves each following 64-byte gap.
Recording changes the next raster read; raster feedback changes the next
playback read. The renderer reads the same copied views.

Historical JSON retains every previous field, field order and omission tag.
PanA/PanB exist only in a temporary serialization projection after Vertical
and before ScrollSources. JSON restoration ignores these derived fields and
uses Stream, including when a legacy projection conflicts with it. No second
mutable raster state, RAM VM or CPU result injection was introduced.

The [failure list](RASTER_INPUT_STORAGE_FAILURES.md), new tests and six complete
legacy JSON fixtures preceded implementation. The old code produced real
runtime failures for independent owners, recording-to-raster reads,
arena-to-playback reads, unavailable active feedback storage and conflicting
legacy projections. The six legacy JSON byte assertions passed before the
change and remain exact afterward. The fixture includes zero state, a default
original world, three source7 worlds including active pan, and a case with
explicit optional Dragon fields for serialization coverage.

The previous original-world red log is preserved independently. Both special
world modes previously failed at frame 4054, `field=Stream`. The original
special test source, CPU fixture, manifest and original red log retain their
pre-change SHA-256 identities. The test decoder now explicitly verifies each
original PanA/PanB capture against its Stream view; the existing world
comparator still compares every Stream byte.

| Final check | Result |
| --- | --- |
| Native race, 12 top-level focused/world tests | Pass, 187.611 s |
| Wasm, same 12 tests with minimal environment | Pass, 49.982 s |
| Source7 special, per-pass and single initial state | Each 3,700 passes, 50,006 callbacks, 24,968 ordered draws, 209 head callbacks, 4 restores |
| Source7 ordinary, per-pass and single initial state | Each 3,144 passes, 41,797 callbacks, complete cleanup |
| Source7 child callbacks | 4,321 first + 505 pan + 6,497 second callbacks, 24 restores |
| Historical two-AI session | Exact complete report after excluding only the prior rules-data digest |
| Engine/runtimebridge vet; runtimebridge Wasm compile | Pass |

The historical two-AI session still completes in 3,676 ticks and 42,815
callbacks. Its final world digest remains
`4d9ff38eaa68f696b66729a56874b3a6ffcdb2ed98d6445b3adca3e4b951921d`;
the complete world/event trace remains
`a9ac2016d8e1ce6759a292c5c741cc2757f5e5ea6bb6a4bc7eb4607afa443c44`.
No world, event or lifecycle report field was masked.

All reproduction commands, source/fixture/log SHA-256 identities, captured
coverage and precise comparison results are in
[raster-input-validation.json](testdata/raster-input-validation.json).
The parent task owns the final 280-report legacy matrix and broader browser
acceptance; these results do not claim that additional scope.

Cleanup removed only the one-time fixture-generation Go test after retaining
its complete source in testdata. The frozen original failure, test-first
evidence, final logs and shared active Go caches remain in this worktree.
