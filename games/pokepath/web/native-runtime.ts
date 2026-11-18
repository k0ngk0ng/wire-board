import type { DeploymentPreview, DeploymentRange } from "./placement";

export type NativeState = Record<string, unknown>;
export type NativeAction = Record<string, unknown>;
export type NativeProjection = Record<string, unknown>;
export type NativeHandle = Readonly<{ nativeHandle: number; projectionRevision: number; geometryRevision?: number; clock: number; epoch: number; sequence: number; physicsClock: number; tick: number; battleFrame: number; remainderMS: number; speedRemainder: number; paused: boolean; route: number }>;

export function readBattlePhysics(state: Pick<NativeHandle, "battleFrame" | "remainderMS" | "speedRemainder">) {
  return { step: state.battleFrame, interpolation: (state.remainderMS + state.speedRemainder) * 0.06 };
}
export type NativeRequest = <Value>(value: Record<string, unknown>) => Value;
export interface NativePredictionRuntime<View = NativeProjection, Action = NativeAction> {
  import(state: unknown): NativeHandle;
  clone(state: NativeHandle): NativeHandle;
  apply(state: NativeHandle, actor: number, action: Action): NativeHandle;
  advance(state: NativeHandle, milliseconds: number): NativeHandle;
  project(state: NativeHandle): View;
  frame(state: NativeHandle): View;
  deployment(state: NativeHandle, pokemonID: number): DeploymentPreview;
  deploymentRange(state: NativeHandle, pokemonID: number, tileID: number): DeploymentRange;
  inspect<Value = NativeProjection>(state: NativeHandle, pokemonID: number): Value;
  export(state: NativeHandle): NativeState;
  release(state: unknown): void;
  dispose(): void;
  readonly liveStates: number;
}
export type NativeProgress = { loaded: number; total?: number; phase: string; cached: boolean; path?: string; fileLoaded?: number; fileTotal?: number; resources?: readonly { path: string; bytes: number }[] };
export type NativeResourceExpectation = { bytes: number; sha256: string; mime: string };
export type NativeLoadOptions = { baseURL: string | URL; catalogBaseURL?: string | URL; route?: number; resolveResource?: (logicalPath: string) => string | URL | undefined; readResource?: (logicalPath: string, expected?: NativeResourceExpectation) => Promise<ArrayBuffer | undefined>; signal?: AbortSignal; onProgress?: (progress: NativeProgress) => void };

export interface NativeRuntime<State = NativeState, View = NativeProjection, Action = NativeAction> {
  readonly sourceSHA256: string;
  readonly buildSHA256: string;
  readonly bytes: number;
  readonly catalog: Record<string, unknown>;
  installCatalog(data: unknown): void;
  installRoute(data: unknown): void;
  ensureRoute(route: number, onProgress?: (progress: NativeProgress) => void): Promise<void>;
  createPredictionRuntime(): NativePredictionRuntime<View, Action>;
  create(options: unknown, players: number, seed: number | string, now: number): State;
  apply(state: State, actor: number, action: Action): State;
  advance(state: State, milliseconds: number): State;
  project(state: State): View;
  clone(state: State): State;
  restore(state: unknown): State;
}

type ResourceFile = { path: string; bytes: number; sha256: string; mime: string };
type Manifest = { format: number; engine: string; abi: string; sourceSHA256: string; buildSHA256: string; files: ResourceFile[]; catalog: { manifest: ResourceFile; core: string } };
type Bridge = { abi: string; ready: boolean; request: (value: string) => string };
type GoInstance = { importObject: WebAssembly.Imports; run: (instance: WebAssembly.Instance) => Promise<void> };
type RuntimeGlobals = typeof globalThis & { Go?: new () => GoInstance; wireBoardPokepathNative?: Bridge };
type Entry = { url: URL; catalogURL: URL; route: number; resourceGeneration: number; resolveResource?: NativeLoadOptions["resolveResource"]; readResource?: NativeLoadOptions["readResource"]; key: string; controller: AbortController; callbacks: Set<(progress: NativeProgress) => void>; latest: NativeProgress; promise: Promise<NativeRuntime>; ready?: NativeRuntime };

