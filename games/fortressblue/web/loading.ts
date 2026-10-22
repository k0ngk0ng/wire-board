import type { GameRoom } from "../../../web/src/game-api/module";
import {
  mapById,
  maps,
  tanks,
  items,
  tankPortraits,
  explosion,
  sounds,
  type View,
} from "./types";
import { animations } from "./render";
import { projectilePaths, projectileEffectPaths } from "./projectiles";
import { supplyPaths } from "./supplies";
export function resources(room: GameRoom) {
  const d = room.game?.data as View,
    map = mapById.get(d.mapId) ?? maps[0];
  return [
    map.bg,
    map.fg,
    map.fg2,
    map.music,
    explosion.url,
    ...Object.values(sounds),
    ...supplyPaths,
    ...projectileEffectPaths,
    ...Object.values(animations).flatMap((a) => a.atlases),
    ...tanks.flatMap((t) => [t.bullet?.url, ...projectilePaths(t.id)]),
    ...Object.values(tankPortraits).map((t) => t.url),
    ...items.map((i) => i.icon),
  ].filter((p): p is string => !!p);
}
