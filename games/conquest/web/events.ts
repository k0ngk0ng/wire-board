import type { Event } from "./types";
type Snapshot = { events: Event[]; round: number };
export function visibleAction(events: Event[], now: number): Event | undefined {
  return events
    .filter((e) => e.at <= now && (e.displayUntil ?? e.until) > now)
    .at(-1);
}
export type EventCursor = {
  epoch: number;
  lastId: number;
  round: number;
  arrivals: Set<number>;
};
export function createCursor(snapshot: Snapshot, epoch = 0): EventCursor {
  return {
    epoch,
    lastId: Math.max(0, ...snapshot.events.map((e) => e.id)),
    round: snapshot.round,
    arrivals: new Set(snapshot.events.map((e) => e.id)),
  };
}
export function consumeEvents(
  cursor: EventCursor,
  snapshot: Snapshot,
  epoch = 0,
  now: number,
  audible: boolean,
  motion: boolean,
) {
  if (cursor.epoch !== epoch) {
    Object.assign(cursor, createCursor(snapshot, epoch));
    return { reset: true, clips: [] as string[], events: [] as Event[] };
  }
  const fresh = snapshot.events.filter((e) => e.id > cursor.lastId);
  const clips: string[] = [];
  for (const event of fresh)
    if (event.type === "move" && !event.after && event.until > now)
      clips.push("march");
  for (const event of snapshot.events)
    if (event.after && !cursor.arrivals.has(event.id)) {
      cursor.arrivals.add(event.id);
      if (event.captured && now >= event.until && now - event.until < 2500)
        clips.push("capture");
    }
  if (snapshot.round > cursor.round && !clips.length) clips.push("round");
  cursor.round = snapshot.round;
  cursor.lastId = Math.max(cursor.lastId, ...snapshot.events.map((e) => e.id));
  cursor.arrivals = new Set(
    snapshot.events.filter((e) => cursor.arrivals.has(e.id)).map((e) => e.id),
  );
  return {
    reset: false,
    clips: audible ? [...new Set(clips)] : [],
    events: motion
      ? fresh.filter((e) => (e.displayUntil ?? e.until) > now)
      : [],
  };
}
