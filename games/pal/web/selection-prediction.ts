import type { LocalFieldInput } from "./field-prediction";
import type { PalView } from "./view";
import type { RasterGraph } from "./raster-renderer";

type Choice = { key: string; selected: number; options: number[]; mode: "menu" | "battle" };
type Pending = { seq: number; keys: number; at: number };

const limit = 8, horizonMS = 350;
const icons = [[27, 140], [0, 155], [54, 155], [27, 170]];

function choice(view: PalView, seat: number): Choice | undefined {
  if (seat < 0) return;
  const identity = `${view.epoch}:${view.scene}:${seat}`;
  const { battle: b, menu } = view;
  if (view.phase === "menu" && view.leader === seat && menu.kind === 1 &&
      menu.items.length === 4 && menu.items.every((item, i) =>
        item.enabled && item.value === i + 1 && item.x === 16 && item.y === 50 + i * 18))
    return { key: identity + ":main-menu", selected: menu.selected, options: [1, 2, 3, 4], mode: "menu" };
  if (view.raster && view.phase === "battle" && b?.phase === 0 && b.state === 1 && b.menu === 0 &&
      view.party[b.current]?.controller === seat)
    return { key: identity + ":battle:" + b.current, selected: b.selected, options: [0, 1, 2, 3], mode: "battle" };
}

function activeNodes(graph: RasterGraph): number[] {
  const active = new Set<number>(), remaining = [graph.root];
  while (remaining.length) {
    const id = remaining.pop()!;
    if (id <= 0 || id > graph.nodes.length || active.has(id)) continue;
    active.add(id);
    const [, a, b] = graph.nodes[id - 1];
    remaining.push(a, b);
  }
  return [...active].sort((a, b) => b - a).map(id => id - 1);
}

function battleIcon(graph: RasterGraph, active: number[], slot: number): number | undefined {
  const coords = icons[slot];
  if (!coords) return;
  return active.find(index => {
    const n = graph.nodes[index];
    return n[0] === 1 && n[5] === 2 && n[6] === 0 && n[7] === 40 + slot &&
      n[8] === coords[0] && n[9] === coords[1];
  });
}

function available(view: PalView, active: number[], slot: number) {
  const graph = view.raster!, index = battleIcon(graph, active, slot);
  if (index === undefined) return false;
  const node = graph.nodes[index];
  return node[10] === 0 || (node[10] === 3 && node[11] === 0 && node[12] === -4);
}

function paint(view: PalView, c: Choice, selected: number, active: number[]): PalView {
  if (selected === c.selected) return view;
  if (c.mode === "menu" && !view.raster) {
    const overlay = view.overlay ?? [];
    const old = view.menu.items.find(item => item.value === c.selected);
    const next = view.menu.items.find(item => item.value === selected);
    if (!old || !next) return view;
    const rows = [old, next].map(item => [...item.label].map((glyph, i) => {
      for (let index = overlay.length - 1; index >= 0; index--) {
        const op = overlay[index];
        if (op.kind === 0 && op.id === glyph.codePointAt(0) && op.x === item.x + i * 16 &&
            op.y === item.y && op.color !== 0 && !op.small) return index;
      }
      return -1;
    }));
    if (rows.some(row => !row.length || row.includes(-1))) return view;
    const color = overlay[rows[0][0]].color;
    if (color < 249 || color > 254 ||
        !rows[0].every(index => overlay[index].color === color) ||
        !rows[1].every(index => overlay[index].color === 79)) return view;
    const result = overlay.slice();
    for (const index of rows[0]) result[index] = { ...result[index], color: 79 };
    for (const index of rows[1]) result[index] = { ...result[index], color };
    return { ...view, overlay: result };
  }
  const raster = view.raster!, changes = new Map<number, number[]>();
  if (c.mode === "battle") {
    const old = battleIcon(raster, active, c.selected), next = battleIcon(raster, active, selected);
    if (old === undefined || next === undefined || raster.nodes[old][10] !== 0 ||
        !available(view, active, selected)) return view;
    const before = [...raster.nodes[old]], after = [...raster.nodes[next]];
    before[10] = 3;
    before[11] = 0;
    before[12] = -4;
    after[10] = after[11] = after[12] = 0;
    changes.set(old, before);
    changes.set(next, after);
  } else {
    const old = view.menu.items.find(item => item.value === c.selected);
    const next = view.menu.items.find(item => item.value === selected);
    if (!old || !next) return view;
    const rows = [old, next].map(item => [...item.label].map((glyph, i) =>
      active.find(index => {
        const n = raster.nodes[index];
        return n[0] === 2 && n[5] === glyph.codePointAt(0) && n[6] === item.x + i * 16 &&
          n[7] === item.y && n[8] !== 0 && n[9] === 0;
      })));
    if (rows.some(row => !row.length || row.some(index => index === undefined))) return view;
    const color = raster.nodes[rows[0][0]!][8];
    if (color < 249 || color > 254 ||
        !rows[0].every(index => raster.nodes[index!][8] === color) ||
        !rows[1].every(index => raster.nodes[index!][8] === 79)) return view;
    for (const [side, row] of rows.entries()) {
      for (const index of row) {
        const node = [...raster.nodes[index!]];
        node[8] = side === 0 ? 79 : color;
        changes.set(index!, node);
      }
    }
  }
  const nodes = raster.nodes.slice();
  for (const [index, node] of changes) nodes[index] = node;
  return { ...view, raster: { ...raster, nodes } };
}

