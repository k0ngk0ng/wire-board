import type { Room } from "./types";

export type AttackSelection = {
  from: number | null;
  target: number | null;
  wheat: boolean;
  sources: number[];
  destinations: number[];
};
export const emptyAttackSelection = (): AttackSelection => ({
  from: null,
  target: null,
  wheat: false,
  sources: [],
  destinations: [],
});
export const attackCardNames: Record<string, string> = {
  capture: "俘获",
  knighthood: "授勋",
  swift_knight: "迅捷骑士",
  treason: "叛变",
};
export function attackCanRespond(room: Room) {
  const g = room.game?.catan,
    a = g?.attack;
  return !!(
    a?.canAct &&
    room.status === "playing" &&
    !room.game?.finished &&
    !room.spectating &&
    room.you >= 0 &&
    !g?.players[room.you]?.eliminated &&
    !room.seats[room.you]?.autoPlay
  );
}
export function attackChoices(room: Room, pick: AttackSelection) {
  const a = room.game?.catan?.attack;
  if (!a || !attackCanRespond(room))
    return { tiles: [] as number[], edges: [] as number[], sources: false };
  if (a.endPlan) {
    const move = a.moveChoices?.find((c) => c.from === pick.from);
    return {
      tiles: [],
      edges: (pick.wheat ? move?.wheat : move?.normal) || [],
      sources: false,
    };
  }
  if (a.pending?.card === "capture")
    return { tiles: a.targets || [], edges: [], sources: false };
  if (a.pending?.card === "treason") {
    const sources = pick.sources.length < (a.fromBoard || 0);
    if (a.treasonPlans) {
      const plans = a.treasonPlans.filter((plan) =>
        pick.sources.every((id) => plan.sources.includes(id)),
      );
      return {
        tiles: [
          ...new Set(
            plans.flatMap((plan) =>
              sources
                ? plan.sources.filter(
                    (id) => id >= 0 && !pick.sources.includes(id),
                  )
                : plan.destinations,
            ),
          ),
        ],
        edges: [],
        sources,
      };
    }
    return {
      tiles: sources
        ? a.sources || []
        : (a.destinations || []).filter((id) => !pick.sources.includes(id)),
      edges: [],
      sources,
    };
  }
  return { tiles: [], edges: a.edges || [], sources: false };
}
export function attackSelectedAction(
  room: Room,
  pick: AttackSelection,
): Record<string, unknown> | null {
  const a = room.game?.catan?.attack;
  if (!a || !attackCanRespond(room)) return null;
  const legal = attackChoices(room, pick);
  if (a.endPlan) {
    return pick.from !== null &&
      pick.target !== null &&
      legal.edges.includes(pick.target)
      ? {
          type: "catan_attack_move",
          prompt: a.endPlan.id,
          choice: pick.wheat ? "wheat" : "normal",
          edge: pick.from,
          target: pick.target,
        }
      : null;
  }
  const q = a.pending;
  if (!q) return null;
  const base = { type: "catan_attack_card", prompt: q.id, choice: q.card };
  if (q.card === "treason") {
    const from = pick.sources,
      to = pick.destinations;
    const count = a.treasonCount ?? 2;
    if (
      from.length !== (a.fromBoard || 0) ||
      new Set(from).size !== from.length ||
      from.some((id) => !a.sources?.includes(id)) ||
      to.length !== count ||
      new Set(to).size !== count ||
      to.some((id) => !a.destinations?.includes(id) || from.includes(id))
    )
      return null;
    if (
      a.treasonPlans &&
      !a.treasonPlans.some(
        (plan) =>
          from.every((id) => plan.sources.includes(id)) &&
          to.every((id) => plan.destinations.includes(id)),
      )
    )
      return null;
    return {
      ...base,
      give: [...from, ...Array(count - from.length).fill(-1)],
      take: to,
    };
  }
  return pick.target !== null &&
    (legal.tiles.includes(pick.target) || legal.edges.includes(pick.target))
    ? {
        ...base,
        ...(q.card === "capture"
          ? { tile: pick.target }
          : { edge: pick.target }),
      }
    : null;
}
