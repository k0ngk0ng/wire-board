import type { Catalog, View } from "./types";
export type DamageLabel = { eventID: number; enemyID: number; x: number; y: number; startMS: number; text: string; color: string };
export type DamageLabels = { epoch?: number; route?: number; seen: number; labels: DamageLabel[]; enemies: Map<number, { x: number; y: number; height: number; at: number }> };
export const createDamageLabels = (): DamageLabels => ({ seen: 0, labels: [], enemies: new Map() });

export function updateDamageLabels(state: DamageLabels, view: View, catalog?: Catalog, reset = false): DamageLabel[] {
  const clock = view.physicsClock ?? view.clock;
  if (reset || state.epoch !== view.epoch || state.route !== view.route) {
    state.epoch = view.epoch;
    state.route = view.route;
    state.seen = Math.max(0, ...view.events.map(event => Number(event.id ?? 0)));
    state.labels = [];
    state.enemies.clear();
  }
  for (const enemy of view.enemies) state.enemies.set(enemy.id, { x: enemy.x, y: enemy.y, height: catalog?.enemies[enemy.key]?.sprite.frameHeight ?? 40, at: clock });
  state.labels = state.labels.filter(label => clock - label.startMS < 1000);
  for (const event of view.events) {
    const eventID = Number(event.id ?? 0);
    const at = Number(event.at ?? clock);
    if (eventID <= state.seen || at > clock) continue;
    state.seen = eventID;
    if (event.type !== "hit" || clock - at >= 1000) continue;
    const enemyID = Number(event.target ?? 0);
    const enemy = state.enemies.get(enemyID);
    const x = enemy?.x ?? Number(event.x ?? 0);
    const top = (enemy?.y ?? Number(event.y ?? 0)) - (enemy?.height ?? 40) / 2;
    const add = (text: string, color: string, miss = false) => {
      let y = top + (miss ? 0 : 2);
      for (const label of state.labels) {
        if (label.enemyID !== enemyID) continue;
        const current = label.y - 60 * (clock - label.startMS) / 1000;
        if (Math.abs(y - current) < 12) y = Math.min(y, current - 12);
      }
      state.labels.push({ eventID, enemyID, x, y, startMS: at, text, color });
    };
    if (event.evaded) { add("MISS", "#b9c9e0", true); continue; }
    const critical = Boolean(event.critical);
    const health = Number(event.displayHPDamage ?? 0);
    const armor = Number(event.displayArmorDamage ?? 0);
    const color = critical ? "#ffeb3b" : ({ burn: "#c96937", scorch: "#87150f", poison: "#70ac4c" } as Record<string, string>)[String(event.sourceKind)] ?? "#e06666";
    if (health > 0) add(`-${health}${critical ? "!" : ""}`, color);
    if (armor > 0) add(`-${armor}${critical ? "!" : ""}`, critical ? "#ffeb3b" : "#66ccff");
  }
  for (const label of state.labels) label.x = state.enemies.get(label.enemyID)?.x ?? label.x;
  if (state.enemies.size > 4096) for (const [id, enemy] of state.enemies) if (clock - enemy.at >= 1000) state.enemies.delete(id);
  return state.labels;
}
