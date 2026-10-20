// Source and exact original file digests: ../assets/manifest-v2.json.
export const spriteFrames = {
  role1: { file: "Role1.png", width: 48, height: 64, columns: 6, rows: 4 },
  role2: { file: "Role2.png", width: 56, height: 67, columns: 6, rows: 4 },
  trapped1: {
    file: "Role1Ani.png",
    width: 48,
    height: 64,
    columns: 8,
    rows: 1,
  },
  trapped2: {
    file: "Role2Ani.png",
    width: 56,
    height: 70,
    columns: 8,
    rows: 1,
  },
  death1: {
    file: "Role1Die.png",
    width: 48,
    height: 100,
    columns: 11,
    rows: 1,
  },
  death2: { file: "Role2Die.png", width: 56, height: 98, columns: 11, rows: 1 },
  bubble: { file: "Popo.png", width: 44, height: 41, columns: 3, rows: 1 },
  bigBubble: {
    file: "BigPopo.png",
    width: 72,
    height: 72,
    columns: 9,
    rows: 1,
  },
  blast: { file: "Explosion.png", width: 40, height: 40, columns: 14, rows: 5 },
  ground: {
    file: "TownGround.png",
    width: 40,
    height: 40,
    columns: 7,
    rows: 1,
  },
  houseRed: {
    file: "TownHouseRed.png",
    width: 40,
    height: 57,
    columns: 1,
    rows: 1,
  },
  houseBlue: {
    file: "TownHouseBlue.png",
    width: 40,
    height: 57,
    columns: 1,
    rows: 1,
  },
  houseYellow: {
    file: "TownHouseYellow.png",
    width: 40,
    height: 57,
    columns: 1,
    rows: 1,
  },
  tree: { file: "TownTree.png", width: 40, height: 67, columns: 1, rows: 1 },
  hedge: { file: "TownBush.png", width: 40, height: 57, columns: 1, rows: 1 },
  brickRed: {
    file: "TownBlockRed.png",
    width: 40,
    height: 44,
    columns: 1,
    rows: 1,
  },
  brickOrange: {
    file: "TownBlockYellow.png",
    width: 40,
    height: 44,
    columns: 1,
    rows: 1,
  },
  crate: { file: "TownBox.png", width: 40, height: 44, columns: 1, rows: 1 },
  itemBomb: { file: "Gift1.png", width: 42, height: 45, columns: 3, rows: 1 },
  itemSpeed: { file: "Gift2.png", width: 42, height: 45, columns: 3, rows: 1 },
  itemRange: { file: "Gift3.png", width: 42, height: 45, columns: 3, rows: 1 },
} as const;
export type SpriteName = keyof typeof spriteFrames;
export const spriteSourceCandidates = (
  base: string,
  file: string,
): string[] => {
  const local = `/bubbleparty/v2/sprites/${file}`;
  return base ? [`${base.replace(/\/$/, "")}${local}`, local] : [local];
};
export function frameRect(name: SpriteName, column = 0, row = 0) {
  const f = spriteFrames[name];
  return {
    sx: Math.max(0, Math.min(f.columns - 1, Math.floor(column))) * f.width,
    sy: Math.max(0, Math.min(f.rows - 1, Math.floor(row))) * f.height,
    sw: f.width,
    sh: f.height,
  };
}
type Atlas = {
  images: Partial<Record<SpriteName, HTMLImageElement>>;
  promise: Promise<{ loaded: number; failed: SpriteName[] }>;
};
const atlases = new Map<string, Atlas>();
let current: Atlas | undefined;
export function getSprite(name: SpriteName): HTMLImageElement | null {
  const image = current?.images[name];
  return image?.complete && image.naturalWidth > 0 ? image : null;
}
export function loadSprites(base = ""): Atlas["promise"] {
  const cached = atlases.get(base);
  if (cached) {
    current = cached;
    return cached.promise;
  }
  const images: Atlas["images"] = {};
  const atlas = {
    images,
    promise: Promise.resolve({ loaded: 0, failed: [] as SpriteName[] }),
  };
  current = atlas;
  atlases.set(base, atlas);
  atlas.promise = Promise.all(
    (Object.keys(spriteFrames) as SpriteName[]).map(async (name) => {
      if (typeof Image === "undefined") return name;
      for (const url of spriteSourceCandidates(base, spriteFrames[name].file)) {
        const image = new Image();
        const ok = await new Promise<boolean>((resolve) => {
          image.onload = () => resolve(image.naturalWidth > 0);
          image.onerror = () => resolve(false);
          image.src = url;
        });
        if (ok) {
          images[name] = image;
          return null;
        }
      }
      return name;
    }),
  ).then((results) => ({
    loaded: results.filter((x) => x === null).length,
    failed: results.filter((x): x is SpriteName => x !== null),
  }));
  return atlas.promise;
}
