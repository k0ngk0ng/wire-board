import type {
  PresentationSnapshot,
  PresentationFrame,
  RoomPresentation,
} from "../../../web/src/game-api/module";
import type { View, MoveEvent } from "./types";
export const moveDuration = 280;
type Entry = { before: PresentationSnapshot; event: MoveEvent };
// Buffer display only. Keep capture/promotion/results at the pre-move state
// until arrival; two castling pieces share one event and one completion time.
export function createPlayback(
  reduced = () =>
    typeof matchMedia !== "undefined" &&
    matchMedia("(prefers-reduced-motion: reduce)").matches,
): RoomPresentation {
  let latest: PresentationSnapshot;
  let active: (Entry & { until: number }) | undefined;
  const queue: Entry[] = [];
  const data = (s: PresentationSnapshot) =>
    s.room.game?.data as View | undefined;
  function reset(s: PresentationSnapshot): PresentationFrame {
    latest = s;
    active = undefined;
    queue.length = 0;
    return { ...s, blocked: false };
  }
  function sample(now: number): PresentationFrame {
    if (reduced()) return reset(latest);
    if (active && now >= active.until) active = undefined;
    if (!active && queue.length)
      active = { ...queue.shift()!, until: now + moveDuration };
    if (!active) return { ...latest, blocked: false };
    const old = data(active.before)!;
    return {
      room: {
        ...latest.room,
        status: "playing",
        turnDeadline: 0,
        result: undefined,
        game: {
          ...active.before.room.game!,
          finished: false,
          winners: [],
          actors: [],
          attention: [],
          data: {
            ...old,
            canMove: false,
            canResign: false,
            canRespond: false,
            motion: { event: active.event, duration: moveDuration },
          },
        },
      } as PresentationFrame["room"],
      clock: latest.clock,
      blocked: true,
      wakeAt: active.until,
    };
  }
  return {
    update(s, now, resync) {
      const next = data(s),
        old = latest && data(latest);
      if (
        resync ||
        !latest ||
        !old ||
        !next ||
        s.room.id !== latest.room.id ||
        next.round !== old.round ||
        ["waiting", "closed"].includes(s.room.status) ||
        reduced()
      )
        return reset(s);
      if (s.room.version < latest.room.version) return sample(now);
      const before = latest;
      latest = s;
      if (next.last.id !== old.last.id) {
        if (next.last.id !== old.last.id + 1 || queue.length >= 4)
          return reset(s);
        queue.push({ before, event: next.last });
      }
      return sample(now);
    },
    sample,
  };
}
