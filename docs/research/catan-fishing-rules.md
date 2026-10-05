# Fishing on CATAN — implementation record

## Scope and evidence

Rules target the 2025 Traders & Barbarians edition, pp. 9–10, and its
five/six-player extension, p. 5. Component counts were checked against rendered
printed pages, not inferred from extraction order. Official FAQ questions 43–52
were also checked. Pinned references and data are in
`catan-fishing-sources.json`; original PDF hashes are in
`../board-expansion-rule-sources.json`.

The **internal token economy, map, setup/production pipeline, replacement
responses, five paid actions, boot passing, bots and persistence** are
implemented. Fishing is not exposed in room configuration. Original-art UI
and complete scenario/combination acceptance remain; it is not ready for
public play.

## Token economy (completed 2026-10-05)

- Three/four players: eleven 1-fish, ten 2-fish, eight 3-fish tokens and one
  old boot. Five/six players add four 1-fish, five 2-fish and five 3-fish tokens.
- Aggregate lake/coast entitlements once, then issue tokens one at a time,
  starting at the active player and proceeding clockwise. Face-down supply
  recycles the shuffled face-up discard when empty.
- Fish token faces remain private; counts, face-up discard and boot ownership
  are public. No view contains supply order. Own faces and returned public
  arrays do not alias game state.
- Seven fish tokens maximum, excluding the boot. A player at the cap may
  decline or discard one owned token for one blind replacement, then forfeit
  their remaining draws. Drawing the boot uses a draw without occupying a
  fish-token slot. This last point is an interpretation of the printed
  one-token-at-a-time rule, not a dedicated FAQ ruling.
- A payment names distinct owned fish tokens and pays for exactly one action;
  overpayment produces neither change nor credit. Game integration must
  validate the action and its cost before committing the entire action.
- The boot may pass to another player with at least the owner's **public**
  score. It changes the victory requirement, never the recorded score.
  The game layer must enforce whose turn it is and calculate public scores.
- Restore verifies full inventory conservation, uniqueness, hand limits,
  boot location and the pending replacement queue. Invalid restore or action
  leaves the original state unchanged.

Six targeted tests pass, covering official inventories for 3–6 players,
18 draw/payment/recycle/save cycles per player count, ordered production,
replacement and decline, boot replacement, separate payments, privacy,
aliasing and malformed restore rejection. `go vet ./internal/game` passes.
These are unit-level results, not HTTP restart or complete-game acceptance.

## Map and aggregate production (completed 2026-10-05)

The official diagram has six base coastal Vs and eight in the extended frame.
Each V spans two coastal edges and all three of their vertices; choosing an
arbitrary non-port vertex in the existing generic map does not reproduce this
frame. Both port edges and fishing Vs were transcribed using tile/side
coordinates. Topology labels were overlaid on the printed diagrams, then
separate renders of generated boards were checked against them. Exact locations
are recorded in `catan-fishing-sources.json`.

The lake is restricted to the seven inner hexes; both extended lakes are
restricted to the fourteen inner hexes. Rejection sampling conditions the
existing variable map on interior deserts, then replaces them with lake faces.
It retains resource/number inventories, nonadjacent red numbers and the
existing extended number spiral. The two lake faces are randomly assigned
to their locations. The outstanding independent verification of the base
extension's 2025 letter/number correspondence is **not** resolved by this work.

Port types and ground numbers are randomized with the correct inventories.
The robber begins offboard. Lake production uses its own four/two numbers,
independent of the ordinary tile number; a robber blocks that lake's entire
production. A robber on an adjacent resource tile cannot block coastal fishing.
Every producing settlement claims one token, every city two, and eliminated
players receive none. All lake/coast claims combine into one player array
before entering the token economy. Calculating claims does not change ordinary
resources or their production counters.

Four new tests cover 24 random maps for each of 3–6 players (96 total), printed
inventories, interior lakes, topology, coastal Vs, nonadjacent red numbers,
JSON map round trips, all coastal/lake settlement and city positions, all
production totals, robber blocking, combined claims, eliminated seats and
invalid maps. Combined claims are exercised against the token economy to
check the single replacement limit. All ten fishing tests pass (0.644s), as
do related base/fixed/paired/five-six regressions (7.649s including fishing)
and `go vet ./internal/game`.

