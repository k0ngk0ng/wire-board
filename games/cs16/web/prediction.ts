import type { GameEvent, GameInput, InputAction, MotionFunction, Player } from './types.ts';
import { emptyInput, hasInputEdge } from './input.ts';
import { DisplayCorrection, type SmoothTest } from './presentation.ts';

type MotionPlayer = Pick<Player, 'x' | 'y' | 'z' | 'vx' | 'vy' | 'vz' | 'yaw' | 'pitch' | 'alive' | 'life' | 'appliedSeq' | 'receivedSeq' | 'crouch'> & Partial<Player>;
type Pending = { frame: number; input: InputAction; edge: boolean };

export class MotionPrediction {
  private motion: MotionFunction;
  private player?: MotionPlayer;
  private frame = 0;
  private originFrame = 0;
  private originAt = 0;
  private identity = '';
  private pending: Pending[] = [];
  private applied: GameInput = emptyInput();
  private enabled = true;
  private immediate = 0;
  private facing = { seq: 0, yaw: 0, pitch: 0 };
  private correction: DisplayCorrection;
  predictedFrame = 0;

  constructor(motion: MotionFunction, canSmooth: SmoothTest = () => false) { this.motion = motion; this.correction = new DisplayCorrection(canSmooth); }
  get pendingCount() { return this.pending.length; }

  confirm(player: MotionPlayer, frame: number, at: number, identity: string, resetDisplay = false) {
    const reset = identity !== this.identity || player.life !== this.player?.life;
    if (!reset && frame < this.frame) return;
    const priorFrame = this.frame;
    const previous = !reset && !resetDisplay && this.player ? this.correction.sample(this.sampleAt(at, false), at) : undefined;
    if (reset || resetDisplay) this.correction.reset();
    if (reset) {
      this.pending = [];
      this.applied = emptyInput(player.yaw, player.pitch);
      this.originAt = at;
      this.originFrame = frame;
      this.predictedFrame = frame;
      this.immediate = frame;
      this.facing = { seq: player.appliedSeq, yaw: player.yaw, pitch: player.pitch };
      this.enabled = true;
    } else {
      for (const pending of this.pending) if (pending.input.seq <= player.appliedSeq) this.applied = { ...pending.input };
      this.pending = this.pending.filter(pending => pending.input.seq > player.appliedSeq);
      if (player.appliedSeq >= this.facing.seq) this.facing = { seq: player.appliedSeq, yaw: player.yaw, pitch: player.pitch };
      if (frame > this.target(at)) this.originFrame += frame - this.target(at);
      if (!this.pending.length) this.immediate = frame;
    }
    this.player = { ...player };
    this.frame = frame;
    this.identity = identity;
    if (!player.alive) { this.pending = []; this.applied = emptyInput(player.yaw, player.pitch); }
    if (previous && frame > priorFrame) this.correction.reconcile(previous, this.sampleAt(at, false), at);
  }

  input(input: InputAction, at: number) {
    if (!this.player?.alive || !this.enabled) return;
    const tail = this.pending.at(-1);
    const edge = hasInputEdge(tail?.input ?? this.applied, input, !!input.cancel);
    const merge = !edge && tail && !tail.edge && tail.frame > this.predictedFrame;
    const frame = merge ? tail.frame : Math.max(this.frame + 1, this.predictedFrame + 1, this.target(at) + 1, (tail?.frame ?? this.frame) + 1);
    const pending = { frame, input: { ...input }, edge };
    if (merge) this.pending[this.pending.length - 1] = pending;
    else this.pending.push(pending);
    if (this.pending.length > 64) this.pending = this.pending.slice(-64);
    this.facing = { seq: input.seq, yaw: input.yaw, pitch: input.pitch };
    this.immediate = frame;
  }

  sample(at: number): Player { return this.sampleAt(at, true); }
  present(player: Player, at: number): Player { return this.correction.sample(player, at); }

  private sampleAt(at: number, displayed: boolean): Player {
    if (!this.player) throw Error('预测状态尚未初始化');
    const target = this.enabled && this.player.alive ? Math.min(this.frame + 15, Math.max(this.frame, this.target(at), this.immediate)) : this.frame;
    let current = { ...this.player } as Player;
    let applied = this.applied;
    let frame = this.frame;
    for (const pending of this.pending) {
      if (pending.frame > target) break;
      const next = Math.max(frame, pending.frame - 1);
      if (next > frame) current = this.motion(current, applied, next - frame);
      frame = next;
      applied = pending.input;
    }
    if (target > frame) current = this.motion(current, applied, target - frame);
    current.yaw = this.facing.yaw;
    current.pitch = this.facing.pitch;
    if (displayed) this.predictedFrame = target;
    return current;
  }

  face(yaw: number, pitch: number) { this.facing = { ...this.facing, yaw, pitch }; }
  resume(at: number) { this.enabled = true; this.originFrame = this.frame; this.originAt = at; this.immediate = this.frame; }
  suspend() { this.enabled = false; this.pending = []; this.applied = emptyInput(this.facing.yaw, this.facing.pitch); this.immediate = this.frame; this.correction.reset(); }
  private target(at: number) { return this.originFrame + Math.max(0, Math.floor((at - this.originAt) / 10)); }
}

export class EventCursor {
  private epoch = '';
  private last = 0;
  consume<T extends Pick<GameEvent, 'id'>>(epoch: string, latest: number, events: T[]): T[] {
    if (epoch !== this.epoch) { this.epoch = epoch; this.last = latest; return []; }
    const result = events.filter(event => event.id > this.last).sort((a, b) => a.id - b.id);
    this.last = Math.max(this.last, latest, ...result.map(event => event.id));
    return result;
  }
}
