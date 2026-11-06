import type { PvzAction, View } from "./types";

type EngineState = Record<string, unknown> & { epoch: string; tick: number; phase: View["phase"]; phaseEndTick: number; players: View["players"]; events: View["events"] };
type Reply = { ok: boolean; error?: string; state?: EngineState; digest?: string };
type Operation = { serial?: number; tick: number; actor: number; action: PvzAction };
type Packet = View & { revision?: number; applied?: Operation[]; snapshot?: EngineState };
type GoInstance = { importObject: WebAssembly.Imports; run(instance: WebAssembly.Instance): Promise<void> };
type RuntimeGlobal = typeof globalThis & { Go?: new () => GoInstance; pvzWasm?: (request: string) => string };
type Port = { call(request: Record<string, unknown>): Reply };

const loaded = new Map<string, Promise<Port>>();
let loader: Promise<void> | undefined;
let instanceSerial = 0;
const TICK_MS = 50;
const MAX_PREDICTION = 5;
const MAX_PENDING = 32;
const url = (base: string, file: string) => `${base.replace(/\/$/, "")}/${file.replace(/^\//, "")}`;

async function loadRuntime(assets: string): Promise<Port> {
  const base = url(assets, "pvz/v1/runtime");
  const previous = loaded.get(base); if (previous) return previous;
  const pending = (async () => {
    const manifestResponse = await fetch(url(base, "build.json"), { signal: AbortSignal.timeout(45000) });
    if (!manifestResponse.ok) throw Error("植物大战僵尸运行时版本加载失败");
    const manifest = await manifestResponse.json() as { sourceSHA256: string; files: Record<string, { sha256: string; bytes: number }> };
    const file = manifest.files?.["pvz.wasm"];
    if (!/^[a-f0-9]{64}$/.test(manifest.sourceSHA256) || !file || !/^[a-f0-9]{64}$/.test(file.sha256) || !Number.isSafeInteger(file.bytes) || file.bytes <= 0) throw Error("植物大战僵尸运行时清单无效");
    const runtime = globalThis as RuntimeGlobal;
    if (!runtime.Go) {
      loader ??= new Promise<void>((resolve, reject) => {
        const script = document.createElement("script"); script.src = url(base, "wasm_exec.js"); script.crossOrigin = "anonymous";
        const hash = manifest.files["wasm_exec.js"]?.sha256;
        if (hash && /^[a-f0-9]{64}$/.test(hash)) script.integrity = "sha256-" + btoa(String.fromCharCode(...hash.match(/../g)!.map(n => parseInt(n, 16))));
        script.onload = () => runtime.Go ? resolve() : reject(Error("浏览器运行时初始化失败"));
        script.onerror = () => { script.remove(); reject(Error("浏览器运行时加载失败")); };
        document.head.append(script);
      }).catch(error => { loader = undefined; throw error; });
      await loader;
    }
    const response = await fetch(url(base, "pvz.wasm"), { signal: AbortSignal.timeout(60000) });
    if (!response.ok) throw Error("植物大战僵尸规则程序加载失败");
    const bytes = await response.arrayBuffer();
    const hash = [...new Uint8Array(await crypto.subtle.digest("SHA-256", bytes))].map(n => n.toString(16).padStart(2, "0")).join("");
    if (bytes.byteLength !== file.bytes || hash !== file.sha256) throw Error("规则程序与版本清单不匹配，请重新加载");
    const go = new runtime.Go!();
    const result = await WebAssembly.instantiate(bytes, go.importObject);
    let stopped: unknown;
    void go.run(result.instance).catch(error => { stopped = error; });
    const call = runtime.pvzWasm;
    if (!call) throw Error("植物大战僵尸规则程序未就绪");
    return { call(request: Record<string, unknown>) {
      if (stopped) throw Error("植物大战僵尸规则程序已停止");
      return JSON.parse(call(JSON.stringify(request))) as Reply;
    } };
  })().catch(error => { loaded.delete(base); throw error; });
  loaded.set(base, pending); return pending;
}

export type Predictor = {
  update(view: View): void;
  sample(now: number): View;
  apply(action: PvzAction, actor: number): boolean;
  reset(): void;
  dispose(): void;
};

/** Replays confirmed operations in the shared Go engine, then overlays only a
 * bounded window of this client's pending actions. Confirmed events are the
 * sole source of audio; prediction never publishes an outcome or rating. */
