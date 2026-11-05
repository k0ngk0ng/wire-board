# Platform integration contract and recovery plan

This investigation started from worktree `54a5b9ae`. The root task owns platform
registration, server/module and client changes. This worker initially inspected
the existing contracts, then received the independent `renderstate` scope. Audio
DSP/score/PCM work remains separate; this document does not establish release
readiness.

## Existing host contracts to reuse

| Concern | Actual source | Consequence |
| --- | --- | --- |
| Authenticated seat and automation | `internal/gameapi/module.go`, `internal/server/modules.go` | `Context.Actor` is the platform seat. `Automated` comes from bot/explicit autoplay metadata. |
| Waiting-room preparation | `internal/server/modules.go:prepareModule`, `games/basketball/server/preparation.go` | The host calls game Prepare, normalizes Configure, persists options and clears human ready flags. `PreparationContext.Bots` contains occupied seats only. |
| Ordered realtime actions | `internal/server/realtime_stream.go`, `realtime_batch.go` | Reuse `/api/ws?stream=1` and `BoardProps.act`; authenticate every action. No game-owned socket is needed. |
| Rare full recovery | `internal/server/module_sync.go`, `games/basketball/server/wire_view.go` | `Module.SyncSnapshot` is explicitly requested on initial/gap/mismatch recovery. Copy under the lock, compute/encode outside it. |
| Authoritative timer | `internal/server/module_realtime.go`, `games/basketball/server/module.go:onTimer` | The host wakes every 50 ms; basketball batches six 60 Hz ticks every 100 ms, caps catch-up to 250 ms and never waits for a peer ACK. |
| Slow receiver isolation | `internal/server/realtime_stream.go` | A coalesced single pending frame, 32 bounded ACK replies and per-connection 5-second writes isolate the receiver. A frame means current state, not queued old snapshots. |
| Persistence | `internal/server/realtime_checkpoint.go` | Ordinary state stays in memory with periodic save; checkpoint-key/lifecycle transitions persist before publication. Do not change the key every tick. |
| Prediction/replay | `games/basketball/web/network-scene.ts`, `runtimebridge/bridge.go`, `syncwire/inputs.go` | Confirmed and predicted forks remain in Wasm. Initial/gap checkpoints cross JS only on recovery. Normal traffic carries inputs/ACK/frame/digests. |
| Input cancellation | `games/basketball/web/network-input.ts` | Eight ordered ordinary flights, one cancellation lane, 64 queued events, 500 ms playable input age and 750 ms loss expiry are useful existing limits. |
| Shared load UI | `web/src/game-api/loading.tsx`, `load-assets.ts`, `internal/server/client_loading.go` | Declare asyncBoard/resources. Report actual asset decode, Wasm and Canvas initialization before `onReady`; preserve token/build/client fencing. |

The Yuyu MD controller is **12 bits**, unlike basketball's eight-bit transport.
Do not copy its `buttons<=255` validation or four-byte packing. Start with four
explicit uint16 Held masks per RLE run plus the four automation bits. A 48-bit
numeric encoding is possible but JavaScript bitwise operators truncate to 32
bits; it provides no necessary first-version benefit.

## Waiting options and identity

The root task has independently implemented:

```go
type Options struct {
    Arena uint16 `json:"arena"`
    TeamBattle bool `json:"teamBattle"`
    Characters []uint8 `json:"characters"`
}
```

Characters are in platform seat order, exact capacity 2..4, IDs 0..10. The
original logical slot mapping is:

| Platform capacity | Original slots | Human channels | Colors |
| --- | --- | --- | --- |
| 2 | 0, 3 | 0, 1 | 0, 1 |
| 3 | 0, 1, 3 | 0, 1, 2 | 0, 1, 2 |
| 4 | 0, 1, 2, 3 | 0, 1, 2, 3 | 0, 1, 2, 3 |

Original parent IDs are `8400 + slot*400` in hexadecimal. Platform seats, not
those addresses/slots, own authentication, input sequences, ready state and
results. HumanChannel equals the platform seat. Explicit bot/autoplay seats use
the original AI through InputChannel 255 and Match.Controllers 0.

Unused original slots are empty, not fixed AI players. Do not copy basketball's
`len(seatActors)<4` rating flag. `Match.HasComputerPlayers` refers to game-owned
AI outside platform seats; Yuyu currently has none. Platform bot/autoplay rating
policy remains with the shared host.

