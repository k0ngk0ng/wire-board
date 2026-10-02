export type Sound =
  "preview" | "take" | "buy" | "reserve" | "turn" | "finish" | "move";

// Local synthesis keeps the game independent of external asset/CDN requests.
// AudioContext is created/resumed during a user gesture, then reused for events.
export class GameAudio {
  private context?: AudioContext;

  async unlock(): Promise<boolean> {
    try {
      this.context ??= new AudioContext();
      if (this.context.state === "suspended") await this.context.resume();
      return this.context.state === "running";
    } catch {
      return false;
    }
  }

  async play(sound: Sound): Promise<boolean> {
    if (!(await this.unlock())) return false;
    const ctx = this.context!;
    const notes: Record<Sound, number[]> = {
      preview: [523.25, 659.25, 783.99],
      take: [880, 1174.66, 1318.51],
      buy: [523.25, 659.25, 783.99, 1046.5],
      reserve: [440, 659.25],
      turn: [659.25, 880, 880],
      finish: [523.25, 659.25, 783.99, 1046.5, 1318.51],
      move: [587.33, 783.99],
    };
    const step = sound === "turn" || sound === "finish" ? 0.18 : 0.09;
    notes[sound].forEach((frequency, i) => {
      const at = ctx.currentTime + 0.02 + i * step;
      const duration = sound === "finish" ? 0.36 : 0.22;
      const oscillator = ctx.createOscillator();
      const envelope = ctx.createGain();
      oscillator.type = "triangle";
      oscillator.frequency.setValueAtTime(frequency, at);
      envelope.gain.setValueAtTime(0, at);
      envelope.gain.linearRampToValueAtTime(0.18, at + 0.008);
      envelope.gain.exponentialRampToValueAtTime(0.001, at + duration);
      oscillator.connect(envelope);
      envelope.connect(ctx.destination);
      oscillator.onended = () => {
        oscillator.disconnect();
        envelope.disconnect();
      };
      oscillator.start(at);
      oscillator.stop(at + duration + 0.02);
    });
    return true;
  }
}
