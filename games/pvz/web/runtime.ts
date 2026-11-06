import type { PvzAction, View } from "./types";

type EngineView = Record<string, unknown> & Pick<View, "epoch" | "tick" | "phase" | "players" | "plants" | "zombies" | "projectiles" | "mowers"> & { phaseEndTick: number };
type EngineState = EngineView & { events: View["events"] };
type Reply = { ok: boolean; error?: string; state?: EngineState; view?: EngineView; digest?: string };
type Motion = { motionX?: number; motionY?: number; motionHeight?: number };
type Operation = { serial?: number; tick: number; actor: number; action: PvzAction };
type Packet = View & { revision?: number; applied?: Operation[]; snapshot?: EngineState; projected?: Partial<EngineState> };
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

async function loadRuntime(_legacyAssets: string): Promise<Port> {
  const base = url(import.meta.env.BASE_URL, "pvz/v1/runtime");
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
  reject(seq: number): void;
  readonly lastError: string;
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
  let authority: EngineState | undefined, projected: EngineView | undefined;
  let metadata: View | undefined, cached: View | undefined;
  let pending: { tick: number; actor: number; action: PvzAction }[] = [];
  let revision = -1, anchorAt = 0, anchorTick = 0, targetTick = 0;
  let authorityDigest: string | undefined, displayedTick = 0;
  let lastError = "";
  let disposed = false, recovering: Promise<void> | undefined, lastRecovery = 0;
  const load = (id: string, state: EngineState) => {
    const reply = port.call({ op: "load", id, snapshot: state });
    if (!reply.ok || !reply.state) throw Error(reply.error ?? "恢复规则状态失败");
    authorityDigest = reply.digest;
    return reply.state;
  };
  const stepAuthority = (ticks: number) => {
    if (!ticks) return authority!;
    const reply = port.call({ op: "step", id: authID, ticks });
    if (!reply.ok || !reply.state) throw Error(reply.error ?? "规则推进失败");
    authorityDigest = reply.digest;
    return reply.state;
  };
  const local = (request: Record<string, unknown>) => {
    const reply = port.call({ ...request, id: localID, compact: true });
    if (!reply.ok || !reply.view) throw Error(reply.error ?? "本地呈现失败");
    return reply.view;
  };
  const stepLocal = (ticks: number) => ticks ? local({ op: "step", ticks }) : projected!;
  const merge = () => {
    if (!metadata || !projected || !authority) return cached as View;
    cached = { ...metadata, ...projected, phase: metadata.phase, round: metadata.round, rounds: metadata.rounds, winnerTeam: metadata.winnerTeam, serverAt: metadata.serverAt, phaseUntil: metadata.phaseUntil, canAct: metadata.canAct, catalog: metadata.catalog, events: authority.events, snapshot: authority } as View;
    return cached;
  };
  const rebuild = () => {
    if (!authority) return;
    projected = local({ op: "fork", source: authID });
    const end = Math.max(authority.tick, Math.min(targetTick, authority.tick + MAX_PREDICTION, authority.phaseEndTick - 1));
    const retained: typeof pending = [];
    for (const item of pending) {
      const at = Math.max(projected.tick, Math.min(item.tick, end));
      projected = stepLocal(at - projected.tick);
      const reply = port.call({ op: "apply", id: localID, actor: item.actor, action: item.action, compact: true, atTick: Math.max(projected.tick, item.tick) });
      if (reply.ok && reply.view) { projected = reply.view; retained.push(item); }
    }
    pending = retained;
    projected = stepLocal(Math.max(0, end - projected.tick));
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
  const alignClock = (now: number) => {
    if (!authority) return;
    const elapsed = anchorTick + Math.max(0, now - anchorAt) / TICK_MS;
    // An authority packet can catch up to the browser, but cannot restart its
    // fractional clock. Duplicate confirmations never add a display buffer.
    if (elapsed < authority.tick) anchorTick += authority.tick - Math.floor(elapsed);
  };
  const accept = (view: View, recovery = false) => {
    if (disposed) return;
    const packet = view as Packet, now = performance.now();
    if(packet.projected){
      if(!authority||!metadata||authority.epoch!==packet.epoch){recover();return;}
      if(packet.tick<authority.tick)return;
      if(packet.tick===authority.tick&&(packet.revision??revision)===revision){metadata={...metadata,...packet};return;}
      authority=load(authID,{...authority,...packet.projected,projection:true} as EngineState);
      if(packet.digest&&authorityDigest!==packet.digest){recover();return;}
      metadata={...metadata,...packet};revision=packet.revision??revision;
      pending=pending.filter(item=>item.action.epoch===authority!.epoch&&item.action.seq>(authority!.players[item.actor]?.seq??0));
      alignClock(now);targetTick=Math.max(targetTick,authority.tick);rebuild();return;
    }
    if (packet.snapshot) {
      // Ordinary HTTP refreshes can race a newer ordered socket update.
      if (!recovery && authority?.epoch === packet.epoch && packet.tick < authority.tick) return;
      const changed = !authority || authority.epoch !== packet.epoch || recovery;
      authority = load(authID, packet.snapshot);
      if (packet.digest && authorityDigest !== packet.digest) { recover(); return; }
      metadata = changed ? packet : {...metadata,...packet};
      revision = packet.revision ?? 0;
      if (changed) { pending = []; anchorAt = now; anchorTick = authority.tick; targetTick = authority.tick; displayedTick = authority.tick; }
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
        authority = stepAuthority(op.tick - authority.tick);
        const reply = port.call({ op: "apply", id: authID, actor: op.actor, action: op.action });
        if (!reply.ok || !reply.state) { recover(); return; }
        authority = reply.state; authorityDigest = reply.digest; expected = op.serial ?? expected + 1;
      }
      authority = stepAuthority(packet.tick - authority.tick);
      if (packet.digest && authorityDigest !== packet.digest) { recover(); return; }
      revision = packet.revision ?? expected;
      metadata = { ...metadata, ...packet };
    }
    pending = pending.filter(item => item.action.epoch === authority!.epoch && item.action.seq > (authority!.players[item.actor]?.seq ?? 0));
    alignClock(now);
    targetTick = Math.max(targetTick, authority.tick);
    rebuild();
  };
  return {
    update(view) { cached ??= view; try { accept(view); } catch { recover(); } },
    sample(now) {
      if (!authority || !projected || disposed) return cached as View;
      const timeTick = Math.max(authority.tick, Math.min(anchorTick + Math.max(0, now - anchorAt) / TICK_MS, authority.tick + MAX_PREDICTION, authority.phaseEndTick - 1));
      const tick = Math.floor(timeTick);
      if (tick > projected.tick) {
        const crossesPreview = pending.some(item => item.tick > projected!.tick && item.tick <= tick);
        targetTick = tick;
        if (crossesPreview) rebuild();
        else { projected = stepLocal(tick - projected.tick); merge(); }
      }
      displayedTick = timeTick;
      const fraction = Math.max(0, Math.min(1, timeTick - projected!.tick));
      if (!fraction || metadata?.phase !== "battle") return cached as View;
      // RAF only extrapolates disclosed position deltas from the current
      // integer tick. No next-tick simulation, JSON snapshot, digest, collision
      // or event is evaluated here. Entity/health changes remain at that tick.
      return {
        ...cached!, tick: timeTick,
        zombies: projected!.zombies.map(z => ({ ...z, x: z.x + ((z as typeof z & Motion).motionX ?? 0) * fraction })),
        projectiles: projected!.projectiles.map(q => { const motion = q as typeof q & Motion; return { ...q, x: q.x + (motion.motionX ?? 0) * fraction, y: (q.y ?? q.row * 1000 + 500) + (motion.motionY ?? 0) * fraction, height: Math.max(0, (q.height ?? 0) + (motion.motionHeight ?? 0) * fraction) }; }),
        mowers: projected!.mowers.map(m => ({ ...m, x: m.x + ((m as typeof m & Motion).motionX ?? 0) * fraction })),
      };
    },
    apply(action, actor) {
      if (!authority || !projected || disposed) { lastError = "规则状态尚未就绪，请稍后重试"; return false; }
      if (pending.length >= MAX_PENDING) { lastError = "操作仍在确认，请稍后重试"; return false; }
      const atTick = Math.max(projected.tick, Math.floor(displayedTick) + 1);
      const reply = port.call({ op: "apply", id: localID, actor, action, compact: true, atTick });
      if (!reply.ok || !reply.view) { lastError = reply.error ?? "操作未能执行"; return false; }
      lastError = "";
      pending.push({ tick: atTick, actor, action }); projected = reply.view; merge(); return true;
    },
    reject(seq) {
      if (disposed || !pending.some(item => item.action.seq === seq)) return;
      pending = pending.filter(item => item.action.seq !== seq);
      if (authority) rebuild();
    },
    get lastError() { return lastError; },
    reset() {
      pending = [];
      targetTick = displayedTick = anchorTick = authority?.tick ?? 0;
      anchorAt = performance.now();
      if (authority) rebuild();
    },
    dispose() { disposed = true; pending = []; for (const id of [authID, localID]) port.call({ op: "drop", id }); },
  };
}