Prepare uses `character` with an optional seat (default actor), `arena` for the
host and `mode` with TeamBattle for the host. A player can choose their own
character; the host can choose an occupied bot seat's character. An unoccupied
capacity index must not be guessed to be a bot. TeamBattle=true requires four
players. Reject unknown/mixed fields atomically.

WinnerCode is an original base-team/slot code, not a platform seat. In free-for-
all, invert the slot map (two players may return 1 or 4; three may return 1, 2
or 4). Four-player teams use BaseTeams 1/1/4/4. `MatchDrawCode` is -32768 and must
be represented as the platform draw result. Never infer a winner from a client.

## Pixel-free presentation state

Current `runtimebridge.Session.Step` calls the independent engine, applies HUD
events, composes sprites and writes updated GraphicCache into WorldObject only
when its current DrawRequest still equals the callback-time request. It also
persists TileSlots and `presentation.HUDState`, whose Slots contain actual
32-byte indexed tiles. `Save()` therefore cannot be the server checkpoint.

At the inspected nonzero-pan two-player boundary:

| Payload | JSON bytes | gzip bytes |
| --- | ---: | ---: |
| Current full research save | 96,679 | 7,740 |
| World portion | 79,823 | 4,279 |
| Sprite slot portion | 6,360 | 1,158 |
| Pixel HUD portion | 9,929 | 1,782 |
| Full world-sprites catalog | 3,660,897 | 639,790 |
| Computational rules | 2,103,508 | 103,902 |
| New complete render metadata | 486,408 | 111,445 |

The sprite catalog contains 34,520 tiles, or 1,104,640 actual pixel bytes.
None enter render metadata. The new bundle keeps 1,500 keys, tile-bank IDs,
transfer stamps and stable tile identities, 320 default bindings, preload flags,
piece counts, HUD key domains, eight resource identities and exact source SHA
bindings. Deduplicating transfer definitions saves less than 1 KB compressed
and was not needed.

Piece counts are computationally necessary: `presentation/compose.go` changes
Flags after the 80-piece boundary and thereby suppresses later actors' cache and
slot updates. Culling, the final-piece branch, raw A's signed byte budget and
callback ordering must all match. There are 146 preloaded-only frames. A current
World cannot reconstruct inherited slot contents by selecting a current frame.

HUD refs retain `{kind,key,index}` for base, bar, glyph and portrait assignments.
They are recorded when ordered events execute, not inferred from pixels. Four
source tile values have ambiguous pixel aliases. A 76-slot observed HUD maps to
roughly 3.5 KB JSON/0.6 KB gzip using tuple refs; exact exported State uses typed
object refs. State has at most 2,048 sprite/HUD logical slots. SpriteSlots has a
fixed 8,448-byte binary upper bound before serialization/compression.

The shared API is now implemented in `games/yuyuhakusho/renderstate`:

```go
DecodeLibrary(raw []byte) (Library, error)
(Library).Identity() Identity
New(library Library) (State, error)
Advance(world engine.ChuWorldState, state State,
        events []engine.ChuWorldEvent, library Library) (engine.ChuWorldState, State, error)
Validate(world engine.ChuWorldState, state State, library Library) error
RulesDigest(world engine.ChuWorldState) (string, error)
Digest(world engine.ChuWorldState, state State) (string, error)
```

`State.SpriteSlots` and `State.HUD` are their sole persistent owners.
WorldObject.GraphicCache stays the sole cache owner; Advance updates it on a
detached object map with the existing reuse guard. Count/Flags are temporary.
RulesDigest excludes only GraphicCache; the complementary Digest covers caches,
slots and HUD refs. No controller history, Stream, RNG or gameplay field is
silenced to conceal divergence.

Server initialization explicitly embeds only `chu-world-rules.json.gz` and
`renderstate-metadata.json.gz` in the root-owned `data/runtime_rules.go`.
Decode once before module callbacks. No callback filesystem IO; no wildcard
embedding of the data directory. Client pixels/audio/Wasm remain client assets.

These four Identity hashes cover common rule/sprite/HUD/render metadata, not
the selected arena or A-aux payload. The root task requested a separate small
`runtime-resource-descriptor.json`: thirteen logical resource entries containing
only raw JSON SHA/size and gzip SHA/size/path, including all eight arenas and
aux. It may be explicitly embedded as identity metadata, with its own hash in
the network checkpoint. This descriptor work follows the frozen renderstate
scope; it must not change its state algorithm or Identity.

