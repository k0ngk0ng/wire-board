import {
  NativeRuntime,
  decodeCheckpoint,
  type NativeInput,
} from "./native-runtime";
import { nativeFrame, nativePhase } from "./native-frame";
import type { LocalMeta, LocalAction } from "./local-types";

type PendingInput = NativeInput & { seq: number; at: number; boundary: string };

export class LocalSimulation {
  private authority!: NativeRuntime;
  private predicted!: NativeRuntime;
  private pending: PendingInput[] = [];
  private queued: PendingInput[] = [];
  private remaining = 0;
  private at = 0;
  private confirmedAt = 0;
  private direction = 4;
  private until = 0;
  private meta: LocalMeta;
  suspended = false;
  private constructor(meta: LocalMeta) {
    this.meta = meta;
  }
  static async create(
    compiled: WebAssembly.Module,
    encoded: Uint8Array,
    meta: LocalMeta,
    now: number,
  ) {
    const simulation = new LocalSimulation(meta),
      checkpoint = decodeCheckpoint(encoded);
    if (
      checkpoint.snapshot.buildID !== meta.core ||
      (meta.digest && checkpoint.snapshot.memoryID !== meta.digest)
    )
      throw Error("仙剑本地核心或检查点不一致");
    const [authority, predicted] = await Promise.all([
      NativeRuntime.create(compiled, checkpoint),
      NativeRuntime.create(compiled, checkpoint),
    ]);
    if ((await authority.digest()) !== checkpoint.snapshot.memoryID)
      throw Error("仙剑本地检查点校验失败");
    simulation.authority = authority;
    simulation.predicted = predicted;
    simulation.confirmedAt = now;
    simulation.rebase(now);
    return simulation;
  }
  get pendingCount() {
    return this.pending.length;
  }
  frame() {
    return nativeFrame(this.predicted, this.meta);
  }
  previewMenus() {
    return this.predicted.previewMenus();
  }
  private reset() {
    this.predicted.copyFrom(this.authority);
    this.remaining = this.meta.remainingMS;
    this.at = this.meta.serverAt;
    this.direction = this.meta.direction ?? 4;
    this.until = this.meta.until ?? 0;
    this.queued = [...this.pending];
  }
  stop() {
    this.pending = [];
    this.queued = [];
    this.suspended = true;
    this.reset();
  }
  cancel(sequence: number, now: number) {
    this.pending = this.pending.filter((input) => input.seq !== sequence);
    this.rebase(now);
  }
  private observeBoundary(before: ReturnType<LocalSimulation["frame"]>) {
    const snapshot = this.predicted.snapshot,
      next = nativePhase(snapshot),
      v = snapshot.view;
    const boundary =
      before.phase !== next ||
      before.menu.kind !== v.menu.kind ||
      (before.battle &&
        v.battle.active &&
        (before.battle.current !== v.battle.current ||
          before.battle.state !== v.battle.state ||
          before.battle.menu !== v.battle.menu ||
          before.battle.phase !== v.battle.phase));
    if (boundary) this.predicted.release();
    if (snapshot.event.kind === 6) {
      const waiting = this.frame();
      this.predicted.step(1);
      this.observeBoundary(waiting);
    }
  }
  private blockedInput() {
    for (let i = 0; i < 64 && this.queued.length; i++) {
      const command = this.queued[0],
        phase = nativePhase(this.predicted.snapshot);
      if (
        command.at > this.at ||
        command.boundary !== this.predicted.inputBoundary() ||
        (phase !== "dialogue" && phase !== "system") ||
        !(command.keys & 3)
      )
        return;
      this.queued.shift();
      this.direction = 4;
      this.until = 0;
      const before = this.frame();
      this.predicted.release();
      this.predicted.step(1);
      this.observeBoundary(before);
      this.remaining = [1, 4].includes(this.predicted.snapshot.event.kind)
        ? this.predicted.snapshot.event.value
        : 0;
      this.finishReadPoll();
    }
  }
  private finishReadPoll() {
    const snapshot = this.predicted.snapshot;
    if (
      snapshot.event.kind !== 1 ||
      snapshot.event.value !== 100 ||
      !snapshot.view.dialogWaiting
    )
      return;
    const before = this.frame();
    const advanced = this.predicted.advance(0, 0, () => ({
      direction: 4,
      keys: 0,
    }));
    this.remaining = advanced.remainingMS;
    this.observeBoundary(before);
  }
  private inputAt(at: number) {
    const command = this.queued[0];
    if (
      command &&
      command.at <= at &&
      (command.boundary === this.predicted.inputBoundary() ||
        (command.direction === 4 && command.keys === 0))
    ) {
      this.queued.shift();
      this.direction = command.direction;
      this.until = at + 300;
      return command;
    }
    return { direction: this.until >= at ? this.direction : 4, keys: 0 };
  }
  private advance(elapsed: number, immediate = false) {
    const before = this.frame(),
      start = this.at;
    const advanced = this.predicted.advance(
      immediate ? 0 : this.remaining,
      elapsed,
      (offset) => this.inputAt(start + offset),
    );
    this.remaining = advanced.remainingMS;
    if (advanced.steps) this.observeBoundary(before);
    if (advanced.inputReset) {
      this.direction = 4;
      this.until = 0;
    }
    this.at += elapsed;
    this.blockedInput();
  }
  private flushImmediate() {
    this.blockedInput();
    const command = this.queued[0];
    if (
      !command ||
      command.at > this.at ||
      command.boundary !== this.predicted.inputBoundary()
    )
      return false;
    const phase = nativePhase(this.predicted.snapshot);
    if (command.keys === 0) {
      this.inputAt(this.at);
      return true;
    }
    if (
      this.predicted.snapshot.event.kind === 4 ||
      (this.predicted.snapshot.event.kind === 1 && phase === "menu")
    ) {
      this.advance(0, true);
      return true;
    }
    return false;
  }
  private advanceTo(target: number) {
    for (let steps = 0; steps < 8192; steps++) {
      if (this.flushImmediate()) continue;
      if (this.at >= target) return;
      const command = this.queued[0];
      const next =
        command && command.at > this.at ? Math.min(target, command.at) : target;
      this.advance(Math.min(250, next - this.at));
    }
    throw Error("仙剑本地重放超过步数限制");
  }
  private rebase(now: number) {
    this.reset();
    this.finishReadPoll();
    this.blockedInput();
    const target = Math.min(now, this.meta.serverAt + 1000);
    this.advanceTo(target);
  }
  input(
    input: { seq: number; direction: number; keys: number },
    now: number,
  ): LocalAction | undefined {
    if (
      this.suspended ||
      this.pending.length >= 64 ||
      now - this.confirmedAt > 1000
    ) {
      this.stop();
      return undefined;
    }
    this.tick(now);
    if (this.suspended) return undefined;
    const boundary = this.predicted.inputBoundary(),
      command = { ...input, at: now, boundary };
    this.pending.push(command);
    this.queued.push(command);
    this.flushImmediate();
    return {
      type: "local_input",
      epoch: this.meta.generation,
      seq: input.seq,
      direction: input.direction,
      keys: input.keys,
      boundary,
      controlAt: this.meta.serverAt,
    };
  }
  tick(now: number) {
    if (this.suspended) return false;
    if (
      now - this.confirmedAt > 1000 ||
      (this.pending[0] && now - this.pending[0].at > 1000)
    ) {
      this.stop();
      return true;
    }
    const clock = this.predicted.snapshot.view.clock,
      boundary = this.predicted.inputBoundary();
    const target = Math.min(now, this.meta.serverAt + 1000);
    this.advanceTo(target);
    return (
      clock !== this.predicted.snapshot.view.clock ||
      boundary !== this.predicted.inputBoundary()
    );
  }
  async confirm(meta: LocalMeta, now: number) {
    if (
      meta.generation !== this.meta.generation ||
      meta.core !== this.meta.core ||
      meta.revision < this.meta.revision
    ) {
      this.stop();
      throw Error("仙剑操作代次已变化");
    }
    const changed = meta.revision !== this.meta.revision;
    try {
      let revision = this.meta.revision;
      for (const record of meta.records ?? []) {
        if (record.revision <= revision) continue;
        if (record.revision !== revision + 1)
          throw Error("仙剑规则同步缺少步骤");
        for (const operation of record.operations)
          this.authority.replay(operation);
        revision = record.revision;
      }
      if (revision !== meta.revision) throw Error("仙剑规则同步需要恢复");
      if (changed && (await this.authority.digest()) !== meta.digest)
        throw Error("仙剑本地规则校验不一致");
      const discarded = meta.discardedSeq > this.meta.discardedSeq;
      this.meta = { ...meta, checkpoint: undefined, records: undefined };
      this.confirmedAt = now;
      this.pending = discarded
        ? []
        : this.pending.filter((input) => input.seq > meta.appliedSeq);
      if (changed || discarded) this.rebase(now);
      return changed || discarded;
    } catch (error) {
      this.stop();
      throw error;
    }
  }
}
