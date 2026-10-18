# Original campaign settlement data oracle

This task adds source fixtures and a reproducible generator only. It does not implement settlement, advance a runtime, or recapture an entire original campaign. The parent integration must consume an already confirmed RuntimeOutcome and preserve its current RNG.

## Scope and source boundaries

The supplied ROM SHA-256 remains `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`.

The original first result entry is reused from `engine/testdata/match-transition.ram.gz`, record8, original emulator frame25593. This is the same retained natural match result already used by the existing transition tests. Existing `campaign/testdata/reward-calls.json.gz` supplies the112 independently verified reward lookups; the new award cases check their selected item against those records.

Existing REA evidence was reused:

- `BA40` result and `BD8E` continuation: `ev_7945a86e38a813fa472f9a63045cf673ae9f0c3702f1a79b6c8a17e20eebeefb`, `.local/basketball/rea/match-transition/`.
- `BE39` defeat/continue selection: `ev_37af32e9b034716efaee6f2adaafab2a2553752baa05f1e2889edf15118919bd`, `.local/basketball/rea/campaign-audit/followup-analysis/`.

`verify_campaign_settlement.py` executes unchanged original instructions under the existing py65 mapper. It explicitly omits C000 sound requests, C021 palette waits, C024 background uploads, C036 message uploads and BCF0 result-score glyph uploads/NMI waits. Every omitted call is listed in each phase record. Result calls stop at BC96 before celebration actors, reward calls at BE63 before award actors, Continue at BF35 before rendering, and quit at F79D before machine reset.

These are controlled semantic calls. Full raw RAM is retained for source inspection, but this is not PPU/audio/NMI timing acceptance or a complete celebration-scene comparison. Initial scores, win masks, inventory and equipment are declared controlled inputs; they are not presented as naturally earned histories.

## Fixture contract

- `engine/testdata/campaign-settlement-cases.json.gz`: JSON object containing `cases`, `choices`, `naturalLossContinue`.
- `engine/testdata/campaign-settlement-source.json`: source identities, ranges, counts, skipped presentation boundaries and fixture hash.

All720 main cases use `kind: "settlement"`:

```text
label
startingProgress                 original even-period preparation before this match
confirmedProgress                result has applied period/win changes; reward not inserted yet
opponent, won
nextStage                        "prepare" | "defeat" | "complete"
rewardItem                       original offered/displayed reward, or -1 for no reward phase
rewardStored                     whether inventory actually received the offered item
inventoryShape, capacityScope
before                           raw result-entry snapshot; not necessarily leaf-valid progress
confirmed                        confirmed result snapshot
 after.progress / after.random   settlement output before award presentation ticks
phases[]                         result-entry, result-confirm, optional reward-entry
enteredRepeat                    separate original already-entered award observation
```

Each phase has `before`, `after`, `rawBefore`/`rawAfter` (base64-encoded2048 bytes; Go can receive `[]byte`), original entry/stop addresses, exact relevant RAM writes, inventory/equipment writes, RNG call count and omitted presentation calls. Existing `transitionRAM` can inspect the raw records; the pure settlement comparisons should use the explicit progress snapshots.

`startingProgress`, `confirmedProgress` and `after.progress` are deliberately separate. Immediately before BA40 first entry, the match has already incremented Period by two but has not updated its win masks or loss rollback. That intermediate `before.progress` must not be passed off as a leaf-valid campaign preparation snapshot.

For a first away win against opponent1/rewardSet0:

- starting: Period0, masks[0,0], empty inventory;
- confirmed: Period2, masks[1,0], inventory still empty;
- after: item1 at inventory index0, everything else preserved;
- `nextStage="prepare"`, `rewardItem=1`, `rewardStored=true`;
- `after.random == confirmed.random`.

## Original behaviors established

The720 complete data flows comprise560 ordinary wins,128 defeats and32 final wins. They cover original Mode0/1, every reward selector0..3, every opponent1..7 for ordinary awards, both away/home reward columns, starting periods0/2 and late-period24/26 boundaries, duplicate items, existing per-character equipment, one remaining inventory slot and full inventory.

### Ordinary victory

BA40 first entry records the victory in the correct mask after the existing half-period advance. The confirmation branch selects Scene1/phase7. On the first BD8E entry:

