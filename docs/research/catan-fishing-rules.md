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
implemented, with original-art desktop/mobile UI verified on local engine fixtures.
Fishing is not exposed in room configuration; complete scenario/combination
acceptance remains, and it is not ready for
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

## 前端交互验收（2026-10-05）

原图地图及私有筹码面板已接入，五种支付、传靴子、满额盲换/保留、合法目标与旧选择校验均已验证。六人扩充使用原湖泊四点数（2/3/11/12）和额外湖泊两点数（4/10）；沿海渔场按三个顶点定位，不拦截道路/建筑点击。自己的靴子额外目标显示在地图栏，公众只见他人筹码数量。

本地真实引擎测试牌桌在1440/390/320px完成消费与选择交互、图片加载、无横向溢出、换筹码高亮/标题/120秒提示和观战隐私核验。52项前端测试及构建通过。仅是界面与引擎夹具验收，未开放公开房间、未发布，未完成的组合、动画和整体验收仍需继续；详见总进度文档本阶段记录。

## Fishing + Cities & Knights (2026-10-05)

Pinned the one-page official August 2025 combination PDF in the rule-source manifest and visually checked the page. `NewCatanFishingCitiesKnights` is internal only. It builds the Fishing board, enables C&K components and second-city setup, preserves fish secrecy and the separate token economy, and derives 13/14 victory targets from the saved combination plus boot owner.

The 7-fish action is `catan_fish_progress`, with a public track index and owned token IDs. It draws from the chosen nonempty face-down stack; it never offers hidden individual card identities. The 3-fish action includes commodity-only hands even before the first barbarian attack (per the existing official FAQ); the 4-fish action rejects commodities. Progress cards use the existing immediate-play, public-point, active-player end-of-turn cap and private-view rules. Fish-only production still has a zero resource/commodity production count for Aqueduct. Any capped fish replacements finish before the Aqueduct response starts, without repaying normal production.

Seven new engine tests cover setup, payments, continuations, limits, immediate wins, privacy, progress effects excluding fish and bot independence. Four full 3–6 player bot games completed with per-step conservation and periodic restore. Six HTTP paths and 18 real restarts verify payment phases, permissions/privacy and manual/autoplay/timeout replacement→Aqueduct→original clock recovery; race passes. 53 frontend tests, build and game/server vet pass. Real-engine browser fixtures at1440/320px verified the original three progress-card backs, seven-fish payment, private card receipt and replacement→Aqueduct UI handoff. Full evidence/timings are in the main expansion checklist. No public option or deployment.

Helpers is not silently enabled: the pinned 2022 12-helper rules only expressly list base CATAN and Seafarers plus their extensions. No official Fishing interpretation for the now-absent desert was found, and the current official Helpers FAQ still describes the older 10-character edition. This is an evidence gap, not a claim that every possible unofficial combination is forbidden.

The separately pinned July 2025 Fishing + Seafarers PDF expressly excludes Pirate Islands. Other scenarios have different lake/ground placements and pirate/ship rules, including number-token relocation in Through the Desert; these remain implementation work. Its shipbuilding paragraph says “fish tokens” whereas the base action table prices fish values; preserve this wording issue for implementation review rather than silently changing payment units.

## Fishing + Seafarers: Four Islands map stage (2026-10-05)

Visually checked both pages of the pinned July 2025 combination PDF. The three/four-player Four Islands recipe has no lake: 4/8 belong to one small island, 6/10 to the other; 5 and 9 belong to different large islands. The current fixed and variable island outlines support these placements without overlapping ports. The default chooses a deterministic legal placement; an internal caller can choose other legal coastal edge pairs and swap the compatible island groups.

`FishingCoasts` enumerates concave coastal Vs from board topology, excluding ports, fog and convex corners around a single land hex. Saved grounds derive all three production vertices and an optional `seaTile`; frame grounds omit the latter. Validation checks the saved geometry, island groups, numbers, overlaps and water association. A pirate blocks only grounds in its own sea hex; ships do not produce fish. This optional field does not change base Fishing saves.

