import type { GameRoom } from "../../../web/src/game-api/module";
import { classicSpritePaths } from "./classic-terrain";
import type { View } from "./types";
export function resources(room: GameRoom, media: Record<string, number>) {
  const map = (room.game?.data as View).mapId;
  return [
    ...classicSpritePaths(map),
    ...Object.keys(media).filter(
      (p) =>
        p.startsWith("/bubbleparty/v2/") ||
        p.startsWith("/bubbleparty/v4/characters/") ||
        p.startsWith("/bubbleparty/v4/mounts/") ||
        p.startsWith("/bubbleparty/v1/audio/"),
    ),
  ];
}