const entries = new Map<string, Entry>();
const abi = "wire-board-pokepath-go-v1";
const runtimeGlobals = globalThis as RuntimeGlobals;
const abortError = () => new DOMException("游戏加载已取消", "AbortError");
const integer = (value: number, label: string) => {
  if (!Number.isSafeInteger(value)) throw Error(`${label}必须为安全整数`);
};

function iterableProjection(value: Record<string, unknown>) {
  for (const key of ["team", "box", "towers", "enemies", "projectiles", "events", "items", "tiles", "paths"]) if (value[key] === null) value[key] = [];
  return value;
}

export function createNativePredictionRuntime<View = NativeProjection, Action = NativeAction>(request: NativeRequest, catalog: () => Record<string, unknown>): NativePredictionRuntime<View, Action> {
  const owned = new Map<number, NativeHandle>();
  const geometry = new Map<number, Record<string, unknown>>();
  const sprites = new Map<string, unknown>();
  const frameViews = new Map<string, Record<string, unknown>>();
  let disposed = false;
  const add = (state: NativeHandle) => {
    if (disposed || !state || !Number.isSafeInteger(state.nativeHandle) || state.nativeHandle < 1) throw Error("预测状态未能建立");
    const value = Object.freeze({ ...state });
    owned.set(value.nativeHandle, value);
    return value;
  };
  const use = (state: NativeHandle) => {
    if (disposed || !state || owned.get(state.nativeHandle) !== state) throw Error("预测状态已释放或不属于此视图");
    return state.nativeHandle;
  };
  const sprite = (kind: "species" | "enemies", key: string) => {
    const identity = `${kind}:${key}`;
    if (!sprites.has(identity)) {
      const table = catalog()[kind] as Record<string, { sprite?: Record<string, unknown> }> | undefined;
      const source = table?.[key]?.sprite;
      if (!source) throw Error("预测画面缺少角色素材信息");
      const defaults = { base: "", image: "", frames: 0, rows: 0, hold: 0, width: 0, height: 0, frameWidth: 0, frameHeight: 0, fractionalFrames: false };
      sprites.set(identity, Object.fromEntries(Object.entries(defaults).map(([key, value]) => [key, source[key] ?? value])));
    }
    return sprites.get(identity);
  };
  const party = (value: unknown, kind: "species" | "enemies", defaults?: Record<string, unknown>) => {
    if (!Array.isArray(value)) return value;
    return value.map(member => {
      const { spriteKey, negativeZeros, ...current } = member as Record<string, unknown>;
      if (Array.isArray(negativeZeros)) for (const key of negativeZeros) current[String(key)] = -0;
      if (defaults) return { ...defaults, ...current, conditions: current.conditions ?? [], ...(spriteKey ? { sprite: current.sprite ?? sprite(kind, String(spriteKey)) } : {}) };
      return spriteKey ? { ...current, sprite: current.sprite ?? sprite(kind, String(spriteKey)) } : current;
    });
  };
  const freeze = (value: unknown): unknown => {
    if (!value || typeof value !== "object" || Object.isFrozen(value)) return value;
    for (const child of Object.values(value)) freeze(child);
    return Object.freeze(value);
  };
  const project = (state: NativeHandle): View => {
    const handle = use(state);
    const geometryVersion = state.geometryRevision ?? state.projectionRevision;
    const base = geometry.get(geometryVersion);
    const value = iterableProjection(request<Record<string, unknown>>({ op: "session.project", handle, compact: Boolean(base) }));
    if (!base) {
      geometry.set(geometryVersion, Object.fromEntries(["tiles", "paths", "background", "width", "height"].map(key => [key, value[key]])));
      if (geometry.size > 4) geometry.delete(geometry.keys().next().value!);
      return value as View;
    }
    const { enemyDefaults, ...dynamic } = value;
    return { ...base, ...dynamic, team: party(value.team, "species"), box: party(value.box, "species"), enemies: party(value.enemies, "enemies", enemyDefaults as Record<string, unknown> | undefined) } as View;
  };
  return {
    import(state) { return add(request<NativeHandle>({ op: "session.import", state })); },
    clone(state) { return add(request<NativeHandle>({ op: "session.clone", handle: use(state) })); },
    apply(state, actor, action) {
      integer(actor, "操作席位");
      return add(request<NativeHandle>({ op: "session.apply", handle: use(state), actor, action }));
    },
    advance(state, milliseconds) {
      integer(milliseconds, "推进时长");
      return add(request<NativeHandle>({ op: "session.advance", handle: use(state), milliseconds }));
    },
    project,
    frame(state) {
      use(state);
      const key = `${state.projectionRevision}:${state.sequence}`;
      let base = frameViews.get(key);
      if (!base) {
        base = freeze(project(state)) as Record<string, unknown>;
        frameViews.set(key, base);
        if (frameViews.size > 4) frameViews.delete(frameViews.keys().next().value!);
      }
      if (base.clock === state.clock && base.remainderMS === state.remainderMS) return base as View;
      const value: Record<string, unknown> = { ...base, clock: state.clock, remainderMS: state.remainderMS };
      const progression = base.progression as Record<string, unknown> | undefined;
      const profileStats = progression?.profileStats as Record<string, unknown> | undefined;
      if (progression && profileStats && profileStats.timePlayed !== Math.floor(state.clock / 60000)) value.progression = { ...progression, profileStats: { ...profileStats, timePlayed: Math.floor(state.clock / 60000) } };
      return freeze(value) as View;
    },
    deployment(state, pokemonID) {
      integer(pokemonID, "伙伴标识");
      return request<DeploymentPreview>({ op: "session.deployment", handle: use(state), pokemonID });
    },
    deploymentRange(state, pokemonID, tileID) {
      integer(pokemonID, "伙伴标识");
      integer(tileID, "部署图格");
      return request<DeploymentRange>({ op: "session.deploymentRange", handle: use(state), pokemonID, tileID });
    },
    inspect<Value = NativeProjection>(state: NativeHandle, pokemonID: number) {
      integer(pokemonID, "伙伴标识");
      return request<Value>({ op: "session.inspect", handle: use(state), pokemonID });
    },
    export(state) { return request<NativeState>({ op: "session.export", handle: use(state) }); },
    release(state) {
      if (!state || typeof state !== "object") return;
      const value = state as NativeHandle;
      if (owned.get(value.nativeHandle) !== value) return;
      request({ op: "session.release", handles: [value.nativeHandle] });
      owned.delete(value.nativeHandle);
    },
    dispose() {
      if (disposed) return;
      try { if (owned.size) request({ op: "session.release", handles: [...owned.keys()] }); }
      finally { disposed = true; owned.clear(); geometry.clear(); sprites.clear(); frameViews.clear(); }
    },
    get liveStates() { return owned.size; }
  };
}

