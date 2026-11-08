// oft.cpp::oft_test / the source web client's BATTLE_SLOT_POINTS and
// BATTLE_MONSTER_PLACE_NO. Direction 3 faces upper-left, 7 lower-right.
const points = [
  [453, 432],
  [485, 408],
  [517, 384],
  [549, 360],
  [581, 336],
  [293, 408],
  [357, 360],
  [421, 312],
  [485, 264],
  [549, 216],
  [37, 198],
  [69, 174],
  [101, 150],
  [133, 126],
  [165, 102],
  [69, 298],
  [133, 250],
  [197, 202],
  [261, 154],
  [325, 106],
];
const place = [
  2, 1, 3, 0, 4, 7, 6, 8, 5, 9, 12, 11, 13, 10, 14, 17, 16, 18, 15, 19,
];
export function facing(team: number) {
  return team === 0 ? 3 : 7;
}
export function position(team: number, row: number, pet: number) {
  const [x, y] = points[place[(team === 0 ? 0 : 10) + pet * 5 + row]];
  return { x: (x / 640) * 100, y: (y / 480) * 100 };
}
export function eventPosition(
  players: readonly { team: number; seat: number }[],
  target: number,
) {
  if (!Number.isInteger(target) || target < 0 || target >= players.length * 2)
    return undefined;
  const player = players[Math.floor(target / 2)];
  const row = players.filter(
    (p) => p.team === player.team && p.seat < player.seat,
  ).length;
  return position(player.team, row, target % 2);
}
