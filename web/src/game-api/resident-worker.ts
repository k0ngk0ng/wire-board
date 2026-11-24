type Resource = { url: string; bytes: number };
type Contract = {
  client: string;
  lease: string;
  build: string;
  phase: "boot" | "loading" | "playing";
  origins: string[];
  resources: Resource[];
  aliases: Record<string, string>;
  failed: boolean;
  failures?: Record<string, boolean>;
  summary?: { loaded: number; total: number };
};
type LifetimeEvent = { waitUntil: (work: Promise<unknown>) => void };
type ResourceEvent = LifetimeEvent & { request: Request; clientId: string; resultingClientId: string; respondWith: (work: Promise<Response>) => void };
type CommandEvent = LifetimeEvent & { data: unknown; source: { id: string } | null; ports: readonly MessagePort[] };
type ResidentScope = {
  location: Location;
  clients: { claim: () => Promise<void>; get: (id: string) => Promise<{ postMessage: (value: unknown) => void } | undefined> };
  skipWaiting: () => Promise<void>;
  addEventListener(type: "install" | "activate", listener: (event: LifetimeEvent) => void): void;
  addEventListener(type: "fetch", listener: (event: ResourceEvent) => void): void;
  addEventListener(type: "message", listener: (event: CommandEvent) => void): void;
};
const scope = self as unknown as ResidentScope;
const assetCacheName = "wire-board-resident-assets-v1";
const contractCacheName = "wire-board-resident-contracts-v1";
const mappingCacheName = "wire-board-resident-mappings-v1";
const contracts = new Map<string, Promise<Contract | undefined>>();
const requirements = new WeakMap<Contract, Map<string, number>>();
const writes = new Map<string, Promise<void>>();
const active = new Map<string, Set<Promise<unknown>>>();
type StoredResource = { target: string; bytes: number };
type SharedDownload = { response: Promise<Response>; stored: Promise<StoredResource>; controller: AbortController; owners: Set<string>; claimed: boolean };
const downloads = new Map<string, SharedDownload>();
const ownerKey = (client: string, lease: string) => JSON.stringify([client, lease]);
const observedCacheName = (client: string, lease: string) => "wire-board-resident-observed-v1-" + encodeURIComponent(client) + "-" + encodeURIComponent(lease);
function cancelLease(client: string, lease: string) {
  const owner = ownerKey(client, lease);
  for (const entry of downloads.values()) { entry.owners.delete(owner); if (!entry.owners.size) entry.controller.abort(); }
}
type Budget = { lease: string; loaded: number; total: number; at: number; timer?: number; files: Map<string, { loaded: number; total: number }> };
const budgets = new Map<string, Budget>();

function notifyBudget(contract: Contract, force = false) {
  const budget = budgets.get(contract.client);
  if (!budget || budget.lease !== contract.lease || contract.phase !== "loading") return;
  const now = Date.now();
  if (!force && now - budget.at < 50) {
    if (budget.timer === undefined) budget.timer = setTimeout(() => { budget.timer = undefined; notifyBudget(contract, true); }, 50);
    return;
  }
  budget.at = now;
  const message = { type: "resident-progress", lease: budget.lease, loaded: budget.loaded, total: budget.total };
  void scope.clients.get(contract.client).then(client => client?.postMessage(message)).catch(() => {});
}

function noteResource(contract: Contract, lease: string, source: string, loaded: number, total: number, target?: string) {
  if (contract.lease !== lease || contract.phase !== "loading") return;
  let budget = budgets.get(contract.client);
  if (!budget || budget.lease !== lease) {
    budget = { lease, loaded: 0, total: 0, at: 0, files: new Map() };
    budgets.set(contract.client, budget);
  }
  const key = target ?? contract.aliases[source] ?? source;
  const alias = key === source ? undefined : budget.files.get(source);
  if (alias) {
    budget.loaded -= alias.loaded;
    budget.total -= alias.total;
    budget.files.delete(source);
    loaded = Math.max(loaded, alias.loaded);
    total = Math.max(total, alias.total);
  }
  const previous = budget.files.get(key) ?? { loaded: 0, total: 0 };
  const next = { loaded: Math.max(previous.loaded, loaded), total: Math.max(previous.total, total, loaded) };
  budget.files.set(key, next);
  budget.loaded += next.loaded - previous.loaded;
  budget.total += next.total - previous.total;
  notifyBudget(contract);
}

