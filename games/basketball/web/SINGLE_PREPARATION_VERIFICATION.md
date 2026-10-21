# Complete original single-match preparation

The waiting-room preparation now offers all eight source courts, all eight
teams and four source characters per team. Teams use labelled selectors and
may not match the opponent. Source PNG court previews, portraits and selected
body images remain visible; changing the authoritative team refreshes images
without changing legal character indices. No numeric badges are added to art.

Only the participating host edits courts/teams. Player-character controls retain
the existing host/own-seat permission model and protect a human teammate's
choice. Team requests have the shape `{type:"team",side:0|1,team:0..7}`. The
server owns acceptance and authoritative options; the component does not
optimistically rewrite the roster or invent original menu restrictions.

Source scope is documented in `selection/NAMES_VERIFICATION.md` and
`scripts/basketball/probe_selection_constraints.py`: 56 ordered different-team
pairs, eight courts and four character choices. This UI change removes the old
partial-development gate; it does not treat asset extraction alone as rules
verification.

## Repeatable acceptance

```sh
source scripts/env.sh
python3 scripts/basketball/accept_single_preparation.py
```

The isolated React browser test was written first and failed on the missing
eighth court before implementation. It now passes 17 checks, including all 56
team pairs, host/guest/spectator/busy permissions, fourth-character and eighth-
court native mouse actions, native mobile touch, all source PNG fallback loads,
320px width, minimum 44px targets and computed text contrast of at least 4.5:1.
React updates and source-image effects are allowed two animation frames before
the 56-pair imagery assertions.

Artifacts: `.local/single-preparation/result.json`, `contrast.json`,
`desktop.png` and `mobile-320.png`. Both screenshots were visually inspected.
The harness records requests; it does not simulate server authorization or
claim a complete match. The host's separate HTTP/rule tests cover acceptance.
The original shared waiting-room framework is unchanged.