function notify(entry: Entry, update: NativeProgress) {
  entry.latest = { ...update, resources: update.resources ?? entry.latest.resources };
  for (const callback of entry.callbacks) callback({ ...entry.latest });
}

function validManifest(value: unknown): Manifest {
  if (!value || typeof value !== "object") throw Error("浏览器规则清单无效");
  const manifest = value as Manifest;
  if (manifest.format !== 1 || manifest.engine !== "wire-board-pokepath-go" || manifest.abi !== abi || !/^[a-f0-9]{64}$/.test(manifest.sourceSHA256) || !/^[a-f0-9]{64}$/.test(manifest.buildSHA256) || !Array.isArray(manifest.files) || manifest.files.length !== 3) throw Error("浏览器规则版本或清单不兼容");
  const expected = new Set(["engine.wasm", "wasm_exec.js", "go-runtime-LICENSE.txt"]);
  for (const file of manifest.files) {
    if (!file || !expected.delete(file.path) || !Number.isSafeInteger(file.bytes) || file.bytes <= 0 || file.bytes > 64 * 1048576 || !/^[a-f0-9]{64}$/.test(file.sha256)) throw Error("浏览器规则资源清单无效");
  }
  if (expected.size || manifest.files.reduce((sum, file) => sum + file.bytes, 0) > 128 * 1048576) throw Error("浏览器规则资源超出允许大小");
  if (!manifest.catalog || manifest.catalog.core !== "catalog.json" || manifest.catalog.manifest?.path !== "../resources.json" || !Number.isSafeInteger(manifest.catalog.manifest.bytes) || manifest.catalog.manifest.bytes <= 0 || manifest.catalog.manifest.bytes > 16 * 1048576 || !/^[a-f0-9]{64}$/.test(manifest.catalog.manifest.sha256)) throw Error("游戏数据清单无效");
  return manifest;
}

