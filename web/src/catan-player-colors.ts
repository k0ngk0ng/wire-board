import type { CatanState } from "./types";

export const catanPlayerColors = [
  "#3078be",
  "#c94737",
  "#f5eee1",
  "#e5902e",
  "#794287",
  "#388146",
];
export const catanPieceColors = [
  "blue",
  "red",
  "white",
  "orange",
  "purple",
  "green",
];

export function catanColorIndex(game: CatanState, seat: number) {
  return game.seafarers?.pirateIslands?.colors[seat] ?? seat;
}

export function catanSeatColor(game: CatanState, seat: number) {
  return catanPlayerColors[catanColorIndex(game, seat)];
}
