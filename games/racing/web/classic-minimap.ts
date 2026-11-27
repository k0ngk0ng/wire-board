import type { Point } from "./types";

export const minimapMarkerColors = [
  "#ff4757",
  "#2e9bff",
  "#2ed573",
  "#ffc312",
  "#c56cf0",
  "#ff7f2a",
  "#18dcff",
  "#ff6eb4",
] as const;

export function classicSeatColor(seat: number): string {
  const index = Math.trunc(Number.isFinite(seat) ? seat : 0);
  const length = minimapMarkerColors.length;
  return minimapMarkerColors[((index % length) + length) % length];
}

export type MinimapBounds = {
  minX: number;
  maxX: number;
  minY: number;
  maxY: number;
};

export type MinimapTransform = {
  scale: number;
  offsetX: number;
  offsetY: number;
};

export function minimapBounds(points: readonly Point[]): MinimapBounds {
  if (!points.length) return { minX: 0, maxX: 1, minY: 0, maxY: 1 };
  let minX = Infinity;
  let maxX = -Infinity;
  let minY = Infinity;
  let maxY = -Infinity;
  for (const p of points) {
    if (p.x < minX) minX = p.x;
    if (p.x > maxX) maxX = p.x;
    if (p.y < minY) minY = p.y;
    if (p.y > maxY) maxY = p.y;
  }
  return { minX, maxX, minY, maxY };
}

export function minimapTransform(
  bounds: MinimapBounds,
  width: number,
  height: number,
  pad: number,
): MinimapTransform {
  const spanX = Math.max(1, bounds.maxX - bounds.minX);
  const spanY = Math.max(1, bounds.maxY - bounds.minY);
  const usableW = Math.max(1, width - pad * 2);
  const usableH = Math.max(1, height - pad * 2);
  const scale = Math.min(usableW / spanX, usableH / spanY);
  const offsetX = (width - spanX * scale) / 2 - bounds.minX * scale;
  const offsetY = (height - spanY * scale) / 2 - bounds.minY * scale;
  return { scale, offsetX, offsetY };
}

export function orderedMinimapCars<T extends { seat: number }>(
  cars: readonly T[],
  followSeat: number,
): T[] {
  const others: T[] = [];
  let self: T | undefined;
  for (const car of cars) {
    if (car.seat === followSeat) self = car;
    else others.push(car);
  }
  return self ? [...others, self] : others;
}
