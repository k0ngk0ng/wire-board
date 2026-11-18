import { gunzipSync, unzlibSync } from "fflate";
import type { SceneFrame, SceneDraw, UICommand } from "./scene-renderer";
import type { RasterBase, RasterGraph } from "./raster-renderer";
import type { FieldState, FieldActor } from "./field-rules";
import type { PalView } from "./view";

export type NativeInput = {
  direction: number;
  keys: number;
  response?: number;
};
export type ReplayOperation = {
  kind: "step" | "release" | "advance";
  response?: number;
  remainingMS?: number;
  elapsedMS?: number;
  inputs?: (NativeInput & { at: number })[];
};
type NativeAudio = {
  serial: number;
  commands: {
    seq: number;
    clock: number;
    kind: number;
    id: number;
    loop: boolean;
    fadeMS: number;
  }[];
};
type NativeBattle = NonNullable<PalView["battle"]> & {
  active: boolean;
  result: number;
  auto: boolean;
  repeat: boolean;
  force: boolean;
  flee: boolean;
  enemies: (NonNullable<PalView["battle"]>["enemies"][number] & {
    hp: number;
    state: number;
  })[];
};
export type NativeView = SceneFrame & {
  ending?: NonNullable<SceneFrame["ending"]> & { rate: number };
  audio?: NativeAudio;
  gameOver?: PalView["gameOver"];
  cooperation: {
    pending: boolean;
    player: number;
    magic: number;
    target: number;
    affectedMask: number;
    contributorMask: number;
    hpCost: number;
    name: string;
  };
  battle: NativeBattle;
  menu: PalView["menu"];
  dialogWaiting: boolean;
  dialogLine: number;
  dialogPage: number[];
  face: number;
  dialogPosition: number;
  heldDirection: number;
  pendingKeys: number;
  keys: number;
  mode: number;
  entering: boolean;
  movingPlayer: number;
  scene: number;
  x: number;
  y: number;
  party: number[];
  sprite: number;
  hp: number;
  mp: number;
  cash: number;
  seed: number;
  clock: number;
  pc: number;
  messages: number;
};
export type NativeSnapshot = {
  entry: string;
  version: number;
  buildID: string;
  resourceID: string;
  memoryID: string;
  stackPointer: number;
  memory: null;
  event: { kind: number; value: number };
  view: NativeView;
};
export type ClientCheckpoint = {
  snapshot: NativeSnapshot;
  words: string[];
  messages: string[];
  roles: string[];
  memory: Uint8Array;
};

export function decodeCheckpoint(encoded: Uint8Array): ClientCheckpoint {
  if (encoded.length > 32 * 1024 * 1024) throw Error("仙剑同步数据过大");
  const payload = gunzipSync(encoded),
    header = new DataView(
      payload.buffer,
      payload.byteOffset,
      payload.byteLength,
    );
  if (
    payload.length < 16 ||
    new TextDecoder().decode(payload.subarray(0, 8)) !== "PALNET01"
  )
    throw Error("仙剑同步版本无效");
  const size = header.getUint32(8, true),
    memory = header.getUint32(12, true);
  if (
    size > 8 * 1024 * 1024 ||
    memory > 128 * 1024 * 1024 ||
    memory % 65536 ||
    payload.length !== 16 + size + memory
  )
    throw Error("仙剑同步结构无效");
  const metadata = JSON.parse(
    new TextDecoder().decode(payload.subarray(16, 16 + size)),
  ) as Omit<ClientCheckpoint, "memory">;
  return { ...metadata, memory: payload.subarray(16 + size) };
}

const services = new Set([
  "wb_audio_info",
  "wb_raster_seed_info",
  "wb_input",
  "wb_budget",
  "wb_release",
  "wb_view",
  "wb_scene_ops",
  "wb_scene_state",
  "wb_field_info",
  "wb_palette_state",
  "wb_ui_ops",
  "wb_dialog_info",
  "wb_menu_info",
  "wb_battle_info",
  "wb_battle_assets",
  "wb_coop_info",
  "wb_raster_graph",
  "wb_platform_players",
]);
const draw = (v: number[]): SceneDraw => ({
  kind: v[0],
  a: v[1],
  b: v[2],
  c: v[3],
  d: v[4],
  x: v[5],
  baseY: v[6],
  sortY: v[7],
});
const overlay = (v: number[]): UICommand => ({
  kind: v[0],
  source: v[1],
  id: v[2],
  frame: v[3],
  x: v[4],
  y: v[5],
  color: v[6],
  small: v[7],
});
const rows = (v: number[], width: number) =>
  Array.from({ length: v.length / width }, (_, i) =>
    v.slice(i * width, (i + 1) * width),
  );
