# Game-area fullscreen acceptance

Regression scenarios defined before the area-only correction:

- Waiting rooms have no fullscreen button. The control appears only after the
  game loads, grouped with autoplay in the room tools. The game surface has an
  upper-right exit control only while fullscreen is active.
- Native fullscreenElement is the game surface. It never contains the room
  heading, platform menu, public player strip, invitations or room management.
- Page fallback covers the viewport and makes the outside page inert; exit,
  room departure and late requests restore focus, scrolling and inert states.
- Game widgets, choices, dialogs, results and animation portals remain within
  the game surface. Fullscreen/resize must not remount or restart the board.
- All 25 opening boards have desktop and 320px evidence. Bubbleparty and tank
  play are checked with native input; the tank expands to a 2:1 view.
- Poker controls and the timer text remain reachable, within the viewport and
  without overlap. The timer uses at least 4.5:1 computed text contrast.
- Escape exits fullscreen and continues to reach game cancellation handlers.
- Sound controls are outside game fullscreen; leaving fullscreen restores them.

Run `npm --prefix web run build`, followed by:

```sh
python3 -B scripts/fullscreen/check-browser.py --out .local/fullscreen/<fresh-directory>
```

Screenshots, measurements and source/build digests are retained. Profiles,
SQLite fixtures and temporary files are removed. These are game-area/fullscreen
checks, not an exhaustive full-match or rule-combination matrix.
