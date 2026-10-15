import type {
  PresentationSnapshot,
  PresentationFrame,
  RoomPresentation,
} from "../../../web/src/game-api/module";
import type { View, Event } from "./types";
export const durationOf = (event: Event) =>
  event.type === "swap"
    ? 240
    : Math.min(
        800,
        280 + Math.max(0, ...event.moves.map((m) => m.path.length - 2)) * 65,
      );
export function createPlayback(
  reduced = () =>
    typeof matchMedia !== "undefined" &&
    matchMedia("(prefers-reduced-motion: reduce)").matches,
): RoomPresentation {
  let latest: PresentationSnapshot;
  type Entry = { before: PresentationSnapshot; event: Event; duration: number };
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
    if (!active && queue.length) {
      const next = queue.shift()!;
      active = { ...next, until: now + next.duration };
    }
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
            canArrange: false,
            canConfirm: false,
            canUnready: false,
            canMove: false,
            canVote: false,
            canOffer: false,
            canResign: false,
            motion: { event: active.event, duration: active.duration },
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
        const event = next.last;
        if (
          event.moves.length &&
          (event.type === "move" ||
            (event.type === "swap" && event.seat === s.room.you))
        )
          queue.push({ before, event, duration: durationOf(event) });
      }
      return sample(now);
    },
    sample,
  };
}
