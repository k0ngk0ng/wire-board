import type { GameEvent } from "./types";

export type Seen = { scope: string; id: number };

export function nextEvents(
  events: GameEvent[],
  seen: Seen,
  scope: string,
): { events: GameEvent[]; seen: Seen } {
  if (seen.scope !== scope) {
    const max = events.reduce((value, event) => Math.max(value, event.id), 0);
    return { events: [], seen: { scope, id: max } };
  }
  const fresh = events.filter((event) => event.id > seen.id);
  const max = fresh.reduce((value, event) => Math.max(value, event.id), seen.id);
  return { events: fresh, seen: { scope, id: max } };
}

export type SoundKey =
  | "select"
  | "countdown"
  | "lock"
  | "crash"
  | "chain"
  | "power"
  | "diamond"
  | "garbage"
  | "topout"
  | "win"
  | "lose";

export function soundFor(
  event: GameEvent,
  seat: number,
  spectating: boolean,
  won: boolean,
): SoundKey | null {
  switch (event.type) {
    case "select":
      return !spectating && event.seat === seat ? "select" : null;
    case "countdown":
    case "start":
      return "countdown";
    case "lock":
      return !spectating && event.seat === seat ? "lock" : null;
    case "clear":
      if (spectating || event.seat !== seat) return null;
      return (event.chain ?? 1) > 1 ? "chain" : "crash";
    case "power_merge":
      return !spectating && event.seat === seat ? "power" : null;
    case "diamond":
      return !spectating && event.seat === seat ? "diamond" : null;
    case "garbage":
      return !spectating && event.seat === seat ? "garbage" : null;
    case "topout":
      return !spectating && event.seat === seat ? "topout" : null;
    case "finish":
      return won ? "win" : "lose";
    default:
      return null;
  }
}

export function fxFor(event: GameEvent): { type: string; cells: { x: number; y: number }[] } | null {
  if (event.type === "clear" || event.type === "diamond" || event.type === "power_merge") {
    return { type: event.type, cells: event.cells ?? [] };
  }
  if (event.type === "garbage") {
    return { type: "garbage", cells: [] };
  }
  return null;
}
