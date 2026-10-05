import type { CatanState, CatanTwoChoice, Room } from "./types";

export type TwoSelection = {
  sequence: number;
  edge: number;
  vertex: number;
  owner: number | null;
};

export function twoResponder(room: Room) {
  const q = room.game?.catan?.two;
  return room.status === "playing" &&
    !room.game?.finished &&
    q &&
    (q.pending || q.trade) &&
    q.actor >= 0
    ? q.actor
    : undefined;
}
export function twoCanRespond(room: Room) {
  return (
    !room.spectating &&
    room.you >= 0 &&
    twoResponder(room) === room.you &&
    !!room.game?.catan?.two?.canAct &&
    !room.game.catan.players[room.you]?.eliminated
  );
}
export function twoChoices(g: CatanState, selected: TwoSelection | null) {
  if (!selected || !g.two?.pending || selected.sequence !== g.two.sequence)
    return [];
  return (g.two.choices || []).filter(
    (c) => c.edge === selected.edge && c.vertex === selected.vertex,
  );
}
export function twoPick(
  g: CatanState,
  edge: number,
  vertex: number,
): TwoSelection | null {
  const choices = (g.two?.choices || []).filter(
    (c) => c.edge === edge && c.vertex === vertex,
  );
  return g.two?.pending && choices.length
    ? {
        sequence: g.two.sequence,
        edge,
        vertex,
        owner: choices.length === 1 ? choices[0].owner : null,
      }
    : null;
}
export function twoSelected(
  g: CatanState,
  selected: TwoSelection | null,
): CatanTwoChoice | undefined {
  return twoChoices(g, selected).find((c) => c.owner === selected?.owner);
}
export function twoReturnValid(hand: number[], give: number[]) {
  return (
    give.length === 5 &&
    give.every(
      (n, i) => Number.isInteger(n) && n >= 0 && n <= (hand[i] || 0),
    ) &&
    give.reduce((a, b) => a + b, 0) === 2
  );
}
export const twoNeutralName = (owner: number) => `中立势力 ${-owner - 1}`;
