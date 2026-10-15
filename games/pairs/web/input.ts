import type { Grid, PairEvent } from "./types";

export type InputSnapshot = {
  scope: string;
  round: string;
  revision: number;
  stamp: number;
  playable: boolean;
  board: Grid | null;
  event: PairEvent;
};
type Pair = { a: number; b: number; icon: number };
type Request = {
  type: "match";
  round: string;
  revision: number;
  a: number;
  b: number;
  token: number;
};

// Input intent only: the server still decides paths, removals and winners.
// Selection is synchronous; network acknowledgements serialize matched pairs.
export class PairInput {
  private snapshot?: InputSnapshot;
  private selection: { index: number; icon: number } | null = null;
  private pairs: Pair[] = [];
  private pending?: Request & { stamp: number; done: boolean };
  private serial = 0;
  version = 0;
  get selected() {
    return this.selection?.index ?? null;
  }
  get queued() {
    return this.pairs.length;
  }
  get hasWork() {
    return !!this.pending || this.pairs.length > 0;
  }
  reserved(index: number) {
    return (
      !!(
        this.pending &&
        (this.pending.a === index || this.pending.b === index)
      ) || this.pairs.some((p) => p.a === index || p.b === index)
    );
  }
  cancel() {
    if (this.selection || this.pairs.length) {
      this.selection = null;
      this.pairs = [];
      this.version++;
    }
  }
  sync(s: InputSnapshot) {
    const old = this.snapshot;
    if (!old || old.scope !== s.scope || !s.playable) {
      this.cancel();
      if (this.pending) {
        this.pending = undefined;
        this.version++;
      }
    } else if (old.revision !== s.revision) {
      const pending = this.pending,
        pair = s.event.pair;
      const ownMatch =
        pending &&
        s.revision === pending.revision + 1 &&
        s.event.type === "match" &&
        pair &&
        ((pair[0] === pending.a && pair[1] === pending.b) ||
          (pair[1] === pending.a && pair[0] === pending.b));
      if (!ownMatch || s.event.recovery) this.cancel();
    }
    this.snapshot = s;
    if (
      this.selection &&
      !this.tile(this.selection.index, this.selection.icon)
    ) {
      this.selection = null;
      this.version++;
    }
  }
  private tile(index: number, icon?: number) {
    const c = this.snapshot?.board?.cells[index];
    return c?.kind === "tile" &&
      !c.ice &&
      (icon === undefined || c.icon === icon)
      ? c
      : null;
  }
  click(index: number) {
    if (!this.snapshot?.playable || this.reserved(index)) return;
    const c = this.tile(index);
    if (!c || c.icon === undefined) return;
    if (this.selection?.index === index) this.selection = null;
    else if (this.selection && this.selection.icon === c.icon) {
      this.pairs.push({ a: this.selection.index, b: index, icon: c.icon });
      this.selection = null;
    } else this.selection = { index, icon: c.icon };
    this.version++;
  }
  next(s: InputSnapshot, busy: boolean): Request | null {
    this.sync(s);
    if (!s.playable || busy) return null;
    if (this.pending) {
      if (
        !this.pending.done ||
        (s.revision === this.pending.revision && s.stamp <= this.pending.stamp)
      )
        return null;
      this.pending = undefined;
      this.version++;
    }
    const pair = this.pairs.shift();
    if (!pair) return null;
    this.version++;
    if (!this.tile(pair.a, pair.icon) || !this.tile(pair.b, pair.icon)) {
      this.cancel();
      return null;
    }
    const request: Request = {
      type: "match",
      round: s.round,
      revision: s.revision,
      a: pair.a,
      b: pair.b,
      token: ++this.serial,
    };
    this.pending = { ...request, stamp: s.stamp, done: false };
    return request;
  }
  complete(token: number) {
    if (this.pending?.token === token) {
      this.pending.done = true;
      this.version++;
    }
  }
}
