import type { RuntimeBridge, RuntimeSnapshot } from "./runtime-bridge";

const frameMS = 1000 / 60;
const maxLead = 12n;
const staleMS = 250;

/** Separate native fork for responsive drawing. Nothing from this fork enters
 * the authoritative digest, public resources, sound stream or settlement. */
export class RuntimePrediction {
  private authority?: RuntimeSnapshot;
  private predicted?: RuntimeSnapshot;
  private target = 0;
  private epoch = "";
  private pads: [number, number] = [0, 0];
  private samples = new Map<string, number>();
  private actor = -1;
  private confirmedAt = 0;
  private tickAt?: number;
  private dirty = true;
  private until = 0n;
  private held = 0;
  private pressed = 0;

  private bridge: RuntimeBridge;
  constructor(bridge: RuntimeBridge) {
    this.bridge = bridge;
  }

  get frame() {
    return this.predicted?.frame;
  }

  confirm(state: RuntimeSnapshot, pads: [number, number], now: number) {
    if (this.epoch && this.epoch !== state.epoch)
      throw Error("预测状态的对局身份 epoch 已改变");
    if (
      this.authority &&
      BigInt(state.revision) < BigInt(this.authority.revision)
    )
      return;
    this.epoch = state.epoch;
    this.until = BigInt(this.predicted?.frame ?? state.frame);
    this.authority = state;
    this.pads = [...pads];
    this.confirmedAt = now;
    this.tickAt ??= now;
    this.dirty = true;
    for (const frame of this.samples.keys())
      if (BigInt(frame) <= BigInt(state.frame)) this.samples.delete(frame);
    if (state.phase !== "playing" || state.paused || state.finalized)
      this.cancel();
  }

  private safe(state: RuntimeSnapshot) {
    const a = this.authority!;
    return (
      state.phase === "playing" &&
      state.level === a.level &&
      !state.paused &&
      !state.finalized &&
      !state.outcome &&
      state.players.every(
        (p, i) =>
          p.lives === a.players[i].lives &&
          p.gameOver === a.players[i].gameOver &&
          p.state === a.players[i].state,
      )
    );
  }

  private advance(mask: number) {
    const p = this.predicted!;
    // Pause/menu/life borrowing are confirmed by the server, never predicted.
    const pads: [number, number] = [this.pads[0] & ~48, this.pads[1] & ~48];
    pads[this.actor] = mask & ~48;
    const next = this.bridge.call<RuntimeSnapshot>({
      op: "pads",
      session: p.session,
      epoch: p.epoch,
      revision: String(BigInt(p.revision) + 1n),
      frame: String(BigInt(p.frame) + 1n),
      pads,
    });
    if (!this.safe(next)) {
      this.predicted = undefined;
      this.dirty = true;
      return false;
    }
    this.predicted = next;
    return true;
  }

  sample(
    now: number,
    actor: number,
    mask: number,
  ): RuntimeSnapshot | undefined {
    const a = this.authority;
    if (
      !a ||
      actor < 0 ||
      actor > 1 ||
      a.phase !== "playing" ||
      a.paused ||
      a.finalized ||
      a.players[actor]?.state !== 1 ||
      a.players[actor]?.gameOver ||
      now - this.confirmedAt > staleMS ||
      (this.tickAt !== undefined && now - this.tickAt > staleMS)
    ) {
      this.cancel();
      return;
    }
    if (actor !== this.actor) {
      this.samples.clear();
      this.held = this.pressed = 0;
      this.dirty = true;
      this.actor = actor;
    }
    this.pressed |= mask & ~this.held & ~48;
    this.held = mask;
    if (this.dirty) {
      const fork = this.bridge.call<RuntimeSnapshot>({
        op: "fork",
        session: a.session,
        epoch: a.epoch,
        ...(this.target ? { target: this.target } : {}),
      });
      this.target = fork.session;
      this.predicted = fork;
      this.dirty = false;
      const end =
        this.until < BigInt(a.frame) + maxLead
          ? this.until
          : BigInt(a.frame) + maxLead;
      while (BigInt(this.predicted!.frame) < end) {
        const next = String(BigInt(this.predicted!.frame) + 1n);
        if (!this.advance(this.samples.get(next) ?? mask)) return;
      }
    }
    this.tickAt ??= now - frameMS;
    while (now + 0.05 >= this.tickAt + frameMS) {
      this.tickAt += frameMS;
      if (BigInt(this.predicted!.frame) >= BigInt(a.frame) + maxLead) break;
      const applied = mask | this.pressed;
      if (!this.advance(applied)) return;
      this.pressed = 0;
      this.samples.set(this.predicted!.frame, applied);
    }
    return this.predicted;
  }

  cancel() {
    this.samples.clear();
    this.predicted = undefined;
    this.until = 0n;
    this.held = this.pressed = 0;
    this.tickAt = undefined;
    this.dirty = true;
  }

  dispose() {
    this.cancel();
    if (this.target)
      this.bridge.call({
        op: "dispose",
        session: this.target,
        epoch: this.epoch,
      });
    this.target = 0;
    this.authority = undefined;
  }
}
