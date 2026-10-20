export type Point = { x: number; y: number };
export type Aim = { angle: number; facing: 1 | -1 };
export type Camera = { x: number; y: number; worldViewW: number; worldViewH: number };
export type PreviewInput = {
  width: number;
  height: number;
  mask: Uint8Array;
  maskCols: number;
  maskRows: number;
  cell: number;
  startX: number;
  startY: number;
  angle: number;
  power: number;
  facing: number;
  rangeScale: number;
  wind: number;
  limit?: number;
};

export function clamp(value: number, low: number, high: number) {
  return Math.max(low, Math.min(high, value));
}

/** Match the server's muzzle geometry (simulateArc starts at y - 26). */
export function aimFromPoint(
  tank: { x: number; y: number },
  point: Point,
  maxAngle: number,
  muzzleY = 24,
): Aim {
  const dx = point.x - tank.x;
  const dy = tank.y - muzzleY - point.y;
  const angle = clamp((Math.atan2(Math.max(0, dy), Math.max(1, Math.abs(dx))) * 180) / Math.PI, 0, maxAngle);
  return { angle, facing: dx < 0 ? -1 : 1 };
}

export function stepPower(value: number, delta: number) {
  if (!delta) return clamp(value, 20, 100);
  return clamp(value + (delta < 0 ? 2 : -2), 20, 100);
}

export function pointerWorld(
  clientX: number,
  clientY: number,
  rect: { left: number; top: number; width: number; height: number },
  camera: Camera,
): Point {
  return {
    x: camera.x + ((clientX - rect.left) / Math.max(1, rect.width)) * camera.worldViewW,
    y: camera.y + ((clientY - rect.top) / Math.max(1, rect.height)) * camera.worldViewH,
  };
}

export function decodeMask(value: string): Uint8Array {
  if (!value) return new Uint8Array(0);
  const raw = atob(value);
  const bytes = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
  return bytes;
}

export function carveCraters(
  source: Uint8Array,
  cols: number,
  rows: number,
  cell: number,
  craters: { x: number; y: number; r: number }[],
): Uint8Array {
  const mask = source.slice();
  for (const crater of craters) {
    if (crater.r <= 0) continue;
    const fromX = Math.max(0, Math.floor((crater.x - crater.r) / cell));
    const toX = Math.min(cols - 1, Math.floor((crater.x + crater.r) / cell) + 1);
    const fromY = Math.max(0, Math.floor((crater.y - crater.r) / cell));
    const toY = Math.min(rows - 1, Math.floor((crater.y + crater.r) / cell) + 1);
    for (let cy = fromY; cy <= toY; cy++) {
      for (let cx = fromX; cx <= toX; cx++) {
        const px = cx * cell + cell / 2;
        const py = cy * cell + cell / 2;
        if (Math.hypot(px - crater.x, py - crater.y) > crater.r) continue;
        const index = cy * cols + cx;
        mask[index >> 3] &= ~(1 << (7 - (index & 7)));
      }
    }
  }
  return mask;
}

export function solidAt(
  mask: Uint8Array,
  cols: number,
  rows: number,
  cell: number,
  x: number,
  y: number,
  width: number,
  height: number,
) {
  if (x < 0 || y < 0 || x >= width || y >= height) return false;
  const cx = Math.floor(x / cell);
  const cy = Math.floor(y / cell);
  if (cx < 0 || cy < 0 || cx >= cols || cy >= rows) return false;
  const index = cy * cols + cx;
  return (mask[index >> 3] & (1 << (7 - (index & 7)))) !== 0;
}

/**
 * Client-side mirror of the server's ballistic simulation. It is only a
 * cosmetic aiming aid: the server still resolves every shot authoritatively.
 */
export function previewPath(input: PreviewInput): Point[] {
  const speed =
    (120 + clamp(input.power, 20, 100) * 7.0) * clamp(input.rangeScale, 0.7, 1.3);
  const rad = (clamp(input.angle, 0, 90) * Math.PI) / 180;
  const facing = input.facing < 0 ? -1 : 1;
  let vx = Math.cos(rad) * speed * facing;
  let vy = -Math.sin(rad) * speed;
  let x = input.startX + facing * 10;
  let y = input.startY - 26;
  const path: Point[] = [{ x, y }];
  const limit = input.limit ?? 400;
  const dt = 1 / 240;
  const windAcc = input.wind * 14;
  for (let step = 0; step < 1800 && path.length < limit; step++) {
    vx += windAcc * dt;
    vy += 260 * dt;
    const nx = x + vx * dt;
    const ny = y + vy * dt;
    const distance = Math.hypot(nx - x, ny - y);
    const sub = Math.max(1, Math.ceil(distance / 2));
    for (let s = 1; s <= sub; s++) {
      const t = s / sub;
      const px = x + (nx - x) * t;
      const py = y + (ny - y) * t;
      if (px < -60 || px > input.width + 60 || py > input.height + 320 || py < -240) {
        if (s === sub) path.push({ x: px, y: py });
        return path;
      }
      if (
        solidAt(
          input.mask,
          input.maskCols,
          input.maskRows,
          input.cell,
          px,
          py,
          input.width,
          input.height,
        )
      ) {
        path.push({ x: px, y: py });
        return path;
      }
    }
    x = nx;
    y = ny;
    if (step % 4 === 0) path.push({ x, y });
  }
  return path;
}