At the map stage this was a calculation component, without production
continuation, HTTP restart, bot match, original-art UI or browser acceptance. Owned
research crops, overlays, generated map renders/JSON and temporary export
code were cleaned after verification. No assets uploaded, push or deployment.

## Runtime production and response stage (2026-10-05)

An internal constructor now attaches the map and tokens to the persistent Catan
state. The ordinary roll action computes all fish claims, distributes normal
resources once, then draws fish. At the cap, the responding player may exchange
one owned token or keep their hand; further claims for that player are lost,
including when the replacement is the boot. The active turn stays on the
original roller throughout. A saved roll ID prevents running that production
twice, and saved ordinary-resource counts continue the turn without repaying
resources. Five/six-player second actions do not produce again.

Second starting settlements adjoining a lake or ground receive one token and
continue to their starting road. The literal 2025 setup sentence says “a random
fish token”; this is implemented as one award even beside both lakes, rather
than applying city/ordinary production rules to setup. The FAQ explicitly
confirms a token beside a coastal ground; no dedicated two-lake setup FAQ was
found. This interpretation remains recorded for final combination review.

Fish are excluded from resource discard/trade/theft machinery. The robber may
enter a lake. The boot owner needs one additional victory point, with unchanged
score. Public views contain map, supply size, face-up discards, ownership,
counts and the viewer's own faces; the saved draw pile, raw hands and continuation
are always removed. Bot replacement decisions use only their own faces.

Each responder receives 120 seconds; successive responders reset that response
window, and completing the queue restores the original action's remaining
time. Manual, autoplay and timeout paths use the same game actions. Removal is
a platform policy outside printed rules: eliminated players' fish return
faceup to discard and their boot is shuffled into the face-down supply. This
keeps absent players from trapping components.

Five new engine tests cover starting lake/coast awards, boot victory target,
replacement/decline, frozen resources, repeat rejection, paired turns, private
views and hidden-information-independent bots, seven/robber/removal, all 3–6
player bot setup sequences with restoration after every action, and atomic
invalid-state/unsupported-combination rejection. Three HTTP paths (manual,
autoplay, timeout) enter the fixture through the actual roll action and each
restart the real service three times: before responses, after replacing with
the boot, and after completing production. Full Room equality, ownership,
privacy, illegal actions, 120-second response windows, original 45-second
restoration and nonrepeated resources pass. HTTP tests pass in 1.630s; race
checks pass in 11.451s. Related fishing/gold/event/fixed/paired/helper/friendly/harbor/production regressions pass (game 155.890s, server 95.170s), including all 15 fishing engine tests. Game/server static checks also pass.

At the production stage there were no paid fish actions, client UI or complete bot match. Helpers,
Seafarers, Cities & Knights, Friendly Robber, Harbors and event-card combinations
are deliberately refused at the internal boundary until their integrations
are verified; they remain in the overall scope. No public room option or
deployment was added, no assets were downloaded/uploaded, and test temporary
directories are cleaned automatically.

## Paid actions, boot passing and complete bot games (2026-10-05)

All five costs now execute real game effects: 2 removes the robber without
theft, 3 steals a random ordinary resource from a present opponent, 4 takes a
chosen available bank resource, 5 builds a legal connected road using normal
route/award/victory completion, and 7 buys a face-down development card and
marks it newly purchased. Inventory limits and new-card use restrictions
remain. Each action identifies distinct owned fish tokens, validates its
effect, then pays separately; overpayment gives neither change nor credit.
Invalid actions leave the entire game unchanged. Fish are never temporarily
converted into resource cards to reuse a building cost.

The printed p.10 wording is “During your turn ... take special actions”, with
each action completed before another. The implementation therefore permits
the pre-roll and ordinary action phases, but no interruption of setup, seven
discards, robber theft, free-road resolution or other pending responses. This
is the direct printed-text interpretation, not a claimed dedicated timing FAQ.
The 5–6 rule on p.5 explicitly permits the second player to spend fish and pass
the boot in their action phase. All successful ordinary fish actions retain
the current phase and action deadline.

Boot passing uses only public points, excludes absent players and does not
change anyone's score. Lowering the current player's target by passing the boot
can immediately end the game. Fish-funded roads can immediately win through
Longest Road; a bought hidden point also counts immediately, while a bought
Knight remains unplayable until the next own action phase.

