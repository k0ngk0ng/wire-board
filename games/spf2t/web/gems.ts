import type { Color, GemKind, GemView } from "./types";

export const GEM_COLORS: Record<Color, string> = {
  r: "#c85555",
  y: "#c9a84a",
  g: "#5f9a68",
  b: "#4f7fb5",
};

export const GEM_DARK: Record<Color, string> = {
  r: "#6e2424",
  y: "#6a531d",
  g: "#274d30",
  b: "#1f3e62",
};

export const GEM_LIGHT: Record<Color, string> = {
  r: "#f0a0a0",
  y: "#f2dd9a",
  g: "#a8d5ad",
  b: "#9cc0ea",
};

export const COLOR_LABEL: Record<Color, string> = {
  r: "红",
  y: "黄",
  g: "绿",
  b: "蓝",
};

export const KIND_LABEL: Record<GemKind, string> = {
  normal: "普通宝石",
  crash: "碎裂宝石",
  power: "强化宝石",
  counter: "反击宝石",
  diamond: "钻石",
};

export const gemLabel = (gem: Pick<GemView, "kind" | "color" | "timer">) =>
  `${gem.color ? COLOR_LABEL[gem.color] : ""}${KIND_LABEL[gem.kind]}${
    gem.kind === "counter" && gem.timer ? `，${gem.timer} 回合后转化` : ""
  }`;

export const warningClass = (pending: number) =>
  pending <= 0
    ? "is-clear"
    : pending <= 10
      ? "is-caution"
      : pending <= 30
        ? "is-warning"
        : "is-danger";

// Expected atlas layout (16px tiles): rows normal/crash/counter/power, four
// color columns, diamond at the fifth row. The packed image is OSS/local-only;
// this metadata is committed so the frontend and asset generator agree.
export type AtlasTile = { x: number; y: number; w: number; h: number };
export const ATLAS_CELL = 16;
export const ATLAS_ROWS = 5;
export const ATLAS_COLS = 4;
export const ATLAS_IMAGE = "gems.webp";

const colorIndex: Record<Color, number> = { r: 0, y: 1, g: 2, b: 3 };
const kindRow: Partial<Record<GemKind, number>> = {
  normal: 0,
  crash: 1,
  counter: 2,
  power: 3,
};

export function gemTile(kind: GemKind, color?: Color): AtlasTile | null {
  if (kind === "diamond") {
    return { x: 0, y: 4 * ATLAS_CELL, w: ATLAS_CELL, h: ATLAS_CELL };
  }
  const row = kindRow[kind];
  if (row === undefined || !color) return null;
  return { x: colorIndex[color] * ATLAS_CELL, y: row * ATLAS_CELL, w: ATLAS_CELL, h: ATLAS_CELL };
}

export const atlasSources = (base: string) => {
  const local = `/spf2t/v1/${ATLAS_IMAGE}`;
  return base ? [`${base.replace(/\/$/, "")}/spf2t/v1/${ATLAS_IMAGE}`, local] : [local];
};