Four new engine tests cover 96 generated fixed/variable maps, caller-selected swapped groups, JSON restore, unchanged scenario state, production at every ground vertex with both building levels and every sea-hex pirate position, ships producing nothing, corrupt maps and rejected placement atomicity. Runtime combination validation intentionally still rejects Fishing + Seafarers: this is a map/production stage, not the completed playable combination or browser acceptance.

Remaining before enabling this combination: original robber setup interpretation, the printed “fish tokens” payment wording, fish-paid ships/pirate removal, UI/AI/HTTP and full-game acceptance. Other scenarios need their own recipes and continuations (especially gold followed by fish responses); the three-player New Shores map has no desert and needs a sourced lake placement interpretation. Five/six-player maps are not silently treated as the three/four-player Four Islands recipe. Helpers and all other originally planned combination work remain in scope.

The initial expanded regression exposed a test-fixture assumption: fixed terrain still randomizes ports, so the default placement need not include a frame ground. The production-only fixture now removes ports to exercise both frame and sea-hex grounds deterministically; the 96 map construction checks retain the real official ports. Ten repeat runs of all four new tests pass (1.792s). This changes test setup, not game port placement or rules.

## Fishing + Four Islands runtime stage (2026-10-05)

Resolved the payment terminology using the official German legacy combination linked by the current German T&B product page. Its visually checked first page explicitly says `5 Fische` for a ship and `2 Fischen` for pirate removal. Together with the 2025 base payment definition, this corroborates fish **face values**, not physical token count. The source is pinned separately; its older Fog/Desert lake instructions are not imported into the 2025 implementation. No claim of a new official English erratum is made.

`NewCatanFishingSeafarers` now internally constructs 3/4-player fixed/variable Four Islands and optional validated ground placements. Per the 2025 instruction to use scenario setup except the listed lake/ground changes, it retains the Four Islands robber and pirate setup. It adds no lake, preserves the 13-point target and raises only the boot holder to14. Starting fishing uses the existing second-settlement entitlement. Six Islands, other sea scenarios, Helpers and the triple C&K combination remain rejected until their independent recipes and rules are ready.

Added `catan_fish_ship` (5fish) and `catan_fish_pirate` (2fish), with concrete-effect validation before payment. Ship placement reuses connectivity, enemy buildings, pirate blocking and the15-piece supply, marks new ships immovable this action phase, and reuses route completion/scoring/victory. It does not spend ordinary resources or consume Road Building's free placements. Pirate removal does not steal or move the robber. Both may run before rolling or during the normal action phase, with no turn/deadline reset; any required response must finish first. New legal ship targets are private to the active player. Bots evaluate public sea routes and blocked own fisheries/ships; pirate placement considers fish production without looking at opponents' token faces.

Seven engine tests cover constructor/setup/boot/restore, both payment phases, overpayment by face value, invalid request atomicity, blocked/occupied/disconnected/supply-exhausted ships, legal views/privacy, bot decisions without hidden pile order, real fishing production after pirate removal, capped response restoration, and a fifth ship's longest-route award at13/14 points. Four 3/4-player fixed/variable full bot games finish with fish/resource/piece conservation and periodic serialization. HTTP covers both payments in both phases plus manual/autoplay/timeout response chains, with17 actual server restarts across seven paths. Full evidence and timing are in the main checklist.

This stage is engine/service work. The ship/pirate controls and blocked-ground highlight still need original-art desktop/mobile UI and browser acceptance; no public room option, asset upload, push or deployment. Other scenario maps, gold/fish continuations,5–6 recipes and remaining expansion scope are unchanged.

## Four Islands action UI (2026-10-05)

