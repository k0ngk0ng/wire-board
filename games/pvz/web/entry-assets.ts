import type { View } from "./types";
import { selectionUnits } from "./cards.ts";

export function neededUnits(view: View, selected: string | null = null, actor = -1) {
  const kinds = new Set<string>(["mower", "sun"]);
  for (const kind of selectionUnits(view, actor, selected)) kinds.add(kind);
  const catalog = [
    ...(view.catalog?.plants ?? []),
    ...(view.catalog?.zombies ?? []),
  ];
  for (const player of view.players ?? []) {
    for (const kind of player.deck ?? []) {
      const spec = catalog.find((c) => c.kind === kind);
      if ((spec?.unlockAt ?? 0) <= view.tick + 200) kinds.add(kind);
    }
    if (player.deck?.includes("imitater") && player.imitaterKind)
      kinds.add(player.imitaterKind);
  }
  for (const plant of view.plants ?? []) {
    kinds.add(plant.kind);
    if (plant.targetKind) kinds.add(plant.targetKind);
  }
  for (const zombie of view.zombies ?? []) {
    kinds.add(zombie.kind);
    if (zombie.flags?.stolenPlant) {
      const held = view.catalog?.plants.find(
        (spec) => spec.originalId === zombie.flags!.stolenPlant - 1,
      );
      if (held) kinds.add(held.kind);
    }
  }
  for (const pickup of view.pickups ?? []) kinds.add(pickup.kind);
  for (const event of view.events ?? [])
    if (event.kind && ["zombie-death", "plant-death"].includes(event.type))
      kinds.add(event.kind);
  if (kinds.has("dancing")) kinds.add("backupdancer");
  if (
    kinds.has("gargantuar") ||
    kinds.has("redeye") ||
    kinds.has("redgargantuar") ||
    kinds.has("gigagargantuar")
  )
    kinds.add("imp");
  if (kinds.has("zomboss"))
    for (const zombie of view.catalog?.zombies ?? []) kinds.add(zombie.kind);
  return [...kinds];
}
