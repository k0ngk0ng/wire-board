# Fishing on CATAN — implementation record

## Scope and evidence

Rules target the 2025 Traders & Barbarians edition, pp. 9–10, and its
five/six-player extension, p. 5. Component counts were checked against rendered
printed pages, not inferred from extraction order. Official FAQ questions 43–52
were also checked. Pinned references and data are in
`catan-fishing-sources.json`; original PDF hashes are in
`../board-expansion-rule-sources.json`.

This is **internal token economy only**. Fishing is not attached to a room,
map, production pipeline, legal player actions or UI and is not playable yet.

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

## Integration still required

- Source-based fishing-ground/frame positions (six base, eight extended),
  inner lake placement and the distinct lake production faces. Existing
  generic port positions cannot simply be assumed to match the printed frame.
- Initial settlement token, ordinary production, robber blocking of lakes,
  lake/coast claim aggregation, pending response and production continuation.
  The token helper deliberately has no global turn serial; integration must
  prevent a second replacement claim in the same production.
- Five separately paid actions (2/3/4/5/7 fish), legal timing, deck/bank/piece
  shortages, atomic payment/effect, public-VP boot passing and per-player
  victory thresholds. Timing of spending before production remains to verify.
- Three/six-player turn handling, paired second action, bots, 120-second
  responses, true service restart, private views and combinations. Fish do not
  count as resources for seven/robber/trading or Aqueduct compensation.
- Original assets, room options, desktop/mobile UI and complete-game QA.
  No public entry, push or deployment until these acceptance requirements pass.
