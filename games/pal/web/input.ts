import type { GameController } from "../../../web/src/controllers/game-controller";
import type { ControllerSink } from "../../../web/src/controllers/pointer-controller";
import type { BoardProps } from "../../../web/src/game-api/module";
import type { PalView } from "./view";
import type { LocalInputRequest } from "./local-types";

type Action = Record<string, unknown> & { epoch: number };
const keys: Record<string, number> = {
  up: 16,
  down: 4,
  left: 8,
  right: 32,
  confirm: 2,
  cancel: 1,
  status: 16384,
  defend: 1024,
  item: 2048,
  throw: 4096,
  repeat: 256,
  force: 32768,
  flee: 8192,
};
const directions: Record<string, number> = {
  up: 2,
  down: 0,
  left: 1,
  right: 3,
};
export function palInputAllowed(props: BoardProps<PalView>) {
  const { room, data } = props,
    seat = room.you;
  if (
    props.busy ||
    room.loading ||
    room.status !== "playing" ||
    room.game?.paused ||
    room.spectating ||
    seat < 0 ||
    room.seats[seat]?.autoPlay ||
    room.seats[seat]?.left
  )
    return false;
  if (data.phase === "dialogue")
    return !!data.dialogue && !data.dialogue.read[seat];
  if (data.phase === "cooperation")
    return !!data.cooperation?.required.includes(seat);
  if (
    data.phase === "field" ||
    data.phase === "menu" ||
    data.phase === "ending" ||
    data.phase === "system"
  )
    return data.leader === seat;
  const b = data.battle;
  return (
    data.phase === "battle" &&
    !!b &&
    b.phase === 0 &&
    b.state !== 0 &&
    data.party[b.current]?.controller === seat
  );
}

