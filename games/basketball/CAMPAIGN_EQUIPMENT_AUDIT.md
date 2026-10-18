# Original campaign, equipment and preparation flow

This audit identifies the original rules and the next reconstruction units. It does not implement a campaign, unlock production content, or claim that fourteen matches have been reproduced. Engine/runtime and product UI were not modified.

The supplied ROM remains SHA-256 `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`. All new menu evidence uses normal controller input, no RAM writes, and the existing workspace FCEUX recorder. Existing matching captures are reused. A legal password route is explicitly identified as password restoration, not a naturally earned win.

## What the original campaign actually offers

- Story mode supports **one player or two cooperating players**, with the remaining court characters controlled by the original AI. Normal first-match captures establish Mode 0 with assignments `[0,FF,FF,FF]` and Mode 1 with `[0,1,FF,FF]`.
- The campaign's human team is Nekketsu/team 0. The map offers opponents 1–7 in map order **1,4,7,3,5,2,6**; map index 0/Niagara is skipped in story-mode selection.
- This is **seven opponents, each with an away and a home match**, not a succession of newly unlocked arbitrary maps. `Period` counts half-periods. Its bit 1 chooses the current away/home round and the corresponding remaining-opponent mask.
- Away matches use the selected opponent's court. On a home match, preparation retains that selected opponent, then original `B29F/B2BF` changes Court to **0/Niagara** before entering the match. Normal password-to-equipped-home-match input now demonstrates this exact team/court distinction.
- The map skips opponents already beaten in the current mask. A victory records that opponent in the appropriate win mask; a defeat rolls Period back by two. Continue returns to the map with the retained progress, rather than forcing a new empty campaign.
- Fourteen wins advance Period to **28** and enter the ending. There is no invented unlimited campaign or arbitrary post-ending schedule.
- The existing platform uses original Mode 128 for isolated matches, so its “solo” option is currently not this original story campaign. A platform campaign option would need its own persisted progression and preparation contract.

## Original state that must survive

| Original field | Meaning and handling |
|---|---|
| `058D` | Mode: natural story 0/1; bit 7 disables campaign equipment/enhancement paths. Preserve low-bit player selection separately from platform seat mapping. |
| `058C` | Period/half-period progression; also supplies AI difficulty. It must continue across campaign matches, not restart at zero. |
| `06D2`, `06D3` | Two seven-bit opponent-win masks; current map uses `(Period >> 1) & 1`. Store both. |
| `06D4` | Reward-set selector, originally chosen by one RNG draw masked with 3 at `A1F8/A205..A20A`. Keep the chosen set for the run. |
| `D3..E2` | Sixteen ordered inventory slots; FF denotes empty. Equip removes/compacts this inventory; swapping returns the old item into the selected slot. |
| `06E4..06EF` | Four characters × three equipped slots, at pointers `06E4/06E7/06EA/06ED`. This is indexed by roster character, not whichever platform seat currently controls that character. |
| `03E3..03E6` | Current four match character choices; changing the roster must retain each Nekketsu character's equipment. |
| `0666/0667` | Team tactics; normal preparation menu offers profiles 1–5, with default 4. |
| `0586/0587`, `058B` | Team choices and actual court. Home-court substitution must not accidentally change the opponent. |
| `04E8..04EB` | Original controller assignments, separate from platform seat authorization. |
| Shared RNG | Carries through selection, reward and presentation; do not restart from a new arbitrary seed for each match. |
| Menu cursors / preview state | Original `06D7..06DF` and upload/scratch bytes are temporary menu state, not extra copies of equipped inventory or authoritative match state. |

Password glyph digits `03CB..03D0` overlap display/match caches. They must not become a second permanent copy of campaign progress. Existing result-phase aliases also need explicit conversion when character fields are temporarily replaced for celebration graphics.

## Equipment rules recovered from actual source

`91DD` assigns the category from the item ID, with no free choice of slot:

| Item IDs | Slot | Count |
|---|---|---:|
| 0–9 | Shirt | 10 |
| 10–20 | Pants | 11 |
| 21–31 | Shoes | 11 |

Names are nine encoded original glyphs through fixed-bank pointer table `C2C8`; source pointers and raw bytes for all 32 items are exported in the audit artifact. These are source identities, not invented equipment names or a claim that every item can be obtained in an ordinary run.

The normal equipment menu offers equip, finish, remove, and change character. Source entries:

- Scene 2 phase 23, `903F`: equipment presentation initialization.
- Phase 24, `90A9`: command menu; empty inventory prevents equip, and an empty equipped character prevents remove.
- Phase 25, `9149`: finish and return to the normal character preparation view.
- Phase 26, `915E`: change which Nekketsu character is being equipped.
- Phase 27/29, `91DD`: select an owned item, derive shirt/pants/shoes, then either consume-and-compact the inventory or swap with the old item in the same category.
- Phase 28, `931D`: remove the selected category and append its item to the first empty inventory slot.
- `942C/949A`: equipped-name/inventory-name rendering; `8CEE` and existing `ACBB/AD14/AD3C/AD8F` provide the associated preview/attribute paths.

