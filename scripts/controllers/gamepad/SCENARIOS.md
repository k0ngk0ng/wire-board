# Physical gamepad acceptance — failure inventory before implementation

- Browser API missing, insecure/denied context, empty/sparse slots, malformed
  values or unplugged devices must leave keyboard/touch usable.
- Standard Xbox/PlayStation/Switch-style browser mappings need D-pad + left stick,
  buttons, diagonals and turbo. Non-standard devices need explicit custom bindings.
- Device IDs, indexes, browser layouts and accounts must not share held state or
  overwrite each other's bindings. Multiple connected devices need explicit choice.
- A held button at connect, reconnect, profile swap, focus return or dialog close
  must not resume input until released. After release, gamepad play must work
  without manually restoring keyboard focus to the board. Disconnect, blur, hidden page, disable and
  disposal must release physical input without cancelling independent sources.
- Analog dead zones and hysteresis prevent drift/jitter; out-of-range, nonfinite
  input never reaches game rules. Opposite directions cancel; ordinary A/B stays
  held during a turbo gap. Gamepad buttons are not synthetic keyboard events.
- Keyboard and device maps save independently per user and logical controller
  model; stale writes, invalid indexes, duplicate bindings, unknown fields,
  excessive payloads and unauthorized writes are rejected. Restart retains data.
- Remapping captures fresh physical button/axis changes only, rejects conflicts,
  supports unbinding/default reset, and cannot issue game input from settings.
- 320px and desktop: live device state, readable labels, reachable save/reset,
  keyboard settings unchanged, disconnected devices remain editable.
- Real local rooms exercise the existing controller consumers: Contra,
  Basketball and Table Tennis. Record accepted game input, release behavior and
  screenshots. No game state/physics is substituted. A virtual Gamepad API source
  is labelled separately from native mouse/keyboard input and physical hardware.

Tests are scoped to the shared controller contract, HTTP persistence and affected
input adapters. Do not run unrelated complete-match/rules matrices.

## Directional navigation (before implementation)

- Directions must focus adjacent cards/cells without selecting or submitting.
- Confirm toggles the focused hand card; playing remains a separate action.
- Grid confirmation selects a cell, then the existing confirmation submits it.
- Held confirm cannot double-submit; held directions repeat at a bounded rate.
- Spatial navigation respects rows and columns, stable snapshots, disabled and
  hidden controls. Focus and the target remain visible at desktop and 320px.
- Game dialogs receive navigation exclusively; platform/settings dialogs block
  the game. Fullscreen has the same navigation and selection semantics.
- Remapping uses per-game keyboard and hardware profiles; old arrows do not
  bypass a changed mapping. Only explicitly integrated games expose a scheme.

## Native action games (before implementation)

- Bubbleparty and Battlecity preserve their existing keyboard aliases and touch
  controls. Per-game remaps replace the old hardcoded handlers; semantic actions
  feed the existing intent/prediction/send loops, without synthetic key events.
- Direction changes are immediate, latest held direction wins for four-way
  movement, releasing a newer direction restores the older one. Opposites cancel.
- Touch and keyboard/gamepad sources are independent; disconnect releases only
  physical input. Held confirm/fire cannot repeat a one-shot bubble placement.
- Selection panels can be navigated and confirmed through the same game-owned
  action vocabulary; no gameplay intent escapes a picker or platform dialog.
- Release on loss of focus, visibility, configuration, disabled state and room
  change; held sticks/buttons must return to neutral before reconnect/resume.
- E2E must prove rendered movement and accepted input, bubble creation/shooting,
  remapped native keyboard response and old-key inactivity, menu selection,
  fullscreen, 320px controls, settings blocking and device disconnect.

## Falling-block inputs and prediction (before implementation)

- Tetris and SPF2T expose their own move/rotate/drop/hold actions and preserve
  existing keyboard aliases, including standalone Ctrl and Shift bindings.
- Held lateral/soft-drop inputs retain each game's repeat delay and interval;
  held rotation, hard drop and hold trigger once, including across a new piece.
- Settings, blur, disconnect and round changes stop repeats without cancelling
  a still-held independent source. Native keyboard remapping must take effect.
