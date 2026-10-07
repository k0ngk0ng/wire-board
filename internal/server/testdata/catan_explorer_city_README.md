# Private Explorer + Cities & Knights HTTP fixtures

These are internal acceptance states, not public room recipes. Each snapshot
uses the complete private combination constructor for the full three-mission
scenario with 3 or 6 players, followed by a legal completed opening. Lair
numbers are explicitly synthetic test components: `3,4,5,9,10,11`, plus `3,4`
for six players. They must never be taken as verified retail components.

- `catan_explorer_city_roll_{3,6}.json`: just after legal setup, before the
  first production roll. No resource, score or progress injection. Natural
  HTTP matches use these states through actual random production and actions.
- `catan_explorer_city_action_{3,6}.json`: separately generated legal setup
  and real production, then a controlled funded action state. The actor
  receives four of each of the eight bank cards; others receive one of each.
  Progress cards 7, 10, 18 and 20 are moved from the decks to the actor, and
  cards 2 and 13 to the next seat. No fake pending response, score or knight is
  inserted. These states test explicit response, replay, clock and departure
  paths, not natural game economy.

Snapshots are validated in the game package and loaded through production
JSON restoration. The server tests register, join and ready real clients,
then install these private states before using normal authentication, HTTP,
WebSocket, persistence, response timeout and autoplay code. This bypasses
only the still-gated room recipe; it does not prove the public option works.

To regenerate deliberately, from the repository root:

```sh
. scripts/env.sh
WIRE_BOARD_UPDATE_EXPLORER_CITY_FIXTURES=1 go test ./internal/game \
  -run '^TestCatanExplorerCityHTTPFixtures$' -count=1
```

Regeneration changes the random layout/dice. Normal test runs only read and
validate the committed snapshots. The opening-path search chooses an actual
completable legal path; these fixtures do not resolve the independently
tracked city-first/last-harbor setup deadlock for arbitrary human choices.
