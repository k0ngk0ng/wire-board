export type ResidentResource = { url: string; bytes: number };
export type ResidentProgress = { loaded: number; total: number };
const observers = new Map<string, (progress: ResidentProgress) => void>();
let listening = false;
export function watchResidentResources(lease: string, update: (progress: ResidentProgress) => void) {
  if (!("serviceWorker" in navigator)) return () => {};
  if (!listening) {
    listening = true;
    navigator.serviceWorker.addEventListener("message", ({ data }: MessageEvent<unknown>) => {
      if (!data || typeof data !== "object") return;
      const value = data as Record<string, unknown>;
      if (value.type !== "resident-progress" || typeof value.lease !== "string" || typeof value.loaded !== "number" || typeof value.total !== "number" || !Number.isSafeInteger(value.loaded) || !Number.isSafeInteger(value.total) || value.loaded < 0 || value.total < value.loaded) return;
      observers.get(value.lease)?.({ loaded: value.loaded, total: value.total });
    });
  }
  observers.set(lease, update);
  return () => { observers.delete(lease); };
}
let workerReady: Promise<ServiceWorker> | undefined;
let workerSource = "";

function activated(worker: ServiceWorker): Promise<void> {
  if (worker.state === "activated") return Promise.resolve();
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => finish(Error("本地资源服务未能启动")), 15000);
    const update = () => {
      if (worker.state === "activated") finish();
      else if (worker.state === "redundant") finish(Error("本地资源服务已更新，请重新打开页面"));
    };
    const finish = (error?: Error) => {
      clearTimeout(timer);
      worker.removeEventListener("statechange", update);
      if (error) reject(error); else resolve();
    };
    worker.addEventListener("statechange", update);
    update();
  });
}

function controlled(script: string): Promise<ServiceWorker> {
  const current = navigator.serviceWorker.controller;
  if (current?.scriptURL === script) return Promise.resolve(current);
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => finish(undefined, Error("页面尚未连接本地资源服务")), 15000);
    const update = () => {
      const worker = navigator.serviceWorker.controller;
      if (worker?.scriptURL === script) finish(worker);
    };
    const finish = (worker?: ServiceWorker, error?: Error) => {
      clearTimeout(timer);
      navigator.serviceWorker.removeEventListener("controllerchange", update);
      if (worker) resolve(worker); else reject(error);
    };
    navigator.serviceWorker.addEventListener("controllerchange", update);
    update();
  });
}

function connect(source: string, build: string): Promise<ServiceWorker> {
  if (!("serviceWorker" in navigator) || !("caches" in globalThis)) return Promise.reject(Error("当前浏览器无法保存本地游戏资源"));
  const published = new URL(source, location.href);
  const script = published.origin === location.origin ? published : new URL("/asset-cache-worker.js", location.href);
  script.searchParams.set("build", build);
  if (!workerReady || workerSource !== script.href) {
    workerSource = script.href;
    const pending = (async () => {
      const registration = await navigator.serviceWorker.register(script.href, { scope: "/", updateViaCache: "all" });
      const worker = registration.installing ?? registration.waiting ?? registration.active;
      if (!worker) throw Error("本地资源服务不可用");
      await activated(worker);
      return controlled(worker.scriptURL);
    })();
    workerReady = pending;
    void pending.catch(() => { if (workerReady === pending) workerReady = undefined; });
  }
  return workerReady;
}

async function send(worker: ServiceWorker, payload: Record<string, unknown>, signal?: AbortSignal): Promise<void> {
  signal?.throwIfAborted();
  return new Promise((resolve, reject) => {
    const channel = new MessageChannel();
    const timeout = setTimeout(() => finish(Error("本地资源准备超时")), 60000);
    const cancelled = () => finish(signal?.reason ?? Error("资源加载已取消"));
    const finish = (error?: unknown) => {
      clearTimeout(timeout);
      signal?.removeEventListener("abort", cancelled);
      channel.port1.close();
      if (error) reject(error); else resolve();
    };
    channel.port1.onmessage = ({ data }: MessageEvent<{ ok: boolean; error?: string }>) => finish(data.ok ? undefined : Error(data.error || "本地资源准备失败"));
    signal?.addEventListener("abort", cancelled, { once: true });
    worker.postMessage(payload, [channel.port2]);
  });
}

export async function beginResidentResources(source: string, lease: string, build: string, resources: readonly ResidentResource[], origins: readonly string[], signal: AbortSignal) {
  const worker = await connect(source, build);
  signal.throwIfAborted();
  await send(worker, { type: "begin", lease, build, resources, origins }, signal);
}

export async function declareResidentResources(lease: string, resources: readonly ResidentResource[], signal: AbortSignal) {
  if (!workerReady) throw Error("本地资源服务尚未启动");
  await send(await workerReady, { type: "declare", lease, resources }, signal);
}

export async function aliasResidentResource(lease: string, url: string, source: string, signal: AbortSignal) {
  if (!workerReady) throw Error("本地资源服务尚未启动");
  await send(await workerReady, { type: "alias", lease, url, source }, signal);
}

export async function sealResidentResources(lease: string, signal: AbortSignal) {
  if (!workerReady) throw Error("本地资源服务尚未启动");
  await send(await workerReady, { type: "seal", lease }, signal);
}

export async function releaseResidentResources(lease: string) {
  if (!workerReady) return;
  try { await send(await workerReady, { type: "release", lease }); } catch {}
}