const logicalPath = (path: string, catalog = false) => `/pokepath/v1/${catalog ? "" : "native/"}${path}`;

function resourceURL(entry: Entry, path: string, catalog = false): URL {
  const logical = logicalPath(path, catalog);
  const supplied = entry.resolveResource?.(logical);
  const url = supplied === undefined ? new URL(path, catalog ? entry.catalogURL : entry.url) : new URL(supplied, window.location.href);
  if (!["http:", "https:"].includes(url.protocol) || url.username || url.password) throw Error("游戏资源地址无效");
  return url;
}

async function readResource(entry: Entry, file: ResourceFile, total?: number, catalog = false): Promise<ArrayBuffer> {
  const path = logicalPath(file.path, catalog);
  const supplied = await entry.readResource?.(path, { bytes: file.bytes, sha256: file.sha256, mime: file.mime });
  const chunks: Uint8Array[] = [];
  let size = 0;
  const append = (bytes: Uint8Array) => {
    size += bytes.byteLength;
    if (size > file.bytes) throw Error(`浏览器规则大小校验失败：${file.path}`);
    chunks.push(bytes);
    notify(entry, { loaded: entry.latest.loaded + bytes.byteLength, total, phase: "下载并校验游戏程序", cached: false, path, fileLoaded: size, fileTotal: file.bytes });
  };
  if (supplied !== undefined) append(new Uint8Array(supplied));
  else {
    const response = await fetch(resourceURL(entry, file.path, catalog), { signal: entry.controller.signal });
    if (!response.ok) throw Error(`浏览器规则加载失败：${file.path}（HTTP ${response.status}）`);
    const reader = response.body?.getReader();
    if (reader) {
      try {
        while (true) {
          const next = await reader.read();
          if (next.done) break;
          append(next.value);
        }
      } finally {
        reader.releaseLock();
      }
    } else append(new Uint8Array(await response.arrayBuffer()));
  }
  const data = new Uint8Array(size);
  let position = 0;
  for (const chunk of chunks) { data.set(chunk, position); position += chunk.length; }
  const digest = [...new Uint8Array(await crypto.subtle.digest("SHA-256", data.buffer))].map(byte => byte.toString(16).padStart(2, "0")).join("");
  if (size !== file.bytes || digest !== file.sha256) throw Error(`浏览器规则 SHA-256 或大小校验失败：${file.path}`);
  return data.buffer;
}

