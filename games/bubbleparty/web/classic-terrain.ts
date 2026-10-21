import catalog from "../data/classic-2002.json" with { type: "json" };

// Canvas coordinates currently use 24px cells; original resources use 40px.
// Callers can pass their cell size without changing source geometry or anchors.
export const CLASSIC_CELL = 24;
const COLS = 15,
  ROWS = 13,
  NATIVE_CELL = 40;
type NativeMap = {
  id: string;
  width: number;
  height: number;
  family: string;
  ground: number[][];
  objects: number[][];
};
type Sequence = { name: string; resource: string; framePairsRaw: number[][] };
type Tile = {
  id: number;
  category: string;
  movable?: boolean;
  sequences: Sequence[];
};
type Sprite = { name: string; file: string; framesRaw: number[][] };
export type ClassicTerrain = NativeMap & { offsetX: number; offsetY: number };
export type ClassicImages = Record<string, HTMLImageElement | null | undefined>;
export type ClassicView = { nativeObjects?: number[] };
export type ClassicDrawOptions = {
  cellSize?: number;
  reduced?: boolean;
  opacity?: number;
};
export type ClassicObject = {
  id: number;
  x: number;
  y: number;
  depth: number;
  passable: boolean;
};
const maps = new Map(
  (catalog.maps as NativeMap[]).map((map) => [
    map.id,
    {
      ...map,
      offsetX: Math.floor((COLS - map.width) / 2),
      offsetY: Math.floor((ROWS - map.height) / 2),
    },
  ]),
);
const tiles = new Map((catalog.tiles as Tile[]).map((tile) => [tile.id, tile]));
const sprites = new Map(
  (catalog.sprites as Sprite[]).map((sprite) => [sprite.name, sprite]),
);
const pathFor = (sprite: Sprite) =>
  `/bubbleparty/v4/classic/sprites/${sprite.file}`;

