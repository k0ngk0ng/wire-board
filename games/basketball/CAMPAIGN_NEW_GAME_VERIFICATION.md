# Original new-campaign factory

## Failure scenarios recorded before implementation

- Using an independent random reward selection, the old register's low bits, or a fixed selector instead of the returned original random byte masked with3.
- Consuming both the outer CABC draw and the inner A205 draw inside a factory whose input is explicitly before A205; consuming zero or more than one draw.
- Making Mode0/1 affect the reward draw, or accepting versus/unsupported modes.
- Returning an advanced random register or partially created preparation after invalid input.
- Initializing inventory/equipment to item0 instead of Empty/FF, or retaining previous period/win masks.
- Treating the observed boot seed31313131 as universal, fabricating reset-time RAM, or implicitly invoking the factory from match construction, restore or defeat Continue.
- Confusing a password restoration with creation: password data owns its restored RewardSet and must not be overwritten by this factory.
- Losing ordered empty inventory, character equipment or the returned RNG across JSON save/restore; treating composed source phases as a naturally played continuous match.

## API and exact boundary

`NewOriginalCampaign(mode, randomBeforeRewardDraw)` returns `CampaignStart{Progress, Random}`. It accepts only story Mode0/1, consumes exactly one existing `NextRandom` call, uses its returned byte masked with3, and delegates empty preparation to the already verified leaf `campaign.New`.

The random argument is **immediately before A205's inner reward draw**, after any caller-owned CABC outer-loop draw. Hardware/reset seeding and unknown bootstrap RAM are outside this API. No observed seed is hardcoded. The output random register is the post-draw register, so subsequent original roster selection continues the same stream.

Unsupported modes return an error, unchanged Random and no created Progress. The factory has no hidden mutable state. It is an explicit new-game operation, not a restore, Continue, player-count change, match constructor or ordinary render hook. No existing runtime, server, UI or preparation code invokes it implicitly.

## Source evidence reused

The original investigation is preserved in `.local/basketball/rea/campaign-new-game/calls.json`, SHA-256 `08836c0a95cf13a9319f65506efd721f57b3bab34adfc360db67773ac7342c03`.

- Original C9C6/A08C clearing/initialization establishes Period0, zero win masks and FF/-1 in all inventory/equipment slots.
- 32 actual unentered A1F8 calls cover sixteen returned-byte values across0..255 and two prior reward selectors. Each makes one original F9C3 call and writes the returned byte&3. The same corpus separately records32 entered-gate calls making no new reward draw.
- Original1P/2P confirmation supplies Mode0/1 independently of the reward draw.

`verify_campaign_new_game.py` composes those independent source phases into **64 typed expected cases**. It does not use Go's RNG to generate expected results. These are not64 naturally recorded complete menu sequences.

The retained natural video boundary615→616 independently establishes:

```text
outer-loop input 61834046
CABC outer draw -> B0C1A023
A205 factory input B0C1A023 -> 5860D011, RewardSet0
```

The factory test starts from B0C1A023 and verifies both the full output register and the original observed fresh progress. Checking only RewardSet would miss an extra draw when successive masked values happen to match.

Existing REA evidence is `ev_37af32e9b034716efaee6f2adaafab2a2553752baa05f1e2889edf15118919bd` and `ev_efaf6f5b71e4d47395c79ea8a9a16de695c11c8f081322e086bd9b7b108fdc1c`. The legacy navigation capture lacks the newer source.json manifest; its scenario/RAM and visible PRG pages were validated and that limitation remains in the fixture metadata. No new whole-machine capture was performed.

## Tests-first acceptance

The new tests and source fixture were written before `campaign_new_game.go`; the initial undefined-API compilation failure remains in `.local/campaign-new-game-factory/before.log`.

Coverage includes all four reward selectors, both normal story modes, exact source random outputs, valid empty progress, deterministic immutable inputs, JSON round trips and24 unsupported-mode/random combinations. Mode0/1 are independent of the reward choice. Invalid input does not return an advanced random register or a usable partial campaign.

Reproduction from the isolated worktree:

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_campaign_new_game.py --check
go test ./games/basketball/engine -run '^TestNewOriginalCampaign' -count=1
go test -race ./games/basketball/engine -run '^TestNewOriginalCampaign' -count=1
GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run '^TestNewOriginalCampaign' -count=1
```

The exporter can regenerate the committed `engine/testdata/campaign-new-game-*` fixture without re-running the original machine. `--check` verifies compressed-byte equality without rewriting it. Platform new-game UI, preparation integration, match startup and full campaign completion remain separate work; this factory does not claim them complete.

Native, race and Wasm targeted checks passed. Exact source/fixture/log hashes and result index: worktree `.local/campaign-new-game-factory/result.json`.