- phase7 becomes entered;
- FF99's selected reward table is indexed by `(opponent-1)*2 + ((Period>>1)&1)`;
- the item goes into the first negative/empty inventory cell;
- duplicate items are not deduplicated;
- existing equipment, reward selector and other inventory cells are retained;
- this award entry makes **zero original F9C3 RNG calls**.

Existing occupied equipment does not absorb the reward, and the reward does not automatically equip itself.

### Full inventory

The original loop is explicit: BDC1 initializes Y=0; BDC3 inspects D3+Y; BDC8 increments Y; BDC9 compares with16; BDCD branches to BDD4 when the sixteen cells are exhausted. It does not scan past16, overwrite an old item, undo the victory, or reset inventory.

All112 full-inventory ordinary-win cases show **no inventory writes**, no writes into adjacent E3..F4, and unchanged inventory. The original still computes the offered item and enters the award presentation. Therefore `rewardItem` retains that item while `rewardStored=false`. This is an abnormal-capacity controlled domain, not a claim that an ordinary thirteen-award campaign naturally fills all16 slots. A parent result type may expose the difference, but must not silently claim the item was stored.

### Already-entered award

Every ordinary-win case includes a separate subsequent BD8E call with its entered bit intact. None inserts another item. That call can advance presentation RNG/timer; it is **not** the pure settlement output in `after`.

The platform's repeatable pure result operation should derive from the immutable confirmed runtime/start snapshot each time. It must not rerun BA40, increment Period twice, OR another win, append to its own prior return value, or consume the award animation's RNG. Identical repeated calls should yield the same result and leave the RuntimeState unchanged.

### Final victory

Starting at Period26, original BA40 writes the last win, reaches Period28 and confirms Scene7/phase0. No BD8E entry occurs. Inventory, equipment and reward selector remain unchanged; `rewardItem=-1`, `rewardStored=false`, `nextStage="complete"`.

A standalone table lookup for post-period28 exists in the leaf oracle because BD8E itself can compute it. That does not authorize an extra award: the original result dispatcher never takes that reward branch for final completion.

### Defeat

BA40 first entry subtracts two from the already advanced Period, restoring the starting period. It does not set a new win bit. Confirmation selects Scene7/phase0. Every defeat case returns exactly the starting Progress, including ordered duplicate inventory, all character equipment and rewardSet. `rewardItem=-1`, `nextStage="defeat"`.

The pure wrapper must use the live confirmed period/win masks, rather than applying this original rollback a second time.

### Continue and quit

Six controlled BE39 cases start from the retained original defeat menu with normal empty, equipped and synthetic-full inventories:

- Continue confirms the original yes option, clears the source's three camera bytes, and sets Scene2/phase0. Progress is unchanged; no reward is added and no RNG is drawn in this scoped branch.
- Quit uses original right input followed by confirmation in the same source call. It reaches F79D, the machine-reset entry. Progress has not changed **before that reset entry**. The oracle deliberately stops there; it does not claim the eventual reset/new-game path retains campaign state.

`naturalLossContinue` separately reuses the actual controller-only capture at videos24000 and26400. Its first24,000-frame RAM prefix and screenshot had already been verified identical to `match-clock-complete`. This natural evidence shows defeat→Continue returning to the map with Period0 and unchanged empty progress. It does not claim a naturally played full-inventory or equipped defeat.

## Tests-first use by the parent

Expected parent failure scenarios are: applying results before confirmation; ignoring the immutable starting Progress; awarding twice; wrong away/home reward parity; deduplicating inventory; overwriting existing equipment; treating full inventory as successful storage; rewarding final completion; rolling a loss back twice; reseeding or consuming RNG; using celebration character aliases as next-match roster; and assuming quit's pre-reset snapshot is a completed reset result.

The full Runtime wrapper and actual equipped match→reward→next-opponent flow belong to the parent. These fixtures establish source data transformations and branch destinations; they do not by themselves prove the next runtime or the complete campaign.

## Reproduction

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_campaign_settlement.py
# Re-execute the same original calls and compare without rewriting fixtures:
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_campaign_settlement.py --check
```

The second command is suitable while another agent reads the fixtures. It checks exact compressed output equality, original reward-loop write bounds, original equipment preservation and consistency with the existing112 reward oracle. No full original-machine run is started.
