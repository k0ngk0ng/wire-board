import { isTransientSyncError } from "../../../web/src/game-api/sync-error.ts";
import type { GameRoom } from "../../../web/src/game-api/module";
import type {
  RuntimeBundle,
  RuntimeCheckpoint,
  RuntimePoll,
  RuntimeSnapshot,
  RuntimeSnapshotHeader,
} from "./runtime-bridge";
import type { RuntimeTransport } from "./runtime-client";

export type RoomRuntimeView = {
  runtimeBuildID: string;
  visualsID: string;
  protocol: number;
  epoch: string;
  actor: number;
  seatActors: number[];
  inputs: { sequence: number; held: number; automated: boolean }[];
  snapshot: RuntimeSnapshotHeader &
    Partial<Pick<RuntimeSnapshot, "presentation" | "audio">>;
  revision: string;
  frames: RuntimePoll["frames"];
  stage: { level: number; path: string; sha256: string; digest: string };
  bundle: Pick<RuntimeBundle, "id" | "version">;
  rulesBuildID: string;
  stageLoading?: { level: number; ready: boolean[] };
  resync?: boolean;
};
type Binding = {
  room: Pick<
    GameRoom,
    "id" | "version" | "status" | "you" | "seats" | "loading"
  >;
  view: RoomRuntimeView;
};
type Sync = {
  version: number;
  view: RoomRuntimeView & { snapshot: RuntimeSnapshot };
  checkpoint: RuntimeCheckpoint;
};
type Options = {
  runtimeBuildID?: string;
  roomID: string;
  epoch: string;
  actor: number;
  current: () => Binding;
  act: (action: Record<string, unknown>) => Promise<void>;
  sync: () => Promise<Sync>;
  prepare: (view: RoomRuntimeView) => Promise<void>;
};
const recoverable = (error: unknown) =>
  isTransientSyncError(error) ||
  (error instanceof Error &&
    /stale or conflicting input sequence|input queue full|取消托管/.test(
      error.message,
    ));

/** Room actions share one ordered sequence with stage acknowledgements. The
 * public platform still authenticates the seat and decides every accepted pad.
 * Snapshot updates are read from the host; /sync is explicit recovery only. */