export class SelectionPrediction {
  private pending: Pending[] = [];
  private boundary?: string;
  private discardedSeq = 0;
  private blockedAt = 0;

  clear() {
    const changed = this.pending.length > 0;
    this.pending = [];
    this.boundary = undefined;
    this.blockedAt = 0;
    return changed;
  }

  update(view: PalView, seat: number, at: number) {
    const c = choice(view, seat), discarded = view.discardedSeq ?? 0;
    let changed = false;
    if (this.boundary !== c?.key || discarded > this.discardedSeq ||
        this.pending.some(input => at - input.at >= horizonMS))
      changed = this.clear();
    this.boundary = c?.key;
    this.discardedSeq = discarded;
    if (this.blockedAt && this.blockedAt <= view.appliedSeq) this.blockedAt = 0;
    const count = this.pending.length;
    this.pending = this.pending.filter(input => input.seq > view.appliedSeq);
    return changed || count !== this.pending.length;
  }

  input(view: PalView, seat: number, input: LocalFieldInput, at: number) {
    const changed = this.update(view, seat, at);
    if (!this.boundary || input.epoch !== view.epoch || input.seq <= view.appliedSeq ||
        input.seq <= this.discardedSeq || !input.keys) return changed;
    if (![4, 8, 16, 32].includes(input.keys)) {
      this.blockedAt = input.seq;
      if (!this.pending.length) return changed;
      this.pending = [];
      return true;
    }
    if (this.blockedAt || this.pending.length >= limit) return changed;
    this.pending.push({ seq: input.seq, keys: input.keys, at });
    return true;
  }

  frame(view: PalView, seat: number) {
    const c = choice(view, seat);
    if (!c || this.boundary !== c.key || !this.pending.length) return view;
    const active = view.raster ? activeNodes(view.raster) : [];
    let selected = c.selected;
    for (const input of this.pending) {
      if (input.seq <= view.appliedSeq || input.seq <= (view.discardedSeq ?? 0)) continue;
      if (c.mode === "menu") {
        const index = c.options.indexOf(selected);
        if (index < 0) return view;
        const direction = input.keys === 4 || input.keys === 32 ? 1 : -1;
        selected = c.options[(index + direction + c.options.length) % c.options.length];
      } else {
        const slot = input.keys === 16 ? 0 : input.keys === 8 ? 1 : input.keys === 32 ? 2 : 3;
        if (available(view, active, slot)) selected = slot;
      }
    }
    return paint(view, c, selected, active);
  }
}