async function loadSupport(entry: Entry, file: ResourceFile) {
  const script = document.createElement("script");
  script.src = resourceURL(entry, file.path).href;
  script.async = true;
  script.crossOrigin = "anonymous";
  script.integrity = "sha256-" + btoa(String.fromCharCode(...file.sha256.match(/../g)!.map(part => parseInt(part, 16))));
  await new Promise<void>((resolve, reject) => {
    const abort = () => { script.remove(); reject(abortError()); };
    script.onload = () => { entry.controller.signal.removeEventListener("abort", abort); resolve(); };
    script.onerror = () => { entry.controller.signal.removeEventListener("abort", abort); script.remove(); reject(Error("游戏支持文件加载或完整性校验失败")); };
    entry.controller.signal.addEventListener("abort", abort, { once: true });
    if (entry.controller.signal.aborted) return abort();
    document.head.appendChild(script);
  });
  if (typeof runtimeGlobals.Go !== "function") throw Error("游戏支持文件未注册运行器");
  return runtimeGlobals.Go;
}

async function performLoad(entry: Entry): Promise<NativeRuntime> {
  let bytes = await entry.readResource?.(logicalPath("manifest.json"));
  if (bytes === undefined) {
    const response = await fetch(resourceURL(entry, "manifest.json"), { signal: entry.controller.signal });
    if (!response.ok) throw Error(`浏览器规则清单加载失败（HTTP ${response.status}）`);
    bytes = await response.arrayBuffer();
  }
  if (bytes.byteLength > 1048576) throw Error("浏览器规则清单过大");
  const manifest = validManifest(JSON.parse(new TextDecoder().decode(bytes)));
  notify(entry, { loaded: bytes.byteLength, phase: "读取当前路线资源清单", cached: false, path: logicalPath("manifest.json"), fileLoaded: bytes.byteLength, fileTotal: bytes.byteLength });
  const catalogIndexBytes = await readResource(entry, { ...manifest.catalog.manifest, path: "resources.json" }, undefined, true);
  const catalogIndex = JSON.parse(new TextDecoder().decode(catalogIndexBytes)) as { files?: ResourceFile[] };
  if (!Array.isArray(catalogIndex.files)) throw Error("游戏数据资源清单无效");
  const dataFiles = new Map(catalogIndex.files.map(file => [file.path, file]));
  const dataFile = (path: string) => {
    const file = dataFiles.get(path);
    if (!file || !Number.isSafeInteger(file.bytes) || file.bytes <= 0 || file.bytes > 64 * 1048576 || !/^[a-f0-9]{64}$/.test(file.sha256)) throw Error(`当前路线资源清单缺少 ${path}`);
    return file;
  };
  const coreFile = dataFile(manifest.catalog.core);
  const firstRoute = dataFile(`routes/${entry.route}.json`);
  const baseBytes = bytes.byteLength + catalogIndexBytes.byteLength + coreFile.bytes + manifest.files.reduce((sum, file) => sum + file.bytes, 0);
  const total = baseBytes + firstRoute.bytes;
  const baseResources = [{ path: logicalPath("manifest.json"), bytes: bytes.byteLength }, { path: logicalPath("resources.json", true), bytes: catalogIndexBytes.byteLength }, ...manifest.files.map(file => ({ path: logicalPath(file.path), bytes: file.bytes })), { path: logicalPath(coreFile.path, true), bytes: coreFile.bytes }];
  const routeResources = (file: ResourceFile) => [...baseResources, { path: logicalPath(file.path, true), bytes: file.bytes }];
  notify(entry, { loaded: entry.latest.loaded, total, phase: "下载并校验游戏程序", cached: false, resources: routeResources(firstRoute) });
  const resources = await Promise.all([
    ...manifest.files.map(async file => [file.path, await readResource(entry, file, total)] as const),
    ...[coreFile, firstRoute].map(async file => [file.path, await readResource(entry, file, total, true)] as const)
  ]);
  entry.controller.signal.throwIfAborted();
  notify(entry, { loaded: total, total, phase: "初始化游戏程序", cached: false });
  const support = manifest.files.find(file => file.path === "wasm_exec.js")!;
  const Go = await loadSupport(entry, support);
  const go = new Go();
  const wasm = resources.find(([path]) => path === "engine.wasm")![1];
  const instantiated = await WebAssembly.instantiate(wasm, go.importObject);
  entry.controller.signal.throwIfAborted();
  let active = true;
  let executionError = "";
  const execution = go.run(instantiated.instance);
  void execution.then(() => { active = false; }, error => { active = false; executionError = error instanceof Error ? error.message : String(error); });
  const bridge = runtimeGlobals.wireBoardPokepathNative;
  if (!bridge || !bridge.ready || bridge.abi !== abi || typeof bridge.request !== "function") throw Error("游戏程序尚未注册同步接口");
  const request = <Value>(value: Record<string, unknown>): Value => {
    if (!active) throw Error(executionError || "游戏运行器已停止");
    const response = JSON.parse(bridge.request(JSON.stringify(value))) as { ok: boolean; value?: Value; error?: string };
    if (!response.ok) throw Error(response.error || "游戏操作失败");
    return response.value as Value;
  };
  const information = request<{ abi: string; ready: boolean }>({ op: "info" });
  if (information.abi !== abi || information.ready !== true) throw Error("游戏程序初始化未完成");
  let activeCatalog = JSON.parse(new TextDecoder().decode(resources.find(([path]) => path === coreFile.path)![1])) as Record<string, unknown>;
  const initialRoute = JSON.parse(new TextDecoder().decode(resources.find(([path]) => path === firstRoute.path)![1])) as { id?: number };
  if (initialRoute.id !== entry.route) throw Error("当前路线数据与清单不一致");
  request({ op: "installCatalog", data: activeCatalog });
  request({ op: "installRoute", data: initialRoute });
  const installedRoutes = new Set([entry.route]);
  const pendingRoutes = new Map<string, Promise<void>>();
  const runtime: NativeRuntime = {
    sourceSHA256: manifest.sourceSHA256,
    buildSHA256: manifest.buildSHA256,
    bytes: total,
    createPredictionRuntime() { return createNativePredictionRuntime(request, () => activeCatalog); },
    get catalog() { return activeCatalog; },
    installCatalog(data) {
      request({ op: "installCatalog", data });
      activeCatalog = data as Record<string, unknown>;
      installedRoutes.clear();
      pendingRoutes.clear();
    },
    installRoute(data) {
      request({ op: "installRoute", data });
      const route = (data as { id?: number })?.id;
      if (Number.isSafeInteger(route)) installedRoutes.add(route!);
    },
    async ensureRoute(route, onProgress) {
      integer(route, "路线");
      const file = dataFile(`routes/${route}.json`);
      const routeTotal = baseBytes + file.bytes;
      if (installedRoutes.has(route)) {
        onProgress?.({ loaded: routeTotal, total: routeTotal, phase: "当前路线已就绪", cached: true, resources: routeResources(file) });
        return;
      }
      const pendingKey = `${entry.resourceGeneration}:${route}`;
      let pending = pendingRoutes.get(pendingKey);
      if (!pending) {
        const routeEntry: Entry = { ...entry, controller: new AbortController(), callbacks: new Set(onProgress ? [onProgress] : []), latest: { loaded: baseBytes, total: routeTotal, phase: "读取当前路线", cached: false, resources: routeResources(file) } };
        onProgress?.({ ...routeEntry.latest });
        pending = readResource(routeEntry, file, routeTotal, true).then(raw => {
          const data = JSON.parse(new TextDecoder().decode(raw)) as { id?: number };
          if (data.id !== route) throw Error("路线数据与资源清单不一致");
          request({ op: "installRoute", data });
          installedRoutes.add(route);
          onProgress?.({ loaded: routeTotal, total: routeTotal, phase: "当前路线已就绪", cached: false, resources: routeResources(file) });
        }).finally(() => { if (pendingRoutes.get(pendingKey) === pending) pendingRoutes.delete(pendingKey); });
        pendingRoutes.set(pendingKey, pending);
      }
      await pending;
    },
    create(options, players, seed, now) {
      integer(players, "席位数");
      integer(now, "逻辑时刻");
      if (typeof seed === "number") integer(seed, "随机种子");
      else if (!/^\d{1,20}$/.test(seed)) throw Error("随机种子无效");
      return request({ op: "new", options, players, seed, now });
    },
    apply(state, actor, action) {
      integer(actor, "操作席位");
      return request({ op: "apply", state, actor, action });
    },
    advance(state, milliseconds) {
      integer(milliseconds, "推进时长");
      return request({ op: "advance", state, milliseconds });
    },
    project(state) { return iterableProjection(request({ op: "project", state })); },
    clone(state) { return request({ op: "clone", state }); },
    restore(state) { return request({ op: "restore", state }); }
  };
  entry.ready = runtime;
  notify(entry, { loaded: total, total, phase: "游戏程序已就绪", cached: false });
  return runtime;
}

