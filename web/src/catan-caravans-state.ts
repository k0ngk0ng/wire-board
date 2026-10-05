import type { CatanState, CatanWagon, Room } from "./types";

export type CaravanSelection = { edge: number; from: number | null };

export function caravanResponder(room: Room) {
  const c = room.game?.catan?.caravans;
  return room.status === "playing" &&
    !room.game?.finished &&
    c?.pending &&
    c.actor >= 0
    ? c.actor
    : undefined;
}

export function caravanCanRespond(room: Room) {
  return (
    !room.spectating &&
    room.you >= 0 &&
    caravanResponder(room) === room.you &&
    !!room.game?.catan?.caravans?.canAct &&
    !room.game.catan.players[room.you]?.eliminated
  );
}

export function caravanGeometry(g: CatanState, w: CatanWagon) {
  const e = g.edges[w.edge];
  if (!e || (w.from !== e.a && w.from !== e.b)) return null;
  const from = g.vertices[w.from],
    to = g.vertices[e.a === w.from ? e.b : e.a];
  if (!from || !to) return null;
  const dx = to.x - from.x,
    dy = to.y - from.y;
  const length = Math.hypot(dx, dy);
  if (!Number.isFinite(length) || length <= 0) return null;
  return {
    x: (from.x + to.x) / 2,
    y: (from.y + to.y) / 2,
    rotation: (Math.atan2(dy, dx) * 180) / Math.PI - 90,
    from,
    to,
    dx: dx / length,
    dy: dy / length,
    direction:
      Math.abs(dx) < 0.01
        ? dy > 0
          ? "朝南"
          : "朝北"
        : dx > 0
          ? dy > 0
            ? "朝东南"
            : "朝东北"
          : dy > 0
            ? "朝西南"
            : "朝西北",
  };
}

export function caravanPick(
  g: CatanState,
  edge: number,
): CaravanSelection | null {
  const choices = g.caravans?.choices?.filter((w) => w.edge === edge) || [];
  return choices.length
    ? { edge, from: choices.length === 1 ? choices[0].from : null }
    : null;
}

export function caravanSelected(
  g: CatanState,
  selected: CaravanSelection | null,
) {
  return (g.caravans?.choices || []).find(
    (w) => w.edge === selected?.edge && w.from === selected?.from,
  );
}

export function caravanBonus(g: CatanState, vertex: number) {
  return (
    (g.caravans?.wagons || []).filter((w) => {
      const e = g.edges[w.edge];
      return e && (e.a === vertex || e.b === vertex);
    }).length >= 2
  );
}

// Animate only one confirmed placement between consecutive public snapshots.
// Initial loads, missed updates and a different viewer must never replay moves.
export function caravanAdded(before: Room, after: Room): CatanWagon | null {
  const a = before.game?.catan,
    b = after.game?.catan,
    old = a?.caravans,
    next = b?.caravans;
  if (
    !a ||
    !b ||
    !old ||
    !next ||
    before.id !== after.id ||
    before.you !== after.you ||
    !!before.spectating !== !!after.spectating ||
    before.status !== "playing" ||
    !["playing", "finished"].includes(after.status) ||
    after.version !== before.version + 1 ||
    a.setupStep < (a.setupLimit ?? a.players.length * 2) ||
    b.setupStep < (b.setupLimit ?? b.players.length * 2) ||
    a.rollId !== b.rollId ||
    old.sequence !== next.sequence ||
    !old.pending ||
    !["vote", "place"].includes(old.pending.kind) ||
    next.pending ||
    next.wagons.length !== old.wagons.length + 1 ||
    !old.wagons.every(
      (w, i) =>
        next.wagons[i]?.edge === w.edge && next.wagons[i]?.from === w.from,
    )
  )
    return null;
  const added = next.wagons.at(-1)!;
  return old.choices?.some(
    (w) => w.edge === added.edge && w.from === added.from,
  ) && caravanGeometry(b, added)
    ? added
    : null;
}
