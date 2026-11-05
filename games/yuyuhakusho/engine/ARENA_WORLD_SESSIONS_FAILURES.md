# Arena world/session integration: pre-implementation failures

This checklist precedes the new integration fixtures, tests, and production edits.
The existing independently recovered callbacks remain owned by their workers.

- Reject odd/out-of-range arena IDs and any missing scene/resource registration before constructing a session. A selected arena must enter the normal match manager, with no imported original state.
- Preserve the zero-option default session, default resource value/JSON, old rule schemas 1/2/3, canonical object ownership, Stream-only pan storage, and existing state hashes.
- Accept the optional arena-scenes/resource extension only as a complete pair. Reject incomplete linear rows, arena-4 rows/colors, wave data, missing/duplicate arena registrations, invalid palette words, or a resource/behavior inconsistent with MatchStart. Failed decoding returns no partial library.
- Require the selected resource source and all 16 palette words at readiness. Reject duplicate, wrong-arena, wrong-stage, missing-library, and tampered deliveries atomically.
- Dispatch all seven recovered arena callbacks from the current object/global views. Merge immediately without duplicate scratch, row, palette, or pan owners. Pause/slow gates must retain exact state; feedback writes only the existing Stream spans and preserves its gaps.
- Supply the real LogicFrame low byte to scene C, and retain the VDP register-7 BackdropColor byte without changing default serialized bytes.
- Arena 4 merges only its three primary colors; palette status and other colors retain their canonical owners.
- Arena A creates the canonical 9e00 child, runs it later in the same object pass, reads its actual parent, and emits six immediate raw sprite requests in exact order even while gated. No SAT or hardware sprite counters enter world state.
- A later callback error rolls back every earlier mutation and all events, including child creation, row/pan changes, palette changes, and sprite emissions.
- Whole-pass capture must use the existing unique observer-free 3602-frame controls and legal arena inputs. The hooked cold replay must match every original RAM/video hash and start at the actual practice-ready boundary (2700), never menu frame 1800. Captures retain producer/input/control/fixture SHA-256 bindings.
- Per-pass and single-initial continuous E2E comparisons cover all actors, all owned bytes, callback order/stride, palette, camera, rows, Stream, pause/feedback, save/restore, and ordered raw sprites. No expected intermediate state may be injected into production APIs.
- Freeze tests and a genuine unsupported-callback or missing-symbol red result before editing production code. Retain native and Node/Wasm verification artifacts using worktree-local caches only.
