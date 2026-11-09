import type {
  GameRoom,
  PresentationSnapshot,
  PresentationFrame,
  RoomPresentation,
} from "../../../web/src/game-api/module";
import type { View } from "./types";
import { impactOffset } from "./motion.ts";

// Project only the visible frame. The latest server snapshot remains untouched
// for authentication, actions, persistence and settlement.
export function createPlayback(): RoomPresentation {
  let latest: PresentationSnapshot;
  let movie:
    | { room: GameRoom; clock: PresentationSnapshot["clock"]; end: number }
    | undefined;
  function sample(now: number): PresentationFrame {
    if (!movie || now >= movie.end) return { ...latest, blocked: false };
    const v = movie.room.game!.data as View;
    const playback = v.playback!;
    const elapsed =
      movie.clock.serverNow + now - movie.clock.receivedAt - playback.startedAt;
    let players = playback.before;
    let boundary = playback.duration;
    for (const event of v.events) {
      const impact = event.at + impactOffset(event);
      if (elapsed >= impact) players = event.players;
      else {
        boundary = impact;
        break;
      }
    }
    const room: GameRoom = {
      ...latest.room,
      status: "playing",
      turnDeadline: 0,
      game: {
        ...movie.room.game!,
        finished: false,
        winners: [],
        actors: [],
        attention: [],
        data: { ...v, players },
      },
    };
    // A settled authoritative result must not open the host modal early.
    delete (room as GameRoom & { result?: unknown }).result;
    const wakeAt = Math.max(now + 1, now + boundary - elapsed);
    return {
      room,
      clock: movie.clock,
      blocked: true,
      wakeAt: Math.min(movie.end, wakeAt),
    };
  }
  return {
    update(s, now, reset) {
      const v = s.room.game?.data as View | undefined;
      const old = latest?.room.game?.data as View | undefined;
      if (
        latest &&
        !reset &&
        s.room.id === latest.room.id &&
        s.room.version < latest.room.version
      )
        return sample(now);
      latest = s;
      if (["waiting", "closed"].includes(s.room.status)) {
        movie = undefined;
        return { ...s, blocked: false };
      }
      if (
        reset ||
        !old ||
        (movie && s.room.id !== movie.room.id) ||
        v?.session !== old.session
      )
        movie = undefined;
      if (v?.phase === "resolving" && v.playback) {
        // Metadata packets can reuse an older clock anchor. Count the time
        // since receipt too, or every such update extends the completed movie.
        const elapsed =
          s.clock.serverNow + now - s.clock.receivedAt - v.playback.startedAt;
        movie = {
          room: s.room,
          clock: s.clock,
          end: now + Math.max(0, v.playback.duration - elapsed),
        };
      }
      return sample(now);
    },
    sample,
  };
}
