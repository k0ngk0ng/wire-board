import { useEffect, useState } from "react";
import { ATLAS_FRAMES } from "./atlas";

export type Frame = { x: number; y: number; w: number; h: number };
export type Atlas = {
  image: HTMLImageElement;
  frames: Record<string, Frame>;
};

export const assetSources = (base: string, path: string): string[] => {
  const local = `/battlecity/v1/${path}`;
  if (!base) return [local];
  const prefix = base.replace(/\/+$/, "");
  return [`${prefix}/battlecity/v1/${path}`, local];
};

export const atlasSources = (base: string): string[] => {
  const local = "/battlecity/v2/atlas.webp";
  return base
    ? [`${base.replace(/\/+$/, "")}/battlecity/v2/atlas.webp`, local]
    : [local];
};

export const soundSources = (base: string, file: string): string[] =>
  assetSources(base, `audio/${file}.mp3`);

const imageCache = new Map<string, Promise<HTMLImageElement | null>>();

function loadImage(sources: string[]): Promise<HTMLImageElement | null> {
  const key = sources.join("|");
  const cached = imageCache.get(key);
  if (cached) return cached;
  const promise = new Promise<HTMLImageElement | null>((resolve) => {
    let index = 0;
    const next = () => {
      if (index >= sources.length) {
        resolve(null);
        return;
      }
      const image = new Image();
      image.onload = () => resolve(image);
      image.onerror = () => {
        index++;
        next();
      };
      image.src = sources[index];
    };
    next();
  });
  imageCache.set(key, promise);
  return promise;
}

export function useAtlas(base: string): Atlas | null {
  const [atlas, setAtlas] = useState<Atlas | null>(null);
  useEffect(() => {
    let live = true;
    void loadImage(atlasSources(base)).then((image) => {
      // The frame table is committed; only the image is cross-origin.  This
      // avoids requiring a CORS response from atlas.json on the CDN.
      if (live && image) setAtlas({ image, frames: ATLAS_FRAMES });
    });
    return () => {
      live = false;
    };
  }, [base]);
  return atlas;
}

export function drawFrame(
  ctx: CanvasRenderingContext2D,
  atlas: Atlas | null,
  name: string,
  x: number,
  y: number,
  w?: number,
  h?: number,
): boolean {
  const frame = atlas?.frames[name];
  if (!atlas || !frame) return false;
  ctx.drawImage(
    atlas.image,
    frame.x,
    frame.y,
    frame.w,
    frame.h,
    x,
    y,
    w ?? frame.w,
    h ?? frame.h,
  );
  return true;
}
