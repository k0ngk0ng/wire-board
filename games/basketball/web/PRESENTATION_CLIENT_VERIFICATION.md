# Post-result browser lifecycle

The bridge and browser prediction now stop at `RuntimeFinished`, rather than
`Outcome.Confirmed`. Confirmation fixes the match result; an owned campaign
reward/ending continues until the original handoff/reset. Legacy/versus sessions
without an owned presentation retain their parked result behavior. Presentation
state and compact render snapshots are persisted by the Go runtime. The browser
uses the local Go/Wasm projection and exported indexed artwork; it does not run
ROM instructions or receive per-pixel raster rows from the server.

## Test-first failure and verification

`TestBridgeEarnedPresentationLifecycle` was added before the bridge changes. It
failed because bridge advancement remained at video 22385 while 60 native ticks
reached 22445. With the lifecycle fix, the earned match advances through its reward
to video 22752. Replay, JSON restore, immutable terminal state and no replay sound
emission are checked. This fixture uses the existing original menu configuration
and actually plays both periods; its result is not injected or shortened.

Repeat from the worktree root:

```sh
source scripts/env.sh
go test ./games/basketball/runtimebridge -count=1
python3 scripts/basketball/accept_presentation_client.py
```

The browser acceptance uses actual Go/Wasm, production artwork loader, renderer,
and network scene. It covers 367 earned-reward video frames and 15 checks: preload, live reward
rendering, native/Wasm state equality, confirmed-presentation prediction,
spectator replay and reconnect without replaying old sound/pictures. Inspectable
artifacts are `.local/presentation-client/result.json`, `desktop.png`, and
`mobile-320.png`. The browser profile is task-owned and closed after execution.
The mobile screenshot shows the reward building and original actors inside a
320px viewport.

The existing production-renderer source-frame acceptance also passed 16 source
frames and 983040 pixels with zero differences, including hidden resource-page
transitions and rejection of visible unknown artwork. Run
`accept_postresult_raster.py --renderer` with the existing workspace Python that
provides Pillow. Its result and screenshots remain in `.local/postresult-raster/`.
The controlled ending also advances 12,884 actual Go/Wasm ticks and paints 441
sampled frames, covering phases 0,1,2,3,4,5,6,8,9,10,7. Its terminal state matches
native Go exactly; Start is delivered only at the recorded source confirmation
boundary. `ending-desktop.png` and `ending-mobile-320.png` show the sunset team
picture before confirmation. This begins at a controlled original result
boundary, not fourteen earned wins. The same browser script regenerates its
native fixture using `TestRuntimePresentationDisplayLifecycle`.

The existing `accept_network_scene.py` regression also passed all 52 checks for
input-history replay, correction, handoff, legacy result parking and preparation.

TypeScript checks pass with the current main compiler; raster return arrays now
explicitly use owned `ArrayBuffer` storage accepted by `ImageData`.

## Scope

These checks prove the earned reward client path, controlled ending playback,
and the 16 retained source-frame projections. They do not claim a 14-win earned campaign or original-console timing
identity for every live ending frame. Runtime raster timing uses the declared
canonical interrupt boundary; measured source-frame projection and continuous
canonical playback are separate evidence. Full platform multiplayer, deployment,
and original source lifecycle validation remain separate acceptance scopes.
