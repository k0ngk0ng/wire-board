import type { Room, CatanTransport } from "./types";
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