Views expose affordable actions and public eligible targets only to the active
viewer. Bot payments minimize overpayment, then free small-token slots; choices
use own resources/fish and public points, counts, map and bank. Lake and coast
production influence settlement/city positions, and lake robber valuation uses
all printed lake numbers. Bots may remove an own blocking robber before rolling,
pass the boot, buy cards, build roads, steal or fill a resource shortfall.

Six new engine tests cover 20 cost/phase/player cases, immediate route/hidden-VP
wins, payment and stock/connection/piece failures, public-point boot passing,
private views and hidden-information-independent bot choices. Four complete
bot games (3/4/5/6 players) finished at the correct boot-adjusted targets;
every action checked fish/resource/development/piece inventories, with periodic
full state restoration. They used 21/16/14/31 paid fish actions and ended in
rounds 29/14/17/13 (4.825s including test startup). These are rule/bot matches,
not client/browser acceptance.

Eighteen HTTP cases cover all six actions in pre-roll, ordinary 3-player turns
and 6-player second actions. Each really restarts the service before and after
payment (36 restarts total), checking full Room equality, permissions, private
faces, newly bought cards, resource effects and unchanged deadline. They pass
in 6.780s, with race checks passing in 65.278s. Game/server static checks pass.

The related regression exposed an existing random-fixture issue: the paired
bank-trade test assumed 4:1 even when setup gave a 2:1 or 3:1 harbor. The test
now uses the actual rate to verify second-player trade permission. Production
trading behavior was not changed. The corrected case passes 30 consecutive runs (1.157s); related game regressions pass in 3.710s and server response regressions in 1.530s.

No room/UI option, assets or deployment was added. Existing test temporary
directories clean themselves; no owned downloads or screenshots were produced.

## Original artwork (completed 2026-10-05)

Nine original component images were extracted from the pinned official 2025
PDFs: base and extended lakes, coastal ground, 1/2/3-fish tokens, old boot,
token back and blue number disk. The selected image objects were matched
visually to the printed component rows. Nested/clipped PDF examples contain
other similar images; the extraction script therefore pins both object IDs
and native dimensions rather than picking the largest object automatically.

The artwork retains native proportions and pixel dimensions. PDF soft masks
have different pixel grids from their color images, so only the mask is sampled
to the color image's native grid. Lossless WebP output totals 201,294 bytes.
Every uploaded CDN image returned HTTP 200, decoded as WebP, and matched its
native dimensions and SHA-256; a second extraction reproduced all nine hashes.
Evidence is in `catan-fishing-art-sources.json`; the reproducible script is
`../../scripts/prepare_catan_fishing_assets.py`.

Integration notes for the next UI stage:

- The lake and ground images have no baked-in production numbers; those were
  separate overlays in the printed rulebook. Render the actual saved map's
  lake number arrays and shuffled ground numbers using the blue disk and
  readable text. Clip the disk to a circle to remove its white square corners.
- The ground image faces right, with its land-facing tip near native pixel
  `(181.5, 116.5)` and ends near `(115.5, 0)` and `(115.5, 233)`. Anchor the tip
  to the ground's middle vertex and rotate the sea side toward the vector from
  that vertex to the midpoint of its two endpoints. Preserve proportions and
  scale to the two coastal edges; do not treat the ground as another hex.
- Choose the extended lake artwork for the saved two-number face, not from a
  fixed tile index. Opponents expose token counts only; do not derive or show
  their total fish value. The boot changes the corresponding per-player target.

Owned contact sheets, prepared WebPs, repeat-extraction files and temporary
research files were cleaned after verification. Original pinned rulebooks
remain for other expansion research. No game/frontend code changed and no
browser acceptance, room entry, push or deployment occurred in this stage.

## Integration still required

- Combinations, including fish-only Aqueduct compensation and Catan for Two's
  different initial fish allotment/discount. The existing base 5–6 number
  catalogue verification and two-lake setup interpretation remain for final
  acceptance; all original expansion/variant scope is retained.
- Room options, original-art desktop/mobile UI and complete-game QA.
  No public entry, push or deployment until these acceptance requirements pass.