- SPF2T selection works by gamepad; gameplay inputs do not leak out of selection.
- Visual movement/rotation responds before delayed outgoing inputs reach the
  server. SPF2T prediction mirrors its own collision/kick rules, projects only
  the active pair and ghost, never locks gems, clears them, scores or spawns.
- New authoritative piece/ack, walls, blocked rotation, hard drop, soft drop and
  rapid direction changes must converge without replaying accepted inputs.
- Real-room E2E captures accepted sequence/active piece, visible geometry,
  delayed-send measurements, held-button behavior, 320px/fullscreen screenshots.

## Charged actions and Fortress Blue (before implementation)

- Normal release must fire at the chosen power; disconnect, blur, editing,
  settings, profile changes, cancelled touch and turn changes must cancel charge.
- The sampler must distinguish a device disappearing from a normal release,
  including disappearance before the browser's disconnect event. An absent idle
  device cannot cancel a keyboard charge. Other held input sources remain active.
- Fortress Blue must support tank selection, movement, angle/facing, weapon,
  four item slots and camera controls through its per-game scheme. Retain original
  mouse aim, wheel/pan and touch charge controls; no fabricated keyboard events.
- A full charge fires once; a held key/button cannot fire again on the next turn.
- E2E checks authoritative movement, charge cancellation/rearm, remapped native
  keyboard, weapon/item activation, actual shot, fullscreen/ESC and 320px touch.

## Board navigation, checkbox controls and disappearing tiles (before implementation)

- Chess, Xiangqi and Go directions change focused squares without submitting;
  selection, destination and final confirmation retain each game's validation.
- Game-owned cancel clears selection, while cancel inside a dialog closes the
  dialog first. Action groups remain reachable and board edges do not jump to
  unrelated controls. View rotation and Go zoom must follow displayed geometry.
- Checkbox/radio controls (such as offering a draw) can be focused and confirmed;
  text editors, settings and platform dialogs continue to block game input.
- Pairs preserves its normal two-tile matching interaction. After a tile becomes
  disabled or disappears, the next navigation input chooses a nearby remaining
  tile in the same group, without jumping to the toolbar or automatically picking.
- Verify native remapped keyboard and gamepad navigation in real rooms, confirmed
  server actions, cancellation, stable snapshots, 320px focus visibility and
  fullscreen. Test source geometry rather than assuming fixed seat orientation.

## Additional card games and select controls (before implementation)

- Guandan/Shengji retain independent profiles and shared hand navigation, multi-
  selection, cancellation and row movement. Selection cannot submit a play.
- Reach normal action buttons through group navigation, execute a legal play,
  and handle Shengji declaration/bury via the original controls when applicable.
- Game-owned select fields can change options with up/down, then confirmation
  leaves the field; text entry and platform settings still block game input.
- Closing a game dialog preserves underlying hand selection. Verify the shared
  select path in the real card-skin dialog and keyboard remapping at 320px.

## Continuous axes and racing (before implementation)

- Continuous actions preserve button pressure and signed axis magnitude; digital
  actions keep their existing threshold/pressed semantics. Dead zones, malformed
  values, turbo, opposing directions, rearm and cancel versions remain valid.
- Keyboard values are full-scale; simultaneous sources merge without releasing
  a still-held source. The game consumes continuous values in its existing loop.
- Racing retains analog steering/throttle and adds optional proportional braking.
  Omitted brakeAmount keeps old boolean braking (including stored races/bots).
  Reject nonfinite, out-of-range and contradictory pressure; resets, timeout and
  recovery use full braking and cannot inherit an old partial pressure.
- Server integration and local prediction use the same brake magnitude; pending
  inputs retain their own pressure across save/restore and replay.
- E2E covers light/full inputs, original/remapped keys, drift/rear view/reset/map,
  mixed sources, disconnect/neutral resume, visible response and 320px touch.

## Monopoly, Junqi, Sanguosha and numeric controls (before implementation)

- Numeric game fields accept gamepad stepping with native min/max/step rules and
  update the original React draft. They do not submit a bid/trade until its
  existing confirm button is pressed. Native typing remains editor input.
- Monopoly must reach property details, roll, auction, bid, trade recipient,
  asset checkboxes, cash fields and explicit offer confirmation.
