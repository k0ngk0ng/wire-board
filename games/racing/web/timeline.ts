// All cars, impact art and sounds share the same scene time. Remote driving is
// boundedly extrapolated for a local driver; observers use a 110ms buffer.
export const sceneTime = (serverNow: number, driving: boolean) =>
  serverNow - (driving ? 0 : 110);
export function effectAge(at: number, visualAt: number): number | null {
  const age = visualAt - at;
  return age < 0 || age >= 550 ? null : age;
}
export function soundDelay(
  at: number,
  serverNow: number,
  driving: boolean,
): number | null {
  const delta = at - sceneTime(serverNow, driving);
  return delta <= -550 ? null : Math.max(0, delta);
}
export class RaceAudioQueue {
  private timers = new Set<number>();
  private schedule: (fn: () => void, delay: number) => number;
  private cancel: (timer: number) => void;
  constructor(
    schedule = (fn: () => void, delay: number) => window.setTimeout(fn, delay),
    cancel = (timer: number) => window.clearTimeout(timer),
  ) {
    this.schedule = schedule;
    this.cancel = cancel;
  }
  get pending() {
    return this.timers.size;
  }
  enqueue(play: () => void, delay: number) {
    const timer = this.schedule(() => {
      this.timers.delete(timer);
      play();
    }, delay);
    this.timers.add(timer);
  }
  clear() {
    for (const timer of this.timers) this.cancel(timer);
    this.timers.clear();
  }
}