const neutral = (): NativeInput => ({ direction: 4, keys: 0 });

export class NativeRuntime {
  private instance!: WebAssembly.Instance;
  snapshot: NativeSnapshot;
  private response = 0;
  private yielded?: NativeSnapshot["event"];
  private constructor(readonly checkpoint: ClientCheckpoint) {
    this.snapshot = checkpoint.snapshot;
  }
  static async create(
    compiled: WebAssembly.Module,
    checkpoint: ClientCheckpoint,
  ) {
    const runtime = new NativeRuntime(checkpoint),
      imports: WebAssembly.Imports = {};
    for (const entry of WebAssembly.Module.imports(compiled)) {
      imports[entry.module] ??= {};
      imports[entry.module][entry.name] = () => {
        throw Error("Unbound PAL import " + entry.module + "." + entry.name);
      };
    }
    imports.pal.yield = (kind: number, value: number) => {
      if (runtime.call("asyncify_get_state") === 2) {
        runtime.call("asyncify_stop_rewind");
        return runtime.response;
      }
      runtime.yielded = { kind, value };
      runtime.call("asyncify_start_unwind", runtime.call("wb_continuation"));
      return 0;
    };
    imports.pal.fail = (address: number) => {
      const bytes = runtime.memory();
      let end = address;
      while (end < bytes.length && bytes[end]) end++;
      throw Error(new TextDecoder().decode(bytes.subarray(address, end)));
    };
    imports.pal.inflate = (
      address: number,
      length: number,
      output: number,
      capacity: number,
    ) => {
      const data = unzlibSync(
        runtime.memory().subarray(address, address + length),
        { out: new Uint8Array(capacity) },
      );
      runtime.memory().set(data, output);
      return data.length;
    };
    runtime.instance = await WebAssembly.instantiate(compiled, imports);
    runtime.call("_initialize");
    runtime.restore(checkpoint.snapshot, checkpoint.memory);
    return runtime;
  }
  private get exports() {
    return this.instance.exports;
  }
  private get stack() {
    return this.exports.__stack_pointer as WebAssembly.Global;
  }
  memory() {
    return new Uint8Array((this.exports.memory as WebAssembly.Memory).buffer);
  }
  restore(snapshot: NativeSnapshot, memory: Uint8Array) {
    const target = this.exports.memory as WebAssembly.Memory;
    if (target.buffer.byteLength < memory.length)
      target.grow((memory.length - target.buffer.byteLength) / 65536);
    if (target.buffer.byteLength !== memory.length)
      throw Error("仙剑同步内存大小不一致");
    this.memory().set(memory);
    this.stack.value = snapshot.stackPointer;
    this.snapshot = { ...snapshot };
  }
  copyFrom(other: NativeRuntime) {
    this.restore(other.snapshot, other.memory());
  }
  previewMenus(): NativeView[] {
    const saved = { ...this.snapshot, event: { ...this.snapshot.event } },
      memory = this.memory().slice(),
      views: NativeView[] = [];
    const view = () => this.snapshot.view;
    if (
      ![0, 3].includes(view().mode) ||
      view().entering ||
      ![1, 4].includes(this.snapshot.event.kind)
    )
      return views;
    const step = (keys: number) =>
      this.advance(0, 0, () => ({ direction: 4, keys }));
    try {
      for (let i = 0; i < 8 && view().mode === 3 && view().menu.kind !== 1; i++)
        step(1);
      if (view().mode === 0) step(1);
      for (
        let i = 0;
        i < 8 &&
        view().menu.kind !== 1 &&
        [1, 4].includes(this.snapshot.event.kind);
        i++
      )
        step(0);
      if (view().menu.kind !== 1) return views;
      for (let i = 0; i < 4 && view().menu.selected !== 1; i++) step(16);
      step(2);
      for (
        let i = 0;
        i < 8 &&
        view().menu.kind !== 2 &&
        [1, 4].includes(this.snapshot.event.kind);
        i++
      )
        step(0);
      if (view().menu.kind !== 2) return views;
      views.push(view());
      for (let i = 1; i < view().party.length; i++) {
        step(32);
        views.push(view());
      }
      return views;
    } finally {
      this.restore(saved, memory);
    }
  }
  inputBoundary() {
    const v = this.snapshot.view,
      b = v.battle,
      dialogue = this.snapshot.event.kind === 2;
    return JSON.stringify([
      this.snapshot.event.kind,
      v.scene,
      v.mode,
      v.entering,
      v.menu.kind,
      v.menu.depth,
      b.active,
      b.phase,
      b.state,
      b.menu,
      b.current,
      dialogue ? v.pc : 0,
      dialogue ? v.messages : 0,
    ]);
  }
  private call(name: string, ...args: number[]): number {
    const fn = this.exports[name];
    if (typeof fn !== "function") throw Error("Missing PAL export " + name);
    const previous = this.stack.value;
    if (services.has(name))
      this.stack.value =
        Number((this.exports.wb_service_stack as WebAssembly.Global).value) +
        262144;
    try {
      return Number(fn(...args) ?? 0);
    } finally {
      if (services.has(name)) this.stack.value = previous;
    }
  }
  private ints(address: number, length: number): number[] {
    if (
      address < 0 ||
      length < 0 ||
      length > 262144 ||
      address % 4 ||
      address + length * 4 > this.memory().length
    )
      throw Error("Invalid PAL native view");
    return Array.from(new Int32Array(this.memory().buffer, address, length));
  }
  private read(name: string, length: number, ...args: number[]) {
    return this.ints(this.call(name, ...args), length);
  }
  private boundary() {
    const battle = this.read("wb_battle_info", 6),
      view = this.read("wb_view", 20),
      menu = this.read("wb_menu_info", 2);
    return JSON.stringify([
      ...(battle[0] ? battle : [0, 0, 0, 0, 0, 0]),
      view[1],
      view[17],
      view[18],
      menu[1],
      view[4],
      ...view.slice(5, 5 + view[4]),
    ]);
  }
  private run(input: NativeInput) {
    this.yielded = undefined;
    this.response = input.response ?? 0;
    this.call("wb_input", input.direction, input.keys);
    this.call("wb_budget", 1000000);
    this.call("asyncify_start_rewind", this.call("wb_continuation"));
    this.call(
      this.snapshot.entry === "campaign" ? "wb_run_campaign" : "wb_run_opening",
    );
    if (this.call("asyncify_get_state") !== 1 || !this.yielded)
      throw Error("PAL continuation did not yield");
    this.call("asyncify_stop_unwind");
    this.snapshot.event = this.yielded;
  }
  private audio(): NativeAudio | undefined {
    if (!this.exports.wb_audio_info) return undefined;
    const address = this.call("wb_audio_info"),
      header = this.ints(address, 3);
    if (header[0] !== 1 || header[2] < 0 || header[2] > 128)
      throw Error("Invalid PAL audio journal");
    return {
      serial: header[1] >>> 0,
      commands: rows(this.ints(address + 12, header[2] * 6), 6).map((v) => ({
        seq: v[0] >>> 0,
        clock: v[1] >>> 0,
        kind: v[2],
        id: v[3],
        loop: !!v[4],
        fadeMS: v[5],
      })),
    };
  }
  private rasterBase(slot: number): RasterBase {
    const address = this.call("wb_raster_seed_info", slot),
      h = this.ints(address, 8),
      data = rows(this.ints(address + 32, (h[6] + h[7]) * 8), 8);
    return {
      sceneReady: !!h[1],
      clearColor: h[2],
      map: h[3],
      cameraX: h[4],
      cameraY: h[5],
      draws: data
        .slice(0, h[6])
        .map(draw)
        .sort((a, b) => a.sortY - b.sortY),
      overlay: data.slice(h[6]).map(overlay),
    };
  }
  private raster(): RasterGraph | undefined {
    const address = this.call("wb_raster_graph"),
      h = this.ints(address, 3);
    if (!h[1]) return undefined;
    const graph: RasterGraph = {
      version: h[0],
      root: h[2],
      nodes: rows(this.ints(address + 12, h[1] * 16), 16),
    };
    for (const row of graph.nodes)
      if (row[0] === 9) {
        if (row[5] === 0 && !graph.base) graph.base = this.rasterBase(0);
        if (row[5] === 1 && !graph.backup) graph.backup = this.rasterBase(1);
      }
    return graph;
  }
  private field(): FieldState | undefined {
    const address = this.call("wb_field_info"),
      h = this.ints(address, 16);
    if (!h[0]) return undefined;
    const data = this.ints(address, 365 + h[13] * 8),
      trail = rows(data.slice(16, 31), 3) as [number, number, number][];
    return {
      frame: h[1] >>> 0,
      anchor: [h[2], h[3]],
      camera: [h[4], h[5]],
      offset: [h[6], h[7]],
      layer: h[8],
      direction: h[9],
      step: h[10],
      party: h[11],
      followers: h[12],
      radius: h[14],
      trail,
      checks: trail.map((v) => [v[0], v[1]]),
      cells: btoa(String.fromCharCode(...data.slice(31, 365))),
      actors: rows(data.slice(365), 8) as FieldActor[],
    };
  }
  observe() {
    const get = this.read("wb_view", 32),
      audio = this.audio();
    const ending = this.exports.wb_ending_info
      ? this.read("wb_ending_info", 9)
      : undefined;
    const gameOver = this.exports.wb_game_over_info
      ? this.read("wb_game_over_info", 3)
      : undefined;
    const scene = this.call("wb_scene_state"),
      palette = this.read("wb_palette_state", 12),
      redStep = this.exports.wb_palette_red_step
        ? this.call("wb_palette_red_step")
        : 0;
    const b = this.read("wb_battle_info", 87);
    const battle: NativeBattle = {
      active: !!b[0],
      result: b[1],
      phase: b[2],
      state: b[3],
      menu: b[4],
      current: b[5],
      selected: b[6],
      action: b[7],
      object: b[8],
      target: b[9],
      auto: !!b[10],
      repeat: !!b[11],
      force: !!b[12],
      flee: !!b[13],
      players: Array.from({ length: b[14] }, (_, i) => {
        const v = b.slice(16 + i * 12, 28 + i * 12);
        return {
          role: v[0],
          hp: v[1],
          maxHP: v[2],
          mp: v[3],
          maxMP: v[4],
          level: v[5],
          state: v[6],
          action: v[7],
          object: v[8],
          target: v[9],
          frame: v[10],
          defending: !!v[11],
        };
      }),
      enemies: Array.from({ length: b[15] }, (_, i) => {
        const v = b.slice(52 + i * 7, 59 + i * 7);
        return {
          object: v[0],
          hp: v[1],
          state: v[2],
          frame: v[3],
          x: v[4],
          y: v[5],
          colorShift: v[6],
        };
      }),
    };
    const battleAssets: [6 | 8, number][] = [];
    if (battle.active && this.exports.wb_battle_assets) {
      const bits = this.read("wb_battle_assets", 16);
      for (let group = 0; group < 2; group++)
        for (let word = 0; word < 8; word++)
          for (let bit = 0; bit < 32; bit++)
            if ((bits[group * 8 + word] >>> bit) & 1)
              battleAssets.push([group ? 8 : 6, word * 32 + bit]);
    }
    const raster = this.raster(),
      c = this.read("wb_coop_info", 8);
    const cooperation = c[0]
      ? {
          pending: true,
          player: c[1],
          magic: c[2],
          target: c[3],
          affectedMask: c[4],
          contributorMask: c[5],
          hpCost: c[6],
          name: this.checkpoint.words[c[2]],
        }
      : {
          pending: false,
          player: 0,
          magic: 0,
          target: 0,
          affectedMask: 0,
          contributorMask: 0,
          hpCost: 0,
          name: "",
        };
    const menuAddress = this.call("wb_menu_info"),
      m = this.ints(menuAddress, 6);
    const menu = {
      depth: m[0],
      kind: m[1],
      selected: m[3],
      role: m[4],
      items: rows(this.ints(menuAddress + 24, m[2] * 5), 5).map((v) => ({
        value: v[0],
        word: v[1],
        enabled: !!v[2],
        x: v[3],
        y: v[4],
        label: this.checkpoint.words[v[1]],
      })),
    };
    const uiAddress = this.call("wb_ui_ops"),
      overlayOps = rows(
        this.ints(uiAddress + 4, this.ints(uiAddress, 1)[0] * 8),
        8,
      ).map(overlay);
    const d = this.read("wb_dialog_info", 38),
      drawAddress = this.call("wb_scene_ops"),
      draws = rows(
        this.ints(drawAddress + 4, this.ints(drawAddress, 1)[0] * 8),
        8,
      )
        .map(draw)
        .sort((a, b) => a.sortY - b.sortY);
    const field = this.field();
    const view: NativeView = {
      ...(audio ? { audio } : {}),
      ...(ending && (ending[1] || ending[8])
        ? {
            ending: {
              clip: ending[1],
              frame: ending[2],
              frames: ending[3],
              rate: ending[4],
              elapsedMS: ending[6],
              serial: ending[7],
              finished: !!ending[8],
            },
          }
        : {}),
      ...(gameOver?.[1]
        ? { gameOver: { stage: gameOver[1], slot: gameOver[2] } }
        : {}),
      ...(field ? { field } : {}),
      paletteEffect: {
        palette: palette[0],
        night: !!palette[1],
        factors: palette.slice(3, 3 + palette[2]),
        ...(redStep ? { redStep } : {}),
      },
      sceneReady: !!(scene & 1),
      clearColor: (scene >>> 8) & 255,
      ...(raster ? { raster } : {}),
      cooperation,
      battle,
      ...(battleAssets.length ? { battleAssets } : {}),
      menu,
      overlay: overlayOps,
      dialogWaiting: !!d[0],
      dialogLine: d[1],
      dialogPage: d.slice(6, 6 + d[4]),
      cameraX: get[23],
      cameraY: get[24],
      map: get[25],
      palette: get[26],
      night: !!get[27],
      face: get[30],
      dialogPosition: get[31],
      draws,
      heldDirection: get[20],
      pendingKeys: get[21] >>> 0,
      keys: get[22] >>> 0,
      mode: get[17],
      entering: !!get[18],
      movingPlayer: get[19],
      scene: get[1],
      x: get[2] >>> 0,
      y: get[3] >>> 0,
      party: get.slice(5, 5 + get[4]),
      sprite: get[8],
      hp: get[9],
      mp: get[10],
      cash: get[11] >>> 0,
      seed: get[12] >>> 0,
      clock: get[13] >>> 0,
      pc: get[14] >>> 0,
      messages: get[15] >>> 0,
    };
    this.snapshot = {
      ...this.snapshot,
      view,
      stackPointer: Number(this.stack.value),
      memoryID: "",
    };
    return view;
  }
  advance(
    remainingMS: number,
    elapsedMS: number,
    inputAt: (offset: number) => NativeInput,
  ) {
    const timed = () =>
      this.snapshot.event.kind === 1 || this.snapshot.event.kind === 4;
    if (elapsedMS < 0 || elapsedMS > 250)
      throw Error("Invalid PAL clock budget");
    if (!timed()) return { remainingMS, steps: 0, inputReset: false };
    if (remainingMS > elapsedMS)
      return {
        remainingMS: remainingMS - elapsedMS,
        steps: 0,
        inputReset: false,
      };
    let boundary = this.boundary(),
      steps = 0,
      inputReset = false;
    const budget = elapsedMS;
    while (timed() && remainingMS <= elapsedMS) {
      if (++steps > 4096) throw Error("PAL clock exceeded native step budget");
      const at = budget - elapsedMS + remainingMS;
      elapsedMS -= remainingMS;
      this.run(inputReset ? neutral() : inputAt(at));
      if (this.snapshot.view.audio) this.audio();
      const next = this.boundary();
      if (boundary !== next) {
        inputReset = true;
        this.call("wb_release");
      }
      boundary = next;
      remainingMS = timed() ? this.snapshot.event.value : 0;
    }
    if (timed()) remainingMS -= elapsedMS;
    this.observe();
    return { remainingMS, steps, inputReset };
  }
  step(response = 0) {
    this.run({ ...neutral(), response });
    this.observe();
  }
  release() {
    this.call("wb_release");
    this.observe();
  }
  replay(operation: ReplayOperation) {
    if (operation.kind === "release") this.release();
    else if (operation.kind === "step") this.step(operation.response);
    else if (operation.kind === "advance") {
      let index = 0;
      this.advance(
        operation.remainingMS ?? 0,
        operation.elapsedMS ?? 0,
        (at) => {
          const input = operation.inputs?.[index++];
          if (!input || input.at !== at) throw Error("PAL replay input gap");
          return input;
        },
      );
      if (index !== (operation.inputs?.length ?? 0))
        throw Error("PAL replay input overflow");
    } else throw Error("Unknown PAL replay operation");
  }
  async digest() {
    const bytes = new Uint8Array(this.memory().length + 4);
    bytes.set(this.memory());
    new DataView(bytes.buffer).setUint32(
      bytes.length - 4,
      Number(this.stack.value),
      true,
    );
    return Array.from(
      new Uint8Array(await crypto.subtle.digest("SHA-256", bytes)),
      (byte) => byte.toString(16).padStart(2, "0"),
    ).join("");
  }
}
