import type {
  PresentationSnapshot,
  RoomPresentation,
} from "../../../web/src/game-api/module";
import type { RuntimeSnapshot } from "./runtime-bridge";

/** Only delay the common result UI; preserve authority records and scoring. */
export function createRuntimeRoomPresentation(
  read: (key: string) => RuntimeSnapshot | undefined,
): RoomPresentation {
  let latest: PresentationSnapshot;
  const sample = (now: number) => {
    const room = latest.room;
    const epoch = (room.game?.data as { epoch?: string } | undefined)?.epoch;
    const visible = epoch ? read(`${room.id}:${epoch}`) : undefined;
    if (
      room.status === "finished" &&
      room.game?.finished &&
      visible &&
      visible.epoch === epoch &&
      !visible.finalized
    ) {
      return {
        ...latest,
        room: {
          ...room,
          status: "playing",
          game: { ...room.game, finished: false },
        },
        blocked: true,
        wakeAt: now + 1000 / 60,
      };
    }
    return { ...latest, blocked: false };
  };
  return {
    update: (value, now) => {
      latest = value;
      return sample(now);
    },
    sample,
  };
}
