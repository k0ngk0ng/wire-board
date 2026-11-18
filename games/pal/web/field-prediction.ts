import { fieldCell, stepField, type FieldState } from "./field-rules";
import type { PalView } from "./view";

export type LocalFieldInput = {
  epoch: number;
  seq: number;
  direction: number;
  keys: number;
};
type PredictedStep = {
  seq: number;
  direction: number;
  at: number;
  target: number;
  once: boolean;
};
const frameMS = 100,
  horizonMS = 300,
  maxSteps = 3,
  correctionMS = 80;

// Only the original field motor is predicted. Native scripts, NPC decisions,
// role permissions and resources remain authoritative. Taps retire on applied
// sequence, never received ACK. Continued holds also retain their future frames.
export class FieldPrediction {
  private base?: PalView;
  private identity = "";
  private authorityAt = 0;
  private sequence = 0;
  private held = 4;
  private heldSeq = 0;
  private idleSeq = 0;
  private nextAt = 0;
  private lastDirectionAt = -Infinity;
  private records: PredictedStep[] = [];
  private pulses: { seq: number; direction: number; at: number }[] = [];
  private motor?: FieldState;
  private shown?: FieldState;
  private correction?: { x: number; y: number; at: number };
  get current() {
    return this.shown;
  }
  get pending() {
    return this.records.length + this.pulses.length;
  }

  clear(): boolean {
    const changed =
      !!this.shown || !!this.records.length || !!this.pulses.length;
    this.held = 4;
    this.idleSeq = 0;
    this.nextAt = 0;
    this.lastDirectionAt = -Infinity;
    this.records = [];
    this.pulses = [];
    this.motor = undefined;
    this.shown = undefined;
    this.correction = undefined;
    return changed;
  }

  update(view: PalView, seat: number, at: number): boolean {
    const key = `${view.epoch}:${view.scene}:${view.leader}:${seat}`;
    const eligible =
      view.phase === "field" && view.leader === seat && !!view.field;
    if (!eligible || key !== this.identity || !this.base?.field) {
      const changed = this.clear();
      this.identity = key;
      this.base = eligible ? view : undefined;
      this.authorityAt = at;
      this.sequence = view.seq;
      return changed;
    }
    const previous = this.base.field,
      next = view.field!;
    if (next.frame === previous.frame) {
      this.base = view;
      // Same-frame ACK/lease packets do not renew the prediction horizon or
      // move the original 100ms phase. Their field pose has not advanced.
      return false;
    }
    const distance = (next.frame - previous.frame) >>> 0;
    if (distance > 4 || at - this.authorityAt > horizonMS) {
      const changed = this.clear();
      this.base = view;
      this.authorityAt = at;
      return changed;
    }
    const old = this.shown?.camera;
    this.base = view;
    this.authorityAt = at;
    this.records = this.records.filter((record) => at - record.at <= horizonMS);
    this.pulses = this.pulses.filter((pulse) => at - pulse.at <= horizonMS);
    const kept: PredictedStep[] = [];
    let target = next.frame;
    for (const record of this.records) {
      if (
        record.seq <= view.appliedSeq &&
        (record.once || record.target <= next.frame)
      )
        continue;
      record.target = Math.max(record.target, target + 1);
      kept.push(record);
      target = record.target;
    }
    this.records = kept;
    if (this.idleSeq <= view.appliedSeq) this.idleSeq = 0;
    this.rebuild();
    const desired = this.motor ?? next;
    if (old && (old[0] !== desired.camera[0] || old[1] !== desired.camera[1])) {
      const x = old[0] - desired.camera[0],
        y = old[1] - desired.camera[1];
      this.correction =
        Math.abs(x) <= 64 && Math.abs(y) <= 32 ? { x, y, at } : undefined;
    }
    this.render(at);
    return true;
  }

