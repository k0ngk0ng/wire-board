// Simulation time stops at the final result; the platform clock keeps advancing.
// Anchoring to it prevents refreshes and later snapshots replaying old effects.
export function animationNow(
  serverAt: number,
  clock: { serverNow: number; receivedAt: number; paused?: boolean },
  monotonicNow: number,
) {
  return (
    Math.max(serverAt, clock.serverNow || serverAt) +
    (clock.paused ? 0 : Math.max(0, monotonicNow - clock.receivedAt))
  );
}