No item is assigned directly to a match seat. The existing Go `Equipment[character][slot]` storage shape is appropriate, but `validateRuntimeConfig` currently checks only -1..31, not original category/ownership constraints. Runtime acceptance of a numeric item ID therefore does not establish a valid preparation choice.

The original removal code clears the equipped slot before scanning for a vacancy. Its behavior with an artificially full inventory requires a bounded oracle, rather than assuming the source provides transactional rollback. A normal campaign has a bounded number of awards; this audit does not claim that such an overflow state is naturally reachable.

### Rewards and acquisition

Phase 7 after a nonfinal victory calls bank4 `BD8E`:

1. Read one of four original reward-table pointers at fixed `FF99`, selected by `06D4`.
2. Index by `(opponent-1)*2 + ((Period>>1)&1)` after the result has advanced the period.
3. Add that exact item to the first free inventory slot. Do not draw a new random reward or offer an invented reward picker.
4. Present the award, then on its source timer/input condition restore celebration character aliases and return to Scene 2 phase 0/map.

The four tables contain 14 entries each and have duplicates. Their union omits IDs **21 and 24**. This only establishes absence from these four ordinary reward tables; it does not prove absence from every original special/password path. Final Period-28 completion goes directly to the ending instead of this ordinary award phase.

The first minimum slice must preserve the distinction between awarding an item and equipping it. A victory alone does not equip its reward.

## Passwords, failure and ending

The displayed six-character code packs three bytes from six hexadecimal-like digits. Bank1 `BF52` decodes:

- first byte: mode low bit and win mask `06D2`;
- second byte: win mask `06D3`;
- third byte: reward-set nibble and checksum nibble;
- checksum: low nibble of the sum of both masks, reward selector and mode;
- period: twice the number of set win bits;
- reject completed all-opponent masks and invalid high mask bits; the count of first-mask wins must equal the second-mask count or exceed it by one.

Bank3 `A8C1` rebuilds the earned inventory from win masks and the preserved reward table. The password does **not** encode exact equipped choices or current inventory order; the normal restored menu has unequipped characters. Server save/reconnect should retain the actual typed inventory/equipment state, while any original-password import must follow the original reset/rebuild semantics.

The normal reward selector generator only emits 0–3, while the raw password decoder reads a nibble and does not visibly validate it against four table rows. Inputs outside the normal generated selector domain require further source-bound analysis; do not invent a clamp or silently index another source region.

A retained original natural loss already exists in `match-clock-complete`: Mode0, score12–61, result entry rolling Period2 back to0, then Scene7 phase11. The new continuation capture reuses the identical 24,000-frame prefix and its exact final screenshot, then presses A on the original default “yes” option. It returns to the campaign map, Period0, with the same empty inventory/progress. This is verified ordinary input, not a forced scene transition.

Scene7 dispatch `B995` sends a non-Period28 result to `BE39`, the defeat/continue screen. Its yes branch returns Scene2/phase0; no invokes `F79D`, the original return/restart routine. Period28 follows ending phases 0–10. The ending movie, final return behavior and a naturally completed fourteen-win run remain outstanding; existing `NeedsCampaignEnding` metadata is not an implementation of them.

## Normal tactic menu: five choices, not six

`B4B9` wraps its cursor over 0–4 and writes `profile=cursor+1` at `B4FE`. Normal captured selection cycles 4→5→1→2→3→4. The sixth engine table row/profile0 is not an extra menu choice, even though profiles0 and1 share ordinary AI tables.

| Profile | Original visible name | Source name pointer |
|---:|---|---|
| 1 | ガンガンせめろ！ | bank3 B733 |
| 2 | マークしろ！ | bank3 B73C |
| 3 | みせるプレーだ！ | bank3 B745 |
| 4 | おまかせ | bank3 B74E |
| 5 | ビシビシぼうぎょ！ | bank3 B757 |

Original default is **4**. The menu appears after roster preparation when a team contains a human and computer teammate. The natural two-human cooperative opening bypasses the irrelevant teammate tactic selection. Root's engine coverage of profile0 remains useful alias/domain evidence, but the platform should not label it as a sixth original tactic. Chinese labels, if later desired, must be marked as editorial translations rather than original official names.

## Minimal original end-to-end evidence now available

All runs and exact snapshots are indexed in `.local/basketball/rea/campaign-audit/verified-selection.json`.