Added fish-paid ship and pirate controls, preserving ordinary resource construction as separate actions. Only sea games show ship/pirate fish actions; the base/knights menu retains its own development/progress variant. Choosing a ship sets a dedicated map mode, highlights only current private legal ship edges, and never opens the ordinary construction confirmation. Payment requires owned distinct tokens and rechecks the chosen ship edge or current pirate availability. The panel can collapse to the map and reopens on selection. Selection now scrolls confirmation into the desktop side panel or the middle of the mobile viewport; this also fixes the existing fish-road flow.

Grounds accept the optional saved sea-hex association. A matching pirate removes production glow and adds a small red cross badge plus public text; original art and number colors remain intact. The sea frame and base grounds never inherit a sea-hex blockade, including the edge case of sea hex zero. Pirate removal immediately clears the notice and restores the matching production glow.

Three new state tests cover sea actions/payment/targets/stale or inactive views, variant menus/road-versus-ship modes, and pirate/sea-frame matching. All56 frontend tests and the production TypeScript/Vite build pass. Browser fixtures use the actual Go constructor, action engine and per-viewer public views:1440px desktop and390/320px mobile payments, collapse→select→confirm→ship placement, pirate removal and production indicator update,31 referenced images decoding, no horizontal overflow or JS errors. The320px observer sees public blockade/counts but zero private token faces and zero action buttons. Base fish-road payment still completes, without sea controls or duplicate normal confirmation. These are full-board/engine fixtures, not a newly exposed production-room option; previous service restart/clock tests remain the service evidence.

Owned browser/server processes, profiles, screenshots, test/build logs and temporary fixture files were cleaned. No new assets, upload, public room option, push or deployment. Other maps, gold/fish sequencing,5–6 recipes and all remaining originally scoped expansions/combinations remain outstanding.

## Fishing + Fog Islands engine/service stage (2026-10-05)

The pinned July2025 combination p.2 says no lake and freely placed grounds along the faceup islands. The internal constructor now supports3/4-player fixed/variable Fog Islands, six grounds and optional valid host placements, retaining the scenario's12-point target (13 with the boot). Initial coast geometry comes from the existing printed map recipe plus actual ports, not the current discovered island labels or hidden stacks. Revealing land or sea does not introduce new placement sites or move saved grounds. Five/six recipes and unverified Helpers/C&K triple combinations remain gated.

Fixed production's Fishing early return losing gold claims: ordinary bank-limited production → fish draws/replacement queue → saved gold claims → original turn. Deferred counts and already-received amounts persist; fish and gold response queues never overlap. Shared starting-production continuation also prevents gold from overwriting a fish response. Gold discovery through fish-paid ships/roads resumes the original pre-roll/action phase and route completion, including new-ship movement restrictions.

Seven engine tests cover96 generated maps and full exploration, placement/corruption/restore/privacy, six production branches (full/nonfull hands × full/short/empty bank), starting continuation with an explicitly synthetic gold coast, and both route types/phases. Four complete bot games finish with conservation and periodic restore. Three HTTP paths (manual/autoplay/timeout),15 actual server restarts, private views/invalid actor rejection and independent120-second responses restoring the original45seconds pass. Detailed timings are recorded in the main expansion checklist.

No new UI/browser acceptance in this stage. Next verify the combined fish→gold UI and map on desktop/mobile. No public room option, asset upload, push or deployment; all other scenario/expansion scope remains outstanding.

## Fog Islands browser acceptance (2026-10-05)

Verified the full CatanBoard against a local real Go engine/public-view fixture.1440px desktop: blind replacement→next player's keep→two gold claims→original turn; actual fish replacement, resource receipts and engine log agree. Collapsing gold to70px and reopening preserves selection; quantity limits disable excess additions.320px mobile: all seven fish faces and response buttons fit, gold resources/stock/confirmation remain usable, and observer views expose no private faces or choosing controls.390px mobile:2+3fish payment→collapse→choose ship edge→confirm→discover gold and sea→choose resource→resume pre-roll, preserving the new ship.33 referenced images decoded successfully. Four-player variable initial board also checked at1440/390, including12 fog positions and six grounds;22 referenced images decoded. No horizontal overflow or JS errors.