const record = (value: unknown): value is Record<string, unknown> => typeof value === "object" && value !== null && !Array.isArray(value);
const normalized = (value: string) => { const url = new URL(value, scope.location.href); url.hash = ""; url.search = ""; return url.href; };
const contractURL = (client: string) => scope.location.origin + "/__wire-board-resident/" + encodeURIComponent(client);
const fresh = (client: string): Contract => ({ client, lease: "", build: "", phase: "boot", origins: [], resources: [], aliases: {}, failed: false, failures: {} });
const failure = (cause: "resident" | "download" = "resident") => new Response("本地资源不可用，请重新打开游戏", { status: 504, headers: { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "no-store", "X-Wire-Board-Resident": cause } });

function readContract(client: string): Promise<Contract | undefined> {
  let pending = contracts.get(client);
  if (!pending) {
    pending = (async () => {
      const cache = await caches.open(contractCacheName);
      const response = await cache.match(contractURL(client));
      if (!response) return;
      const value: unknown = await response.json();
      if (!record(value) || value.client !== client || !["boot", "loading", "playing"].includes(String(value.phase)) || !record(value.aliases) || !Array.isArray(value.resources) || !Array.isArray(value.origins)) return;
      const contract = value as unknown as Contract;
      requirements.set(contract, new Map(contract.resources.map(file => [file.url, file.bytes])));
      return contract;
    })();
    contracts.set(client, pending);
  }
  return pending;
}

function persist(contract: Contract): Promise<void> {
  const prior = writes.get(contract.client) ?? Promise.resolve();
  const next = prior.catch(() => {}).then(async () => {
    const cache = await caches.open(contractCacheName);
    await cache.put(contractURL(contract.client), new Response(JSON.stringify(contract), { headers: { "Content-Type": "application/json" } }));
  });
  writes.set(contract.client, next);
  void next.finally(() => { if (writes.get(contract.client) === next) writes.delete(contract.client); }).catch(() => {});
  return next;
}

function track(client: string, work: Promise<unknown>) {
  let pending = active.get(client);
  if (!pending) active.set(client, pending = new Set());
  pending.add(work);
  void work.finally(() => { pending.delete(work); if (!pending.size) active.delete(client); }).catch(() => {});
}

async function rangeResponse(response: Response, request: Request): Promise<Response> {
  const headers = new Headers(response.headers);
  const length = Number(headers.get("X-Wire-Board-Bytes"));
  const range = request.headers.get("Range");
  if (!range) {
    if (request.method === "HEAD") void response.body?.cancel().catch(() => {});
    return new Response(request.method === "HEAD" ? null : response.body, { status: response.status, statusText: response.statusText, headers });
  }
  const match = /^bytes=(\d*)-(\d*)$/.exec(range);
  let start = Number(match?.[1] || "0");
  let end = match?.[2] ? Number(match[2]) : length - 1;
  if (match && !match[1] && match[2]) { start = Math.max(0, length - Number(match[2])); end = length - 1; }
  if (!match || !Number.isSafeInteger(length) || length <= 0 || !Number.isSafeInteger(start) || !Number.isSafeInteger(end) || start < 0 || start >= length || end < start) return new Response(null, { status: 416, headers: { "Content-Range": "bytes */" + length } });
  end = Math.min(end, length - 1);
  headers.set("Content-Range", `bytes ${start}-${end}/${length}`);
  headers.set("Content-Length", String(end - start + 1));
  headers.set("Accept-Ranges", "bytes");
  headers.delete("Content-Encoding");
  if (request.method === "HEAD") { void response.body?.cancel().catch(() => {}); return new Response(null, { status: 206, headers }); }
  const reader = response.body?.getReader();
  let offset = 0;
  const body = new ReadableStream<Uint8Array>({
    async pull(controller) {
      if (!reader) { controller.close(); return; }
      for (;;) {
        const value = await reader.read();
        if (value.done) { controller.close(); return; }
        const from = Math.max(0, start - offset);
        const to = Math.min(value.value.byteLength, end + 1 - offset);
        offset += value.value.byteLength;
        if (to > from) controller.enqueue(value.value.subarray(from, to));
        if (offset > end) { controller.close(); await reader.cancel(); return; }
        if (to > from) return;
      }
    },
    cancel(reason) { return reader?.cancel(reason); },
  });
  return new Response(request.method === "HEAD" ? null : body, { status: 206, headers });
}

