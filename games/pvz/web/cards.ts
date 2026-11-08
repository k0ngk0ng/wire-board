import type { View } from "./types";

export function hasOpenRoster(view: View) {
  return (view.rulesVersion ?? 0) >= 4;
}

export function rackKinds(view: View, actor: number, selected: string | null) {
  const player = view.players?.[actor];
  if (!player) return [];
  const quickbar = player.deck ?? [];
  if (
    !hasOpenRoster(view) ||
    !selected ||
    quickbar.includes(selected) ||
    !view.catalog[player.side]?.some((card) => card.kind === selected)
  )
    return quickbar;
  return [selected, ...quickbar].slice(0, 10);
}

export function selectionUnits(
  view: View,
  actor: number,
  kind: string | null,
  imitateKind?: string,
) {
  const player = view.players?.[actor];
  const spec =
    player && view.catalog[player.side]?.find((card) => card.kind === kind);
  if (!player || !spec || (spec.unlockAt ?? 0) > view.tick) return [];
  const kinds = [spec.kind];
  const target = imitateKind ?? player.imitaterKind;
  if (
    kind === "imitater" &&
    view.catalog.plants.some(
      (card) =>
        card.kind === target && card.kind !== "imitater" && !card.upgradeFrom,
    )
  )
    kinds.push(target);
  return kinds;
}