- Junqi must preserve displayed orientation/zoom, select two deployment pieces,
  confirm a legal swap, ready, then select/confirm a legal move; directions alone
  never submit. Cancel clears only current selection and dialogs remain scoped.
- Sanguosha must reach general choice, hand cards, players, skills and response
  actions. Selected-card lifting must not invent another navigation row; card
  selection and target selection stay separate from the existing play/response.
- Verify per-game remapping, actual accepted actions, fullscreen and visible
  focus at 320px, retaining real-room results and repeatable browser scripts.
- Numeric short presses remain one native step; a sustained hold accelerates
  only in the focused numeric field and still obeys min/max, allowing timely bids.

## Map navigation and camera reveal (before implementation)

- Carcassonne: directions focus legal positions without placing; confirm previews,
  cancel clears only preview; rotation and placement use the existing action.
  Meeple selection remains separate from deployment. Mouse drag must not suppress
  a later controller confirmation. Off-viewBox focus pans the camera into view,
  including after zoom and at 320px/fullscreen, without changing selection.
- Catan: selectable vertices/edges/tiles form a map group; setup settlement and
  road each require the existing confirmation. Cancel clears draft selection.
  Expansion/explorer maps keep native actions and numeric/select fields.
- Rail: ticket selection and submission stay distinct; routes/cities navigate
  spatially, route/payment dialogs own focus and cancel, drawing cards uses the
  existing legal turn flow. Zoomed targets must be visible in the map viewport.
- Each game has an independent profile; remapping disables old keys; snapshots,
  settings, native mouse/touch and fullscreen retain the original game state.

## Conquest and Starfort (before implementation)

- Map direction changes focus only; confirmation retains original province/pad
  selection. ViewBox/Canvas cameras reveal offscreen targets without submitting.
  Controllers reach every territory/build point after zoom and at 320px.
- Conquest: recruit amounts use native fields, confirm creates a real plan;
  movement/building/undo and submit use original handlers. Direct town building
  (otherwise a long press) needs an independently mapped game action, respects
  ownership/resources, and cannot fire through dialogs or settings. Ordinary
  confirm is still a single press, never a long-press town shortcut.
- Navigation presets may add game-owned discrete actions without losing the
  eight common controls. Extra actions dispatch only in the active game scope.
- Starfort: build/upgrade/target priority/sell, pause/resume and choice dialogs
  remain reachable; map camera must account for fixed HUD/controls obscuring
  pads. Cancel closes dialogs first, then clears tower selection. Existing skill
  tooltip stability must survive controller focus, zoom and fullscreen.
- Use real rooms, authoritative accepted actions, remapped native keyboard,
  virtual pad, zoomed focus geometry and 320px native touch; retain artifacts.

## Snooker (before implementation)

- Per-game keyboard/pad controls cover aiming, fine adjustment, charge and normal
  release, D-area placement, spin, reset, and the existing decisions/menus.
  Left/right aim and analog strength remain responsive locally. Directions in
  placement follow the displayed table orientation and show a visible cursor.
- Menu confirm does not shoot. Entering settings, editors or game menus, loss of
  focus/visibility, disconnect, remapping and phase/geometry changes cancel a
  charge. A remaining keyboard source survives physical disconnect. Returning
  requires neutral input; a cancelled held button cannot restart until released.
- Spin widget consumes mapped directional navigation without submitting a shot;
  menu reset restores neutral spin. Remapped original keys cannot bypass controls
  through old fixed handlers. Mouse/touch pull and cancellation remain intact.
- Verify real foul decisions, server-accepted placement and shot, visible charge,
  disconnect/rearm, mixed sources, settings cancellation, fine aiming, spin,
  remapping, full screen and native 320px touch, with reproducible artifacts.

## Metadata-only production asset delivery

- The production-module fixture must not install the private Contra media route.
  It uses verified runtime identities and serves only the trimmed asset directory.
- Actual CDN-loaded application modules, Canvas images, native audio and Wasm
  must still reach ready state and perform the original keyboard/pad/touch actions.
  Relative media paths follow the production redirect; no asset response or game
  state is fabricated. Run representative clients against the verified build.
