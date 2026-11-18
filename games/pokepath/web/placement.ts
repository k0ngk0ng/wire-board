import type { Point, Tile, View } from "./types";

export type SiteStatus = "empty" | "host" | "occupied" | "blocked";
export type DeploymentPreview = { tileSize: number; sites: Record<number, SiteStatus> };
export type DeploymentRange = { center: Point; range: number; shape: string; innerRange: number; halfWidth: number };
export type PlacementQuery = (view: View, pokemonID: number) => DeploymentPreview | undefined;
export type RangeQuery = (view: View, pokemonID: number, tileID: number) => DeploymentRange | undefined;

export function siteAllowsDeployment(site?: SiteStatus) {
  return site === "empty" || site === "host";
}

export function siteAt(tiles: readonly Tile[], point: Point, size: number, touch = false) {
  const inside = tiles.find(tile => point.x >= tile.x && point.x < tile.x + size && point.y >= tile.y && point.y < tile.y + size);
  if (inside || !touch) return inside;
  let closest: Tile | undefined;
  let radius = Math.max(22, size / 2);
  for (const tile of tiles) {
    const distance = Math.hypot(tile.x + size / 2 - point.x, tile.y + size / 2 - point.y);
    if (distance < radius) { radius = distance; closest = tile; }
  }
  return closest;
}

export function deploymentKey(view: View, pokemonID: number) {
  const world = view.world;
  return JSON.stringify([view.epoch, view.route, view.phase, view.health > 0, view.paused, view.progression?.draft?.active, world?.movementLocked, world?.pressure, pokemonID,
    view.team.map(member => [member.id, member.species, member.item, view.progression?.dna?.[member.id]]), view.towers.map(tower => [tower.pokemonID, tower.tileID, world?.passengerHosts?.[tower.pokemonID]])]);
}

export function deploymentRangeKey(view: View, pokemonID: number) {
  return `${deploymentKey(view, pokemonID)}:${JSON.stringify([view.weather, view.health, view.stars, view.ribbons, view.wave, view.waveActive,
    view.progression?.challenge?.levelCap, view.team.map(member => [member.id, member.level, member.stats.range, member.combatStats?.range, member.combatRangeType, member.combatRangeHalfWidth, member.combatInnerRange]),
    view.towers.map(tower => [tower.pokemonID, tower.x, tower.y])])}`;
}