No production UI changes were required. This is component/engine browser evidence; API clocks/restarts were verified in the previous stage. Browser and Go/Vite processes closed and owned temporary source/pages/profiles/screenshots cleaned. No public entry, push or deployment. Next: Through the Desert's lake replacement and dual-number production.

## Fishing + Through the Desert fixed setup (2026-10-05)

Pinned July2025 combination p.2:3p field-2→lake, disc2→pasture-12;4p hills-11→lake, disc11→hills-2. Added the fixed3/4-player recipe, ordinary lake2/3/11/12, six freely chosen nonoverlapping coastal grounds, unchanged desert exploration regions,14VP/15boot. Extra ordinary production discs persist in public map metadata; production matches either once, with normal bank/robber rules. Starting lake awards fish and no removed terrain resource. AI public production evaluation includes both discs.

Variable setup is still unresolved: the quoted fixed terrain/number identities do not survive the existing separate shuffles. This implementation explicitly rejects variable and5/6 Fishing+Desert instead of assigning fixed coordinates to arbitrary terrain. These remain in the original scope pending sourced recipe interpretation.

Tests cover48 maps,18 production/blockade branches, starting fish, illegal/restore/privacy and two complete bot games, plus six HTTP modes and12 actual restarts restoring the45-second original clock.58 frontend tests/build pass. Real-engine view fixtures at1440/390/320 verify both ordinary discs, both-roll highlights, blocked lake/ordinary production and resource counts. Robber icons were moved above multi-number terrain; blocked lake discs shift downward so all remain visible. Geometry checks confirm no overlap,32 images decode, no horizontal overflow or JS errors. Temporary browser/service/fixture artifacts cleaned. Full timings are in the main checklist; no public option, push or deployment.

## Fishing + Forgotten Tribe engine/service (2026-10-05)

The pinned July2025 Fishing+Seafarers combination, p.2, replaces field-12 with the lake, places grounds on the main island only, and confines subsequent robber moves to that island. Implemented the shared fixed3/4 map: tile24 becomes lake2/3/11/12, six freely chosen nonoverlapping grounds; no number disc migration, changed reward supply, or extra island points. The printed robber begins on outer desert47 as specified by the scenario, may be removed for2fish, and subsequent7/Knight moves may enter the lake but not the outer islands.

The older official German2021 combination's Forgotten Tribe section explicitly says “Beachten Sie, dass die Positionierung der Fischgründe Einfluss auf die mögliche Platzierung der später ins Spiel kommenden Häfen hat.” This supplements the unchanged physical placement constraint: collected ports may not occupy a ground's two edges; the ordinary one-edge separation between ports remains. It does not import the older Fog/Desert recipes. Sources are the already-pinned English/German documents and hashes above. Port legal hints, bot choice and Apply all use the same free-coast filter; successful placement never moves a ground or invalidates future production.

Fixed recipe validation checks the18-hex mainland,49-hex board, one lake/six grounds, topology, current ports, number inventory, and initial/mainland robber positions. Lake/placement changes commit only after validation. Mainland settlement/robber eligibility includes the replacement lake's printed production numbers. Variable and5/6 recipes remain unresolved: their separate terrain/number randomization or printed component changes do not necessarily contain the referenced field-12; they remain in scope and are not silently assigned tile24.

Six new engine tests include24 generated fixed states plus custom reordered/reversed ground placements,16 lake production/blockade branches, ordinary robber lake entry and fish removal, port-ground overlap rejection/restore, six fish-paid ship paths (three rewards × pre-roll/action), hidden fish/development views and starting lake entitlement. Ships collect points/development/ports through normal route completion, retain new-ship movement restriction and resume the original phase after a port response. Two complete bot games finish with fish/resource/development/reward/piece conservation and periodic save restoration.