  input(input: LocalFieldInput, at: number): boolean {
    if (
      !this.base?.field ||
      input.epoch !== this.base.epoch ||
      input.seq <= this.sequence
    )
      return false;
    this.sequence = input.seq;
    if (input.keys & ~(4 | 8 | 16 | 32)) return this.clear();
    this.held = input.direction;
    this.heldSeq = input.seq;
    if (this.held === 4 && this.records.length) this.idleSeq = input.seq;
    else if (this.held !== 4) this.idleSeq = 0;
    let keys = input.keys;
    if ((keys & 20) === 20) keys &= ~20;
    if ((keys & 40) === 40) keys &= ~40;
    const direction =
      keys & 16 ? 2 : keys & 4 ? 0 : keys & 8 ? 1 : keys & 32 ? 3 : 4;
    if (direction !== 4 && this.pulses.length < 32) {
      // Standing gestures do not defer a fresh press once an entire original
      // movement interval has passed. Rapid movement edges still keep cadence.
      if (!this.pulses.length && at - this.lastDirectionAt >= frameMS)
        this.nextAt = Math.min(this.nextAt, at);
      this.pulses.push({ seq: input.seq, direction, at });
    }
    return this.sample(at);
  }

  sample(at: number): boolean {
    if (!this.base?.field) return false;
    if (at - this.authorityAt > horizonMS) return this.clear();
    let changed = false;
    while (
      this.records.length < maxSteps &&
      (this.pulses.length || this.held !== 4 || this.idleSeq) &&
      at >= this.nextAt
    ) {
      const pulse = this.pulses.shift();
      const seq = pulse?.seq ?? (this.held !== 4 ? this.heldSeq : this.idleSeq);
      const direction = pulse?.direction ?? this.held;
      if (pulse && at - pulse.at > horizonMS) continue;
      const state = this.motor ?? this.base.field;
      const due = pulse
        ? Math.max(this.nextAt || at, pulse.at)
        : this.nextAt || at;
      this.nextAt = due + frameMS;
      if (
        fieldCell(
          state,
          state.camera[0] + state.offset[0],
          state.camera[1] + state.offset[1],
        ) & 2
      )
        break;
      const target = (this.records.at(-1)?.target ?? this.base.field.frame) + 1;
      this.records.push({
        seq,
        direction,
        at,
        target,
        once: !!pulse || direction === 4,
      });
      this.motor = stepField(state, direction);
      if (direction !== 4) this.lastDirectionAt = due;
      if (direction === 4) this.idleSeq = 0;
      changed = true;
    }
    return this.render(at) || changed;
  }

  private rebuild() {
    let state = this.base!.field!;
    let count = 0;
    for (const record of this.records) {
      if (
        fieldCell(
          state,
          state.camera[0] + state.offset[0],
          state.camera[1] + state.offset[1],
        ) & 2
      )
        break;
      state = stepField(state, record.direction);
      count++;
    }
    this.records.length = count;
    this.motor = count ? state : undefined;
  }

  private render(at: number): boolean {
    const previous = this.shown;
    let shown = this.motor;
    if (this.correction) {
      const remaining = Math.max(
        0,
        1 - (at - this.correction.at) / correctionMS,
      );
      const dx = Math.round(this.correction.x * remaining),
        dy = Math.round(this.correction.y * remaining);
      if (!remaining || (!dx && !dy)) this.correction = undefined;
      else {
        const source = shown ?? this.base!.field!;
        shown = {
          ...source,
          camera: [source.camera[0] + dx, source.camera[1] + dy],
          actors: source.actors.map((actor) =>
            actor[0] === 1
              ? actor
              : [
                  actor[0],
                  actor[1],
                  actor[2],
                  actor[3] + dx,
                  actor[4] + dy,
                  actor[5],
                  actor[6],
                  actor[7],
                ],
          ),
        };
      }
    }
    this.shown = shown;
    return (
      previous !== shown &&
      (!previous ||
        !shown ||
        previous.camera[0] !== shown.camera[0] ||
        previous.camera[1] !== shown.camera[1] ||
        previous.direction !== shown.direction ||
        previous.step !== shown.step)
    );
  }
}
