import { VerifiedAssetCache } from "./load-cache";
import workerEntry from "./resident-worker.ts?worker&url";
import { aliasResidentResource, beginResidentResources, declareResidentResources } from "./resident-assets";
import { residentResourcePlan } from "./resident-resource-plan";
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
  assetBase?: string;
  home?: { files: File[] };
};
const verifiedAssets = new VerifiedAssetCache();
let retainedManifest: Manifest | undefined;
function resourceSources(url: string, base: string) {
  return /^(?:https?:\/\/|\/(?:bga|_media)\/)/.test(url) || !base ? [url] : [base.replace(/\/$/, "") + url, url];
}
function residentFiles(files: readonly File[], base: string) {
  return files.map(file => ({ ...file, url: new URL(resourceSources(file.url, base)[0], location.href).href }));
}
async function download(
  file: File,
  base: string,
  signal: AbortSignal,
  update: (n: number) => void,
  lease: string,
) {
  const urls = resourceSources(file.url, base);
  let last: unknown;
  for (const url of urls) {
    try {
      const response = await fetch(url, {
        signal: AbortSignal.any([signal, AbortSignal.timeout(45000)]),
      });
      if (response.headers.get("X-Wire-Board-Resident") === "resident" && !response.ok) throw Object.assign(Error("本地资源不可用，请重新打开游戏"), { resident: true });
      if (!response.ok) throw Error(`资源下载失败（${response.status}）`);
      const decode = /\.(png|webp|jpg|jpeg|gif|svg)$/i.test(url);
      const chunks: Uint8Array<ArrayBuffer>[] = [];
      let bytes = 0;
      const reader = response.body?.getReader();
      if (reader) {
        try {
          for (;;) {
            const { done, value } = await reader.read();
            if (done) break;
            bytes += value.length;
          if (decode) chunks.push(new Uint8Array(value));
            update(bytes);
          }
        } finally {
          reader.releaseLock();
        }
      } else {
        const data = new Uint8Array(await response.arrayBuffer());
        bytes = data.byteLength;
        if (decode) chunks.push(data);
        update(bytes);
      }
      if (bytes !== file.bytes) throw Error("资源版本或大小不匹配，请刷新页面");
      if (decode) {
        let decoded = false;
        if (typeof createImageBitmap === "function" && !/\.svg$/i.test(url)) {
          try {
            const image = await createImageBitmap(new Blob(chunks, { type: response.headers.get("Content-Type") || "application/octet-stream" }));
            image.close();
            decoded = true;
          } catch {
            signal.throwIfAborted();
          }
        }
        if (!decoded) {
          const img = new Image();
          img.src = url;
          await img.decode();
        }
        signal.throwIfAborted();
      }
      if (url !== urls[0]) await aliasResidentResource(lease, new URL(urls[0], location.href).href, new URL(url, location.href).href, signal);
      return;
    } catch (error) {
      if ((error as { resident?: boolean }).resident) throw error;
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
  lease: string,
) {
  const counts = new Map<string, number>(),
    total = files.reduce((n, f) => n + f.bytes, 0);
  let index = 0;
  let loaded = 0;
  await Promise.all(
    Array.from({ length: Math.min(16, files.length) }, async () => {
      while (index < files.length) {
        signal.throwIfAborted();
        const file = files[index++];
        const update = (n: number) => {
          const next = Math.min(n, file.bytes);
          loaded += next - (counts.get(file.url) ?? 0);
          counts.set(file.url, next);
          report(loaded, total);
        };
        await verifiedAssets.load(file, build, base, signal, update, () =>
          download(file, base, signal, update, lease),
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
  lease: string,
) {
  progress({ percent: 0, label: "准备对局" });
  let manifest: Manifest;
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
  const programs = manifest.games[room.kind];
  if (!programs) throw Error("未找到游戏资源清单");
  const files = [...new Map([...programs, ...(manifest.home?.files ?? [])].map(file => [file.url, file])).values()];
  const origins = [...new Set([location.origin, new URL(manifest.assetBase || location.origin, location.href).origin])];
  await beginResidentResources(workerEntry, lease, manifest.build, residentFiles(files, ""), origins, signal);
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
    lease,
  );
  const module = await loadGame(room.kind);
  signal.throwIfAborted();
  if (module.legacy) await import("../legacy/room-ui");
  const paths =
    module.resources?.(room, manifest.media) ??
    Object.keys(manifest.media).filter((p) =>
      p.startsWith("/" + room.kind + "/"),
    );
  const media = residentResourcePlan(room.kind, manifest.media, paths, await module.residentResources?.(room, manifest.media, assets));
  await declareResidentResources(lease, residentFiles(media, assets), signal);
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
    lease,
  );
  progress({
    percent: module.assetLoading === "board" ? 35 : 90,
    label: "初始化游戏画面",
  });
  return { module, build: manifest.build, media: manifest.media };
}
