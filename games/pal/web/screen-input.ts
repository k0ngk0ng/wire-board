import type { PalInput } from "./input";
import type { PalView } from "./view";
import type { UICommand } from "./scene-renderer";

type Point = { x: number; y: number };
type Row = Point & { label: string; selected: boolean };
type Press = {
  point: Point;
  last: Point;
  at: number;
  boundary: string;
  field: boolean;
  centre: boolean;
  direction?: string;
  held: boolean;
  consumed: boolean;
  timer?: ReturnType<typeof setTimeout>;
  preview?: number;
};
type Plan = {
  boundary: string;
  next: (view: PalView) => string | undefined | null;
  sent: number;
  at: number;
  steps: number;
  observed: string;
};

export function screenUI(view: PalView): UICommand[] {
  if (!view.raster) return view.overlay ?? [];
  const ops: UICommand[] = [],
    seen = new Set<number>();
  const visit = (id: number) => {
    if (id <= 0 || id > view.raster!.nodes.length || seen.has(id)) return;
    seen.add(id);
    const n = view.raster!.nodes[id - 1];
    visit(n[1]);
    visit(n[2]);
    if (n[0] === 2)
      ops.push({
        kind: 0,
        source: 0,
        id: n[5],
        frame: 0,
        x: n[6],
        y: n[7],
        color: n[8],
        small: n[9],
      });
    if (n[0] === 1)
      ops.push({
        kind: 1,
        source: n[5],
        id: n[6],
        frame: n[7],
        x: n[8],
        y: n[9],
        color: n[10],
        small: 0,
      });
  };
  visit(view.raster.root);
  return ops;
}

export function screenRows(view: PalView): Row[] {
  const b = view.battle;
  const item =
    view.menu.kind === 5 ||
    (view.phase === "battle" &&
      b?.state === 1 &&
      (b.menu === 2 || b.menu === 3));
  const magic =
    view.menu.kind === 10 ||
    (view.phase === "battle" && b?.state === 1 && b.menu === 1);
  const misc =
    view.phase === "battle" && b?.state === 1 && (b.menu === 4 || b.menu === 5);
  const roles =
    view.phase === "menu" && (view.menu.kind === 6 || view.menu.kind === 7);
  if (!item && !magic && !misc && !roles) return [];
  const ops = screenUI(view),
    glyphs = new Map<string, UICommand>();
  for (const op of ops)
    if (op.kind === 0 && op.id && op.color) glyphs.set(`${op.x}:${op.y}`, op);
  const cursor = [...ops]
    .reverse()
    .find((op) => op.kind === 1 && op.source === 2 && op.frame === 69);
  const rows: Row[] = [];
  if (roles) {
    const names = new Set(view.party.map((member) => member.name));
    for (const first of glyphs.values()) {
      for (const name of names) {
        if (
          [...name].every(
            (char, i) =>
              glyphs.get(`${first.x + i * 16}:${first.y}`)?.id ===
              char.codePointAt(0),
          )
        ) {
          rows.push({
            x: first.x,
            y: first.y,
            label: name,
            selected:
              (first.color >= 249 && first.color <= 254) || first.color === 28,
          });
          break;
        }
      }
    }
    return rows;
  }
  for (const first of glyphs.values()) {
    const col = misc
      ? first.x === (b!.menu === 4 ? 16 : 44)
        ? 0
        : -1
      : (first.x - (item ? 15 : 35)) / (item ? 100 : 87);
    if (
      !Number.isInteger(col) ||
      col < 0 ||
      col > 2 ||
      first.y < (misc ? (b!.menu === 4 ? 32 : 62) : item ? 12 : 54) ||
      first.y > (misc ? (b!.menu === 4 ? 104 : 80) : item ? 120 : 172)
    )
      continue;
    let label = "";
    for (let x = first.x; x < first.x + (item ? 86 : 82);) {
      const g = glyphs.get(`${x}:${first.y}`);
      if (!g) break;
      label += String.fromCodePoint(g.id);
      x += g.id > 255 ? 16 : 8;
    }
    if (label)
      rows.push({
        x: first.x,
        y: first.y,
        label,
        selected: misc
          ? first.color >= 249 && first.color <= 254
          : !!cursor && cursor.x - 25 === first.x && cursor.y - 10 === first.y,
      });
  }
  return rows;
}