1. **New single-player story to live first match:** normal mode menu → 1P → New York map selection → roster preparation → tactic confirmation → travel/court opening → actual Scene1/phase129, Mode0, Period0, Court1. Complete controller timeline and screenshots retained; no match timer or outcome was edited.
2. **Two-player cooperative story to live first match:** original 2P selection yields Mode1 and controller assignments `[0,1,FF,FF]`, then enters the same first match without artificial AI replacement.
3. **Legal password restoration → inventory → equip → actual home match:** normal input enters `020010`; original decoding yields Period2, masks `[1,0]`, reward set0, inventory item1 (`ロケットシャツ`). Normal equipment input equips it to character0's shirt slot and removes it from inventory. After preparation, the original enters Court0 against the retained opponent/team1 with the equipped item still stored. This demonstrates restoration/selection/initialization, not a naturally earned first win.
4. **Normal unequip:** from the same equipped menu, choose remove and confirm the shirt; item1 returns to inventory and the equipped slot becomes FF.
5. **Owned-item replacement:** legal password `020120` restores masks `[1,1]`, Period4 and inventory `[0,1]`. Normal equip takes item0 and compacts inventory; a subsequent selection replaces it with item1 and returns item0 to the inventory. Both transitions retain source snapshots.
6. **Natural defeat → continue:** existing 24,000-frame loss and screenshot match byte-for-byte; appended ordinary confirmation returns to the campaign map.

Short earlier exploratory presses that missed the original menu's input timing remain in their original captures/logs. Successful timelines use later explicit presses; no RAM, selected ID, clock or win state was modified to make them pass.

## Proposed recovery order and acceptance gates

1. **Typed campaign preparation and progress contract.** Implement pure source-derived state for mode0/1, period, win masks, reward set, inventory and character-indexed equipment. Tests first: original map legality, away/home opponent/court separation, inventory/category ownership, profile1–5/default4, controller mapping, and save/reconnect. Keep it inside basketball; use the platform's existing waiting-room preparation framework.
2. **First equipped match path.** Reproduce the now-recorded `020010` restoration → item1 equip → home Court0 entry with exact source snapshots and Go/Wasm/browser parity. This is a small real cross-menu-to-match path; merely accepting an arbitrary equipment array does not pass it. Do not reopen roster combinations without their own gameplay evidence.
3. **Post-victory reward and next-map handoff.** Restore the original `BD8E` semantic writes, timer/input exit and display request, using recorded/isolated original reward calls plus an actual source win trajectory. Verify exact reward set/index, no new RNG draw for the item, inventory insertion, aliases, next opponent availability, next period/difficulty and state restoration. Do not just instantiate a fresh runtime with a new seed.
4. **Loss continue/quit and campaign persistence.** Reproduce the retained natural loss/continue route. Verify Period rollback, prior masks/items/equipment retained, map choices, fresh match scores, and no accidental character/equipment reset. Keep original-password import distinct from full server save restoration.
5. **Equipment completeness and later-round AI domains.** Extend source-driven acceptance to every normal category, all actual award entries, swap/unequip/cancel/change-character/intermission, and the specific equipment effects already identified by the existing engine. Late periods change AI difficulty; first-match matrices cannot establish those bands.
6. **Final victory / ending.** Restore original Scene7 phases0–10 and their return destination, with a source-backed Period28 transition and then an integrated complete campaign. Only after the components and intermediate transitions work should a long full original campaign be recorded; do not begin by running fourteen matches to discover basic menu semantics.

Platform decisions to carry forward are story versus isolated match, one or two cooperative human seats, legal remaining opponent, character/owned-equipment choices, the five real tactics, next-stage preparation and defeat continue/quit. The original provides no arbitrary campaign opponent unlock rule, invented purchase system, random reward picker, or free equipment-number editor.

## Evidence and reproducibility

```sh
source scripts/env.sh
.local/monopoly/venv/bin/python scripts/basketball/audit_campaign_selection.py
# Only if matching declared captures are absent:
.local/monopoly/venv/bin/python scripts/basketball/audit_campaign_selection.py --capture
```

The committed `campaign_selection_scenarios.json` contains exact controller-only timelines. The audit validates ROM identity, captured states, screenshot and RAM hashes, actual selected tactic IDs, equip/remove/swap, home-court handoff and identical loss-reference prefix. It writes evidence under `.local` only; it does not alter engine/testdata or UI.

New REA evidence:

- Bank3 campaign/map/equipment/tactic batch: `ev_c779850b25af5ea09638e3a7932acad486df46837988564e2e058cdd7d6a8b4c`.
- Follow-up preparation/ending/name-rendering batch: `ev_37af32e9b034716efaee6f2adaafab2a2553752baa05f1e2889edf15118919bd`.

Both complete bundles and snapshots are under `.local/basketball/rea/campaign-audit/`. Existing result-transition and opening-attribute evidence was reused. Decompiler output around rendering waits contains unreachable-block/infinite-loop warnings; original instruction checks and normal controller captures constrain those conclusions. Static code and a normal menu opening do not prove full reward, ending, all-equipment or complete-campaign gameplay acceptance.