async function storeResponse(source: string, response: Response, expected: number | undefined, progress: (loaded: number, total: number, target: string) => void): Promise<StoredResource> {
  const cache = await caches.open(assetCacheName);
  const target = normalized(response.url || source);
  const headers = new Headers(response.headers);
  const declared = Number(headers.get("Content-Length"));
  if (expected !== undefined) headers.set("X-Wire-Board-Bytes", String(expected));
  else if (!headers.has("Content-Encoding") && Number.isSafeInteger(declared) && declared > 0) headers.set("X-Wire-Board-Bytes", String(declared));
  headers.set("Accept-Ranges", "bytes");
  headers.delete("Content-Encoding");
  if (headers.has("X-Wire-Board-Bytes")) headers.set("Content-Length", headers.get("X-Wire-Board-Bytes")!);
  else headers.delete("Content-Length");
  const total = Number(headers.get("X-Wire-Board-Bytes")) || 0;
  progress(0, total, target);
  const chunks = response.body?.getReader();
  let size = 0;
  const body = new ReadableStream<Uint8Array>({
    async pull(controller) {
      if (!chunks) { controller.close(); return; }
      const value = await chunks.read();
      if (value.done) { controller.close(); return; }
      size += value.value.byteLength;
      progress(size, total, target);
      if (expected !== undefined && size > expected) { controller.error(Error("资源大小不匹配")); await chunks.cancel(); return; }
      controller.enqueue(value.value);
    },
    cancel(reason) { return chunks?.cancel(reason); },
  });
  await cache.put(target, new Response(body, { status: 200, headers }));
  if (expected !== undefined && size !== expected) { await cache.delete(target); throw Error("资源大小不匹配"); }
  const stored = await cache.match(target);
  if (!stored) throw Error("资源未能保存在本地");
  if (Number(stored.headers.get("X-Wire-Board-Bytes")) !== size) {
    const finalHeaders = new Headers(stored.headers);
    finalHeaders.set("X-Wire-Board-Bytes", String(size));
    finalHeaders.set("Content-Length", String(size));
    await cache.put(target, new Response(stored.body, { status: 200, headers: finalHeaders }));
  }
  return { target, bytes: size };
}

function mappingURL(build: string, source: string) {
  const url = new URL("/__wire-board-resident-map", scope.location.origin);
  url.searchParams.set("build", build);
  url.searchParams.set("source", source);
  return url.href;
}

async function mappedTarget(contract: Contract, source: string): Promise<string | undefined> {
  if (contract.aliases[source]) return contract.aliases[source];
  const cache = await caches.open(mappingCacheName);
  const response = await cache.match(mappingURL(contract.build, source));
  if (!response) return;
  const value: unknown = await response.json();
  if (!record(value) || typeof value.target !== "string") throw Error("本地资源索引无效");
  contract.aliases[source] = value.target;
  return value.target;
}

async function remember(contract: Contract, source: string, saved: StoredResource, build: string) {
  const cache = await caches.open(mappingCacheName);
  await cache.put(mappingURL(build, source), new Response(JSON.stringify(saved), { headers: { "Content-Type": "application/json" } }));
  if (contract.build === build) contract.aliases[source] = saved.target;
}