Twelve HTTP paths (3/4 players × pre-roll/action × manual/autoplay/timeout) enter port response through real fish-paid ship Apply. Twenty-four actual restarts preserve entire Room, fish payment, grounded map and route continuation. Invalid seat/spectator/ground-overlap requests are atomic; public views reveal no private fish or unclaimed development faces. Response120seconds pauses/resumes the original~45second turn; no duplicate payment or shifted grounds. Specific engine4.918s/server4.464s, related engine42.708s/server19.185s, static checks pass. Browser acceptance of this combined lake/port/grounds layout remains next; no public option, push or deployment.

The twelve new HTTP paths also pass the race detector (42.037s). All test processes completed; no persistent QA service or browser was started in this stage.

## Forgotten Tribe browser acceptance (2026-10-05)

Used the complete CatanBoard with a temporary local Go service calling the real constructor, Apply and private View.1440px desktop: select2+3fish, choose the final reward ship edge, confirm actual payment, collect the port, choose a legal mainland coast and place it; state/logs show exactly5fish paid, one remaining token, a new ship and return to pre-roll.390px mobile repeats payment→ship→port choice; independent3/4-player port responses verified at390/320, including collapse to66px, map selection reopening the panel, confirmation and continued original phase.

Added combination-specific tribe summary (mainland includes the lake; subsequent robber moves stay there) and explicit port/ground non-overlap guidance. At320px, move the robber from its outer starting desert into the actual lake through the normal confirmation UI; all four blue production numbers remain readable, with DOM bounds proving no robber/number overlap. A newly placed port at edge49 does not overlap any of the six fishing production number bounds.37 distinct referenced assets decode successfully, no page horizontal overflow or browser errors. Observer port response has no private fish hand, fish-action buttons, port choices or confirmation controls.

61 frontend tests and TypeScript/Vite build pass. Temporary Go/Vite services and named browser closed; source fixture, HTML, screenshots and profile removed. This verifies the fixed3/4 recipe UI, not unresolved variable/5–6 recipes or the full expansion release. No public entry, push or deployment.

## Fishing + Cloth for Catan engine/service (2026-10-05)

The pinned July2025 combination p.2 specifies no lake, three fishing grounds on each large island, and a village trade connection before spending2fish to remove the pirate. The internal constructor now supports3/4-player fixed/variable cloth boards, retaining the42-hex map, two10-hex home islands, eight cloth villages and50-cloth inventory. It validates topology and home-island identity, allocates six nonoverlapping port-free grounds with4/5/6/8/9/10, and accepts independently validated custom positions/numbers. Only the third starting settlement awards ordinary resources and fish. The variable board keeps its initial robber choice. Five/six recipes and unverified Helpers/C&K combinations remain rejected.

Fish-paid ships establish ordinary trade relationships and award first-connection cloth, retaining closed-route movement restrictions. Pirate removal uses the same existing trade prerequisite as normal pirate movement and never steals cloth/resources. Production awards cloth and bank-limited ordinary resources before capped fish responses, then checks victory after all draws. A replacement drawing the boot can therefore prevent a14-point win by raising that player's target to15. The five-empty-villages end condition is unchanged.

Seven engine tests cover48 generated maps plus custom placement/corruption, the third-settlement entitlement, four fixed/variable full/nonfull-hand production branches, save/restore and privacy, pirate blocking/permission, four fish-paid trade-route completion paths, and boot-versus-normal replacement at the victory threshold. Four complete3/4-player fixed/variable bot games check fish/resource/cloth/development/piece conservation every step and restoration every31steps. Tests use explicit reachable action fixtures where required, rather than claiming all fixtures are full natural games.

