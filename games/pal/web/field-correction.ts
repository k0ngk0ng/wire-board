import type { PalView } from "./view";
import type { FieldState, FieldActor } from "./field-rules";

export class FieldCorrection {
  private base?: PalView;
  private offset?: { x: number; y: number; at: number };
  private direction?: [number, number];
  current?: FieldState;
  clear() {
    this.base = undefined;
    this.offset = undefined;
    this.current = undefined;
    this.direction = undefined;
  }
  update(view: PalView, at: number, confirmed: boolean) {
    const old = this.current ?? this.base?.field;
    const compatible =
      this.base?.phase === "field" &&
      view.phase === "field" &&
      !view.raster &&
      this.base.scene === view.scene &&
      this.base.epoch === view.epoch;
    if (!compatible) {
      this.offset = undefined;
      this.direction = undefined;
    } else if (!confirmed && this.base?.field && view.field) {
      const x = view.field.camera[0] - this.base.field.camera[0],
        y = view.field.camera[1] - this.base.field.camera[1];
      if (x || y) this.direction = [x, y];
    } else if (confirmed && old && view.field) {
      const x = old.camera[0] - view.field.camera[0],
        y = old.camera[1] - view.field.camera[1];
      this.offset =
        (x || y) &&
        Math.abs(x) <= 64 &&
        Math.abs(y) <= 32 &&
        (!this.direction || x * this.direction[0] + y * this.direction[1] > 0)
          ? { x, y, at }
          : undefined;
    }
    this.base = view;
    this.sample(at);
  }
  sample(at: number) {
    const previous = this.current,
      source = this.base?.field,
      offset = this.offset;
    this.current = undefined;
    if (source && offset) {
      const factor = Math.max(0, 1 - (at - offset.at) / 80),
        x = Math.round(offset.x * factor),
        y = Math.round(offset.y * factor);
      if (!x && !y) this.offset = undefined;
      else
        this.current = {
          ...source,
          camera: [source.camera[0] + x, source.camera[1] + y],
          actors: source.actors.map((actor) =>
            actor[0] === 1
              ? actor
              : ([
                  actor[0],
                  actor[1],
                  actor[2],
                  actor[3] + x,
                  actor[4] + y,
                  actor[5],
                  actor[6],
                  actor[7],
                ] as FieldActor),
          ),
        };
    }
    return (
      !!previous !== !!this.current ||
      previous?.camera[0] !== this.current?.camera[0] ||
      previous?.camera[1] !== this.current?.camera[1]
    );
  }
}
