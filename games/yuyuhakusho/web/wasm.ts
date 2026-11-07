import { fetchVerified, type AssetBytes } from "./assets";
import type { Progress, InputWindow } from "./types";

export type RuntimePlayer = {
  slot: number;
  character_id: number;
  computer: boolean;
  active: boolean;
  health: number;
  energy: number;
  X: number;
  Y: number;
  Pose: number;
  Action: number;
};
export type RuntimeView = {
  frame: string;
  ready: boolean;
  paused: boolean;
  completed: boolean;
  players: RuntimePlayer[];
};
export type RuntimeReply = {
  ok: boolean;
  error?: string;
  asset?: number;
  session?: number;
  view?: RuntimeView;
  rulesDigest?: string;
  presentationDigest?: string;
  snapshot?: string;
  checkpoint?: string;
  renderedFrame?: string;
  drawRequests?: number;
  spritePixels?: number;
  soundRequests?: number[];
};
export interface NetworkWasm {
  request(
    command: Readonly<Record<string, unknown>>,
    ...buffers: Uint8Array[]
  ): RuntimeReply;
  load(arena: number, bytes: AssetBytes): number;
}
type GoInstance = {
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
};
type RuntimeWindow = Window &
  typeof globalThis & {
    Go?: new () => GoInstance;
    yuyuhakushoNetworkWasm?: (
      command: string,
      ...buffers: Uint8Array[]
    ) => string;
  };
const loaded = new Map<string, Promise<NetworkWasm>>();
const progress = new Map<string, Progress>();
const listeners = new Map<string, Set<(value: Progress) => void>>();
let scriptLoading: Promise<void> | undefined;
function ensureGo(url: string) {
  const runtime = window as RuntimeWindow;
  if (runtime.Go) return Promise.resolve();
  if (scriptLoading) return scriptLoading;
  scriptLoading = new Promise<void>((resolve, reject) => {
    const script = document.createElement("script");
    script.src = url;
    const timer = window.setTimeout(() => {
      script.remove();
      reject(Error("游戏程序加载超时，请重试"));
    }, 45000);
    script.onload = () => {
      clearTimeout(timer);
      runtime.Go ? resolve() : reject(Error("游戏运行环境不可用"));
    };
    script.onerror = () => {
      clearTimeout(timer);
      script.remove();
      reject(Error("无法加载游戏运行环境"));
    };
    document.head.append(script);
  }).catch((error) => {
    scriptLoading = undefined;
    throw error;
  });
  return scriptLoading;
}
export function loadNetworkWasm(
  baseURL: string,
  report: (value: Progress) => void = () => {},
): Promise<NetworkWasm> {
  const base = new URL(baseURL.replace(/\/$/, "") + "/", location.href).href;
  const subscribers = listeners.get(base) ?? new Set();
  listeners.set(base, subscribers);
  subscribers.add(report);
  report(progress.get(base) ?? { percent: 0, label: "加载游戏程序" });
  const update = (value: Progress) => {
    progress.set(base, value);
    for (const subscriber of subscribers) subscriber(value);
  };
  let pending = loaded.get(base);
  if (!pending) {
    pending = (async () => {
      await ensureGo(new URL("wasm_exec.js", base).href);
      const response = await fetch(new URL("build.json", base), {
        signal: AbortSignal.timeout(45000),
        cache: "no-cache",
      });
      if (!response.ok) throw Error("无法读取游戏程序版本");
      const { wasm } = (await response.json()) as {
        wasm?: { url: string; sha256: string; bytes: number };
      };
      if (
        !wasm ||
        typeof wasm.url !== "string" ||
        !/^[a-f0-9]{64}$/.test(wasm.sha256) ||
        !Number.isSafeInteger(wasm.bytes) ||
        wasm.bytes < 1 ||
        wasm.bytes > 64 * 1024 * 1024
      )
        throw Error("游戏程序版本无效");
      const url = new URL(wasm.url, base);
      if (url.protocol !== "https:" && url.origin !== location.origin)
        throw Error("游戏程序地址无效");
      const bytes = await fetchVerified(
        url.href,
        wasm,
        AbortSignal.timeout(45000),
        (count) =>
          update({ percent: (count / wasm.bytes) * 90, label: "下载游戏程序" }),
      );
      update({ percent: 94, label: "初始化游戏程序" });
      const runtime = window as RuntimeWindow,
        go = new runtime.Go!();
      const module = await WebAssembly.instantiate(
        new Uint8Array(bytes).buffer,
        go.importObject,
      );
      let failure: unknown;
      void go.run(module.instance).catch((error) => {
        failure = error;
      });
      const call = runtime.yuyuhakushoNetworkWasm;
      if (typeof call !== "function") throw Error("游戏程序未完成初始化");
      const api: NetworkWasm = {
        request(command, ...buffers) {
          if (failure) throw Error("游戏程序已停止：" + String(failure));
          const result = JSON.parse(
            call(JSON.stringify(command), ...buffers),
          ) as RuntimeReply;
          if (!result.ok) throw Error(result.error ?? "游戏运算失败");
          return result;
        },
        load(arena, raw) {
          const reply = this.request(
            { op: "load", arena },
            raw.Rules,
            raw.Sprites,
            raw.Arena,
            raw.HUD,
            raw.ArenaAux,
            raw.RenderMetadata,
            raw.Descriptor,
          );
          if (!reply.asset) throw Error("比赛素材未初始化");
          return reply.asset;
        },
      };
      update({ percent: 100, label: "游戏程序已就绪" });
      return api;
    })().catch((error) => {
      loaded.delete(base);
      progress.delete(base);
      throw error;
    });
    loaded.set(base, pending);
  }
  return pending.finally(() => subscribers.delete(report));
}

// Replay windows and checkpoints remain opaque. The Go adapter alone decodes
// uint64 presentation state and the compact twelve-bit input stream.
export function replay(
  wasm: NetworkWasm,
  session: number,
  window: InputWindow,
  targetFrame: string,
  pixels?: Uint8Array,
) {
  return wasm.request(
    { op: "replay", session, replay: window, targetFrame },
    ...(pixels ? [pixels] : []),
  );
}