Twelve HTTP paths cover3/4 players × fixed/variable × manual/autoplay/timeout. Real Apply rolls enter two full-hand fish responses after cloth/ordinary production. Thirty-six actual server restarts preserve the entire Room, map, payouts and response state. Invalid actor/spectator/unowned-token requests leave state/deadline unchanged; only each owner sees private fish faces, and only the responder receives replacement permission. Each response has120seconds, followed by the original45-second turn; resources/cloth are not paid again. The new HTTP test passes in4.748s; race45.256s, related service regression24.104s, game/server vet pass.

This does not resolve the existing base-cloth common-supply exhaustion ambiguity: the engine still fails closed when the common stock cannot supplement all entitled traders. `TestCatanClothCommonSupplyBoundaryProtection` remains the explicit counterexample. No invented shortage allocation or release claim is made. Combined desktop/mobile board acceptance is next, and this combination remains inaccessible in public room creation. No new asset downloads/uploads, public option, push or deployment.

Related Fishing/Cloth engine regression also passes (55.140s). All test processes have finished; this stage started no persistent browser or QA server.

## Cloth fishing browser acceptance (2026-10-05)

Complete CatanBoard backed by a temporary local real Go constructor/Apply/private-View service:1440px fixed3-player ship payment (2+3fish)→first village trade→one cloth→2fish pirate removal→original pre-roll;390px variable4-player ship payment also establishes trade and enables the previously prohibited pirate action. Village details show actual own trader and remaining stock. Combination-specific copy explains third-settlement resources/fish, three grounds per large island/no lake, and the fish-pirate trade prerequisite; the last disappears once trade is established.

390px variable4-player production: first full-hand player blind-replaces,320px second player keeps; real state resumes action with unchanged cloth payout and village stock. All seven private faces and both controls fit.320px variable4-player initial robber choice collapses to66px, selecting a legal12-number tile reopens confirmation and starts settlement placement.390px fixed3-player setup after real bot-completed first two placement rounds starts with no resources/fish; manually placing a third settlement beside a ground awards wood/grain and one private fish before the road/ship step.

34 referenced images decode, fish-number bounds do not intersect cloth village number/stock bounds, no horizontal overflow or browser errors.320px observer sees no private fish faces, action menu or replacement controls.61 existing frontend tests pass. Named browser and local Go/Vite processes closed; temporary fixture source/pages/profile/screenshots removed. This component/engine QA complements prior HTTP restart/timer evidence. No public entry/push/deployment; common-supply shortage, five/six recipes and remaining expansion scope are still outstanding.

TypeScript/Vite production build passes, retaining the existing >500kB bundle warning.

## Correction: small-board cloth supply bound (2026-10-05)

The earlier four-trader shortage fixture was not physically reachable: each village has three incident edges, different traders cannot share an incoming ship, and established closed ship routes cannot be removed. It is now named `TestCatanClothCorruptCommonSupplyProtection`, retaining its corrupted-save guard without claiming a legal counterexample. Ordinary ship movement, knight interruptions/diplomacy, and platform elimination preserve those occupied entrances; eliminated players do not produce.

For the3/4-player board, each village can need at most2common tokens, only on its final production. Fewer than5villages depleted at the preceding turn end means at most8common tokens consumed. Pre-roll ship connections may empty additional villages but never draw common tokens. All eight village numbers are distinct, so one roll needs at most2more common tokens before the next end-turn check. This proves the10-token common stock sufficient for the audited3/4 ordinary, knights and fishing variants. It is a bound derived from printed rules/topology, not a newly found official FAQ or a rule to substitute counters. Future multi-production variants require a fresh audit.

New tests enumerate16single-village inventories and verify the actual3/4fixed/variable×ordinary/knights/fishing constructors, distinct numbers, degree-three vertices and the worst four-empty→fifth-production→ending accounting envelope. The5/6map explicitly has doubled4/5/9/10, so simultaneous production remains an independent unresolved case; the small-board proof is not extended to it. Earlier historical statements of a blanket cloth-shortage blocker are superseded by this narrower finding. Public expansion gating/release scope is unchanged.

