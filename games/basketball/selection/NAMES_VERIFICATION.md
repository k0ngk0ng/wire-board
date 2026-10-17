# Original preparation names and selection relationships

`names.json` is independent **label evidence**, not a gameplay allowlist. It does
not modify `catalog.json`, server validation, preparation UI, or release status.
Every Chinese string is a proposed editorial translation; no official Chinese
localization is claimed. Original spelling is retained separately.

## Names recovered

The selected ROM is SHA-256
`0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`.
Names were transcribed from original pixels and bound to existing original IDs,
not inferred from scenery, uniforms, popular guidebooks, or array position.

| ID | Original team label | Proposed Chinese label | Original character slots 0 / 1 / 2 / 3 |
| --- | --- | --- | --- |
| 0 | ねっけつ | 热血 | くにお / りき / ごだい / ジョニー |
| 1 | ニューヨーク | 纽约 | マイケル / エディ / セラリ / ルード |
| 2 | サンフランシスコ | 旧金山 | ミック / ショーン / リック / ライン |
| 3 | テキサス | 得克萨斯 | スナイプ / ホーク / ジャガー / ジミー |
| 4 | US NAVY | 美国海军 | アッシュ / ウルフ / トライブ / ブレイク |
| 5 | ハワイ | 夏威夷 | きらうえ / へっど / わいいき / ぼーん |
| 6 | ラスベガス | 拉斯维加斯 | スロット / ルーク / ポーカー / ギャモン |
| 7 | フロリダ | 佛罗里达 | ピエール / ホフマン / エスパ / グリース |

These are the original short team labels, not a claim to have recovered longer
formal team names. Character labels likewise preserve the original four-glyph
HUD/menu spelling. For example, Hawaii slot 2 is literally **わいいき**, bytes
`7f 34 34 3d`; silently changing it to わいきき would contradict the source.
Navy slot 2 begins with unvoiced ト: **トライブ**, not ドライブ.

Court IDs 1–7 have the same location labels as teams 1–7. Court 0 is different:
**ナイアガラ / 尼亚加拉**, while team 0 is ねっけつ / 热血. No team-to-court
constraint follows from a shared number or label.

## Independent source paths

1. Original bank 4 `$B24E` points to eight four-entry character-name tables. Each
   entry addresses a fixed four-byte label, padded with zero; these are **not**
   NUL-terminated or `$FF`-terminated strings. The exporter records each address
   and the actual non-padding bytes in `names.json`.
2. Original bank 3 `$B6A7` points to the eight 13-glyph team labels. The display
   decoder uses fixed `$C13F/$C03F/$C19B` to separate voiced/semi-voiced marks.
   The existing `extract_display_profiles.py` and `display_profiles.go` had
   already independently recovered those display inputs.
3. Original CHR pages 96–99 supply the glyph pixels. The source-bound existing
   `hud-glyphs.png` renders these bytes into
   `.local/basketball/selection-names/verified-labels.png` for visual review.
   A consistency check rejects assigning two different kana to the same source
   byte across the 32 transcriptions; it caught an initially misread Hawaii name.
4. **Natural menus corroborate all 32 names.** Eight normal controller routes
   select each team, enter a court, and show its four-member selection screen.
   No memory writes, artificial roster changes, or external names are involved.
   Each screenshot and trace is linked and hashed in `names.json`.
5. Eight further normal routes select every court with the **same team pair
   `[0,4]`** and read committed court `$058B` only after confirmation. The
   original stage-menu text appears in the pre-confirmation screenshots.

Original menu order differs from ID order:

- Team grid, rightward: `[0,4,2,3,1,7,6,5]`.
- Stage map, rightward: `[0,1,4,7,3,5,2,6]`.
- The member screen places character slots 0/1 in the left column and 2/3 in
  the right column. Display location is not platform seat identity.

`capture_selection_names.py` records these routes in `natural-menu.json`.
The initial menu savestate is identified by SHA-256 there. It was originally
created by the ordinary boot timeline in
`.local/nes-basket/runs/navigation-1/scenario.json` (Start at frames 400 and 600,
A at 800, then save the mode menu after frame 1200). It contains no patched RAM.
The user-supplied ROM, FCEUX, and existing workspace capture setup are required.
This is a reconstruction research tool, not a shipped emulator dependency.

## Selection constraints observed

`probe_selection_constraints.py` executes **all 56 ordered distinct team pairs**
in the original one-human versus menu. While selecting the second team, the
cursor skips the first team's entry. The final `$0586/$0587` values cover
exactly `{(a,b) | 0 <= a,b < 8 and a != b}`. Thus the original observed menu
permits any distinct pair, and does not offer same-team mirror matches.
The earlier focused leftward attempt also skipped the already-selected team.

For **each of eight teams**, the member collision probe selects character 0,
then tries to move the partner cursor from 1 upward to occupied 0. The original
menu refuses that movement: screenshots before/after are pixel-identical.
A following rightward input moves to unoccupied character 3 and produces a
changed screenshot. This establishes duplicate prevention for that concrete
selection route across all eight teams; it is not an exhaustive proof of every
input/character/multiplayer-menu permutation. A separate normal route selected
Riki and Johnny (slots 1 and 3) together successfully.

The eight stage routes prove court choice is independent from the particular
`[0,4]` team pair. They do not constitute all `56 * 8` combinations, complete
matches, campaign progression, secret-code selection, or rules for 2/3/4-human
original menus. Those remain separate verification scopes.

## Reproduce and inspect

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_selection_names.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/probe_selection_constraints.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/export_selection_names.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_selection_names.py
```

Captures are reused only when their complete marker and exact recorded scenario
match. Each original run retains `scenario.json`, `source.json`, `hashes.json`,
RAM, PNGs and the capture log under `.local/nes-basket/runs/selection-names-*`.
The summary files are in `.local/basketball/selection-names/`:

- `natural-menu.json`: eight courts and eight teams, source state identity.
- `team-pairs.json`: all 56 ordered pairs, final IDs and original run paths.
- `member-collisions.json`: eight duplicate attempts and movement controls.
- `team-menu.png`, `courts-menu.png`, `verified-labels.png`: visual review sheets.
- `verification.json`: current metadata/source checks and repeat-export digest.

All source/translation data is separate from the existing 72 preparation image
assets. Images may now be labeled from these proven IDs, but a verified name
never makes an unverified character action or court playable. No preparation
options were widened and no assets were uploaded or published by this work.
