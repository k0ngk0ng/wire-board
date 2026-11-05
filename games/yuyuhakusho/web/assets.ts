import type { Progress } from "./types";
export type Resource = {
  id: string;
  logical_path: string;
  raw_json_sha256: string;
  raw_json_bytes: number;
  gzip_sha256: string;
  gzip_bytes: number;
};
export type Descriptor = { schema: 1; resources: Resource[] };
export type AssetBytes = {
  Rules: Uint8Array;
  Sprites: Uint8Array;
  Arena: Uint8Array;
  HUD: Uint8Array;
  ArenaAux: Uint8Array;
  RenderMetadata: Uint8Array;
  Descriptor: Uint8Array;
};
const files: Record<string, string> = {
  rules: "chu-world-rules.json.gz",
  sprites: "world-sprites.json.gz",
  hud: "hud-assets.json.gz",
  render_meta: "renderstate-metadata.json.gz",
  "arena-default": "arena-assets-default.json.gz",
  "arena-a-aux": "arena-a-aux-assets.json.gz",
};
for (const n of ["2", "4", "6", "8", "a", "c", "e"])
  files["arena-" + n] = "arena-assets-" + n + ".json.gz";
const exactKeys = (value: object, keys: string[]) => {
  const names = Object.keys(value).sort();
  return (
    names.length === keys.length &&
    names.every((name, index) => name === [...keys].sort()[index])
  );
};
const hash = (value: unknown) =>
  typeof value === "string" && /^[a-f0-9]{64}$/.test(value);
export function decodeDescriptor(bytes: Uint8Array): Descriptor {
  const value = JSON.parse(
    new TextDecoder("utf-8", { fatal: true }).decode(bytes),
  ) as Descriptor;
  if (
    !value ||
    !exactKeys(value, ["schema", "resources"]) ||
    value.schema !== 1 ||
    !Array.isArray(value.resources) ||
    value.resources.length !== 13
  )
    throw Error("资源清单格式无效");
  const seen = new Set<string>();
  for (const item of value.resources) {
    if (
      !item ||
      !exactKeys(item, [
        "id",
        "logical_path",
        "raw_json_sha256",
        "raw_json_bytes",
        "gzip_sha256",
        "gzip_bytes",
      ]) ||
      !Object.hasOwn(files, item.id) ||
      seen.has(item.id) ||
      item.logical_path !== "/yuyuhakusho/data/" + files[item.id] ||
      !hash(item.raw_json_sha256) ||
      !hash(item.gzip_sha256) ||
      !Number.isSafeInteger(item.raw_json_bytes) ||
      item.raw_json_bytes < 1 ||
      item.raw_json_bytes > 16 * 1024 * 1024 ||
      !Number.isSafeInteger(item.gzip_bytes) ||
      item.gzip_bytes < 1 ||
      item.gzip_bytes > 8 * 1024 * 1024
    )
      throw Error("资源清单身份无效");
    seen.add(item.id);
  }
  return value;
}
export function resourceIDsForArena(arena: number): string[] {
  if (!Number.isInteger(arena) || arena < 0 || arena > 14 || arena % 2)
    throw Error("场地无效");
  return [
    "rules",
    "sprites",
    "hud",
    "render_meta",
    arena ? "arena-" + arena.toString(16) : "arena-default",
    ...(arena === 10 ? ["arena-a-aux"] : []),
  ];
}
export const assetURL = (base: string, path: string) =>
  base ? base.replace(/\/+$/, "") + path : path;
