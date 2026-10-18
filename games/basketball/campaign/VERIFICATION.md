# Campaign preparation component — tests-first ledger

## Failure scenarios before implementation

- Password nibble parsing, checksum, mode bit, both win masks or half-period reconstruction differ from original BF52. Invalid codes must not produce a usable partial progress state. Source-generated reward selectors are 0..3; broader raw password nibble aliases remain unsupported, never clamped.
- Original encoder uses the current mode/masks/reward set, not inventory order or equipped choices. Password restoration must rebuild inventory in A8C1 order and reset equipment, rather than round-trip an equipment snapshot through a code that cannot contain it.
- Map selection confuses display order [1,4,7,3,5,2,6] with team number; skips the wrong win mask; permits defeated opponents; changes opponent when home court becomes Niagara/0; omits half-period parity.
- Reward selection uses the wrong post-win half-period parity, wrong table, or fresh RNG. This package has no RNG or match-result implementation.
- Equipment is indexed by Nekketsu character, not platform seat. Item0..9/10..20/21..31 must enter shirt/pants/shoes respectively. Equip can only take an actually owned inventory index; it must compact, preserve duplicates, and swap the old item into the selected inventory cell.
- Empty item, out-of-range item/index/character/category, malformed packed inventory or wrong equipped category must not corrupt progress. Synthetic broad source probes must not be called normal-menu acceptance.
- Unequip removes the selected category and inserts into the first empty inventory cell. Establish actual full-inventory behavior by original execution before implementing any policy. Empty slot and cancellation must retain the source behavior.
- B cancels only an unconfirmed item-selection action; it does not undo earlier confirmed equipment transactions. Persisted JSON must preserve ordered inventory, character bindings and duplicate instances exactly.
- Normal menu evidence 020010→item1 equip→Court0/Period2, 020120 two-item replacement, removal and natural loss/continue must be checked from original RAM snapshots, not generated expected values from Go.
- Leaf package must import neither engine nor runtime/server/UI. No source masks, equipment or RNG may be silently reset to make a case pass.

## Recovered API and boundaries

This package imports only Go standard-library packages. It does not import engine, runtime, server or UI, and has no RNG implementation or random-reward selection.

- `New(mode, rewardSet)` initializes original story mode0/1 with empty inventory/equipment. The caller provides the reward selector already drawn by the original RNG path.
- `Progress.Validate()` checks mode, generated selector domain0..3, seven-bit win masks, source count ordering, derived period, packed inventory and correct equipment categories. It is structural validation, not proof that externally supplied item arrays were legitimately earned; persisted state and authorization remain the caller's responsibility.
- `DecodePassword`/`EncodePassword` cover all restorable source-generated progress codes. Decode reconstructs items in original A8C1 order (home-mask rewards, then away-mask rewards, each in opponent ID order) and leaves every character unequipped. Encode does not include inventory order or equipped choices. A full JSON save retains both.
- `AvailableOpponents` preserves original map order1,4,7,3,5,2,6 and uses the correct current mask. `CourtForOpponent` validates an available opponent, returns its city for away games or0 for home games, and never changes the selected team.
- `Equip` consumes an inventory index, derives the item's category, then compacts or swaps exactly as91DD. It never accepts an independent external item ID to create ownership.
- `Unequip` returns `UnequipStep{Progress, DiscardedItem}`. Empty slots are no-ops. Invalid API inputs return the unchanged progress. Original931D first clears the equipment slot; if all16 inventory cells are full, it loses the item. The component retains this source behavior and explicitly reports the discarded ID. This is a synthetic abnormal-state oracle, not a claim that ordinary campaign play produces that state.
- `RewardItem` exposes the fixed table lookup, including post-match period parity. It neither awards inventory nor changes wins/period. The engine must retain the original result-phase branch that sends final Period28 directly to the ending rather than awarding an extra item.
- There is no cancellation identity function. Canceling selection means the adapter submits no confirmed operation. Earlier confirmed equipment transactions remain intact, as in the original.

