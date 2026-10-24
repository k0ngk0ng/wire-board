# Fullscreen acceptance

The initial scenarios were written before implementation. The playing-room and
native keyboard/touch checks extend them following the requested playability review.

- Every registered game: native entry, button exit, Escape exit, browser-driven exit.
- Every registered game: actual opening at desktop and 320px; gameplay components
  remain mounted and game controls remain inside the fullscreen document.
- Keep the complete game UI: boards/canvases, action panels, choices and dialogs.
  This does not run unrelated rule combinations or claim complete-match coverage.
- Unsupported/rejected native fullscreen: page fallback with button/Escape exit.
- Room departure and late native request completion: no stale fullscreen state.
- Tank desktop: approximately 2:1 viewport, wider horizontal camera, original size
  on exit; native keyboard angle/weapon changes and an authoritative fired shot.
- Tank portrait/landscape and compact poker: native touch entry/exit; no control
  overlap in the poker toolbar. No physical iOS verification is claimed.
- Escape continues reaching game handlers so held input can be cancelled.

Run `npm --prefix web run build`, then:

```sh
python3 -B scripts/fullscreen/check-browser.py --out .local/fullscreen/<fresh-directory>
```

`--games` selects comma-separated game IDs for the opening checks; `--tank-only`
skips that inventory. Both still include the tank, poker and lifecycle checks.
The script saves source/build digests, measurements, control inventories and PNGs,
and cleans its own SQLite database, browser profile and temporary directories.