export async function sha256(bytes: Uint8Array): Promise<string> {
  const input = new Uint8Array(bytes.byteLength);
  input.set(bytes);
  return [
    ...new Uint8Array(await crypto.subtle.digest("SHA-256", input.buffer)),
  ]
    .map((n) => n.toString(16).padStart(2, "0"))
    .join("");
}
const cached = new Map<string, Promise<Uint8Array>>();
export async function fetchVerified(
  url: string,
  expected: { sha256: string; bytes: number },
  signal: AbortSignal,
  update: (loaded: number) => void = () => {},
): Promise<Uint8Array> {
  const response = await fetch(url, {
    signal: AbortSignal.any([signal, AbortSignal.timeout(45000)]),
    cache: "no-cache",
  });
  if (!response.ok) throw Error("资源下载失败，请重试");
  const chunks: Uint8Array[] = [];
  let count = 0;
  if (response.body) {
    const reader = response.body.getReader();
    try {
      for (;;) {
        const next = await reader.read();
        if (next.done) break;
        count += next.value.length;
        if (count > expected.bytes) throw Error("资源大小不匹配");
        chunks.push(next.value);
        update(count);
      }
    } finally {
      reader.releaseLock();
    }
  } else {
    const b = new Uint8Array(await response.arrayBuffer());
    chunks.push(b);
    count = b.length;
    update(count);
  }
  if (count !== expected.bytes) throw Error("资源大小不匹配");
  const output = new Uint8Array(count);
  let at = 0;
  for (const chunk of chunks) {
    output.set(chunk, at);
    at += chunk.length;
  }
  if ((await sha256(output)) !== expected.sha256)
    throw Error("资源版本不匹配，请刷新页面");
  return output;
}
async function unzip(compressed: Uint8Array): Promise<Uint8Array> {
  const bytes = new Uint8Array(compressed.byteLength);
  bytes.set(compressed);
  return new Uint8Array(
    await new Response(
      new Blob([bytes.buffer])
        .stream()
        .pipeThrough(new DecompressionStream("gzip")),
    ).arrayBuffer(),
  );
}
export async function loadAssets(
  base: string,
  arena: number,
  descriptorSHA: string,
  signal: AbortSignal,
  report: (progress: Progress) => void,
): Promise<AssetBytes> {
  if (!hash(descriptorSHA)) throw Error("缺少资源版本身份");
  report({ percent: 0, label: "读取资源清单" });
  const response = await fetch(
    assetURL(base, "/yuyuhakusho/data/runtime-resource-descriptor.json"),
    { signal, cache: "no-cache" },
  );
  if (!response.ok) throw Error("无法读取资源清单");
  const raw = new Uint8Array(await response.arrayBuffer());
  if (raw.byteLength > 65536 || (await sha256(raw)) !== descriptorSHA)
    throw Error("资源清单版本不匹配，请刷新页面");
  const descriptor = decodeDescriptor(raw),
    ids = resourceIDsForArena(arena),
    selected = ids.map((id) =>
      descriptor.resources.find((item) => item.id === id)!,
    );
  const total = selected.reduce((sum, item) => sum + item.gzip_bytes, 0),
    counts = new Map<string, number>();
  const values = await Promise.all(
    selected.map(async (item) => {
      const key = item.raw_json_sha256;
      let promise = cached.get(key);
      if (!promise) {
        promise = (async () => {
          const compressed = await fetchVerified(
            assetURL(base, item.logical_path),
            { sha256: item.gzip_sha256, bytes: item.gzip_bytes },
            signal,
            (n) => {
              counts.set(item.id, n);
              report({
                percent: Math.min(
                  96,
                  ([...counts.values()].reduce((a, b) => a + b, 0) / total) *
                    96,
                ),
                label: "加载比赛素材",
              });
            },
          );
          const data = await unzip(compressed);
          if (
            data.byteLength !== item.raw_json_bytes ||
            (await sha256(data)) !== item.raw_json_sha256
          )
            throw Error("解压资源版本不匹配");
          return data;
        })().catch((error) => {
          cached.delete(key);
          throw error;
        });
        cached.set(key, promise);
      }
      const data = await promise;
      signal.throwIfAborted();
      counts.set(item.id, item.gzip_bytes);
      return [item.id, data] as const;
    }),
  );
  const byID = new Map(values);
  report({ percent: 100, label: "比赛素材已校验" });
  return {
    Rules: byID.get("rules")!,
    Sprites: byID.get("sprites")!,
    Arena: byID.get(arena ? "arena-" + arena.toString(16) : "arena-default")!,
    HUD: byID.get("hud")!,
    ArenaAux: byID.get("arena-a-aux") ?? new Uint8Array(),
    RenderMetadata: byID.get("render_meta")!,
    Descriptor: raw,
  };
}
