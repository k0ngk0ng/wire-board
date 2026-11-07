# Live queue merge scope

Freeze tests before modifying production. This is an ownership correction using
the already retained original eleven-character world/actor evidence. No new
native capture, timing claim, CPU state, second persistent queue or history.

Failures to exercise before implementation:

- A component input queue is stale and the component produces no enqueue.
- A real canonical enqueue occurs, its consumer runs, and a nested component
  return would restore the old queue.
- A consumer runs after the last enqueue/leaf return but before an outer
  action-to-actor merge; the outer merge must read the canonical owner afresh.
- Nested submission/tail/action/actor returns retain actual producer changes,
  preserving one canonical queue identity and producer operation ordering.
- Paused/no-enqueue source4 head/boost, source5 child and source7 children must
  merge the canonical queue, not a copied visual/submission view.
- A nil port retains the historical pure-value APIs, including real enqueue
  behavior. No entry/exit comparison may decide whether a queue was written.
- A sound-hook failure, late object failure or late palette failure rolls back
  the complete world transaction. Later external hooks are suppressed and
  expired apply callbacks remain invalid.

The reusable helper is `worldSoundQueueView(value [4]uint8, port
*worldSoundPort) [4]uint8`: nil port returns the value; a live port returns its
canonical queue. Root owns the corresponding merges in chu_world.go and the
new actual object-work integration. Only the helper, actor/action/tail, and
source4/5/7 world wrappers are edited here.
