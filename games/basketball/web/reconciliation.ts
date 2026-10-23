import type { BasketballVideoFrame } from "./network-scene";

type Offset = { x: number; depth: number; height: number };
const durationMS = 160;
const ordinary = (action: number) =>
  (action >= 0 && action <= 3) || (action >= 16 && action <= 19);
const hidden = (pose: number) => (pose & 127) === 127;
const delta = (a: Offset, b: Offset): Offset => ({
  x: a.x - b.x,
  depth: a.depth - b.depth,
  height: a.height - b.height,
});
const same = (a: Offset, b: Offset) =>
  a.x === b.x && a.depth === b.depth && a.height === b.height;
const bounded = (p: Offset) =>
  Math.abs(p.x) <= 32 * 256 &&
  Math.abs(p.depth) <= 16 * 256 &&
  Math.abs(p.height) <= 16 * 256;

/** Presentation-only reconciliation, following the platform's local-motion
 * approach. Authority and the local simulation are never changed. Ordinary
 * locomotion can converge briefly; hits, warps, hidden/changed scenes snap. */
export class DisplayReconciliation {
  private offsets = new Map<number, Offset>();
  private players = new Map<
    number,
    { kind: number; character: number; airborne: boolean }
  >();
  private camera = { x: 0, y: 0 };
  private started = 0;
  private scope = "";
  reset() {
    this.offsets.clear();
    this.players.clear();
    this.camera = { x: 0, y: 0 };
    this.scope = "";
  }
  private key(frame: BasketballVideoFrame) {
    const d = frame.display,
      s = d.Visible;
    return `${d.CourtEpoch}:${s.court}:${s.phase}:${s.teams?.join(",") ?? ""}`;
  }
  correct(
    shown: BasketballVideoFrame,
    corrected: BasketballVideoFrame,
    now: number,
  ) {
    this.reset();
    const a = shown.display,
      b = corrected.display,
      old = a.Visible,
      next = b.Visible;
    if (
      !a.HasVisible ||
      !b.HasVisible ||
      old.phase !== 1 ||
      next.phase !== 1 ||
      old.result ||
      next.result ||
      old.postResult ||
      next.postResult ||
      this.key(shown) !== this.key(corrected)
    )
      return;
    const camera = {
      x: old.camera.x - next.camera.x,
      y: old.camera.y - next.camera.y,
    };
    if (
      Math.abs(camera.x) > 24 ||
      Math.abs(camera.y) > 16 ||
      old.camera.y >> 8 !== next.camera.y >> 8
    )
      return;
    const oldObjects = new Map(old.objects.map((o) => [o.id, o]));
    const translations: Offset[] = [];
    for (const object of next.objects) {
      const before = oldObjects.get(object.id);
      if (
        object.id >= 4 ||
        !before ||
        !ordinary(before.action) ||
        !ordinary(object.action) ||
        before.kind !== object.kind ||
        before.character !== object.character ||
        hidden(before.pose) ||
        hidden(object.pose) ||
        before.airborne !== object.airborne ||
        !old.drawOrder.includes(object.id) ||
        !next.drawOrder.includes(object.id)
      )
        continue;
      const offset = delta(before.position, object.position);
      if (
        !bounded(offset) ||
        (!offset.x && !offset.depth && !offset.height && !camera.x && !camera.y)
      )
        continue;
      this.offsets.set(object.id, offset);
      translations.push(offset);
      this.players.set(object.id, {
        kind: object.kind,
        character: object.character,
        airborne: object.airborne,
      });
    }
    if (!this.offsets.size) return;
    // Props/teammates translated by exactly the same simulation correction stay
    // attached. Never infer a pickup or drag an independently moving projectile.
    for (const object of next.objects) {
      if (this.offsets.has(object.id)) continue;
      const before = oldObjects.get(object.id);
      if (
        !before ||
        before.kind !== object.kind ||
        before.character !== object.character ||
        before.action !== object.action ||
        hidden(before.pose) ||
        hidden(object.pose) ||
        !old.drawOrder.includes(object.id) ||
        !next.drawOrder.includes(object.id)
      )
        continue;
      const offset = delta(before.position, object.position);
      if (
        (offset.x || offset.depth || offset.height) &&
        translations.some((value) => same(value, offset))
      )
        this.offsets.set(object.id, offset);
    }
    this.camera = camera;
    this.started = now;
    this.scope = this.key(corrected);
  }
  frame(raw: BasketballVideoFrame, now: number): BasketballVideoFrame {
    if (!this.offsets.size) return raw;
    const d = raw.display,
      s = d.Visible;
    if (
      !d.HasVisible ||
      this.key(raw) !== this.scope ||
      s.result ||
      s.postResult ||
      now - this.started >= durationMS
    ) {
      this.reset();
      return raw;
    }
    for (const [id, prior] of this.players) {
      const object = s.objects.find((o) => o.id === id);
      if (
        !object ||
        !ordinary(object.action) ||
        object.kind !== prior.kind ||
        object.character !== prior.character ||
        object.airborne !== prior.airborne ||
        hidden(object.pose) ||
        !s.drawOrder.includes(id)
      ) {
        this.reset();
        return raw;
      }
    }
    const blend = Math.max(0, 1 - Math.max(0, now - this.started) / durationMS);
    const camera = {
      x: Math.round(s.camera.x + this.camera.x * blend),
      y: Math.round(s.camera.y + this.camera.y * blend),
    };
    if (
      camera.x < 0 ||
      camera.x > 255 ||
      camera.y < 48 ||
      camera.y > 297 ||
      (camera.y & 255) >= 240
    ) {
      this.reset();
      return raw;
    }
    const dx = camera.x - s.camera.x,
      dy = camera.y - s.camera.y;
    const objects = s.objects.map((object) => {
      if (!s.drawOrder.includes(object.id)) return object;
      const offset = this.offsets.get(object.id);
      const shift = offset
        ? {
            x: Math.round(offset.x * blend),
            depth: Math.round(offset.depth * blend),
            height: Math.round(offset.height * blend),
          }
        : { x: 0, depth: 0, height: 0 };
      return {
        ...object,
        position: {
          x: object.position.x + shift.x,
          depth: object.position.depth + shift.depth,
          height: object.position.height + shift.height,
        },
        projected: {
          x: object.projected.x + Math.round(shift.x / 256) - dx,
          y:
            object.projected.y -
            Math.round(shift.depth / 256) -
            Math.round(shift.height / 256) -
            dy,
        },
      };
    });
    return { ...raw, display: { ...d, Visible: { ...s, camera, objects } } };
  }
}