New bounds tests0.617s, related Cloth/CK-Cloth/Fishing-Cloth regression30.073s, game vet pass. Temporary fetched HTML/plaintext removed; no public option, push or deployment.

## Fishing + Wonders engine/service (2026-10-06)

Pinned July2025 Fishing+Seafarers p.2: no lake, freely placed grounds on large/small island shores; keep the scenario victory rule and add1VP for the boot holder. The internal constructor now supports3/4fixed/variable Wonders:49hexes,25/3/2islands, no pirate, original wonder markers/blocked starting vertices, mainland-only setup, second-settlement resources/fish and small-island bonus. Six grounds validate exact coastline/vertices/numbers, no overlap/ports; no forced mainland-versus-small-island split. Five/six recipes and unverified Helpers/CK triple combinations remain gated.

Wonder score-leading victory now reads the player's own target, correcting the shared-target shortcut:10ordinary/11boot, strictly higher built level than every rival. The separate completed-level4condition remains independent of points. This follows applying the combination's extra VP to the scenario's VP condition, not a new fifth-level requirement. Passing the boot can immediately win at10if already leading, including pre-roll. Panel/detail/VP-card text use the personal target; result text preserves both endings and explains the boot exception.

Six new engine tests include48generated boards plus custom placement/corruption/setup restrictions, real bot setup entitlement, real small-island gold+fish production (fixed/variable × full/nonfull hands) with serialization and sequential fish→gold claims, ship payments before/after roll without unintended resource/fish gain,12point/level/boot winner combinations and both-phase boot transfer. Four full bot games verify fish/resource/development/piece inventory per action and periodic restore; wins include both score-leading and level4. First bot matrix rounds35/47/22/26, paid-fish actions3/0/13/17; an individual bot game need not use fish to be valid.

Twelve HTTP paths cover3/4×fixed/variable×ordinaryVP/bootVP/boot4, using actual fish-paid VP purchases and ordinary-resource wonder construction. Nine→ten wins without boot; with boot ten keeps the original clock, and eleven wins after the next purchase; level4atnine wins with boot. Twenty-eight actual restarts preserve room/map/boot/partial and final wonder states/payment/result; wrong seat/spectator actions are atomic, private fish faces remain private and per-player targets public. NewHTTP4.481s, race42.073s; related Fishing/Wonders service51.716s and Fishing-Wonders/base-Wonders/CK-Wonders engine74.974s; game/server vet pass.62frontend tests and production build pass (existing bundle-size warning).

Next: full desktop/mobile board acceptance for grounds/markers, personal thresholds, VP/four-level action endings and fish→gold UI. This stage does not claim new HTTP response-clock coverage or browser acceptance. No public entry, assets/upload, push or deployment; no persistent QA processes were started. Five/six cloth simultaneous-production analysis and all remaining expansion scope stay outstanding.


## Wonders fishing browser acceptance and shared port layout (2026-10-06)

Full CatanBoard backed by real Go construction/Apply/private View:1440px boot-holder fish-paid VP purchases leave10points playing and win at11with strict wonder-level lead;390px ordinary resource payment completes level4and wins at9with the boot.320px gold+fish production resolves two capped fish responses before a two-grain choice, preserving the selection through collapse/expand and restoring action without duplicate production.320px variable initial robber selection, blocked setup markers,120%zoom and observer privacy pass; Escape returns focus to the wonder detail trigger.38referenced assets decode, no page horizontal overflow or browser errors.

The default Wonders port at edge46 covered fishing numbers8/10. Shared `catanPortLayout` preserves all gameplay edges/endpoints and original clear positions, relocating only conflicting artwork to the nearest searched water-side clear position within existing bounds. It avoids fish/terrain numbers, wonder markers, cloth labels and other ports; the original position is the fallback if no candidate exists. Grounds render artwork before ports and numbers after them, so neither port images nor leader lines obscure the number disks. Geometry is memoized by game state, avoiding repeated search during local map drag/zoom.