The raw source decoder accepts a checksummed selector4 code (`000044`); that would select adjacent pointer data outside the four normal reward tables. The supported API explicitly rejects this unproven alias domain. It does not clamp4 to3 or silently reinterpret it as a normal table.

A completed all-win password is rejected by the original decoder. The encoder API correspondingly exports only restorable codes; the full Progress type may still represent the completed period28 state for its caller.

## Source execution and tests

`verify_campaign_component.py` executes unchanged original instructions under the existing workspace py65 memory mapper:

| Oracle | Coverage |
|---|---:|
| bank1 BF52 + bank3 A8C1 password decode / inventory reconstruction | 51,472 valid generated codes |
| bank1 BECD..BF3B original encoder, stopping before the rendering/NMI wait | 51,472 matching encodes |
| Invalid checksum/count/mask/completed-password cases | 8 original rejected codes |
| bank3 91DD / 931D equipment confirmations, empty selections and B cancel | 884 calls |
| bank3 821A navigation and B29F home-court derivation | 54 map configurations |
| bank4 BD8E actual table lookup and inventory insertion before presentation | 112 calls |

The valid-code set is exhaustive over all seven-bit mask pairs whose win counts satisfy the original0/1 difference, both story modes, and all four normally generated reward selectors, excluding completed campaigns. It does not claim exhaustive malformed passwords or arbitrary adjacent-table behavior.

The equipment matrix covers all32 item categories, all four character indices, multiple inventory indices, duplicate items, empty/occupied destination slots, compaction/swap, empty removal, and0/1/15/16 occupied inventory cells. Twelve full-inventory removal cases establish the original discard. Those source inputs are controlled isolated configurations, not naturally acquired campaigns.

The map probe skips only display/text or sound entry points outside the semantic selection, then stops at the original render call. The equipment probe stops at its first post-write display entry. These boundaries are recorded in source metadata; no ROM instruction or intermediate gameplay state is patched during an executed call; each explicitly controlled starting state is kept separate from normal-menu evidence.

Tests were written before implementation. The initial undefined-API failure is retained in `.local/basketball/rea/campaign-component/before.log`.

## Original normal-menu data flow

The tests separately consume snapshots extracted from previously verified normal-controller FCEUX runs:

- `020010` restores period2, masks[1,0], rewardSet0 and inventory[1]. Equipping character0 consumes inventory item1; JSON restore preserves that equipment. Opponent1 then derives home Court0 and the full Progress equals the original live-match snapshot.
- Original remove returns item1 to inventory and empties the shirt slot.
- `020120` restores inventory[0,1]. First equip compacts to[1], then replacing it swaps to[0]. Every step matches original menu snapshots. Encoding then decoding restores earned inventory, not the equipped save state; this distinction is explicitly checked.
- The retained natural loss and appended ordinary Continue input have identical progress before/after. The leaf does not calculate the loss rollback or advance matches; those remain engine work.

These are source-derived preparation-to-match data-flow checks, not a claim that Runtime/server/UI has been integrated. The underlying original menu captures and screenshot/RAM hashes are indexed in `../CAMPAIGN_EQUIPMENT_AUDIT.md` and `.local/basketball/rea/campaign-audit/verified-selection.json`.

## Reproduction

```sh
source scripts/env.sh
.local/monopoly/venv/bin/python scripts/basketball/verify_campaign_component.py
go test ./games/basketball/campaign -count=1
go test -race ./games/basketball/campaign -count=1
GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/campaign -count=1
```

The fixture `testdata/source.json` binds ROM/base RAM identity, source boundaries and fixture hashes. Original normal-menu RAM traces are not bundled as enormous per-video fixtures; selected snapshots retain their source-run identity and SHA-256.

Native, race and Wasm targeted tests passed. `go list -deps` confirms the only non-standard package is this leaf itself. Current source hashes and result index: `.local/basketball/rea/campaign-component/result.json`.