export function createRoomTransport(options: Options): RuntimeTransport & {
  initial: () => Promise<RuntimeSnapshot>;
  dispose: () => void;
} {
  let sequence = 0,
    chain = Promise.resolve(),
    closed = false,
    recovering = false,
    retryAt = 0,
    synced: Sync | undefined,
    pendingCheckpoint: RuntimeCheckpoint | undefined,
    syncing: Promise<Sync> | undefined;
  const readySent = new Set<string>();
  const current = (): Binding => {
    const b = options.current();
    if (b.room.id !== options.roomID || b.view.epoch !== options.epoch)
      throw Error("对局已切换");
    if (
      options.runtimeBuildID &&
      b.view.runtimeBuildID !== options.runtimeBuildID
    )
      throw Error("运行程序版本已改变，请重新加载");
    return synced && synced.version > b.room.version
      ? { room: b.room, view: synced.view }
      : b;
  };
  const owner = (b: Binding) =>
    b.view.actor === options.actor &&
    options.actor >= 0 &&
    b.room.status === "playing" &&
    !b.room.loading;
  const inputEnabled = () => {
    if (closed || recovering) return false;
    const b = current();
    return (
      owner(b) &&
      (!b.view.stageLoading || b.view.stageLoading.ready[options.actor]) &&
      !b.room.seats[b.room.you]?.autoPlay &&
      !b.room.seats[b.room.you]?.bot &&
      !b.view.snapshot.finalized
    );
  };
  const sync = async () => {
    if (!syncing)
      syncing = (async () => {
        const value = await options.sync();
        if (closed) throw Error("对局已离开");
        if (value.view.epoch !== options.epoch)
          throw Error("恢复快照属于另一局");
        if (
          options.runtimeBuildID &&
          value.view.runtimeBuildID !== options.runtimeBuildID
        )
          throw Error("运行程序版本已改变，请重新加载");
        await options.prepare(value.view);
        synced = value;
        sequence = Math.max(
          sequence,
          value.view.inputs[options.actor]?.sequence ?? 0,
        );
        pendingCheckpoint = value.checkpoint;
        recovering = false;
        return value;
      })().finally(() => {
        syncing = undefined;
      });
    return syncing;
  };
  const send = (
    make: (binding: Binding) => Record<string, unknown> | undefined,
  ) => {
    const work = chain.then(async () => {
      if (closed) return;
      if (recovering) {
        try {
          await sync();
        } catch (e) {
          if (recoverable(e)) {
            retryAt = performance.now() + 500;
            return;
          }
          throw e;
        }
      }
      const b = current();
      if (!owner(b)) return;
      const action = make(b);
      if (!action) return;
      sequence =
        Math.max(sequence, b.view.inputs[options.actor]?.sequence ?? 0) + 1;
      if (!Number.isSafeInteger(sequence)) throw Error("控制序号已耗尽");
      try {
        await options.act({ ...action, epoch: options.epoch, sequence });
      } catch (e) {
        if (!recoverable(e)) throw e;
        recovering = true;
        readySent.clear();
        try {
          await sync();
        } catch (next) {
          if (!recoverable(next)) throw next;
          retryAt = performance.now() + 500;
        }
      }
    });
    chain = work.catch(() => {});
    return work;
  };
  const ready = async () => {
    const b = current(),
      gate = b.view.stageLoading;
    if (
      !owner(b) ||
      !gate ||
      gate.ready[options.actor] ||
      b.room.seats[b.room.you]?.autoPlay ||
      b.room.seats[b.room.you]?.bot
    )
      return;
    const key = `${b.view.bundle.id}:${b.view.rulesBuildID}:${gate.level}`;
    if (readySent.has(key)) return;
    await options.prepare(b.view);
    await send((latest) => {
      if (!latest.view.stageLoading || latest.view.stage.level !== gate.level)
        return;
      return {
        type: "stage-ready",
        bundleID: latest.view.bundle.id,
        rulesBuildID: latest.view.rulesBuildID,
        runtimeBuildID: latest.view.runtimeBuildID,
        level: latest.view.stage.level,
        stageSHA256: latest.view.stage.sha256,
      };
    });
    if (!recovering) readySent.add(key);
  };
  return {
    actor: () => options.actor,
    initial: async () => {
      const value = await sync();
      if (!value.view.snapshot.presentation || !value.view.snapshot.audio)
        throw Error("初始快照缺少画面或音频状态");
      return value.view.snapshot;
    },
    inputEnabled,
    sequenceBase: () =>
      Math.max(sequence, current().view.inputs[options.actor]?.sequence ?? 0),
    checkpoint: async () => {
      if (!pendingCheckpoint) await sync();
      const checkpoint = pendingCheckpoint!;
      pendingCheckpoint = undefined;
      return checkpoint;
    },
    ready,
    input: (held) =>
      send((b) => (inputEnabled() ? { type: "input", held } : undefined)),
    release: () => send(() => ({ type: "release" })),
    poll: async (after) => {
      if (recovering && performance.now() >= retryAt) {
        try {
          await sync();
        } catch (e) {
          if (!recoverable(e)) throw e;
          retryAt = performance.now() + 500;
        }
      }
      let b = current();
      if (recovering) return { snapshot: b.view.snapshot, frames: [] };
      await options.prepare(b.view);
      await ready();
      b = current();
      const frames = b.view.frames.filter(
        (row) => BigInt(row.revision ?? row.snapshot!.revision) > BigInt(after),
      );
      const head = BigInt(b.view.revision),
        first = frames[0]?.revision ?? frames[0]?.snapshot?.revision;
      if (
        head > BigInt(after) &&
        (b.view.resync || !first || BigInt(first) !== BigInt(after) + 1n)
      ) {
        await sync();
        return {
          snapshot: synced!.view.snapshot,
          frames: [],
          checkpoint: pendingCheckpoint,
        };
      }
      if (pendingCheckpoint)
        return {
          snapshot: b.view.snapshot,
          frames: [],
          checkpoint: pendingCheckpoint,
        };
      return { snapshot: b.view.snapshot, frames };
    },
    dispose: () => {
      closed = true;
      readySent.clear();
      pendingCheckpoint = undefined;
    },
  };
}
