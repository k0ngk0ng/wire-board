import type { ContraAudioView } from "./audio";

export type RuntimeAsset = { url: string; bytes: number; sha256: string };
export type RuntimeBundle = {
  version: number;
  id: string;
  stages: { level: number; path: string; sha256: string; digest: string }[];
};
export type RuntimeManifest = {
  version: number;
  rulesBuildID: string;
  visualsID?: string;
  runtime?: { goVersion: string; sourceSHA256: string; builderSHA256: string };
  bundle?: RuntimeBundle;
  resources: RuntimeAsset;
  stages?: Record<string, RuntimeAsset>;
  rgb: RuntimeAsset;
  pages: Record<string, RuntimeAsset>;
  stagePages?: Record<string, number[]>;
  wasm: RuntimeAsset;
  loader: RuntimeAsset;
  audio: string;
  audioCatalog?: RuntimeAsset;
};
export async function fetchRuntimeManifest(
  url: string,
  expected: { runtimeBuildID: string; rulesBuildID: string; visualsID: string },
): Promise<RuntimeManifest> {
  if (
    ![expected.runtimeBuildID, expected.rulesBuildID, expected.visualsID].every(
      (value) => /^[a-f0-9]{64}$/.test(value),
    )
  )
    throw Error("缺少运行程序或画面资源身份");
  const response = await fetch(url);
  if (!response.ok) throw Error("无法加载游戏资源清单");
  const bytes = new Uint8Array(await response.arrayBuffer());
  const digest = [
    ...new Uint8Array(await crypto.subtle.digest("SHA-256", bytes)),
  ]
    .map((value) => value.toString(16).padStart(2, "0"))
    .join("");
  if (digest !== expected.runtimeBuildID)
    throw Error("运行程序资源清单摘要不匹配");
  const manifest = JSON.parse(
    new TextDecoder().decode(bytes),
  ) as RuntimeManifest;
  if (
    manifest.rulesBuildID !== expected.rulesBuildID ||
    manifest.visualsID !== expected.visualsID
  )
    throw Error("规则或画面资源版本不匹配");
  return resolveRuntimeManifest(manifest, url);
}
export function runtimeStageManifest(
  manifest: RuntimeManifest,
  level: number,
): RuntimeManifest {
  if (!manifest.stagePages) return manifest;
  const keys = manifest.stagePages[String(Math.min(level, 7))];
  if (!keys?.length) throw Error("缺少当前关卡图形清单");
  const pages: RuntimeManifest["pages"] = {};
  for (const key of keys) {
    const asset = manifest.pages[String(key)];
    if (!asset) throw Error("关卡图形资源不完整");
    pages[String(key)] = asset;
  }
  return { ...manifest, pages };
}
export type RuntimePresentation = {
  Background: number[];
  OAM: number[];
  Patterns: number[];
  Palette: number[];
  Display: {
    AppliedMask: number;
    SplitActive: boolean;
    RasterActive: boolean;
    SplitScroll: number;
    RasterMask: number;
    RasterControl: number;
  };
  Scroll: { X: number; Y: number; NameTable: number };
};
export function runtimePresentationManifest(
  manifest: RuntimeManifest,
  view: RuntimePresentation,
): RuntimeManifest {
  const pages: RuntimeManifest["pages"] = {};
  for (const page of view.Patterns) {
    const asset = manifest.pages[String(page)];
    if (!asset) throw Error("当前画面缺少图形资源");
    pages[String(page)] = asset;
  }
  return { ...manifest, pages };
}
export type RuntimeSnapshot = {
  review?: { frame: string; revision: string; level: number };
  rulesBuildID: string;
  revision: string;
  session: number;
  epoch: string;
  frame: string;
  audioTick: string;
  digest: string;
  resources: string;
  phase: string;
  level: number;
  paused: boolean;
  finalized: boolean;
  outcome?: string;
  players: {
    x: number;
    y: number;
    state: number;
    lives: number;
    gameOver: number;
  }[];
  projectiles: number;
  presentation: RuntimePresentation;
  audio: ContraAudioView;
};
export type RuntimeCheckpoint = {
  rulesBuildID: string;
  version: number;
  epoch: string;
  resources: string;
  digest: string;
  state: unknown;
};
export type RuntimePoll = {
  snapshot: RuntimeSnapshot;
  frames: {
    kind?: "pads" | "present-paused" | "resume" | "terminal";
    revision?: string;
    frame?: string;
    digest?: string;
    level?: number;
    pads?: [number, number];
    snapshot?: RuntimeSnapshot;
  }[];
  checkpoint?: RuntimeCheckpoint;
};
export interface RuntimeBridge {
  call<T>(request: Record<string, unknown>): T;
}
export function resolveRuntimeURL(value: string, base: string): string {
  if (typeof value !== "string" || !value) throw Error("无效资源地址");
  const url = new URL(value.replace(/^\/+/, ""), base);
  if (url.protocol !== "https:" && url.protocol !== "http:")
    throw Error("资源地址必须使用HTTP或HTTPS");
  return url.href;
}
export function resolveRuntimeManifest(
  value: RuntimeManifest,
  base: string,
): RuntimeManifest {
  if (value.version !== 1 || !/^[a-f0-9]{64}$/.test(value.rulesBuildID))
    throw Error("无效规则程序资源清单");
  const asset = (a: RuntimeAsset): RuntimeAsset => {
    if (
      !a ||
      !Number.isSafeInteger(a.bytes) ||
      a.bytes <= 0 ||
      !/^[a-f0-9]{64}$/.test(a.sha256)
    )
      throw Error("资源清单缺少有效摘要");
    return { ...a, url: resolveRuntimeURL(a.url, base) };
  };
  return {
    ...value,
    resources: asset(value.resources),
    stages:
      value.stages &&
      Object.fromEntries(
        Object.entries(value.stages).map(([k, a]) => [k, asset(a)]),
      ),
    rgb: asset(value.rgb),
    pages: Object.fromEntries(
      Object.entries(value.pages).map(([k, a]) => [k, asset(a)]),
    ),
    wasm: asset(value.wasm),
    loader: asset(value.loader),
    audio: resolveRuntimeURL(value.audio, base),
    audioCatalog: value.audioCatalog && asset(value.audioCatalog),
  };
}
type RuntimeWindow = Window &
  typeof globalThis & {
    Go?: new () => {
      importObject: WebAssembly.Imports;
      run(instance: WebAssembly.Instance): Promise<void>;
    };
    contraRuntime?: (request: string) => string;
  };
