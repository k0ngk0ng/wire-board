import type { TetrisEvent } from "./types";

export type Seen = { scope: string; id: number };

export function nextEvents(
  events: TetrisEvent[],
  seen: Seen,
  scope: string,
): { events: TetrisEvent[]; seen: Seen } {
  if (seen.scope !== scope) {
    const max = events.reduce((value, event) => Math.max(value, event.id), 0);
    return { events: [], seen: { scope, id: max } };
  }
  const fresh = events.filter((event) => event.id > seen.id);
  const max = fresh.reduce((value, event) => Math.max(value, event.id), seen.id);
  return { events: fresh, seen: { scope, id: max } };
}

export function soundFor(
  event: TetrisEvent,
  seat: number,
  spectating: boolean,
): string | null {
  switch (event.type) {
    case "start":
      return "start";
    case "lock":
      return !spectating && event.seat === seat ? "lock" : null;
    case "clear":
      return !spectating && event.seat === seat
        ? (event.count ?? 0) >= 4
          ? "tetris"
          : "clear"
        : null;
    case "garbage":
      return !spectating && event.applied && event.seat === seat
        ? "garbage"
        : null;
    case "level":
      return !spectating && event.seat === seat ? "level" : null;
    case "eliminate":
      return !spectating && event.seat === seat ? "eliminate" : null;
    case "finish":
      return !spectating && event.seat === seat ? "win" : "finish";
    default:
      return null;
  }
}

export function animationFor(event: TetrisEvent): string | null {
  switch (event.type) {
    case "lock":
      return "lock";
    case "clear":
      return "clear";
    case "garbage":
      return event.applied ? "garbage" : null;
    case "level":
      return "level";
    case "eliminate":
      return "eliminate";
    case "finish":
      return "finish";
    default:
      return null;
  }
}