async function observeResource(contract: Contract, lease: string, resource: StoredResource) {
  if (contract.lease !== lease || contract.phase !== "loading") return;
  const cache = await caches.open(observedCacheName(contract.client, lease));
  await cache.put(resource.target, new Response(null, { headers: { "X-Wire-Board-Bytes": String(resource.bytes) } }));
}

async function residentResponse(event: ResourceEvent): Promise<Response> {
  const source = normalized(event.request.url);
  const contract = await readContract(event.clientId);
  const cache = await caches.open(assetCacheName);
  const mapped = contract ? await mappedTarget(contract, source) : undefined;
  const target = mapped ?? source;
  const retained = await cache.match(target);
  if (retained) {
    if (contract && !mapped) {
      const bytes = Number(retained.headers.get("X-Wire-Board-Bytes"));
      if (!Number.isSafeInteger(bytes) || bytes <= 0) return failure();
      await remember(contract, source, { target, bytes }, contract.build);
    }
    if (contract) { const size = Number(retained.headers.get("X-Wire-Board-Bytes")); noteResource(contract, contract.lease, source, size, size, target); await observeResource(contract, contract.lease, {target,bytes:size}); }
    return rangeResponse(retained, event.request);
  }
  if (!contract || contract.phase === "playing" || mapped) return failure();
  if (contract.origins.length && !contract.origins.includes(new URL(source).origin)) return failure();
  const lease = contract.lease;
  const build = contract.build;
  const owner = ownerKey(contract.client, lease);
  const downloadKey = JSON.stringify([build, source]);
  let pending = downloads.get(downloadKey);
  const expected = requirements.get(contract)?.get(source);
  if (!pending) {
    const headers = new Headers(event.request.headers);
    headers.delete("Range"); headers.delete("If-Range");
    const controller = new AbortController();
    const response = fetch(new Request(event.request, { method: "GET", headers, cache: "force-cache", mode: "cors", credentials: "omit", signal: controller.signal })).then(value => {
      if (!value.ok || value.status !== 200 || value.type === "opaque") throw Error("资源下载未完成");
      return value;
    });
    const stored = response.then(value => storeResponse(source, value.clone(), expected, (loaded, total, target) => noteResource(contract, lease, source, loaded, total, target)));
    pending = { response, stored, controller, owners: new Set(), claimed: false };
    const entry = pending;
    downloads.set(downloadKey, entry);
    void stored.finally(() => { if (downloads.get(downloadKey) === entry) downloads.delete(downloadKey); }).catch(() => {});
  }
  pending.owners.add(owner);
  const entry = pending;
  const storing = entry.stored.then(async saved => {
    if (expected !== undefined && expected !== saved.bytes) throw Error("资源大小不匹配");
    noteResource(contract, lease, source, saved.bytes, saved.bytes, saved.target);
    await remember(contract, source, saved, build);
    if (saved.target !== source) await remember(contract, saved.target, saved, build);
    await observeResource(contract, lease, saved);
  }).catch(async error => {
    if (contract.lease === lease) { contract.failed = true; (contract.failures ??= {})[source] = true; await persist(contract); }
    throw error;
  });
  track(owner, storing);
  event.waitUntil(storing);
  void storing.finally(() => entry.owners.delete(owner)).catch(() => {});
  try {
    if (entry.claimed) {
      const saved = await entry.stored;
      const response = await cache.match(saved.target);
      return response ? rangeResponse(response, event.request) : failure();
    }
    entry.claimed = true;
    const response = await entry.response;
    const headers = new Headers(response.headers);
    const declared = Number(headers.get("Content-Length"));
    if (expected !== undefined) headers.set("X-Wire-Board-Bytes", String(expected));
    else if (!headers.has("Content-Encoding") && Number.isSafeInteger(declared) && declared > 0) headers.set("X-Wire-Board-Bytes", String(declared));
    headers.delete("Content-Encoding");
    if (headers.has("X-Wire-Board-Bytes")) headers.set("Content-Length", headers.get("X-Wire-Board-Bytes")!);
    else headers.delete("Content-Length");
    if (event.request.headers.has("Range") && !headers.has("X-Wire-Board-Bytes")) {
      const saved = await entry.stored;
      const retained = await cache.match(saved.target);
      await response.body?.cancel();
      return retained ? rangeResponse(retained, event.request) : failure();
    }
    return rangeResponse(new Response(response.body, { status: 200, headers }), event.request);
  } catch {
    return failure("download");
  }
}