export async function createPredictor(assets: string): Promise<Predictor> {
  const port = await loadRuntime(assets);
  const identity = `pvz-${++instanceSerial}`;
  const authID = `${identity}-auth`, localID = `${identity}-local`;
  let authority: EngineState | undefined, projected: EngineState | undefined;
  let metadata: View | undefined, cached: View | undefined;
  let pending: { tick: number; actor: number; action: PvzAction }[] = [];
  let revision = -1, anchorAt = 0, anchorTick = 0, targetTick = 0;
  let disposed = false, recovering: Promise<void> | undefined, lastRecovery = 0;
  const inspect = (id: string) => port.call({ op: "inspect", id });
  const load = (id: string, state: EngineState) => {
    const reply = port.call({ op: "load", id, snapshot: state });
    if (!reply.ok || !reply.state) throw Error(reply.error ?? "恢复规则状态失败");
    return reply.state;
  };
  const step = (id: string, ticks: number) => {
    const reply = ticks ? port.call({ op: "step", id, ticks }) : inspect(id);
    if (!reply.ok || !reply.state) throw Error(reply.error ?? "规则推进失败");
    return reply.state;
  };
  const merge = () => {
    if (!metadata || !projected || !authority) return cached as View;
    cached = { ...metadata, ...projected, phase: metadata.phase, round: metadata.round, rounds: metadata.rounds, winnerTeam: metadata.winnerTeam, serverAt: metadata.serverAt, phaseUntil: metadata.phaseUntil, canAct: metadata.canAct, catalog: metadata.catalog, events: authority.events, snapshot: authority } as View;
    return cached;
  };
  const rebuild = () => {
    if (!authority) return;
    projected = load(localID, authority);
    const end = Math.max(authority.tick, Math.min(targetTick, authority.tick + MAX_PREDICTION, authority.phaseEndTick - 1));
    const retained: typeof pending = [];
    for (const item of pending) {
      const at = Math.max(projected.tick, Math.min(item.tick, end));
      projected = step(localID, at - projected.tick);
      const reply = port.call({ op: "apply", id: localID, actor: item.actor, action: item.action });
      if (reply.ok && reply.state) { projected = reply.state; retained.push(item); }
    }
    pending = retained;
    projected = step(localID, Math.max(0, end - projected.tick));
    merge();
  };
  const recover = () => {
    if (disposed || recovering || performance.now() - lastRecovery < 1000) return;
    lastRecovery = performance.now();
    recovering = (async () => {
      // The existing authenticated state endpoint supplies a fresh full View.
      // No alternate input route or client-authored authority is introduced.
      const response = await fetch("/api/state", { cache: "no-store", signal: AbortSignal.timeout(10000) });
      if (!response.ok) throw Error("恢复对局失败");
      const state = await response.json();
      if (!disposed && state.room?.kind === "pvz" && state.room.game?.data?.snapshot) accept(state.room.game.data, true);
    })().catch(() => {}).finally(() => { recovering = undefined; });
  };
  const accept = (view: View, recovery = false) => {
    if (disposed) return;
    const packet = view as Packet, now = performance.now();
    if (packet.snapshot) {
      // Ordinary HTTP refreshes can race a newer ordered socket update.
      if (!recovery && authority?.epoch === packet.epoch && packet.tick < authority.tick) return;
      const changed = !authority || authority.epoch !== packet.epoch || recovery;
      authority = load(authID, packet.snapshot);
      if (packet.digest && inspect(authID).digest !== packet.digest) { recover(); return; }
      metadata = packet;
      revision = packet.revision ?? 0;
      if (changed) { pending = []; anchorAt = now; anchorTick = authority.tick; targetTick = authority.tick; }
    } else {
      if (!authority || !metadata || authority.epoch !== packet.epoch) { recover(); return; }
      if (packet.tick < authority.tick || (packet.revision ?? revision) < revision) return;
      if (packet.tick === authority.tick && (packet.revision ?? revision) === revision) { metadata = { ...metadata, ...packet }; return; }
      if (packet.tick - authority.tick > 2000) { recover(); return; }
      let expected = revision;
      for (const op of packet.applied ?? []) {
        if (op.serial !== undefined && op.serial <= revision) continue;
        if (op.serial !== undefined && op.serial !== expected + 1) { recover(); return; }
        if (op.tick < authority.tick || op.tick > packet.tick) { recover(); return; }
        authority = step(authID, op.tick - authority.tick);
        const reply = port.call({ op: "apply", id: authID, actor: op.actor, action: op.action });
        if (!reply.ok || !reply.state) { recover(); return; }
        authority = reply.state; expected = op.serial ?? expected + 1;
      }
      authority = step(authID, packet.tick - authority.tick);
      if (packet.digest && inspect(authID).digest !== packet.digest) { recover(); return; }
      revision = packet.revision ?? expected;
      metadata = { ...metadata, ...packet };
    }
    pending = pending.filter(item => item.action.epoch === authority!.epoch && item.action.seq > (authority!.players[item.actor]?.seq ?? 0));
    targetTick = Math.max(targetTick, authority.tick);
    rebuild();
  };
  return {
    update(view) { cached ??= view; try { accept(view); } catch { recover(); } },
    sample(now) {
      if (!authority || !projected || disposed) return cached as View;
      const tick = Math.max(authority.tick, Math.min(anchorTick + Math.floor(Math.max(0, now - anchorAt) / TICK_MS), authority.tick + MAX_PREDICTION, authority.phaseEndTick - 1));
      if (tick > projected.tick) { projected = step(localID, tick - projected.tick); targetTick = tick; merge(); }
      return cached as View;
    },
    apply(action, actor) {
      if (!authority || !projected || disposed || pending.length >= MAX_PENDING) return false;
      const reply = port.call({ op: "apply", id: localID, actor, action });
      if (!reply.ok || !reply.state) return false;
      pending.push({ tick: projected.tick, actor, action }); projected = reply.state; merge(); return true;
    },
    reset() { pending = []; targetTick = authority?.tick ?? 0; if (authority) rebuild(); },
    dispose() { disposed = true; pending = []; for (const id of [authID, localID]) port.call({ op: "drop", id }); },
  };
}
