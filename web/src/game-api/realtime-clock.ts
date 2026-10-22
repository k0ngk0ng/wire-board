import type { GameClock } from "./module";

// Display-only estimate anchored by server samples and a monotonic local
// receipt clock. Arrival jitter can delay a viewer, never rewind the world.
// No client timestamp is sent to or used to advance authoritative rules.
export class RealtimeClock {
  private identity = "";
  private anchor?: GameClock;

  update(
    identity: string,
    serverNow: number,
    receivedAt: number,
    paused: boolean,
  ): GameClock {
    const old = this.anchor;
    if (!old || identity !== this.identity || paused !== !!old.paused) {
      this.identity = identity;
      return (this.anchor = { serverNow, receivedAt, paused });
    }
    if (receivedAt < old.receivedAt) return old;
    const projected = paused
      ? serverNow
      : old.serverNow + receivedAt - old.receivedAt;
    return (this.anchor = {
      serverNow: Math.max(serverNow, projected),
      receivedAt,
      paused,
    });
  }
}