export class PalInput implements ControllerSink {
  controller?: GameController;
  private epoch = 0;
  private sequence = 0;
  private queue: Action[] = [];
  private inFlight = 0;
  private pointers = new Map<number, readonly string[]>();
  private blocked = new Set<number>();
  private previous = new Set<string>();
  private direction = 4;
  private directionAt = 0;
  private sentAt = 0;
  private repeatAt = 0;
  private cancelVersion = 0;
  private decided = new Set<string>();
  private disposed = false;
  private controlAt = 0;
  private clockSeenAt = 0;
  private discardedSeq = 0;
  private localDirty = false;
  private previews = new Map<number, number>();
  constructor(
    private current: () => BoardProps<PalView>,
    private enabled: () => boolean,
    private error: (message: string) => void,
    private local?: (input: LocalInputRequest, at: number) => void,
    private stopLocal?: () => void,
    private active?: () => boolean,
  ) {}
  update(at = performance.now()) {
    const data = this.current().data;
    const epoch = data.local?.generation ?? data.epoch;
    if (this.epoch !== epoch) {
      this.epoch = epoch;
      this.sequence = data.local?.seq ?? data.seq;
      this.queue = [];
      this.previous.clear();
      this.direction = 4;
      this.directionAt = 0;
      this.sentAt = 0;
      this.repeatAt = 0;
      this.decided.clear();
      this.previews.clear();
      this.controlAt = data.controlAt;
      this.clockSeenAt = at;
      this.discardedSeq = data.discardedSeq ?? 0;
      this.clear(at);
    } else this.sequence = Math.max(this.sequence, data.local?.seq ?? data.seq);
    if (this.controlAt !== data.controlAt) {
      this.controlAt = data.controlAt;
      this.clockSeenAt = at;
    }
    if ((data.discardedSeq ?? 0) > this.discardedSeq) {
      this.discardedSeq = data.discardedSeq!;
      this.clear(at);
    }
  }
  private releaseAllowed() {
    const p = this.current(),
      seat = p.room.you;
    if (
      p.room.loading ||
      p.room.status !== "playing" ||
      p.room.game?.paused ||
      p.room.spectating ||
      seat < 0 ||
      p.room.seats[seat]?.left
    )
      return false;
    if (p.data.local) return p.data.leader === seat;
    return (
      ((p.data.phase === "field" ||
        p.data.phase === "menu" ||
        p.data.phase === "ending") &&
        p.data.leader === seat) ||
      (p.data.phase === "battle" &&
        !!p.data.battle &&
        p.data.battle.phase === 0 &&
        p.data.battle.state !== 0 &&
        p.data.party[p.data.battle.current]?.controller === seat)
    );
  }
  private fresh(at: number) {
    return (
      this.controlAt > 0 &&
      at - this.clockSeenAt <= (this.current().data.local ? 1000 : 500)
    );
  }
  clear(at = performance.now(), interrupted = false) {
    const release = this.direction !== 4;
    const localRelease =
      interrupted && this.localDirty && !!this.current().data.local;
    if (interrupted) this.localDirty = false;
    this.direction = 4;
    this.queue = this.queue.filter(
      (a) =>
        (a.type !== "input" && a.type !== "local_input") ||
        (a.direction === 4 && !a.keys),
    );
    this.stopLocal?.();
    for (const id of this.pointers.keys()) this.blocked.add(id);
    this.previous.clear();
    this.controller?.clear();
    this.cancelVersion = this.controller?.cancelVersion ?? 0;
    if ((release || localRelease) && !this.disposed && this.releaseAllowed())
      this.input(4, 0, at, localRelease);
  }
  pressPointer(id: number, buttons: readonly string[]) {
    this.pointers.set(id, buttons);
    this.sample(performance.now());
  }
  preview(button: string) {
    const at = performance.now();
    this.update(at);
    if (
      !this.current().data.local ||
      !this.enabled() ||
      !this.fresh(at) ||
      !keys[button]
    )
      return undefined;
    const seq = ++this.sequence;
    this.previews.set(seq, this.epoch);
    this.localDirty = true;
    this.local?.(
      {
        epoch: this.epoch,
        seq,
        direction: 4,
        keys: keys[button],
        preview: true,
      },
      at,
    );
    return seq;
  }
  previewValid(seq: number) {
    return this.previews.get(seq) === this.epoch && !!this.active?.();
  }
  commitPreview(seq: number) {
    if (!this.previewValid(seq)) {
      this.cancelPreview(seq);
      return false;
    }
    this.previews.delete(seq);
    this.local?.(
      { epoch: this.epoch, seq, direction: 4, keys: 0, commitPreview: true },
      performance.now(),
    );
    return true;
  }
  cancelPreview(seq: number) {
    if (!this.previews.delete(seq)) return;
    this.local?.(
      { epoch: this.epoch, seq, direction: 4, keys: 0, cancelPreview: true },
      performance.now(),
    );
  }
  holdPreview(id: number, button: string) {
    const at = performance.now();
    this.direction = directions[button];
    this.directionAt = at - 100;
    this.input(this.direction, 0, at);
    this.pressPointer(id, [button]);
  }
  releasePointer(id: number, _cancelled: boolean) {
    this.pointers.delete(id);
    this.blocked.delete(id);
    this.sample(performance.now());
  }
  pulse(button: string) {
    const at = performance.now();
    this.update(at);
    if (this.enabled() && this.fresh(at)) this.edge(new Set([button]), at);
  }
  decide(
    type: "coop_accept" | "coop_cancel" | "system_close",
    target?: number,
  ) {
    if (!this.enabled()) return;
    if (this.current().data.local && type === "system_close") {
      this.input(4, 1, performance.now());
      return;
    }
    const data = this.current().data,
      prompt = data.cooperation?.id;
    this.enqueue({
      type,
      epoch: data.epoch,
      ...(type.startsWith("coop_") ? { prompt } : {}),
      ...(target === undefined ? {} : { target }),
    });
  }
  handover(target: number) {
    if (
      palInputAllowed(this.current()) &&
      this.current().data.phase === "field"
    )
      this.enqueue({ type: "leader", epoch: this.epoch, target });
  }
  private enqueue(action: Action) {
    if (this.disposed || action.epoch !== this.epoch) return false;
    const previous = this.queue.at(-1);
    if (
      action.type === "input" &&
      !action.keys &&
      previous?.type === "input" &&
      !previous.keys
    )
      this.queue[this.queue.length - 1] = action;
    else if (this.queue.length < 64) this.queue.push(action);
    else {
      this.error("操作过快，请稍候再试。");
      return false;
    }
    this.drain();
    return true;
  }
  private drain() {
    const window = Math.max(1, Math.min(8, this.current().inputWindow ?? 1));
    while (!this.disposed && this.inFlight < window && this.queue.length) {
      const action = this.queue.shift()!;
      if (
        action.epoch !==
        (this.current().data.local?.generation ?? this.current().data.epoch)
      )
        continue;
      this.inFlight++;
      void this.current()
        .act(action)
        .catch((error) => {
          if (
            !this.disposed &&
            action.epoch ===
              (this.current().data.local?.generation ??
                this.current().data.epoch)
          ) {
            this.decided.clear();
            if (error && typeof error === "object" && error.transientSync) {
              this.clear();
              return;
            }
            this.error(
              error instanceof Error ? error.message : "操作未送达，请重试。",
            );
          }
        })
        .finally(() => {
          this.inFlight--;
          this.drain();
        });
    }
  }
  private input(
    direction: number,
    mask: number,
    at: number,
    interrupt = false,
  ) {
    this.sentAt = at;
    const action = {
      type: "input",
      epoch: this.epoch,
      seq: ++this.sequence,
      direction,
      keys: mask,
      controlAt: this.current().data.controlAt,
      ...(interrupt ? { interrupt: true } : {}),
    };
    if (this.current().data.local) {
      if (mask || direction !== 4) this.localDirty = true;
      this.local?.(action, at);
    } else if (this.enqueue(action)) this.local?.(action, at);
  }
  commitLocal(action: Action) {
    return this.enqueue(action);
  }
  private edge(buttons: Set<string>, at: number) {
    const { data, room } = this.current();
    if (data.local && data.phase === "dialogue") {
      if (buttons.has("confirm")) this.input(4, 2, at);
      return;
    }
    if (data.phase === "dialogue") {
      const d = data.dialogue,
        key = `${this.epoch}:read:${d?.id}`;
      if (
        buttons.has("confirm") &&
        d &&
        !d.read[room.you] &&
        !this.decided.has(key)
      ) {
        this.decided.add(key);
        this.enqueue({ type: "read", epoch: this.epoch, prompt: d.id });
      }
      return;
    }
    if (data.phase === "cooperation" || data.phase === "system") return;
    let mask = 0;
    for (const b of buttons) mask |= keys[b] ?? 0;
    if (mask) {
      const held =
        data.phase === "field" && at - this.directionAt >= 100
          ? this.direction
          : 4;
      this.input(held, mask, at);
    }
  }
  sample(at: number) {
    if (this.disposed) return;
    this.update(at);
    const physical = this.controller?.sample(at) ?? new Set<string>();
    if ((this.controller?.cancelVersion ?? 0) !== this.cancelVersion) {
      this.cancelVersion = this.controller?.cancelVersion ?? 0;
      for (const id of this.pointers.keys()) this.blocked.add(id);
      this.previous.clear();
    }
    if (!this.enabled() || !this.fresh(at)) {
      this.clear(at, !this.active?.() || !this.fresh(at));
      return;
    }
    const buttons = new Set(physical);
    for (const [id, held] of this.pointers)
      if (!this.blocked.has(id)) for (const b of held) buttons.add(b);
    const rising = new Set(
      [...buttons].filter((b) => !(b in directions) && !this.previous.has(b)),
    );
    let name = "";
    for (const b of buttons)
      if (
        b in directions &&
        !buttons.has(
          b === "left"
            ? "right"
            : b === "right"
              ? "left"
              : b === "up"
                ? "down"
                : "up",
        )
      )
        name = b;
    const dir = name ? directions[name] : 4;
    const previousDirection = this.direction;
    const phase = this.current().data.phase;
    if (phase === "field" || phase === "menu" || phase === "battle") {
      if (dir !== this.direction) {
        this.direction = dir;
        this.directionAt = at;
        this.repeatAt = at + 300;
        if (name) rising.add(name);
      } else if (dir !== 4 && phase !== "field" && at >= this.repeatAt) {
        rising.add(name);
        this.repeatAt = at + 140;
      }
      if (rising.size) this.edge(rising, at);
      else if (
        phase === "field" &&
        dir !== 4 &&
        (at - this.sentAt >= 100 ||
          (this.sentAt === this.directionAt && at - this.directionAt >= 80))
      )
        this.input(dir, 0, at);
      else if (dir === 4 && previousDirection !== 4) this.input(4, 0, at);
    } else this.edge(rising, at);
    this.previous = buttons;
  }
  dispose() {
    this.interrupt();
    this.disposed = true;
    this.pointers.clear();
    this.blocked.clear();
  }
  interrupt() {
    for (const sequence of this.previews.keys()) this.cancelPreview(sequence);
    this.queue = [];
    this.direction = 4;
    this.decided.clear();
    this.clear(performance.now(), true);
  }
}
