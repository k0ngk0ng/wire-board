import type { CatanState, Room } from "./types";

export type FishSelection = {
  kind: string;
  ids: number[];
  color: number | null;
  target: number | null;
  edge: number | null;
  slot?: number | null;
};
export const fishActions = [
  ["catan_fish_robber", "驱离强盗"],
  ["catan_fish_pirate", "驱离海盗"],
  ["catan_fish_steal", "随机偷牌"],
  ["catan_fish_resource", "领取资源"],
  ["catan_fish_road", "修建道路"],
  ["catan_fish_bridge", "修建桥梁"],
  ["catan_fish_ship", "建造船只"],
  ["catan_fish_dev", "购买发展卡"],
  ["catan_fish_progress", "领取进步牌"],
  ["catan_fish_boot", "传递旧靴子"],
] as const;

export function fishActionsFor(g: CatanState) {
  if (g.explorer)
    return [
      ...(g.explorer.pirate
        ? [["catan_fish_pirate", "免海盗通行费"] as const]
        : []),
      ["catan_fish_steal", "随机偷牌"],
      ["catan_fish_resource", "领取资源"],
      ["catan_fish_road", "修建道路"],
      ["catan_fish_ship", "建造船只"],
      ["catan_fish_voyage", "额外航行"],
      ["catan_fish_boot", "传递旧靴子"],
    ] as const;
  return fishActions.filter(([kind]) => {
    if (kind === "catan_fish_robber" && g.attack) return false;
    if (kind === "catan_fish_bridge") return !!g.fishing?.rivers;
    if (kind === "catan_fish_ship") return !!g.seafarers;
    if (kind === "catan_fish_pirate")
      return (
        !!g.seafarers && !g.seafarers.wonders && !g.seafarers.pirateIslands
      );
    return (
      kind !== (g.citiesKnights ? "catan_fish_dev" : "catan_fish_progress")
    );
  });
}
export function fishMapMode(kind: string) {
  return kind === "catan_fish_bridge"
    ? "fish_bridge"
    : kind === "catan_fish_ship"
      ? "fish_ship"
      : kind === "catan_fish_road"
        ? "fish_road"
        : "";
}
export function fishGroundBlocked(g: CatanState, ground: { seaTile?: number }) {
  return (
    Number.isInteger(ground.seaTile) &&
    ground.seaTile! >= 0 &&
    !!g.seafarers &&
    g.seafarers.pirate === ground.seaTile
  );
}

