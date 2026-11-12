export type ClassicAsset = {
  path: string;
  bytes: number;
  sha256: string;
  mime: string;
};
export type ClassicManifest = {
  version: number;
  assets: ClassicAsset[];
  totalBytes?: number;
  propCatalogs?: Record<
    string,
    { path: string; catalogHash: string; count: number }
  >;
};
export type ClassicProgress = { loaded: number; total: number; cached: number };

const TRACKS = new Set(["france", "coastal", "uk", "park"]);
const VEHICLES = new Set(["p911", "fm50", "ldia", "mcf1", "ccor"]);

export function classicResourceBase(value: string): string {
  if (value === "/") return "";
  if (!/^https:\/\/cdn\.ichenj\.com\/bga\/web\/[a-f0-9]{40}\/$/.test(value))
    throw Error("赛车资源必须使用当前构建目录");
  return value.slice(0, -1);
}

export function classicAssetURL(base: string, path: string): string {
  if (
    !path ||
    path.startsWith("/") ||
    path.split("/").some((p) => p === ".." || !p)
  )
    throw Error("无效的赛车资源路径");
  return `${base.replace(/\/+$/, "")}/racing/v3/${path}`;
}

export function selectClassicAssets(
  manifest: ClassicManifest,
  track: string,
  timeOfDay: string,
  vehicles: readonly string[],
): ClassicAsset[] {
  if (manifest.version !== 3 || !Array.isArray(manifest.assets))
    throw Error("赛车资源版本不匹配");
  if (!TRACKS.has(track) || !["day", "night"].includes(timeOfDay))
    throw Error("无效的赛车赛道或时段");
  if (!vehicles.length || vehicles.some((id) => !VEHICLES.has(id)))
    throw Error("车型资源不存在");
  const name = track + (timeOfDay === "night" ? "-night" : "");
  const paths = new Set([
    `tracks/${name}.glb`,
    `tracks/${track}.collision.json`,
    `props/${track}.json`,
    `tracks/${track}.route.json`,
    `tracks/${name}-sky.png`,
    ...vehicles.map((id) => `cars/${id}.glb`),
  ]);
  const result: ClassicAsset[] = [];
  for (const path of paths) {
    const entry = manifest.assets.find((asset) => asset.path === path);
    if (
      !entry ||
      !Number.isSafeInteger(entry.bytes) ||
      entry.bytes <= 0 ||
      entry.bytes > 100 * 1024 * 1024 ||
      !/^[a-f0-9]{64}$/.test(entry.sha256)
    )
      throw Error(`赛车资源清单缺少 ${path}`);
    result.push(entry);
  }
  return result;
}

// Only fully verified bytes enter this bounded LRU. An aborted consumer never
// owns another consumer's request, and a failed request cannot poison retries.
export class ClassicAssetCache {
  private entries = new Map<string, ArrayBuffer>();
  private capacity: number;
  bytes = 0;
  constructor(capacity = 32 * 1024 * 1024) {
    this.capacity = Math.max(0, capacity);
  }
  get(key: string): ArrayBuffer | undefined {
    const value = this.entries.get(key);
    if (value) {
      this.entries.delete(key);
      this.entries.set(key, value);
    }
    return value;
  }
  set(key: string, value: ArrayBuffer): void {
    if (value.byteLength > this.capacity) return;
    const old = this.entries.get(key);
    if (old) {
      this.bytes -= old.byteLength;
      this.entries.delete(key);
    }
    while (this.bytes + value.byteLength > this.capacity && this.entries.size) {
      const first = this.entries.keys().next().value!;
      this.bytes -= this.entries.get(first)!.byteLength;
      this.entries.delete(first);
    }
    this.entries.set(key, value);
    this.bytes += value.byteLength;
  }
}

const verifiedBytes = new ClassicAssetCache();
type FetchOptions = {
  base: string;
  signal?: AbortSignal;
  cache?: ClassicAssetCache;
  fetcher?: (url: string, init?: RequestInit) => Promise<Response>;
  progress?: (value: ClassicProgress) => void;
};

export async function fetchClassicFiles(
  files: readonly ClassicAsset[],
  options: FetchOptions,
): Promise<Map<string, ArrayBuffer>> {
  const cache = options.cache ?? verifiedBytes;
  const fetcher = options.fetcher ?? fetch;
  const unique = [...new Map(files.map((file) => [file.path, file])).values()];
  const result = new Map<string, ArrayBuffer>();
  const loaded = new Map<string, number>();
  const total = unique.reduce((sum, file) => sum + file.bytes, 0);
  let cached = 0;
  const notify = () =>
    options.progress?.({
      loaded: [...loaded.values()].reduce((sum, value) => sum + value, 0),
      total,
      cached,
    });
  const checkAbort = () => options.signal?.throwIfAborted();
  checkAbort();
  const pending: ClassicAsset[] = [];
  for (const file of unique) {
    const key = `${options.base}/${file.path}:${file.sha256}`;
    const bytes = cache.get(key);
    if (bytes?.byteLength === file.bytes) {
      cached += file.bytes;
      loaded.set(file.path, file.bytes);
      result.set(file.path, bytes);
    } else pending.push(file);
  }
  notify();
  let next = 0;
  const worker = async () => {
    while (next < pending.length) {
      checkAbort();
      const file = pending[next++];
      const response = await fetcher(classicAssetURL(options.base, file.path), {
        signal: options.signal,
        cache: "force-cache",
      });
      if (!response.ok) throw Error(`赛车资源读取失败：${file.path}`);
      const chunks: Uint8Array[] = [];
      let size = 0;
      if (response.body) {
        const reader = response.body.getReader();
        try {
          while (true) {
            checkAbort();
            const { value, done } = await reader.read();
            if (done) break;
            size += value.byteLength;
            if (size > file.bytes)
              throw Error(`赛车资源大小不匹配：${file.path}`);
            chunks.push(value);
            loaded.set(file.path, size);
            notify();
          }
        } finally {
          reader.releaseLock();
        }
      } else {
        const value = new Uint8Array(await response.arrayBuffer());
        chunks.push(value);
        size = value.byteLength;
      }
      checkAbort();
      if (size !== file.bytes) throw Error(`赛车资源大小不匹配：${file.path}`);
      const value = new Uint8Array(size);
      let at = 0;
      for (const chunk of chunks) {
        value.set(chunk, at);
        at += chunk.byteLength;
      }
      const hash = await crypto.subtle.digest("SHA-256", value);
      checkAbort();
      const digest = [...new Uint8Array(hash)]
        .map((n) => n.toString(16).padStart(2, "0"))
        .join("");
      if (digest !== file.sha256) throw Error(`赛车资源校验失败：${file.path}`);
      cache.set(`${options.base}/${file.path}:${file.sha256}`, value.buffer);
      loaded.set(file.path, file.bytes);
      result.set(file.path, value.buffer);
      notify();
    }
  };
  await Promise.all(
    Array.from({ length: Math.min(3, pending.length) }, worker),
  );
  checkAbort();
  notify();
  return result;
}
