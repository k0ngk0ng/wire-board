import { VerifiedAssetCache } from "./load-cache";
import { loadGame } from "./registry";
import type { GameRoom } from "./module";
type File = { url: string; bytes: number };
export type LoadProgress = {
  percent: number;
  label: string;
  loaded?: number;
  total?: number;
};
type Manifest = {
  build: string;
  entry: string;
  games: Record<string, File[]>;
  media: Record<string, number>;
};
const retainedImages = new Map<string, HTMLImageElement>();
const verifiedAssets = new VerifiedAssetCache();
let retainedManifest: Manifest | undefined;
async function download(
  file: File,
  base: string,
  signal: AbortSignal,
  update: (n: number) => void,
) {
  const urls = base
    ? [base.replace(/\/$/, "") + file.url, file.url]
    : [file.url];
  let last: unknown;
  for (const url of urls) {
    try {
      const response = await fetch(url, {
        signal: AbortSignal.any([signal, AbortSignal.timeout(45000)]),
      });
      if (!response.ok) throw Error(`资源下载失败（${response.status}）`);
      let bytes = 0;
      const reader = response.body?.getReader();
      if (reader) {
        try {
          for (;;) {
            const { done, value } = await reader.read();
            if (done) break;
            bytes += value.length;
            update(bytes);
          }
        } finally {
          reader.releaseLock();
        }
      } else {
        bytes = (await response.arrayBuffer()).byteLength;
        update(bytes);
      }
      if (bytes !== file.bytes) throw Error("资源版本或大小不匹配，请刷新页面");
      if (
        /\.(png|webp|jpg|jpeg|gif|svg)$/i.test(url) &&
        !retainedImages.has(url)
      ) {
        const img = new Image();
        img.src = url;
        await img.decode();
        signal.throwIfAborted();
        retainedImages.set(url, img);
      }
      return;
    } catch (error) {
      last = error;
      if (signal.aborted) throw error;
      update(0);
    }
  }
  throw last;
}
async function batch(
  files: File[],
  build: string,
  base: string,
  signal: AbortSignal,
  report: (loaded: number, total: number) => void,
) {
  const counts = new Map<string, number>(),
    total = files.reduce((n, f) => n + f.bytes, 0);
  let index = 0;
  await Promise.all(
    Array.from({ length: Math.min(6, files.length) }, async () => {
      while (index < files.length) {
        signal.throwIfAborted();
        const file = files[index++];
        const update = (n: number) => {
          counts.set(file.url, Math.min(n, file.bytes));
          report(
            [...counts.values()].reduce((a, b) => a + b, 0),
            total,
          );
        };
        await verifiedAssets.load(file, build, base, signal, update, () =>
          download(file, base, signal, update),
        );
      }
    }),
  );
}
export async function prepareGame(
  room: GameRoom,
  assets: string,
  signal: AbortSignal,
  progress: (p: LoadProgress) => void,
) {
  progress({ percent: 0, label: "准备对局" });
  let manifest: Manifest;
  // A new loading gate carries the authoritative build identity. Without one,
  // revalidate the manifest so reconnects cannot quietly reuse an old release.
  if (
    retainedManifest &&
    room.loading?.build === retainedManifest.build &&
    document.querySelector(`script[src="${retainedManifest.entry}"]`)
  ) {
    manifest = retainedManifest;
  } else {
    progress({ percent: 0, label: "读取资源清单" });
    const response = await fetch("/game-load-manifest.json", {
      cache: "no-cache",
      signal,
    });
    if (!response.ok) throw Error("无法读取资源清单，请重试");
    manifest = await response.json();
  }
  signal.throwIfAborted();
  if (
    !document.querySelector(`script[src="${manifest.entry}"]`) ||
    (room.loading && room.loading.build !== manifest.build)
  )
    throw Error("页面版本已更新，请刷新后加载");
  retainedManifest = manifest;
  const files = manifest.games[room.kind];
  if (!files) throw Error("未找到游戏资源清单");
  const warmCode = files.every((file) =>
    verifiedAssets.has(file, manifest.build, ""),
  );
  await batch(files, manifest.build, "", signal, (loaded, total) =>
    progress({
      percent: total ? (loaded / total) * 35 : 35,
      label: warmCode ? "复用已缓存的游戏程序" : "加载游戏程序",
      loaded,
      total,
    }),
  );
  const module = await loadGame(room.kind);
  signal.throwIfAborted();
  if (module.legacy) await import("../legacy/room-ui");
  const paths =
    module.resources?.(room, manifest.media) ??
    Object.keys(manifest.media).filter((p) =>
      p.startsWith("/" + room.kind + "/"),
    );
  const media = [...new Set(paths)].map((url) => {
    const bytes = manifest.media[url];
    if (!bytes) throw Error("资源清单缺少 " + url);
    return { url, bytes };
  });
  const warmMedia = media.every((file) =>
    verifiedAssets.has(file, manifest.build, assets),
  );
  await batch(media, manifest.build, assets, signal, (loaded, total) =>
    progress({
      percent: 35 + (total ? (loaded / total) * 55 : 55),
      label: warmMedia ? "复用已缓存的游戏素材" : "加载游戏素材",
      loaded,
      total,
    }),
  );
  progress({ percent: 90, label: "初始化游戏画面" });
  return { module, build: manifest.build };
}
