import type { GameClock } from "../../../web/src/game-api/module";
import type { Flight, Vec } from "./types";
export function gameNow(clock: GameClock, now: number): number {
  return (
    clock.serverNow + (clock.paused ? 0 : Math.max(0, now - clock.receivedAt))
  );
}
export function localPoint(p: Vec, side: number): Vec {
  return side === 0 ? { ...p } : { x: -p.x, y: p.y, z: -p.z };
}
export function sampleFlight(
  flight: Pick<Flight, "segments">,
  at: number,
): Vec {
  let segment = flight.segments[0];
  if (!segment) return { x: 0, y: 0.9, z: 0 };
  for (const candidate of flight.segments) {
    if (candidate.at <= at) segment = candidate;
    else break;
  }
  const t =
    Math.max(0, Math.min(segment.until - segment.at, at - segment.at)) / 1000;
  return {
    x: segment.p.x + segment.v.x * t + 0.5 * segment.a.x * t * t,
    y: segment.p.y + segment.v.y * t + 0.5 * segment.a.y * t * t,
    z: segment.p.z + segment.v.z * t + 0.5 * segment.a.z * t * t,
  };
}
/** Only confirmed impact events. Reset skips history on entry/reconnect. */
export class EventTimeline {
  private watermark = 0;
  private notBefore = -Infinity;
  reset(events: readonly { id: number; at: number }[], now: number) {
    this.notBefore = now;
    this.watermark = events.reduce(
      (n, e) => (e.at <= now ? Math.max(n, e.id) : n),
      0,
    );
  }
  take<T extends { id: number; at: number }>(
    events: readonly T[],
    now: number,
  ): T[] {
    const result: T[] = [];
    for (const event of events) {
      if (event.id <= this.watermark || event.at > now) continue;
      this.watermark = event.id;
      if (event.at > this.notBefore && now - event.at <= 300)
        result.push(event);
    }
    return result;
  }
}
