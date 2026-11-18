import { effectiveResourceSpecies } from "./resource-species";
import type { Catalog, View } from "./types";

export type AudioCue = { path: string; group?: number; category: "ui" | "effects" };

export function audioCues(view: View, catalog: Catalog | undefined, events: View["events"]): AudioCue[] {
  const cues: AudioCue[] = [];
  const groups = new Set<string>();
  for (const event of events) {
    let sound: string | undefined;
    let group: number | undefined;
    let category: AudioCue["category"] = "effects";
    if (event.type === "fire") {
      const tower = view.towers.find(tower => tower.id === event.source);
      const pokemon = view.team.find(pokemon => pokemon.id === tower?.pokemonID || pokemon.id === event.source);
      const species = pokemon && catalog ? effectiveResourceSpecies(pokemon, catalog, view.progression?.dna) : undefined;
      const fact = species?.facts?.projectileSound;
      if (typeof fact !== "string" || !/^[a-zA-Z][A-Za-z0-9_-]*$/.test(fact)) continue;
      sound = fact;
      const mode = pokemon?.combatAttackMode ?? species?.attackType;
      group = ["beam", "stream"].includes(mode ?? "") || sound.startsWith("beam") ? 4 : species?.attackType === "area" || sound.startsWith("area") || ["area", "wave"].includes(mode ?? "") ? 3 : 0;
      if (group === 3) {
        const key = `${event.source}:${event.at}:${sound}`;
        if (groups.has(key)) continue;
        groups.add(key);
      }
    } else if (event.type === "teleport") { sound = "teleport"; group = 1; }
    else if (event.type === "shiny") { sound = "shiny"; group = 2; }
    else if (event.type === "waveComplete") { sound = event.key === "route_complete" ? "results" : "end"; category = "ui"; }
    else if (event.type === "routeDefeat" || event.type === "practice_complete") { sound = "results"; category = "ui"; }
    else if (event.type === "shopEgg" || event.type === "shopItem") {
      category = "ui";
      if (event.type === "shopEgg" || Number(event.target ?? 0) > 0) {
        cues.push({ path: "src/assets/sounds/ui/purchase.wav", category });
        sound = "results";
      } else sound = "purchase";
    }
    else if (event.type === "evolve") { sound = "obtain"; category = "ui"; }
    if (!sound || group !== undefined && view.progression?.settings.mute?.[group]) continue;
    cues.push({ path: `src/assets/sounds/${category === "ui" ? "ui" : "effect"}/${sound}.wav`, group, category });
  }
  return cues;
}
