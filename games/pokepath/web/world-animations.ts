import type { View } from "./types";
export type WorldAnimations = { epoch?: number; route?: number; seen: number; phases: Map<number, string>; flashes: Set<number>; explosions: { id: number; x: number; y: number; radius: number; startMS: number }[] };
export const createWorldAnimations = (): WorldAnimations => ({ seen: 0, phases: new Map(), flashes: new Set(), explosions: [] });
export function updateWorldAnimations(state: WorldAnimations, view: View, reset = false) {
  const clock = view.physicsClock ?? view.clock;
  const zones = view.world?.zones ?? [];
  state.flashes.clear();
  if (reset || state.epoch !== view.epoch || state.route !== view.route) {
    state.epoch = view.epoch; state.route = view.route;
    state.seen = Math.max(0, ...view.events.map(event => Number(event.id ?? 0)));
    state.explosions = [];
    state.phases = new Map(zones.map(zone => [zone.id, zone.phase]));
    return state;
  }
  state.explosions = state.explosions.filter(effect => clock - effect.startMS < 450);
  for (const event of view.events) {
    const id = Number(event.id ?? 0), startMS = Number(event.at ?? clock);
    if (id <= state.seen || startMS > clock) continue;
    state.seen = id;
    if (event.type === "shadowTagExplosion" && clock - startMS < 450) state.explosions.push({ id, x: Number(event.x ?? 0), y: Number(event.y ?? 0), radius: Number(event.amount ?? 60), startMS });
  }
  for (const zone of zones) if (zone.kind === "singularity" && state.phases.get(zone.id) === "waiting" && zone.phase === "pulling") state.flashes.add(zone.id);
  state.phases = new Map(zones.map(zone => [zone.id, zone.phase]));
  return state;
}
