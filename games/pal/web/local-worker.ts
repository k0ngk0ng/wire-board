import { LocalSimulation } from "./local-simulation";
import type { LocalMeta } from "./local-types";

type WorkerMessage =
  | {
      type: "init";
      checkpoint: string;
      core: string;
      meta: LocalMeta;
      now: number;
      sentAt: number;
    }
  | { type: "confirm"; meta: LocalMeta; now: number; sentAt: number }
  | {
      type: "input";
      input: {
        seq: number;
        direction: number;
        keys: number;
        preview?: boolean;
      };
      now: number;
      sentAt: number;
    }
  | { type: "cancel"; seq: number; now: number; sentAt: number }
  | { type: "active"; value: boolean };

let simulation: LocalSimulation | undefined,
  active = false,
  offset = 0,
  queue = Promise.resolve(),
  timer = 0,
  recovering = false;
const prepared = new Set<string>();
const now = () => performance.timeOrigin + performance.now() + offset;
const frame = (reason: "step" | "confirm" | "reset" = "step") => {
  if (!simulation) return;
  const value = simulation.frame(),
    key = `${value.scene}:${value.party.map((player) => player.role).join()}`;
  if (
    (value.phase === "field" || value.phase === "menu") &&
    !prepared.has(key)
  ) {
    prepared.add(key);
    postMessage({ type: "prepare", frames: simulation.previewMenus() });
  }
  postMessage({
    type: "frame",
    frame: value,
    pending: simulation.pendingCount,
    suspended: simulation.suspended,
    reason,
  });
};
const recover = (error: unknown) => {
  simulation?.stop();
  frame("reset");
  if (!recovering) {
    recovering = true;
    postMessage({
      type: "recover",
      reason: error instanceof Error ? error.message : "恢复仙剑同步",
    });
  }
};

async function receive(message: WorkerMessage) {
  if (message.type === "active") {
    if (active === message.value) return;
    active = message.value;
    if (!active) {
      simulation?.stop();
      frame("reset");
    } else if (simulation?.suspended) recover(Error("重新进入游戏"));
    return;
  }
  offset = message.now - message.sentAt;
  if (message.type === "init") {
    const response = await fetch(message.core, {
      signal: AbortSignal.timeout(20000),
      cache: "force-cache",
    });
    if (!response.ok) throw Error("无法加载仙剑本地核心");
    const bytes = await response.arrayBuffer(),
      digest = Array.from(
        new Uint8Array(await crypto.subtle.digest("SHA-256", bytes)),
        (byte) => byte.toString(16).padStart(2, "0"),
      ).join("");
    if (digest !== message.meta.core) throw Error("仙剑本地核心校验失败");
    const compiled = await WebAssembly.compile(bytes),
      checkpoint = Uint8Array.from(atob(message.checkpoint), (char) =>
        char.charCodeAt(0),
      );
    simulation = await LocalSimulation.create(
      compiled,
      checkpoint,
      message.meta,
      now(),
    );
    recovering = false;
    frame("reset");
    postMessage({ type: "ready" });
    clearInterval(timer);
    timer = setInterval(() => {
      if (!active || !simulation || recovering) return;
      try {
        if (simulation.tick(now())) frame();
        if (simulation.suspended) recover(Error("仙剑本地操作确认超时"));
      } catch (error) {
        recover(error);
      }
    }, 8) as unknown as number;
  } else if (message.type === "confirm" && simulation && !recovering) {
    if (await simulation.confirm(message.meta, now())) frame("confirm");
    postMessage({ type: "confirmed" });
  } else if (message.type === "input" && simulation && active && !recovering) {
    const action = simulation.input(message.input, now());
    if (action) {
      frame();
      postMessage({ type: "action", action, preview: message.input.preview });
    } else recover(Error("仙剑本地操作等待同步"));
  } else if (message.type === "cancel" && simulation) {
    simulation.cancel(message.seq, now());
    frame("reset");
  }
}

self.onmessage = (event: MessageEvent<WorkerMessage>) => {
  queue = queue.then(() => receive(event.data)).catch(recover);
};
