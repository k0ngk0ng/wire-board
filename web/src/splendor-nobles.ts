import type { CSSProperties } from "react";

export function extraNobleName(id: number) {
  return id === 101 ? "丝绸之路附赠贵族" : id === 102 ? "日不落附赠贵族" : "";
}

export function nobleArtwork(id: number, assets: string): CSSProperties {
  if (extraNobleName(id)) {
    return assets
      ? {
          backgroundImage: `url("${assets}/splendor/expansions/noble-${id}.webp")`,
          backgroundSize: "cover",
          backgroundPosition: "center",
        }
      : {};
  }
  return {
    backgroundPosition: `${((id - 1) % 5) * 25}% ${Math.floor((id - 1) / 5) * 50}%`,
  };
}