Two geometry tests cover six rotations, deterministic layout, unchanged state/edge/size/endpoints and unaffected ordinary/clear maps. Twenty real Go-generated3/4-player maps (fixed/variable islands,fog,cloth,wonders;fixed desert,tribe) have no port-art/fish-number intersections.1440/320visual checks retain readable3:1art and its original coastline leaders.64frontend tests and TypeScript/Vite build pass; existing bundle-size warning remains. This browser work complements earlier HTTP/restart evidence without claiming new HTTP response-clock coverage.

Named browser, Go QA service and Vite stopped; temporary fixture source/pages/map JSON/screenshots/profiles removed. No public option, new assets/upload, push or deployment. New World ground placement and the rest of the expansion/combination scope remain outstanding.


## Fishing + New World setup engine/service (2026-10-06)

Pinned July2025 Fishing+Seafarers p.2 says no lake; place grounds after ports using the same method. The internal `NewCatanFishingNewWorld` accepts an approved3/4-player map without rerolling terrain/discs. Ten shuffled ports finish first, then six shuffled4/5/6/8/9/10grounds in clockwise order, then ordinary two-round settlement setup. Each layout stage starts with the original first player. Only the current ground is public during the ground stage; no ground face is revealed during ports. Keep personal home islands,1VP foreign-island settlement rewards, second-settlement single fish,12/boot13target and pirate blocking. Five/six recipes and Helpers/CK combinations stay gated.

An exact joint coastline packing check prevents layout dead ends without reserving specific future positions. Boundary loops have port single-edge pieces (no adjacent ports) and concave two-edge grounds (no overlap, but may touch other pieces). Cycle DP considers the cut-crossing ground, first-slot port, and neither; component capacities combine both remaining inventories. Invalid approved layouts are rejected rather than silently repaired. At each step only prefixes incapable of any complete legal layout are excluded, so every complete arrangement remains reachable. It reads only public geometry/existing occupancy, not shuffled identities. This is a digital setup feasibility safeguard, not a new printed allocation rule.

384small-cycle cases match independent exhaustive component-subset enumeration, including seams and competing inventories.48real3/4-player maps have constructive ten-port/six-ground witnesses with unchanged approved geometry. A deterministic compact board reproduces a legal ordinary ninth-port move at edge46which strands fishing; the combination rejects it atomically and finishes through other positions.

Sixteen actual engine setups cover256manual/timeout steps and per-step serialization, turn order, no premature building/resources, private future order invariance, corrupt-state rejection and supported-option gating. A deterministic board exercises second-settlement fish entitlement and sea-ground pirate blocking. Two complete bot games check fish/resource/development/piece conservation every action and periodic restore; first run46/28rounds,19/34fish actions,12/13winning points.

Six HTTP paths cover3/4players × manual/autoplay/timeout, all16layout moves each. Wrong-seat/spectator/invalid-location requests preserve Room and deadline. Thirty real server restarts cover initial state and both sides of final port/ground transitions, preserving map, placed pieces, hidden stacks, actor and clock. Every move starts a fresh120second window, including return to the first settlement; no extra piece or resources are granted. NewHTTP7.025s passes. Public views hide future ground/port/fish order; placement hints belong only to the current player.

Ground-placement UI, desktop/mobile browser acceptance, room configuration and dedicated custom-gold fish→gold response coverage remain next. No new assets/upload, public entry, push or deployment. All other expansion/combination scope remains outstanding.

Related NewWorld/Fishing-NewWorld/capacity/Fishing-Wonders/Fishing-Fog engine regression87.363s; newHTTP plus ordinary NewWorld-port clock race regression67.446s; game/server vet and diff whitespace checks pass. Starting-entitlement assertions additionally verify exactly one pile draw for an eligible second settlement (boot consumes that draw), then pass a focused rerun. Temporary probe source and test logs removed; no browser or persistent QA service started. Local commit only; next is ground-placement UI/browser acceptance.