type GoConstructor = NonNullable<RuntimeWindow["Go"]>;
const goLoaders = new Map<string, Promise<GoConstructor>>();
function runtimeGo(asset: RuntimeAsset): Promise<GoConstructor> {
  let pending = goLoaders.get(asset.sha256);
  if (pending) return pending;
  pending = new Promise<GoConstructor>((resolve, reject) => {
    const script = document.createElement("script");
    script.src = asset.url;
    script.crossOrigin = "anonymous";
    script.integrity =
      "sha256-" +
      btoa(
        String.fromCharCode(
          ...asset.sha256.match(/../g)!.map((v) => parseInt(v, 16)),
        ),
      );
    script.onload = () => {
      const constructor = (window as RuntimeWindow).Go;
      if (constructor) resolve(constructor);
      else reject(Error("Go运行时初始化失败"));
    };
    script.onerror = () => {
      script.remove();
      reject(Error("Go运行时加载或摘要校验失败"));
    };
    document.head.append(script);
  }).catch((error) => {
    goLoaders.delete(asset.sha256);
    throw error;
  });
  goLoaders.set(asset.sha256, pending);
  return pending;
}
export async function fetchRuntimeAsset(
  asset: RuntimeAsset,
  progress?: (loaded: number, total: number) => void,
): Promise<Uint8Array> {
  const response = await fetch(asset.url);
  if (!response.ok) throw Error(`资源加载失败：${asset.url}`);
  const reader = response.body?.getReader(),
    chunks: Uint8Array[] = [];
  let loaded = 0;
  if (reader) {
    for (;;) {
      const item = await reader.read();
      if (item.done) break;
      chunks.push(item.value);
      loaded += item.value.length;
      progress?.(loaded, asset.bytes);
    }
  } else {
    const bytes = new Uint8Array(await response.arrayBuffer());
    chunks.push(bytes);
    loaded = bytes.length;
    progress?.(loaded, asset.bytes);
  }
  const bytes = new Uint8Array(loaded);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.length;
  }
  const digest = [
    ...new Uint8Array(await crypto.subtle.digest("SHA-256", bytes)),
  ]
    .map((v) => v.toString(16).padStart(2, "0"))
    .join("");
  if (loaded !== asset.bytes || digest !== asset.sha256)
    throw Error(`资源摘要不匹配：${asset.url}`);
  return bytes;
}
export async function loadRuntimeBridge(
  manifest: RuntimeManifest,
  progress: (percent: number, label: string) => void,
  level = 0,
): Promise<RuntimeBridge> {
  const target = window as RuntimeWindow;
  const Go = await runtimeGo(manifest.loader);
  const bytes = await fetchRuntimeAsset(manifest.wasm, (loaded, total) =>
    progress((loaded / total) * 70, "加载游戏程序"),
  );
  const go = new Go();
  const instance = await WebAssembly.instantiate(
    bytes.buffer as ArrayBuffer,
    go.importObject,
  );
  let failure: unknown;
  void go.run(instance.instance).catch((e) => {
    failure = e;
  });
  if (typeof target.contraRuntime !== "function")
    throw Error("游戏程序初始化失败");
  const invoke = target.contraRuntime;
  const bridge: RuntimeBridge = {
    call<T>(request: Record<string, unknown>): T {
      if (failure) throw failure;
      const reply = JSON.parse(invoke(JSON.stringify(request)));
      if (!reply.ok) throw Error(reply.error);
      return reply.data as T;
    },
  };
  const identity = manifest.bundle
    ? bridge.call<{ digest: string; rulesBuildID: string }>({
        op: "bundle",
        bundle: manifest.bundle,
      })
    : bridge.call<{ digest: string; rulesBuildID: string }>({
        op: "resources",
        resources: JSON.parse(
          new TextDecoder().decode(await fetchRuntimeAsset(manifest.resources)),
        ),
      });
  if (identity.rulesBuildID !== manifest.rulesBuildID)
    throw Error("规则程序版本不匹配");
  if (manifest.bundle) {
    const asset =
      manifest.stages?.[String(level)] ??
      (level === 0 ? manifest.resources : undefined);
    if (!asset) throw Error("缺少当前关卡资源描述");
    await installRuntimeStage(bridge, manifest.bundle, level, asset);
  }
  progress(75, "游戏规则已就绪");
  return bridge;
}

export async function installRuntimeStage(
  bridge: RuntimeBridge,
  bundle: RuntimeBundle,
  level: number,
  asset: RuntimeAsset,
): Promise<void> {
  if (
    bundle.stages[level]?.level !== level ||
    bundle.stages[level].sha256 !== asset.sha256
  )
    throw Error("关卡资源身份不匹配");
  const bytes = await fetchRuntimeAsset(asset);
  bridge.call({
    op: "stage",
    level,
    stageData: new TextDecoder().decode(bytes),
  });
}
