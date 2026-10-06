import type { Room, CatanTransport, CatanState } from "./types";
export type TransportPick = { edge: number | null; piece: number | null };
export const emptyTransportPick = (): TransportPick => ({
  edge: null,
  piece: null,
});
export const transportSites: Record<string, string> = {
  quarry: "采石场",
  glassworks: "玻璃工坊",
  castle: "城堡",
};
export const transportCargo: Record<string, string> = {
  marble: "大理石",
  sand: "沙",
  glass: "玻璃",
  tools: "工具",
};
export function transportWagonPosition(game: CatanState, seat: number) {
  const wagons = game.transport?.state.wagons,
    wagon = wagons?.[seat];
  if (!wagons || !wagon || wagon.position < 0) return null;
  const vertex = game.vertices[wagon.position];
  if (!vertex) return null;
  const peers = wagons
      .map((w, i) => (w.position === wagon.position ? i : -1))
      .filter((i) => i >= 0),
    offset =
      ((peers.indexOf(seat) - (peers.length - 1) / 2) *
        21 *
        (game.hexSize || 62)) /
      62;
  return { x: vertex.x + offset, y: vertex.y };
}
export function transportCanAct(room: Room) {
  const g = room.game?.catan;
  return !!(
    g?.transport?.canAct &&
    room.status === "playing" &&
    !room.game?.finished &&
    !room.spectating &&
    room.you >= 0 &&
    room.game?.turn === room.you &&
    !g.players[room.you]?.eliminated &&
    !room.seats[room.you]?.autoPlay
  );
}
export function transportEdges(room: Room): number[] {
  const t = room.game?.catan?.transport;
  if (!t || !transportCanAct(room)) return [];
  if (t.barbarianPending || (t.state.travel?.pending ?? -1) >= 0)
    return t.choices.relocate || [];
  return (t.choices.steps || []).map((s) => s.edge);
}
export function transportSelectedAction(
  room: Room,
  pick: TransportPick,
): Record<string, unknown> | null {
  const t = room.game?.catan?.transport;
  if (!t || pick.edge === null || !transportEdges(room).includes(pick.edge))
    return null;
  if (t.barbarianPending)
    return pick.piece !== null && pick.piece >= 0 && pick.piece < 3
      ? {
          type: "catan_transport_barbarian",
          offer: t.barbarianSequence,
          card: pick.piece,
          edge: pick.edge,
        }
      : null;
  return {
    type:
      (t.state.travel?.pending ?? -1) >= 0
        ? "catan_transport_relocate"
        : "catan_transport_step",
    offer: t.state.sequence,
    edge: pick.edge,
  };
}

export function transportStepDescription(
  room: Room,
  step: NonNullable<CatanTransport["choices"]["steps"]>[number],
): string {
  const cost = `${step.mp}移动点`;
  if (!step.toll) return `${cost} · 无过路费`;
  const toBank =
    (step.bank || 0) > 0 ||
    (step.pay >= 0 && room.game?.catan?.players[step.pay]?.eliminated);
  const recipient = toBank ? "银行" : room.seats[step.pay]?.name || "对手";
  return `${cost} · ${step.toll}金币付给${recipient}${step.neutral ? "（中立道路）" : ""}`;
}

export type TransportArrivalMotion = {
  id: string;
  player: number;
  site: number;
  delivered?: string;
  loaded?: string;
  gold: number;
};

// Use only consecutive public snapshots. Loading a room or catching up after
// reconnect must not replay an old delivery or reveal a supply stack's order.
export function transportArrivalMotion(
  before: Room,
  after: Room,
): TransportArrivalMotion | null {
  const a = before.game?.catan?.transport,
    b = after.game?.catan?.transport,
    travel = a?.state.travel;
  if (
    !a ||
    !b ||
    !travel ||
    !travel.ended ||
    travel.arrived < 0 ||
    a.state.arrivalResolved ||
    before.game?.phase !== "catan_transport_move" ||
    before.id !== after.id ||
    before.you !== after.you ||
    !!before.spectating !== !!after.spectating ||
    before.status !== "playing" ||
    !["playing", "finished"].includes(after.status) ||
    after.version !== before.version + 1 ||
    b.state.sequence < a.state.sequence ||
    b.state.sequence > a.state.sequence + 1
  )
    return null;
  const player = travel.player,
    old = a.state.wagons[player],
    next = b.state.wagons[player],
    site = a.map.sites[travel.arrived];
  if (
    !old ||
    !next ||
    !site ||
    old.position !== site.center ||
    next.position !== old.position ||
    old.level !== next.level ||
    b.map.sites[travel.arrived]?.center !== site.center
  )
    return null;
  const delivered = next.delivered - old.delivered;
  if (delivered !== 0 && delivered !== 1) return null;
  if (delivered && (!old.cargo || next.cargo?.id === old.cargo.id)) return null;
  if (!delivered && old.cargo) return null;
  const loaded =
    next.cargo && next.cargo.id !== old.cargo?.id ? next.cargo : undefined;
  if (!delivered && !loaded) return null;
  const gold = next.gold - old.gold,
    origin = ["quarry", "glassworks", "castle"].indexOf(site.kind);
  if (
    origin < 0 ||
    gold !== (delivered ? old.level + 1 : 0) ||
    (loaded && loaded.origin !== site.kind) ||
    a.state.supply.some(
      (count, i) =>
        b.state.supply[i] !== count - (loaded && i === origin ? 1 : 0),
    )
  )
    return null;
  return {
    id: `${after.id}:${a.state.sequence}:${player}`,
    player,
    site: travel.arrived,
    delivered: delivered ? old.cargo!.cargo : undefined,
    loaded: loaded?.cargo,
    gold,
  };
}