function activeViewer(room: Room) {
  return !!(
    room.game?.catan?.fishing &&
    room.status === "playing" &&
    !room.game.finished &&
    !room.spectating &&
    !room.seats?.[room.you]?.autoPlay &&
    room.you >= 0 &&
    room.game.catan.players[room.you] &&
    !room.game.catan.players[room.you].eliminated
  );
}
export function fishResponder(room: Room) {
  return room.game?.phase === "catan_fish_replace"
    ? room.game.catan?.fishing?.tokens.responder
    : undefined;
}
export function fishTurn(room: Room) {
  return (
    activeViewer(room) &&
    room.game!.turn === room.you &&
    (["catan_roll", "catan_turn"].includes(room.game!.phase) ||
      (!!room.game!.catan?.explorer &&
        room.game!.phase === "catan_explorer_move"))
  );
}
export function fishValue(room: Room, ids: number[]) {
  const hand =
    room.game?.catan?.fishing?.tokens.players[room.you]?.tokens || [];
  if (
    new Set(ids).size !== ids.length ||
    ids.some((id) => !hand.some((t) => t.id === id))
  )
    return null;
  return ids.reduce((sum, id) => sum + hand.find((t) => t.id === id)!.fish, 0);
}
export function fishAction(
  room: Room,
  selection: FishSelection,
): Record<string, unknown> | null {
  if (!activeViewer(room)) return null;
  const { kind, ids, color, target, edge, slot } = selection,
    g = room.game!.catan!,
    f = g.fishing!;
  const wrap = (a: Record<string, unknown>) =>
    g.explorer ? { ...a, prompt: g.explorer.sequence } : a;
  if (
    g.explorer &&
    (!Number.isInteger(g.explorer.sequence) || g.explorer.sequence < 1)
  )
    return null;
  const paid = fishValue(room, ids);
  if (paid === null) return null;
  if (fishResponder(room) === room.you && f.canReplace) {
    if (kind === "catan_fish_keep" && ids.length === 0)
      return wrap({ type: kind });
    return kind === "catan_fish_replace" && ids.length === 1
      ? wrap({ type: kind, card: ids[0] })
      : null;
  }
  if (!fishTurn(room)) return null;
  if (kind === "catan_fish_boot")
    return !ids.length &&
      target !== null &&
      f.legal.bootTargets.includes(target)
      ? wrap({ type: kind, target })
      : null;
  const cost = f.legal.costs[kind];
  if (
    !f.legal.actions.includes(kind) ||
    !Number.isInteger(cost) ||
    cost < 1 ||
    paid < cost
  )
    return null;
  const action: Record<string, unknown> = { type: kind, tokens: [...ids] };
  if (kind === "catan_fish_resource") {
    if (
      color === null ||
      !Number.isInteger(color) ||
      color < 0 ||
      color > 4 ||
      !f.legal.resources.includes(color) ||
      !(g.bank[color] > 0)
    )
      return null;
    action.color = color;
  } else if (kind === "catan_fish_progress") {
    if (
      color === null ||
      !Number.isInteger(color) ||
      color < 0 ||
      color > 2 ||
      !g.citiesKnights ||
      !(f.legal.progressTracks || []).includes(color) ||
      !(g.citiesKnights.progressRemaining[color] > 0)
    )
      return null;
    action.color = color;
  } else if (kind === "catan_fish_steal") {
    if (target === null || !f.legal.targets.includes(target)) return null;
    action.target = target;
  } else if (kind === "catan_fish_bridge") {
    if (edge === null || !f.legal.bridges?.includes(edge)) return null;
    action.edge = edge;
  } else if (kind === "catan_fish_road") {
    if (edge === null || !f.legal.roads.includes(edge)) return null;
    action.edge = edge;
  } else if (kind === "catan_fish_ship") {
    if (g.explorer) {
      if (
        slot == null ||
        edge === null ||
        !(f.legal.shipBuilds || []).some(
          (b) => b.slot === slot && b.edge === edge,
        )
      )
        return null;
      action.slot = slot;
    } else if (
      !g.seafarers ||
      edge === null ||
      !(f.legal.ships || []).includes(edge)
    )
      return null;
    action.edge = edge;
  } else if (kind === "catan_fish_pirate") {
    if (g.explorer) {
      if (
        !g.explorer.pirate ||
        g.explorer.pirate.owner < 0 ||
        g.explorer.pirate.owner === room.you ||
        g.explorer.fleet.turn?.fishPirate
      )
        return null;
    } else if (
      !g.seafarers ||
      g.seafarers.pirate < 0 ||
      g.seafarers.wonders ||
      g.seafarers.pirateIslands
    )
      return null;
  } else if (kind === "catan_fish_voyage") {
    if (!g.explorer || slot == null || !(f.legal.voyages || []).includes(slot))
      return null;
    action.slot = slot;
  } else if (!["catan_fish_robber", "catan_fish_dev"].includes(kind))
    return null;
  return wrap(action);
}

// Orient the original right-facing ground toward its land-facing middle vertex.
// The opposite direction points out to sea, regardless of endpoint ordering.
export function fishGroundGeometry(g: CatanState, vertices: number[]) {
  const [a, b, c] = vertices.map((id) => g.vertices[id]);
  if (!a || !b || !c) return null;
  const dx = (a.x + c.x) / 2 - b.x,
    dy = (a.y + c.y) / 2 - b.y;
  const length = Math.hypot(dx, dy);
  if (!length) return null;
  const size = g.hexSize || 62;
  return {
    x: b.x,
    y: b.y,
    angle: (Math.atan2(dy, dx) * 180) / Math.PI - 180,
    scale: size / Math.hypot(66, 116.5),
    labelX: b.x + (dx / length) * size * 0.62,
    labelY: b.y + (dy / length) * size * 0.62,
  };
}
