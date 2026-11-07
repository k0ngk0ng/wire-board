# Object scanner work scope

Freeze the native fixture and missing-API test before production implementation.
Reuse pinned Japan ROM SHA-256
`c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`
and existing Ghidra `input-detail.txt` 3592 evidence. REA raw M68k is already
unsupported; do not install tools, modify core/ROM instructions or open debuggers.

The production API dynamically walks typed headers from8000 toA000. Behavior
and active stride are separate actual reads. The callback behavior is latched;
the stride read then saves the latest size before the synchronous callback.
The callback's real work belongs to its caller; scanner work never prices it
as zero and keeps no callback-duration table or operation history.

Failures to exercise before implementation:

- Entire empty table, first/last active slots, mixed empty and active slots,
  every supported stride64/128/192/256, and exact pool-end completion.
- Active callbacks retain the stride read before their call, even if the real
  original clear callback deletes its own header/size.
- A callback creates a future slot, clears another future slot, changes its
  own stride and creates a slot behind the cursor. Future slots are read fresh;
  earlier slots are not revisited and skipped interior slots stay skipped.
- A completed external update between behavior and stride reads cannot replace
  the old behavior latch, and must affect the later stride read. An empty
  behavior read remains empty even if a slot is filled before its branch.
- The original scanner continues through the pool after a manager clears the
  remaining world. Platform MatchComplete early termination is a separate API
  boundary, not an alternative scanner timing rule.
- Native cycle/refresh boundaries at entry, exact refresh and near refresh;
  compare every scanner operation, not only total time.
- Failed work/read/callback stops all following work and external calls. The
  scanner itself owns no world transaction; Root stages callback side effects.
- Nil required callbacks and invalid active stride0, unaligned or larger than
  256, or one that crossesA000, are rejected. Empty headers do not require a
  valid size. These are typed object-domain guards, not original ROM branches.
- Nil work hook still performs the complete dynamic walk and callbacks.

Evidence routes: passively record real complete3592 scanner PCs and callback
entry/return under unchanged natural video/RAM controls. Independently execute
complete original3592 with actual existing ROM RTS or clear callbacks in the
declared private table domain on both pinned cores. Native callback and IRQ
intervals are explicit external work, not scanner CPU cost or production data.
