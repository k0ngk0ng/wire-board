import type { Definition, DexTowerStats, Pokemon, Species } from "./types";

export type DexMode = "tutorials" | "pokemon" | "enemies";
export type DexEntry = { key: string; definition: Definition; species?: Species; pokemon?: Pokemon; discovered: boolean; name: string; search: string[]; defeated?: number };
type SortValue = (entry: DexEntry) => string | number;
type SortGroup = { label: string; value: SortValue; ascending?: boolean; single?: boolean };

export function localizedValue(value: unknown, language: number): string {
  if (typeof value === "string") return value;
  if (Array.isArray(value)) return String(value[language] || value[0] || "");
  return "";
}

export function dexName(definition: Definition | undefined, language: number): string {
  return localizedValue(definition?.names ?? definition?.name, language);
}

export function dexDescription(definition: Definition | undefined, language: number): string {
  return localizedValue(definition?.descriptions ?? definition?.description, language);
}

export function dexPokemonStats(entry: DexEntry) {
  const stats = entry.pokemon?.combatStats ?? entry.pokemon?.stats;
  return { power: stats?.power ?? Number(entry.species?.powerBase ?? 0), speed: stats?.attackMS ?? Number(entry.species?.attackMsBase ?? 0), critical: stats?.criticalPct ?? Number(entry.species?.criticalBase ?? 0), range: stats?.range ?? Number(entry.species?.rangeBase ?? 0) };
}

const byName: SortGroup = { label: "text.box.alphabetical", value: entry => entry.name, ascending: true };
const byFact = (label: string, key: string, single = false): SortGroup => ({ label, value: entry => Number(entry.definition[key] ?? 0), single });
const byTower = (key: keyof ReturnType<typeof dexPokemonStats>, ascending = false): SortGroup => ({ label: `text.box.${key}`, value: entry => dexPokemonStats(entry)[key], ascending });
const sorts = {
  pokemon: [{ label: "text.box.acquisition", value: (entry: DexEntry) => entry.pokemon?.id ?? 0, ascending: true }, byName, byTower("power"), byTower("speed", true), byTower("critical"), byTower("range")],
  enemies: [{ label: "text.enemy.sort.defeated", value: (entry: DexEntry) => entry.defeated ?? 0 }, byName, byFact("text.enemy.sort.elite", "isElite", true), byFact("text.enemy.sort.boss", "isBoss", true), byFact("text.ui.health", "health"), byFact("text.ui.armor", "armor"), byFact("text.ui.speed", "speed"), byFact("text.ui.power", "power"), byFact("text.enemy.sort.regeneration", "regeneration"), byFact("text.ui.invisible", "invisible", true)]
};

export function dexSorts(mode: "pokemon" | "enemies") {
  return [undefined, ...sorts[mode].flatMap(({ label, value, ascending, single }) => (single ? [0] : [1, -1]).map(direction => ({ label, direction, compare: (first: DexEntry, second: DexEntry) => {
    const left = value(first);
    const right = value(second);
    const delta = typeof left === "string" && typeof right === "string" ? left.localeCompare(right) : Number(left) - Number(right);
    return delta * (ascending ? 1 : -1) * (direction || 1);
  } })))];
}

export const unregisteredEnemies = new Set(["superCaterpie", "wishiwashiSolo"]);
export const tutorialArtAtBottom = new Set(["towersAttackType", "towersForms", "team", "routesStars", "routesModifiers", "enemies", "items", "itemsMegas", "itemsAdmin", "weather", "secrets"]);

export const dexTowerKeys: (keyof DexTowerStats)[] = ["damage", "highestHit", "deployed", "active", "waves", "defeated", "bosses", "challenges"];

export function dexTowerRanks(pokemon: Pokemon[]) {
  const ranks = new Map<number, Partial<Record<keyof DexTowerStats, number>>>();
  for (const key of dexTowerKeys) {
    const winners = [...pokemon].sort((left, right) => (right.dex?.[key] ?? 0) - (left.dex?.[key] ?? 0)).slice(0, 3);
    winners.forEach((owner, index) => {
      const previous = ranks.get(owner.id) ?? {};
      ranks.set(owner.id, { ...previous, [key]: index + 1 });
    });
  }
  return ranks;
}

const terrains = [["FIELD", "CAMPO"], ["GRASS", "HIERBA"], ["WATER", "AGUA"], ["MOUNTAIN", "MONTAÑA"], ["ALL", "TODO"]];

export function dexTerrain(keys: number[], language: number): string {
  return keys.map(key => terrains[key - 1]?.[language === 1 ? 1 : 0] ?? String(key)).join(", ");
}