export function screenPartyCentre(view: PalView): Point {
  return { x: view.x - view.cameraX, y: view.y - view.cameraY - 14 };
}
function fieldDirection(point: Point, centre: Point): string {
  const dx = point.x - centre.x,
    dy = point.y - centre.y;
  if (dy === 0) return dx < 0 ? "left" : "right";
  if (dy < 0) return dx < 0 ? "left" : "up";
  return dx > 0 ? "right" : "down";
}
function distance(a: Point, b: Point) {
  return Math.hypot(a.x - b.x, a.y - b.y);
}
function boundary(v: PalView) {
  return `${v.epoch}:${v.scene}:${v.phase}:${v.dialogue?.id ?? ""}:${v.menu.kind}:${v.battle?.current ?? ""}:${v.battle?.state ?? ""}:${v.battle?.menu ?? ""}:${v.ending?.serial ?? ""}`;
}

function selection(view: PalView): string {
  if (view.menu.items.length)
    return `menu:${view.menu.kind}:${view.menu.selected}`;
  const row = screenRows(view).find((item) => item.selected);
  if (row) return `row:${row.label}:${row.x}:${row.y}`;
  const b = view.battle;
  return `${view.menu.kind}:${b?.selected ?? ""}:${b?.target ?? ""}`;
}