export function classicTerrain(mapId: string): ClassicTerrain | null {
  return maps.get(mapId) ?? null;
}
export function classicSpritePaths(mapId: string): string[] {
  const map = classicTerrain(mapId);
  if (!map) return [];
  const paths = new Set<string>();
  for (const id of [...map.ground.flat(), ...map.objects.flat()]) {
    const sequence = tiles.get(id)?.sequences.find((s) => s.name === "Default");
    const sprite = sequence && sprites.get(sequence.resource);
    if (sprite) paths.add(pathFor(sprite));
  }
  return [...paths].sort();
}
export function classicSpriteSources(base: string, path: string): string[] {
  return base ? [`${base.replace(/\/$/, "")}${path}`, path] : [path];
}
export function classicFrame(
  objectID: number,
  now: number,
  options: ClassicDrawOptions = {},
) {
  const sequence = tiles
    .get(objectID)
    ?.sequences.find((s) => s.name === "Default");
  const sprite = sequence && sprites.get(sequence.resource);
  if (!sequence || !sprite || !sequence.framePairsRaw.length) return null;
  // Relative durations are preserved. 30ms/unit is a reconstruction parameter
  // from the confirmed logical clock, not a measured native animation frequency.
  const pairs = sequence.framePairsRaw;
  const total = pairs.reduce((sum, pair) => sum + Math.max(1, pair[1]), 0);
  let tick = options.reduced
    ? 0
    : Math.floor(Math.max(0, Number.isFinite(now) ? now : 0) / 30) % total;
  let index = pairs[0][0];
  for (const pair of pairs) {
    index = pair[0];
    if (tick < Math.max(1, pair[1])) break;
    tick -= Math.max(1, pair[1]);
  }
  const frame = sprite.framesRaw[index];
  if (!frame || frame[2] <= frame[0] || frame[3] <= frame[1]) return null;
  return {
    path: pathFor(sprite),
    index,
    sx: frame[0],
    sy: frame[1],
    sw: frame[2] - frame[0],
    sh: frame[3] - frame[1],
    ax: frame[4],
    ay: frame[5],
  };
}
export function classicObjectEntries(
  view: ClassicView,
  mapId: string,
): ClassicObject[] {
  const map = classicTerrain(mapId);
  if (!map) return [];
  const result: ClassicObject[] = [];
  for (let y = 0; y < ROWS; y++)
    for (let x = 0; x < COLS; x++) {
      // An explicit server projection always wins, including an empty projection.
      // Never resurrect destroyed or moved boxes from the initial layout.
      const id =
        view.nativeObjects !== undefined
          ? (view.nativeObjects[y * COLS + x] ?? -1)
          : (map.objects[y - map.offsetY]?.[x - map.offsetX] ?? -1);
      if (id < 0) continue;
      const passable = tiles.get(id)?.category === "passable";
      result.push({ id, x, y, depth: y + (passable ? 0.85 : 0.4), passable });
    }
  return result.sort((a, b) => a.depth - b.depth || a.x - b.x);
}
export function classicObjectOpacity(
  objectID: number,
  x: number,
  y: number,
  players: readonly { x: number; y: number }[],
): number {
  if (tiles.get(objectID)?.category !== "passable") return 1;
  return players.some((p) => Math.abs(p.x - x) < 0.7 && Math.abs(p.y - y) < 0.7)
    ? 0.5
    : 1;
}
function drawFrame(
  ctx: CanvasRenderingContext2D,
  objectID: number,
  x: number,
  y: number,
  images: ClassicImages,
  now: number,
  options: ClassicDrawOptions,
) {
  const frame = classicFrame(objectID, now, options);
  const image = frame && images[frame.path];
  if (!frame || !image || !image.complete || image.naturalWidth <= 0)
    return false;
  const scale = (options.cellSize ?? CLASSIC_CELL) / NATIVE_CELL;
  ctx.drawImage(
    image,
    frame.sx,
    frame.sy,
    frame.sw,
    frame.sh,
    (x * NATIVE_CELL - frame.ax) * scale,
    (y * NATIVE_CELL + 39 - frame.ay) * scale,
    frame.sw * scale,
    frame.sh * scale,
  );
  return true;
}
export function drawClassicGround(
  ctx: CanvasRenderingContext2D,
  _view: ClassicView,
  mapId: string,
  images: ClassicImages,
  now: number,
  options: ClassicDrawOptions = {},
): boolean {
  const map = classicTerrain(mapId);
  if (!map) return false;
  const cell = options.cellSize ?? CLASSIC_CELL;
  ctx.save();
  // Village00's extra ring is deliberately distinct and never walkable.
  ctx.fillStyle = "#344b50";
  ctx.fillRect(0, 0, COLS * cell, ROWS * cell);
  ctx.beginPath();
  ctx.rect(
    map.offsetX * cell,
    map.offsetY * cell,
    map.width * cell,
    map.height * cell,
  );
  ctx.clip();
  const groundColors: Record<string, string> = {
    Desert: "#bfa675",
    Ice: "#b3c9cc",
    Xmas: "#c2d1cc",
    Ocean: "#598d9e",
    Patrit: "#998465",
    Factory: "#919891",
    Mine: "#80776f",
    Forest: "#7a9863",
    Village: "#8aab69",
  };
  ctx.fillStyle = groundColors[map.family] ?? "#8a9c83";
  ctx.fillRect(
    map.offsetX * cell,
    map.offsetY * cell,
    map.width * cell,
    map.height * cell,
  );
  for (let y = 0; y < map.height; y++)
    for (let x = 0; x < map.width; x++) {
      const id = map.ground[y][x],
        px = x + map.offsetX,
        py = y + map.offsetY;
      if (id >= 0 && !drawFrame(ctx, id, px, py, images, now, options)) {
        ctx.strokeStyle = "#344b5066";
        ctx.lineWidth = 0.5;
        ctx.strokeRect(px * cell, py * cell, cell, cell);
      }
    }
  ctx.restore();
  return true;
}
export function drawClassicObject(
  ctx: CanvasRenderingContext2D,
  objectID: number,
  x: number,
  y: number,
  images: ClassicImages,
  now: number,
  options: ClassicDrawOptions = {},
): boolean {
  if (objectID < 0) return false;
  ctx.save();
  ctx.globalAlpha *= Math.max(0, Math.min(1, options.opacity ?? 1));
  const drawn = drawFrame(ctx, objectID, x, y, images, now, options);
  if (!drawn) {
    const cell = options.cellSize ?? CLASSIC_CELL;
    const tile = tiles.get(objectID),
      category = tile?.category;
    const px = x * cell,
      py = y * cell;
    ctx.fillStyle =
      category === "passable"
        ? "#398052"
        : category === "box"
          ? "#c9944c"
          : "#6c7a87";
    ctx.fillRect(px + 1, py + 1, cell - 2, cell - 2);
    ctx.strokeStyle = "#283d43";
    ctx.lineWidth = 1.5;
    ctx.strokeRect(px + 1, py + 1, cell - 2, cell - 2);
    ctx.beginPath();
    if (category === "box") {
      ctx.moveTo(px + 4, py + 4);
      ctx.lineTo(px + cell - 4, py + cell - 4);
      ctx.moveTo(px + cell - 4, py + 4);
      ctx.lineTo(px + 4, py + cell - 4);
      if (tile?.movable) {
        ctx.moveTo(px + 4, py + cell / 2);
        ctx.lineTo(px + cell - 4, py + cell / 2);
      }
    } else if (category === "passable") {
      ctx.moveTo(px + cell / 2, py + cell - 3);
      ctx.lineTo(px + cell / 2, py + 4);
      ctx.moveTo(px + 4, py + 7);
      ctx.lineTo(px + cell / 2, py + cell / 2);
      ctx.lineTo(px + cell - 4, py + 7);
    } else {
      ctx.moveTo(px + 2, py + cell / 2);
      ctx.lineTo(px + cell - 2, py + cell / 2);
    }
    ctx.stroke();
  }
  ctx.restore();
  return drawn;
}

export async function loadClassicSprites(
  mapId: string,
  base = "",
  signal?: AbortSignal,
): Promise<{ images: ClassicImages; loaded: number; failed: string[] }> {
  const images: ClassicImages = {};
  const paths = classicSpritePaths(mapId);
  const failed = await Promise.all(
    paths.map(async (path) => {
      for (const source of classicSpriteSources(base, path)) {
        if (signal?.aborted || typeof Image === "undefined") break;
        const image = new Image();
        const loaded = await new Promise<boolean>((resolve) => {
          const finish = (success: boolean) => {
            image.onload = null;
            image.onerror = null;
            signal?.removeEventListener("abort", abort);
            resolve(success);
          };
          const abort = () => {
            finish(false);
            image.src = "";
          };
          image.onload = () => finish(image.naturalWidth > 0);
          image.onerror = () => finish(false);
          signal?.addEventListener("abort", abort, { once: true });
          image.src = source;
        });
        if (loaded && !signal?.aborted) {
          images[path] = image;
          return null;
        }
      }
      return path;
    }),
  );
  return {
    images,
    loaded: Object.keys(images).length,
    failed: failed.filter((path): path is string => path !== null),
  };
}
