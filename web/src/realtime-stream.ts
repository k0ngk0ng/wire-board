import type { State, Game } from "./types";

type StreamState = State & { snapshotAt?: number };
export type GamePacket = {
  type: "game";
  serverInstance?: string;
  roomId: string;
  setupVersion: number;
  version: number;
  status: string;
  turnDeadline: number;
  serverNow: number;
  game: Game;
};

export function mergeStreamState(
  previous: StreamState | undefined,
  incoming: State,
  receivedAt: number,
): StreamState {
  if (previous?.user.id === incoming.user.id && previous?.serverInstance === incoming.serverInstance) {
    if (incoming.serverNow < (previous.snapshotAt ?? 0)) return previous;
    const keepRoom =
      incoming.serverNow < previous.serverNow ||
      (incoming.room?.id === previous.room?.id &&
        (incoming.room?.version ?? 0) < (previous.room?.version ?? 0));
    if (keepRoom)
      return {
        ...incoming,
        room: previous.room,
        serverNow: previous.serverNow,
        receivedAt: previous.receivedAt,
        snapshotAt: incoming.serverNow,
      };
  }
  return { ...incoming, receivedAt, snapshotAt: incoming.serverNow };
}

export function mergeStreamGame(
  previous: StreamState | undefined,
  packet: GamePacket,
  receivedAt: number,
): StreamState | undefined {
  const room = previous?.room;
  if (
    !previous ||
    previous.serverInstance !== packet.serverInstance ||
    !room ||
    room.id !== packet.roomId ||
    room.setupVersion !== packet.setupVersion ||
    packet.version <= room.version ||
    room.status !== "playing"
  )
    return previous;
  return {
    ...previous,
    serverNow: packet.serverNow,
    receivedAt,
    room: {
      ...room,
      version: packet.version,
      status: packet.status,
      turnDeadline: packet.turnDeadline,
      game: packet.game,
    } as State["room"],
  };
}

type Socket = {
  readyState: number;
  bufferedAmount: number;
  send(data: string): void;
};
type Input = {
  serverInstance?: string;
  roomId: string;
  version: number;
  nonce: string;
  action: Record<string, unknown>;
};
type Ack = { durable?: boolean; nonce: string; status: number; version?: number; error?: string };

// A successful ACK means authoritative acceptance; only durable:true confirms
// persistence. Ordinary movement is checkpointed asynchronously. A closed
// stream rejects outstanding requests; it never resurrects held keys on reconnect.
// Games resync their own sequence/epoch from the next authoritative snapshot.
export class StreamInputs {
  private socket?: Socket;
  private pending = new Map<
    string,
    {
      resolve: () => void;
      reject: (e: Error) => void;
      timer: ReturnType<typeof setTimeout>;
    }
  >();
  get connected() {
    return this.socket?.readyState === 1;
  }
  attach(socket: Socket) {
    this.close();
    this.socket = socket;
  }
  send(input: Input): Promise<void> {
    if (!this.connected) return Promise.reject(new Error("实时连接尚未就绪"));
    if (this.pending.size >= 32 || this.socket!.bufferedAmount > 64 * 1024)
      return Promise.reject(new Error("实时连接拥塞，正在重新同步"));
    if (this.pending.has(input.nonce))
      return Promise.reject(new Error("实时操作编号重复"));
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(input.nonce);
        reject(new Error("实时操作确认超时，请等待重新同步"));
      }, 5000);
      this.pending.set(input.nonce, { resolve, reject, timer });
      try {
        this.socket!.send(JSON.stringify({ type: "input", ...input }));
      } catch (error) {
        clearTimeout(timer);
        this.pending.delete(input.nonce);
        reject(error);
      }
    });
  }
  ack(ack: Ack) {
    const pending = this.pending.get(ack.nonce);
    if (!pending) return;
    clearTimeout(pending.timer);
    this.pending.delete(ack.nonce);
    if (ack.status === 200) pending.resolve();
    else pending.reject(new Error(ack.error || "实时操作未生效"));
  }
  close() {
    this.socket = undefined;
    for (const p of this.pending.values()) {
      clearTimeout(p.timer);
      p.reject(new Error("实时连接已断开，正在重新同步"));
    }
    this.pending.clear();
  }
}
