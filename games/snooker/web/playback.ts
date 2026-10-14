import type {
  GameRoom,
  PresentationSnapshot,
  PresentationFrame,
  RoomPresentation,
} from "../../../web/src/game-api/module";
import type { View, Flight } from "./types";

export const cueLeadMs = 320;
type Entry = { room: GameRoom; flight: Flight; at: number; until: number };

function rollingRoom(source: GameRoom, previous?: GameRoom): GameRoom {
  const d = source.game!.data as View,
    f = d.flight!,
    c = f.context;
  const before = previous?.game?.data as View | undefined;
  const oldResults = d.results.filter((r) => r.frame < d.frame);
  const framesWon = [0, 0];
  for (const r of oldResults) framesWon[r.winner]++;
  const reds = c.balls.filter((b) => !b.down && b.id >= 1 && b.id <= 15).length;
  let remaining =
    c.target === "red"
      ? reds * 8 + 27
      : c.target === "colour"
        ? reds * 8 + 34
        : 0;
  if (c.target === "clear")
    for (let v = Math.max(2, c.clear); v <= 7; v++) remaining += v;
  const settled = d.phase !== "rolling" || f.resolved;
  const data: View = {
    ...d,
    phase: "rolling",
    turn: f.seat,
    nextSeat: d.order[(d.order.indexOf(f.seat) + 1) % d.order.length],
    balls: c.balls,
    scores: c.scores,
    framesWon,
    results: oldResults,
    winner: -1,
    finishReason: undefined,
    endExplanation: "",
    target: c.target,
    clear: c.clear,
    inHand: c.inHand,
    freeBall: c.freeBall,
    remaining,
    foul: undefined,
    flight: { ...f, resolved: false },
    break: settled ? (before?.frame === d.frame ? before.break : 0) : d.break,
    bestBreak: settled
      ? (before?.bestBreak ?? d.bestBreak.map(() => 0))
      : d.bestBreak,
    controls: {
      canAct: false,
      canShoot: false,
      canPlace: false,
      canClaim: false,
      canConcede: false,
    },
  };
  return {
    ...source,
    status: "playing",
    turnDeadline: 0,
    game: {
      ...source.game!,
      data,
      finished: false,
      winners: [],
      actors: [],
      attention: [],
      systemDeadline: 0,
      log: settled ? (previous?.game?.log ?? []) : source.game!.log,
    },
  };
}

// Only display state is buffered. The host continues to retain the newest
// authoritative room for commands, permissions, versions and persistence.
export function createPlayback(): RoomPresentation {
  let latest: PresentationSnapshot;
  let seen = 0,
    active: Entry | undefined;
  const queue: Entry[] = [];
  function reset(s: PresentationSnapshot): PresentationFrame {
    latest = s;
    seen = (s.room.game?.data as View | undefined)?.flight?.id ?? 0;
    active = undefined;
    queue.length = 0;
    return { ...s, blocked: false };
  }
  function sample(now: number): PresentationFrame {
    if (active && now >= active.until) active = undefined;
    if (!active && queue.length) {
      active = queue.shift()!;
      active.at = now;
      active.until =
        now +
        (active.flight.cueDuration ?? cueLeadMs) +
        active.flight.motion.duration * 1000;
    }
    if (!active) return { ...latest, blocked: false };
    const room = {
      ...latest.room,
      status: "playing",
      turnDeadline: 0,
      game: active.room.game,
      result: undefined,
    };
    return {
      room,
      clock: {
        serverNow:
          active.flight.start - (active.flight.cueDuration ?? cueLeadMs),
        receivedAt: active.at,
      },
      blocked: true,
      wakeAt: active.until,
    };
  }
  return {
    update(s, now, resync) {
      if (
        resync ||
        !latest ||
        s.room.id !== latest.room.id ||
        ["waiting", "closed"].includes(s.room.status)
      )
        return reset(s);
      if (s.room.version < latest.room.version) return sample(now);
      const previous = latest.room;
      latest = s;
      const f = (s.room.game?.data as View | undefined)?.flight;
      if (f && f.id !== seen) {
        if (f.id < seen || f.id > seen + 1 || !f.context || queue.length >= 4)
          return reset(s);
        seen = f.id;
        queue.push({
          room: rollingRoom(s.room, previous),
          flight: f,
          at: 0,
          until: 0,
        });
      }
      return sample(now);
    },
    sample,
  };
}