Identity hashes refer to exact decompressed JSON bytes, including trailing
newlines. The new Wasm loader must preserve verified Uint8Array bytes or full
text strings into Go. Parsing into a JavaScript object and re-stringifying its
JSON changes that identity; trimming on one side is not a valid fix. The root
task confirmed exact-byte loading for the new pipeline.

## Recovery and replay

Use a versioned checkpoint containing options/identity, external uint64 frame,
world, renderstate.State and transport input state. Bind rule, sprite, HUD,
selected arena/aux and metadata digests. Validate every ID/ref and the identities
before accepting it. A client materializes HUD pixels and sprite frames as
temporary views from its separately verified catalog, then continues with the
shared Advance. Do not persist old pixel Slots/HUD alongside the new State.

Schema-1 research saves contain pixel HUD and inherited slots. Keep a separate
legacy reader if needed, or explicitly reject that research schema for the new
network session. Do not silently migrate by reversing pixel equality. The root
task owns this compatibility decision and its historical pixel regression.

Normal views need only epoch/control revision, frame/serverAt, per-seat ACK and
automation/input metadata, recent bounded accepted input runs and the rule plus
presentation digests. Start from 120 retained frames and a 24-frame ordinary
window as in basketball. Initial attach, gap, rule mismatch or presentation
mismatch requests `/api/rooms/{id}/sync`; never add the full checkpoint to every
view. A raw response is tens of KB; gzip is a few KB at the measured boundary.
Do not promise the estimate as a maximum until 2/3/4-player recovery is measured.

Confirmed replay must use exactly the accepted per-tick Held/automation samples.
Pending short taps must survive press+release within one host batch; retaining
only the final zero Held mask loses the action. Game controller history remains
canonical. Ordinary release clears host transport latches/queued changes; the
next sampled tick generates Released. If automation switches immediately
between ticks, supply an exact control revision/effective-frame operation to
confirmed replay. Prefer applying the world-side switch at the next recorded
tick, while changing authorization and clearing physical input immediately.

Use a new monotonic uint64 network frame; the original Frame/LogicFrame words
wrap. Seed and world RNG stay authoritative. Local prediction forks the latest
confirmed native state, samples immediately and advances up to 12 frames/250 ms;
it never waits for ACK to display movement. Reconcile by replaying unconfirmed
input on a fresh fork. Hits, warp and scene/control changes snap; optional
ordinary movement smoothing must not edit rule state.

The initial load is the only all-human gate. Current prepared world frame 0 has
ResourceBusy=3 and no valid active scene rows. A client can verify actual Canvas
initialization on a disposable preview fork (CompleteResource, first step and
render), then discard it while confirmed authority remains frame 0. This avoids
waiting for the match to start before drawing while the server waits for a drawn
scene before starting. Preview emits no public result or sound. Reconnect and
background recovery initialize only that client and never re-gate the match.

## Required closure and remaining unknowns

The shared metadata package has a pre-implementation failure list, missing-API
red, all-1,500-frame boundaries and 49 frozen old-bridge complete trajectories
(14 legacy render, 28 selected arena render, seven fixed legal-pan). The root
task still needs actual module/client E2E before registry inclusion:

- Authenticated 2/3/4-player room mapping, own/other/bot preparation, readiness,
  full regular and team results, duplicate/stale/forged input, handoff/reclaim.
- Real browser key/touch-to-visible latency with RTT, ACK, authoritative arrival,
  display queue and local frame time measured separately.
- Same peer normal/slow coexistence, bounded queues, loss/gap/digest mismatch,
  serverInstance change, reconnect at inherited SkipTiles/HUD and active pan.
- Full raw/gzip normal/dense message sizes and snapshot-only-on-recovery proof;
  exact binary embed inventory proving no client payload.

The current research Wasm entry is single-session and has no fork/replay/network
restore API. The root client pipeline must provide these; a source-level API
proposal is not browser/network acceptance. The client sprite file is currently
monolithic across eleven characters. Future role-specific assets must retain
global tile identities, not renumber inherited slots.

Audio queue consumption/event identity remains a separate unresolved interface.
Confirmed replay/reconnect must not replay old sound; prediction/rollback must
not double-emit it. If audio consumption changes canonical queue fields, server
and client need one shared post-frame rule contract before including those
fields in the final digest. Missing audio acceptance cannot be called release
completion.
