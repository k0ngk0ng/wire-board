# Live queue merge correction

Priced work can let a sound consumer run without another enqueue, or after the
last enqueue but before an outer component returns. The previous enqueue-time
copy alone could then restore a consumed request from a stale component view.

`worldSoundQueueView(value, port)` now refreshes component queue merges from the
live canonical owner. A nil port retains the historical standalone value. The
actor/action/tail return chain and source4 head/boost, source5 child, and source7
children use the helper. Actual producers still enqueue directly into the sole
canonical queue; no entry/exit comparison decides whether to keep an enqueue.
The actor's internal value adapters may continue calling `acceptActorAction(a)`;
the live path supplies its temporary port. No persistent state fields changed.

Root owns the corresponding chu_world.go merges and actual object-clock wiring.
This correction neither prices character work nor claims native IRQ timing.

`test-first.json` binds the failure plan, new tests, initial missing-helper red
result and seven production sources before implementation. The unchanged tests
then reproduced stale tail/action/actor returns, consumed-request resurrection
at the action-to-actor seam, and all five paused child-wrapper merges. Their
behavior failures remain in `stale-component-red.jsonl`. The interim private
signature compile failure is preserved separately; existing single-argument
adapters remain supported, and old tests were not changed.

Tests cover no enqueue, a real canonical producer followed by consumption,
consumption after the final leaf/action return, nil-port value behavior, shared
owner identity, and whole-frame rollback after producer-hook, late-object and
late-palette errors. Synthetic completed-consumer seams exercise ownership;
they are not newly captured native interrupt observations.

The bounded regression command reuses the retained whole-world captures for
characters0..10, shared actor/action/tail original calls, and original source4,
source5 and source7 child callbacks. Their fixture adapters retain their existing
digest checks and expectations. No new native evidence was acquired.

Reproduce from the worktree root, using fresh output directories:

```sh
python3 -B games/yuyuhakusho/verification/world-queue-merge/verify.py --mode native --out games/yuyuhakusho/verification/world-queue-merge/recheck-native
python3 -B games/yuyuhakusho/verification/world-queue-merge/verify.py --mode native-race --out games/yuyuhakusho/verification/world-queue-merge/recheck-race
python3 -B games/yuyuhakusho/verification/world-queue-merge/verify.py --mode wasm --out games/yuyuhakusho/verification/world-queue-merge/recheck-wasm
python3 -B games/yuyuhakusho/verification/world-queue-merge/verify.py --mode vet --out games/yuyuhakusho/verification/world-queue-merge/recheck-vet
```

Each directory retains the exact command, worktree-local Go environment,
source hashes and result log. Shared Root-owned files are recorded before and
after execution so later integration changes are visible. The complete
shared-clock integration remains subject to Root's final acceptance.
