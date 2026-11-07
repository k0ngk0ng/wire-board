# Object scanner work checkpoint

The user redirected effort toward stable playable audio before scanner production
implementation. This work is parked. No scanner implementation or differential
Go/race/Wasm/vet acceptance is claimed.

The first Go tests were written before implementation and failed with the real
missing `WalkWorldObjectsWork` API. They are retained byte-for-byte as
`world_object_scan_work_test.go.txt`, outside the active Go package so this WIP
cannot block the playable build. The API does not exist in production yet.

Primary native evidence is `native-aux/`: 64 complete private original calls on
two pinned cores, plus 15 passive complete natural passes and 204 actual callback
intervals. There are 51,051 scanner operations. Private uninterrupted native calls
match stepping on both cores. The passive observer preserves2,401 original RAM
and video frame controls.

All9,447 natural scanner operations have native completion boundaries:9,408 at
the next CPU hook and39 at the first delegated auxiliary read's actualMainBefore.
Each of those39 host-clock displacements is explained by the delegated reads'
actualMainBeforeAfter values. The original next-hook endpoints remain intact.
Auxiliary feedback and callback intervals remain explicit external work. This
establishes neither full World timing nor production IRQ integration.

The original353A clear-rest callback proves the scanner continues through the
remaining empty table. Platform MatchComplete termination is a separate upper
boundary. Private3556 callbacks clear their own size, while the scanner advances
using the stride saved before the callback.

`native-first.log` and the archived first producer/observer preserve the initial
failed assumption that a non-scanner clock gap must also change PC. The later
`native-frozen/` capture records those gaps without attribution. `native-aux/`
resolves them using Root's existing delegated Z80-read observation method.
None of the earlier raw inputs, gold or failure results were overwritten.

The retained capture recipe is
`python3 -B scripts/yuyuhakusho/capture_world_object_scan_work.py --out games/yuyuhakusho/verification/world-object-scan-work/NEW_DIRECTORY`.
It was not executed after the user's stop instruction. Resume only if the user
reopens this precision work. Sources, fixture SHA values and stopped status are
bound in `checkpoint.json`.

Open work: implement the independent typed walker, activate its archived tests,
run differential/race/Wasm/vet checks, and let Root integrate it with actual
callback costs. No production callback-duration table, original RAM owner or
arbitrary-PC execution has been introduced.
