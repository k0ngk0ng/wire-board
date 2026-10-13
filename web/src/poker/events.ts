// Transport feedback filtering only; this module knows no card-game rules.
export function nextFeedbackEvents<T extends { id: number; at: number }>(
  events: readonly T[],
  lastID: number,
  clock: { serverNow: number; receivedAt: number },
  clientNow: number,
): { id: number; events: T[] } {
  const now = clock.serverNow + Math.max(0, clientNow - clock.receivedAt);
  return {
    id: Math.max(lastID, ...events.map((event) => event.id)),
    events: events.filter((event) => {
      const age = now - event.at;
      return event.id > lastID && age >= -1000 && age <= 2500;
    }),
  };
}
