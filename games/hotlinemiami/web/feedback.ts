export type FeedbackKind = "throw" | "pickup" | "finish" | "interact";
export type FeedbackPulse = { seq: number; life: number; kind: FeedbackKind; at: number; until: number };

export class ActionFeedback {
  private pulses: FeedbackPulse[] = [];
  private life = 0;
  private sequence = 0;

  push(action: { seq: number; life: number; throw?: boolean; pickup?: boolean; finish?: boolean; interact?: boolean }, at: number) {
    if (action.life !== this.life) {
      this.clear();
      this.life = action.life;
      this.sequence = 0;
    }
    if (action.seq <= this.sequence) return;
    this.sequence = action.seq;
    for (const kind of ["throw", "pickup", "finish", "interact"] as const) {
      if (!action[kind]) continue;
      this.pulses = this.pulses.filter(pulse => pulse.kind !== kind);
      this.pulses.push({ seq: action.seq, life: action.life, kind, at: Math.trunc(at), until: Math.trunc(at) + 150 });
    }
  }

  sample(at: number, life: number, alive: boolean) {
    if (!alive || life !== this.life) this.clear();
    this.pulses = this.pulses.filter(pulse => pulse.until > Math.trunc(at));
    return this.pulses;
  }

  clear() {
    this.pulses = [];
  }
}