export async function loadNativeRuntime<State = NativeState, View = NativeProjection, Action = NativeAction>(options: NativeLoadOptions): Promise<NativeRuntime<State, View, Action>> {
  if (typeof window === "undefined") throw Error("浏览器规则只能在浏览器中初始化");
  if (options.signal?.aborted) throw abortError();
  const url = new URL(options.baseURL, window.location.href);
  const catalogURL = new URL(options.catalogBaseURL ?? "../", url);
  const route = options.route ?? 0;
  integer(route, "路线");
  for (const value of [url, catalogURL]) if (!["http:", "https:"].includes(value.protocol) || value.username || value.password || !value.pathname.endsWith("/")) throw Error("浏览器规则地址必须为 HTTP(S) 目录");
  const key = [options.resolveResource?.("/pokepath/v1/native/manifest.json") ?? url.href, options.resolveResource?.("/pokepath/v1/resources.json") ?? catalogURL.href].join("|");
  let entry = entries.get(key);
  if (entry) {
    if (entry.readResource !== options.readResource || entry.resolveResource !== options.resolveResource) entry.resourceGeneration++;
    entry.url = url;
    entry.catalogURL = catalogURL;
    entry.readResource = options.readResource;
    entry.resolveResource = options.resolveResource;
  }
  if (entry?.ready) {
    await entry.ready.ensureRoute(route, options.onProgress);
    if (options.signal?.aborted) throw abortError();
    return entry.ready as NativeRuntime<State, View, Action>;
  }
  if (!entry) {
    entry = { url, catalogURL, route, key, resourceGeneration: 0, resolveResource: options.resolveResource, readResource: options.readResource, controller: new AbortController(), callbacks: new Set(), latest: { loaded: 0, phase: "读取游戏资源清单", cached: false }, promise: Promise.resolve(undefined as unknown as NativeRuntime) };
    entries.set(key, entry);
    const current = entry;
    entry.promise = performLoad(entry).then(runtime => { current.ready = runtime; return runtime; }).catch(error => { if (entries.get(key) === current) entries.delete(key); throw error; });
  }
  const current = entry;
  const progress = (value: NativeProgress) => { if (!options.signal?.aborted) options.onProgress?.(value); };
  current.callbacks.add(progress);
  progress({ ...current.latest });
  let abort: (() => void) | undefined;
  try {
    const value = await new Promise<NativeRuntime>((resolve, reject) => {
      abort = () => reject(abortError());
      options.signal?.addEventListener("abort", abort, { once: true });
      current.promise.then(resolve, reject);
    });
    if (options.signal?.aborted) throw abortError();
    if (route !== current.route) await value.ensureRoute(route, progress);
    return value as NativeRuntime<State, View, Action>;
  } finally {
    if (abort) options.signal?.removeEventListener("abort", abort);
    current.callbacks.delete(progress);
    if (!current.ready && current.callbacks.size === 0) {
      current.controller.abort();
      if (entries.get(key) === current) entries.delete(key);
    }
  }
}