export class PalScreenInput {
  private presses = new Map<number, Press>();
  private plan?: Plan;
  private multiple = false;
  private multipleMoved = false;
  private multipleAt = 0;
  private frame = 0;
  private disposed = false;
  constructor(
    private canvas: HTMLCanvasElement,
    private input: PalInput,
    private view: () => PalView,
    private enabled: () => boolean,
  ) {
    canvas.addEventListener("pointerdown", this.down);
    canvas.addEventListener("pointermove", this.move);
    canvas.addEventListener("pointerup", this.up);
    canvas.addEventListener("pointercancel", this.cancel);
    canvas.addEventListener("lostpointercapture", this.cancel);
    canvas.addEventListener("contextmenu", this.context);
    window.addEventListener("blur", this.clear);
    document.addEventListener("visibilitychange", this.visibility);
    window.addEventListener("keydown", this.keyboard);
    this.frame = requestAnimationFrame(this.tick);
  }
  private point(e: PointerEvent): Point {
    const r = this.canvas.getBoundingClientRect();
    return {
      x: ((e.clientX - r.left) * 320) / r.width,
      y: ((e.clientY - r.top) * 200) / r.height,
    };
  }
  private down = (e: PointerEvent) => {
    if (!this.enabled() || e.button !== 0) return;
    const v = this.view();
    if (e.pointerType === "mouse" && v.phase === "field") return;
    if (!["dialogue", "field", "menu", "battle", "ending"].includes(v.phase))
      return;
    e.preventDefault();
    this.plan = undefined;
    const point = this.point(e),
      c = screenPartyCentre(v);
    const p: Press = {
      point,
      last: point,
      at: performance.now(),
      boundary: boundary(v),
      field: v.phase === "field",
      centre: Math.abs(point.x - c.x) <= 13 && Math.abs(point.y - c.y) <= 20,
      held: false,
      consumed: false,
    };
    if (p.field && !p.centre) p.direction = fieldDirection(point, c);
    this.presses.set(e.pointerId, p);
    this.canvas.setPointerCapture(e.pointerId);
    if (this.presses.size > 1) {
      this.multiple = true;
      this.multipleMoved = false;
      this.multipleAt = performance.now();
      for (const [id, held] of this.presses) {
        clearTimeout(held.timer);
        if (held.preview !== undefined) {
          this.input.cancelPreview(held.preview);
          held.preview = undefined;
        }
        if (held.held) this.input.releasePointer(id, true);
        held.consumed = true;
      }
      return;
    }
    if (p.field && !p.centre && p.direction)
      p.preview = this.input.preview(p.direction);
    if (
      v.phase === "dialogue" ||
      v.phase === "ending" ||
      (v.phase === "menu" && v.menu.kind === 13)
    )
      return;
    p.timer = setTimeout(
      () => {
        if (
          !this.presses.has(e.pointerId) ||
          (p.preview === undefined
            ? p.boundary !== boundary(this.view()) || !this.enabled()
            : !this.input.previewValid(p.preview)) ||
          this.multiple
        )
          return;
        p.consumed = true;
        if (p.field && !p.centre && p.direction) {
          p.held = true;
          if (p.preview !== undefined) {
            if (!this.input.commitPreview(p.preview)) return;
            p.preview = undefined;
            this.input.holdPreview(e.pointerId, p.direction);
          } else this.input.pressPointer(e.pointerId, [p.direction]);
        } else this.input.pulse("cancel");
      },
      p.field && !p.centre ? 80 : 450,
    );
  };
  private move = (e: PointerEvent) => {
    const p = this.presses.get(e.pointerId);
    if (!p) return;
    p.last = this.point(e);
    if (distance(p.point, p.last) > 10) {
      if (this.multiple) this.multipleMoved = true;
      if (!p.field) clearTimeout(p.timer);
    }
    if (p.field && !p.centre && !this.multiple) {
      const direction = fieldDirection(p.last, screenPartyCentre(this.view()));
      if (p.preview !== undefined && direction !== p.direction) {
        this.input.cancelPreview(p.preview);
        p.preview = this.input.preview(direction);
      }
      p.direction = direction;
      if (p.held) this.input.pressPointer(e.pointerId, [p.direction]);
    }
  };
  private finish(id: number, cancelled: boolean) {
    const p = this.presses.get(id);
    if (!p) return;
    clearTimeout(p.timer);
    if (p.preview !== undefined) this.input.cancelPreview(p.preview);
    if (p.held) this.input.releasePointer(id, cancelled);
    this.presses.delete(id);
  }
  private up = (e: PointerEvent) => {
    const p = this.presses.get(e.pointerId);
    if (!p) return;
    e.preventDefault();
    const v = this.view(),
      valid =
        p.preview === undefined
          ? p.boundary === boundary(v) && this.enabled()
          : this.input.previewValid(p.preview);
    if (p.preview !== undefined && valid && !this.multiple) {
      this.input.commitPreview(p.preview);
      p.preview = undefined;
      p.consumed = true;
    }
    this.finish(e.pointerId, false);
    if (this.multiple) {
      if (this.presses.size === 0) {
        if (
          valid &&
          !this.multipleMoved &&
          performance.now() - this.multipleAt < 450
        )
          this.input.pulse("cancel");
        this.multiple = false;
      }
      return;
    }
    if (!valid || p.consumed) return;
    const point = this.point(e),
      moved = distance(p.point, point);
    if (
      v.phase === "dialogue" ||
      v.phase === "ending" ||
      (v.phase === "menu" && v.menu.kind === 13)
    ) {
      if (moved <= 10) this.input.pulse("confirm");
    } else if (p.field) this.input.pulse(p.centre ? "confirm" : p.direction!);
    else if (moved > 10) {
      const dx = point.x - p.point.x,
        dy = point.y - p.point.y;
      this.input.pulse(
        Math.abs(dx) > Math.abs(dy)
          ? dx > 0
            ? "right"
            : "left"
          : dy > 0
            ? "down"
            : "up",
      );
    } else this.tapMenu(point, v);
  };
  private cancel = (e: PointerEvent) => {
    if (!this.presses.has(e.pointerId)) return;
    this.finish(e.pointerId, true);
    if (!this.presses.size) this.multiple = false;
    this.multipleMoved = true;
  };
  private context = (e: MouseEvent) => {
    e.preventDefault();
    if ((e as PointerEvent).pointerType === "touch") return;
    if (
      this.enabled() &&
      !this.presses.size &&
      this.view().phase !== "dialogue"
    )
      this.input.pulse("cancel");
  };
  private visibility = () => {
    if (document.hidden) this.clear();
  };
  private keyboard = () => {
    this.plan = undefined;
  };
  private clear = () => {
    for (const id of this.presses.keys()) this.finish(id, true);
    this.plan = undefined;
    this.multiple = false;
  };
  private navigate(next: Plan["next"], view: PalView) {
    const key = next(view);
    if (key === null) return;
    if (!key) {
      this.input.pulse("confirm");
      return;
    }
    this.plan = {
      boundary: boundary(view),
      next,
      sent: view.seq,
      at: performance.now(),
      steps: 1,
      observed: selection(view),
    };
    this.input.pulse(key);
  }
  private tapMenu(point: Point, v: PalView) {
    const item = v.menu.items.find(
      (row) =>
        point.x >= row.x - 8 &&
        point.x <= row.x + Math.max(32, [...row.label].length * 16) + 8 &&
        point.y >= row.y - 5 &&
        point.y <= row.y + 20,
    );
    if (item) {
      this.navigate((view) => {
        const target = view.menu.items.findIndex(
            (row) => row.value === item.value,
          ),
          current = view.menu.items.findIndex(
            (row) => row.value === view.menu.selected,
          );
        if (target < 0 || current < 0) return null;
        if (target === current) return undefined;
        const a = view.menu.items[current],
          b = view.menu.items[target];
        if (a.y !== b.y) return target > current ? "down" : "up";
        return b.x > a.x ? "right" : "left";
      }, v);
      return;
    }
    const rows = screenRows(v),
      hit = rows.find(
        (row) =>
          point.x >= row.x - 4 &&
          point.x <= row.x + Math.max(32, [...row.label].length * 16) &&
          point.y >= row.y - 3 &&
          point.y <= row.y + 17,
      );
    if (hit) {
      this.navigate((view) => {
        const currentRows = screenRows(view),
          target = currentRows.find(
            (row) =>
              row.label === hit.label &&
              (!(view.menu.kind === 6 || view.menu.kind === 7) ||
                (row.x === hit.x && row.y === hit.y)),
          ),
          selected = currentRows.find((row) => row.selected);
        if (!target || !selected) return null;
        if (target.label === selected.label) return undefined;
        if (target.y !== selected.y)
          return target.y > selected.y ? "down" : "up";
        return target.x > selected.x ? "right" : "left";
      }, v);
      return;
    }
    const b = v.battle;
    if (v.phase === "battle" && b) {
      if (b.state === 1 && b.menu === 0) {
        const centres = [
          { x: 39, y: 147 },
          { x: 12, y: 162 },
          { x: 66, y: 162 },
          { x: 39, y: 177 },
        ];
        const target = centres.findIndex(
          (p) => Math.abs(p.x - point.x) <= 14 && Math.abs(p.y - point.y) <= 9,
        );
        if (target >= 0)
          this.navigate(
            (view) =>
              view.battle?.selected === target
                ? undefined
                : ["up", "left", "right", "down"][target],
            v,
          );
      } else if (b.state === 2 || b.state === 4) {
        const enemies = b.enemies
          .map((enemy, i) => ({ ...enemy, i }))
          .filter(
            (e) =>
              e.object &&
              Math.abs(e.x - point.x) <= 45 &&
              point.y >= e.y - 75 &&
              point.y <= e.y + 12,
          )
          .sort(
            (a, b) =>
              distance(point, { x: a.x, y: a.y - 24 }) -
              distance(point, { x: b.x, y: b.y - 24 }),
          );
        if (enemies.length) {
          const target = enemies[0].i;
          this.navigate(
            (view) =>
              view.battle?.state === 4 || view.battle?.target === target
                ? undefined
                : target > (view.battle?.target ?? 0)
                  ? "right"
                  : "left",
            v,
          );
        }
      } else if (b.state === 3 || b.state === 5) {
        const target = Math.floor((point.x - 91) / 77);
        if (point.y >= 163 && target >= 0 && target < v.party.length)
          this.navigate(
            (view) =>
              view.battle?.state === 5 || view.battle?.target === target
                ? undefined
                : target > (view.battle?.target ?? 0)
                  ? "right"
                  : "left",
            v,
          );
      }
    } else if (v.menu.kind === 2)
      this.input.pulse(
        point.x < 80 ? "left" : point.x > 240 ? "right" : "cancel",
      );
  }
  private tick = (at: number) => {
    if (this.disposed) return;
    const v = this.view();
    for (const [id, p] of this.presses)
      if (
        p.preview === undefined
          ? !this.enabled() || p.boundary !== boundary(v)
          : !this.input.previewValid(p.preview)
      )
        this.finish(id, true);
    if (!this.presses.size) this.multiple = false;
    const plan = this.plan;
    if (plan) {
      if (
        !this.enabled() ||
        plan.boundary !== boundary(v) ||
        at - plan.at > 3000 ||
        plan.steps > 32
      )
        this.plan = undefined;
      else if (
        (v.local || (v.seq > plan.sent && v.seq === v.appliedSeq)) &&
        selection(v) !== plan.observed
      ) {
        const key = plan.next(v);
        if (!key) this.plan = undefined;
        else {
          plan.sent = v.seq;
          plan.steps++;
          plan.observed = selection(v);
          this.input.pulse(key);
        }
      }
    }
    this.frame = requestAnimationFrame(this.tick);
  };
  dispose() {
    this.disposed = true;
    this.clear();
    cancelAnimationFrame(this.frame);
    this.canvas.removeEventListener("pointerdown", this.down);
    this.canvas.removeEventListener("pointermove", this.move);
    this.canvas.removeEventListener("pointerup", this.up);
    this.canvas.removeEventListener("pointercancel", this.cancel);
    this.canvas.removeEventListener("lostpointercapture", this.cancel);
    this.canvas.removeEventListener("contextmenu", this.context);
    window.removeEventListener("blur", this.clear);
    document.removeEventListener("visibilitychange", this.visibility);
    window.removeEventListener("keydown", this.keyboard);
  }
}