function parseResources(value: unknown): Resource[] {
  if (!Array.isArray(value)) throw Error("资源目录无效");
  return value.map(file => {
    if (!record(file) || typeof file.url !== "string" || typeof file.bytes !== "number" || !Number.isSafeInteger(file.bytes) || file.bytes <= 0) throw Error("资源目录无效");
    const url = new URL(file.url, scope.location.href);
    if (!["http:", "https:"].includes(url.protocol) || url.username || url.password) throw Error("资源地址无效");
    return { url: normalized(url.href), bytes: file.bytes };
  });
}

async function command(event: CommandEvent) {
  const value = event.data;
  if (!record(value) || !event.source?.id || typeof value.type !== "string" || !["begin", "declare", "alias", "seal", "release"].includes(value.type) || typeof value.lease !== "string") throw Error("资源加载请求无效");
  const client = event.source.id;
  let contract = await readContract(client);
  if (!contract) { contract = fresh(client); contracts.set(client, Promise.resolve(contract)); }
  if (value.type === "begin") {
    if (typeof value.build !== "string" || !Array.isArray(value.origins) || !Array.isArray(value.resources)) throw Error("资源目录无效");
    const resources = parseResources(value.resources);
    const origins = value.origins.map(origin => { if (typeof origin !== "string") throw Error("资源来源无效"); return new URL(origin).origin; });
    if (contract.lease && contract.lease !== value.lease) cancelLease(client, contract.lease);
    if (contract.build !== value.build) contract.aliases = {};
    contract.build = value.build;
    contract.lease = value.lease;
    contract.phase = "loading";
    contract.origins = origins;
    contract.resources = resources;
    requirements.set(contract, new Map(resources.map(file => [file.url, file.bytes])));
    contract.failed = false;
    contract.failures = {};
    contract.summary = undefined;
    budgets.delete(client);
    for (const file of resources) noteResource(contract, value.lease, normalized(file.url), 0, file.bytes);
  } else if (contract.lease === value.lease && value.type === "declare") {
    if (contract.phase !== "loading") throw Error("当前对局不能追加下载资源");
    const files = new Map(contract.resources.map(file => [normalized(file.url), file]));
    for (const file of parseResources(value.resources)) {
      const previous = files.get(file.url);
      if (previous && previous.bytes !== file.bytes) throw Error("资源校验信息冲突");
      files.set(file.url, file);
    }
    contract.resources = [...files.values()];
    requirements.set(contract, new Map(contract.resources.map(file => [file.url, file.bytes])));
    for (const file of parseResources(value.resources)) noteResource(contract, value.lease, normalized(file.url), 0, file.bytes);
  } else if (contract.lease === value.lease && value.type === "alias") {
    if (contract.phase !== "loading" || typeof value.url !== "string" || typeof value.source !== "string") throw Error("资源替代请求无效");
    await Promise.allSettled([...(active.get(ownerKey(client, value.lease)) ?? [])]);
    const source = normalized(value.source);
    const primary = normalized(value.url);
    const target = await mappedTarget(contract, source) ?? source;
    const cache = await caches.open(assetCacheName);
    const response = await cache.match(target);
    const bytes = Number(response?.headers.get("X-Wire-Board-Bytes"));
    if (!response || !Number.isSafeInteger(bytes) || bytes <= 0 || requirements.get(contract)?.get(primary) !== bytes) throw Error("替代资源尚未完整加载");
    await remember(contract, primary, { target, bytes }, contract.build);
    if (contract.failures) delete contract.failures[primary];
    contract.failed = Object.keys(contract.failures ?? {}).length > 0;
    noteResource(contract, value.lease, primary, bytes, bytes, target);
  } else if (contract.lease === value.lease && value.type === "seal") {
    if (contract.phase === "playing" && contract.summary) return { ok: true, phase: contract.phase, ...contract.summary };
    await Promise.all([...(active.get(ownerKey(client, contract.lease)) ?? [])]);
    if (contract.failed) throw Error("资源未能完整保存在本地");
    const cache = await caches.open(assetCacheName);
    for (const file of contract.resources) {
      const source = normalized(file.url);
      const target = await mappedTarget(contract, source) ?? source;
      const response = await cache.match(target);
      if (!response) throw Error("资源尚未完整加载");
      if (Number(response.headers.get("X-Wire-Board-Bytes")) !== file.bytes) throw Error("本地资源大小不匹配");
      noteResource(contract, value.lease, source, file.bytes, file.bytes, target);
      await observeResource(contract, value.lease, {target,bytes:file.bytes});
    }
    const observed = await caches.open(observedCacheName(client, value.lease));
    let total = 0;
    for (const request of await observed.keys()) {
      const resource = await observed.match(request);
      total += Number(resource?.headers.get("X-Wire-Board-Bytes") ?? 0);
    }
    contract.summary = { loaded: total, total };
    const budget = budgets.get(client);
    if (budget?.lease === value.lease) { budget.loaded = total; budget.total = total; }
    notifyBudget(contract, true);
    contract.phase = "playing";
  } else if (value.type === "release") {
    cancelLease(client, value.lease);
    await caches.delete(observedCacheName(client, value.lease));
    if (contract.lease === value.lease) {
      contract.phase = "boot";
      contract.lease = "";
      contract.resources = [];
      contract.origins = [];
    }
  }
  await persist(contract);
  if (value.type === "seal" && contract.lease === value.lease) await caches.delete(observedCacheName(client, value.lease));
  return { ok: true, phase: contract.phase, ...contract.summary };
}

