# Fullscreen chat lifecycle acceptance

Failure scenarios recorded before implementation:

- The shared chat entry or composer stays outside the native fullscreen target,
  or inherits `inert` after moving into it.
- Including chat accidentally expands fullscreen to room headings, player bars
  or platform menus.
- Replacing a portal target remounts the composer, losing its draft, unread IDs,
  message scroll position, retry nonce or pending-send guard.
- Entering or exiting fullscreen mounts a second chat, repeats a send, marks an
  unread message seen, or loses messages received while the panel is closed.
- Page fallback differs from native fullscreen, or leaving a room leaves inert
  elements, scroll locking, a detached chat host or a stale fullscreen target.
- Typing in chat leaks game keydown events; suppressing all keyup events leaves
  keys held before focusing chat stuck down.
- At 320px the entry or composer is clipped or covered; normal mobile chat stops
  following its existing document-flow layout after exiting fullscreen.
- A sender's player-page button opens its dialog outside fullscreen, below the
  chat panel, or closing the player page loses the chat draft or current input.

Run after installing the existing web dependencies:

```sh
python3 -B scripts/fullscreen/check-chat.py --out .local/fullscreen-chat/<fresh-directory>
```

This browser harness exercises the production Chat and GameFullscreen components
with controlled incoming messages and delayed sends. It saves source hashes,
measurements and screenshots, and removes only its own browser profile, bundle
and socket files. The game HTTP acceptance separately verifies real chat delivery.
