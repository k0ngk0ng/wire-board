import { effectiveResourceSpecies } from "./resource-species";
import type { Catalog, View } from "./types";

export function pointerTarget(view: View | undefined, catalog: Catalog | undefined, selected: number | undefined, allowed: boolean) {
  if (!view || !catalog || !allowed || view.paused || view.health <= 0 || selected === undefined) return;
  const pokemon = view.team.find(pokemon => pokemon.id === selected);
  if (!pokemon || !view.towers.some(tower => tower.pokemonID === selected)) return;
  const ability = effectiveResourceSpecies(pokemon, catalog, view.progression?.dna)?.abilityId;
  if ((pokemon.combatMoveMode ?? (ability === "blimpKeys" || ability === "unburden" && pokemon.item === "blimpKeys" ? "followPointer" : "")) !== "followPointer") return;
  return { pokemonID: selected, epoch: view.epoch };
}

export type PointerAim = { pokemonID: number; epoch: number; x: number; y: number };
type Options = { now: () => number; send: (point: PointerAim) => void; setTimer: (callback: () => void, delay: number) => number; clearTimer: (id: number) => void; interval?: number };
export function createPointerAim(options: Options) {
  const interval = options.interval ?? 50;
  let timer: number | undefined;
  let pending: PointerAim | undefined;
  let last: PointerAim | undefined;
  let lastAt = -Infinity;
  let active = true;
  const same = (point: PointerAim) => last && last.pokemonID === point.pokemonID && last.epoch === point.epoch && last.x === point.x && last.y === point.y;
  const cancel = () => { if (timer !== undefined) options.clearTimer(timer); timer = undefined; pending = undefined; };
  const send = (point: PointerAim) => { last = { ...point }; lastAt = options.now(); options.send(last); };
  const flush = () => { timer = undefined; const point = pending; pending = undefined; if (active && point && !same(point)) send(point); };
  return {
    move(point: PointerAim) {
      if (!active || !Number.isFinite(point.x) || !Number.isFinite(point.y) || same(point)) return;
      pending = { ...point };
      if (options.now() - lastAt >= interval) { cancel(); send(point); }
      else if (timer === undefined) timer = options.setTimer(flush, Math.max(0, interval - (options.now() - lastAt)));
    },
    edge(point: PointerAim) { if (!active || !Number.isFinite(point.x) || !Number.isFinite(point.y)) return; cancel(); send(point); },
    cancel,
    dispose() { active = false; cancel(); }
  };
}