function isResource(request: Request) {
  const url = new URL(request.url);
  if (!["http:", "https:"].includes(url.protocol)) return false;
  if (url.origin === scope.location.origin && (url.pathname.startsWith("/api/") || ["/game-load-manifest.json", "/cdn-assets-manifest.json", "/asset-cache-worker.js", "/resident-worker.js"].includes(url.pathname))) return false;
  if (request.destination === "script" && /^\/(?:src|@fs|@vite|games)\//.test(url.pathname)) return false;
  return ["image", "audio", "video", "font", "style", "script", "worker"].includes(request.destination) || /^\/(?:bga|_media)\//.test(url.pathname) || /\.(?:wasm|json|gz|zip|flac|mp3|wav|ogg|png|jpg|jpeg|svg|gif|webp|woff2?|ttf|otf|glb|pvzr|bin|idx|rgb|ico)$/.test(url.pathname);
}

scope.addEventListener("install", event => event.waitUntil(scope.skipWaiting()));
scope.addEventListener("activate", event => event.waitUntil(scope.clients.claim()));
scope.addEventListener("message", event => {
  const work = command(event).then(value => event.ports[0]?.postMessage(value), error => event.ports[0]?.postMessage({ ok: false, error: error instanceof Error ? error.message : "本地资源不可用" }));
  event.waitUntil(work);
});
scope.addEventListener("fetch", event => {
  if (event.request.mode === "navigate") {
    event.respondWith((async () => {
      if (event.resultingClientId) { const contract = fresh(event.resultingClientId); contracts.set(contract.client, Promise.resolve(contract)); await persist(contract); }
      return fetch(event.request);
    })());
    return;
  }
  if (!event.clientId || !isResource(event.request)) return;
  event.respondWith(["GET", "HEAD"].includes(event.request.method) ? residentResponse(event).catch(() => failure()) : Promise.resolve(new Response(null, { status: 405 })));
});
