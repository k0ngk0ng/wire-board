# Reset feedback failures (before implementation)

- Default R silently does nothing when the authority temporarily denies reset.
- A valid v3 driving reset is blocked by an extra client speed check.
- Countdown/reset animation/finished race/automatic control are confused, or feedback misreports a new v3 rule as a speed/cooldown requirement.
- Legacy unavailable reset is described as the new v3 animation state.
- The reset button keeps showing the removed extra three seconds after the v3 two-second protection has ended.
- A held keyboard repeat or editor focus triggers repeated reset (existing mapped-input lifecycle remains authoritative).

Server tests own high-speed reset, nearby ground placement, positive lap preservation and non-counting relocation. This helper only explains existing authority/ownership, never grants reset permission.
