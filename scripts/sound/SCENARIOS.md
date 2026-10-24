# Shared sound control acceptance (written before implementation)

- One speaker button in the header; mute, volume and preview live in its popover.
- Mouse and keyboard range input, Escape/outside dismissal, reachable 320px UI.
- Actual Bubbleparty music changes level immediately without restarting; effects,
  preview and continuous loop voices use the same shared setting.
- Muting stops voices/queues; preview cannot unmute. Raising volume from zero is
  a new user action; cancelled effects must not replay.
- Browser preference persists through reload and synchronizes between tabs.
- Game-only fullscreen excludes platform sound controls; exiting restores them.
- Save browser screenshots, source digests and JSON; remove owned profiles and DB.

The native media tests also cover loop ramps, invalid levels and late playback.
Browser acceptance verifies native Chromium media and input, not physical iOS.
