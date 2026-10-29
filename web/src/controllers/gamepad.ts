import {
  sampleLogicalControllerButton,
  type ControllerProfile,
  type ControllerOptions,
  type GamepadBinding,
  type GamepadProfile,
} from "./input.ts";

export type GamepadSnapshot = {
  id: string;
  index: number;
  mapping: string;
  connected: boolean;
  buttons: number[];
  pressed?: boolean[];
  axes: number[];
};
export const gamepadKey = (pad: Pick<GamepadSnapshot, "mapping" | "id">) =>
  `${pad.mapping === "standard" ? "standard" : "raw"}:${pad.id}`;
export const bindingKey = (binding: GamepadBinding) =>
  binding.type === "button"
    ? `button:${binding.index}`
    : `axis:${binding.index}:${binding.direction}`;

export function bindingLabel(binding: GamepadBinding, standard = false) {
  if (binding.type === "axis")
    return `轴 ${binding.index + 1} ${binding.direction < 0 ? "−" : "+"}`;
  const names = [
    "下方键",
    "右侧键",
    "左侧键",
    "上方键",
    "L1 / LB",
    "R1 / RB",
    "L2 / LT",
    "R2 / RT",
    "Select",
    "Start",
    "左摇杆按下",
    "右摇杆按下",
    "十字键 ↑",
    "十字键 ↓",
    "十字键 ←",
    "十字键 →",
  ];
  return (standard && names[binding.index]) || `按钮 ${binding.index + 1}`;
}

export function defaultGamepadProfile(
  pad: Pick<GamepadSnapshot, "mapping">,
  logical: readonly string[],
  defaults?: GamepadProfile["bindings"],
): GamepadProfile {
  const bindings: GamepadProfile["bindings"] = {};
  if (pad.mapping !== "standard") return { deadzone: 0.25, bindings };
  if (defaults)
    return {
      deadzone: 0.25,
      bindings: structuredClone(
        Object.fromEntries(
          logical
            .filter((name) => defaults[name])
            .map((name) => [name, defaults[name]]),
        ),
      ),
    };
  const four = logical.includes("x");
  const buttons: Record<string, number> = {
    b: 0,
    a: 1,
    select: 8,
    start: 9,
    up: 12,
    down: 13,
    left: 14,
    right: 15,
    turboB: four ? 6 : 2,
    turboA: four ? 7 : 3,
    x: 2,
    y: 3,
    l: 4,
    r: 5,
  };
  for (const name of logical)
    if (buttons[name] !== undefined)
      bindings[name] = [{ type: "button", index: buttons[name] }];
  for (const [name, index, direction] of [
    ["left", 0, -1],
    ["right", 0, 1],
    ["up", 1, -1],
    ["down", 1, 1],
  ] as const)
    if (logical.includes(name))
      (bindings[name] ??= []).push({ type: "axis", index, direction });
  return { deadzone: 0.25, bindings };
}

export function readGamepads(): { pads: GamepadSnapshot[]; error?: string } {
  try {
    if (typeof navigator === "undefined" || !navigator.getGamepads)
      return {
        pads: [],
        error: "当前浏览器不支持手柄读取，请使用支持 Gamepad API 的浏览器。",
      };
    const pads = Array.from(navigator.getGamepads()).filter(
      (p): p is Gamepad => !!p && p.connected && !!p.id,
    );
    return {
      pads: pads.map((p) => ({
        id: p.id,
        index: p.index,
        mapping: p.mapping,
        connected: p.connected,
        buttons: Array.from(p.buttons, (b) =>
          finite(
            typeof b === "number" ? b : (b.value ?? (b.pressed ? 1 : 0)),
            0,
            1,
          ),
        ),
        pressed: Array.from(p.buttons, (b) =>
          typeof b === "number" ? b >= 0.5 : b.pressed,
        ),
        axes: Array.from(p.axes, (a) => finite(a, -1, 1)),
      })),
    };
  } catch {
    return {
      pads: [],
      error: "浏览器未允许读取手柄，请检查 HTTPS 和站点权限。",
    };
  }
}
function finite(value: number | undefined, min: number, max: number) {
  return Number.isFinite(value) ? Math.max(min, Math.min(max, value!)) : 0;
}
export function mergeLogicalButtons(...sources: Iterable<string>[]) {
  const result = new Set(sources.flatMap((source) => [...source]));
  for (const [a, b] of [
    ["left", "right"],
    ["up", "down"],
    ["navLeft", "navRight"],
    ["navUp", "navDown"],
  ])
    if (result.has(a) && result.has(b)) {
      result.delete(a);
      result.delete(b);
    }
  return result;
}

// Pure sampler: each device source retains its own edge/turbo state. Re-arming
// requires neutral controls after attachment or cancellation, including a
// button used by the browser to reveal a newly connected controller.
export class GamepadSampler {
  private levels = new Map<string, number>();
  get values(): ReadonlyMap<string, number> {
    return this.levels;
  }
  private cancellations = 0;
  get cancelVersion() {
    return this.cancellations;
  }
  private profile: ControllerProfile;
  private options: ControllerOptions;
  private selected = "";
  private armed = false;
  private pressed = new Set<string>();
  private held = new Map<string, number>();
  constructor(profile: ControllerProfile, options: ControllerOptions = {}) {
    this.profile = structuredClone(profile);
    this.options = structuredClone(options);
  }
  configure(profile: ControllerProfile) {
    this.profile = structuredClone(profile);
    this.selected = "";
    this.clear();
  }
  clear() {
    this.levels.clear();
    if (this.held.size || this.pressed.size) this.cancellations++;
    this.armed = false;
    this.pressed.clear();
    this.held.clear();
  }
  sample(
    devices: readonly (GamepadSnapshot | null)[],
    at: number,
    enabled = true,
  ): Set<string> {
    this.levels.clear();
    if (!enabled || this.profile.gamepadDevice === "none") {
      this.clear();
      return new Set();
    }
    const id = (pad: GamepadSnapshot) => `${pad.index}:${gamepadKey(pad)}`;
    const allowed = devices
      .filter(
        (p): p is GamepadSnapshot =>
          !!p && p.connected && Number.isInteger(p.index) && !!p.id,
      )
      .filter((p) =>
        this.profile.gamepadDevice
          ? gamepadKey(p) === this.profile.gamepadDevice
          : p.mapping === "standard" ||
            Object.values(
              this.profile.gamepads?.[gamepadKey(p)]?.bindings ?? {},
            ).some((bindings) => bindings.length > 0),
      )
      .sort((a, b) => a.index - b.index);
    const pad = allowed.find((p) => id(p) === this.selected) ?? allowed[0];
    if (!pad) {
      this.selected = "";
      this.clear();
      return new Set();
    }
    if (id(pad) !== this.selected) {
      this.selected = id(pad);
      this.clear();
    }
    const mapping =
      this.profile.gamepads?.[gamepadKey(pad)] ??
      defaultGamepadProfile(
        pad,
        Object.keys(this.profile.keyboard),
        this.options.standardGamepad,
      );
    const logical = new Set<string>(),
      pressed = new Set<string>();
    const magnitudes = new Map<string, number>();
    for (const [name, bindings] of Object.entries(mapping.bindings)) {
      if (!(name in this.profile.keyboard)) continue;
      const continuous = this.options.continuous?.includes(name);
      for (const binding of bindings) {
        const key = bindingKey(binding);
        const threshold =
          (binding.type === "button"
            ? continuous
              ? 0.05
              : 0.5
            : mapping.deadzone) * (this.pressed.has(key) ? 0.7 : 1);
        const value =
          binding.type === "button"
            ? !continuous && pad.pressed?.[binding.index]
              ? 1
              : finite(pad.buttons[binding.index], 0, 1)
            : finite(pad.axes[binding.index], -1, 1) * binding.direction;
        if (value >= threshold) {
          pressed.add(key);
          const level = continuous
            ? binding.type === "axis"
              ? Math.max(0, (value - mapping.deadzone) / (1 - mapping.deadzone))
              : value
            : 1;
          if (level > 0) {
            logical.add(name);
            magnitudes.set(name, Math.max(magnitudes.get(name) ?? 0, level));
          }
        }
      }
    }
    if (!this.armed) {
      if (!logical.size) this.armed = true;
      return new Set();
    }
    this.pressed = pressed;
    for (const name of this.held.keys())
      if (!logical.has(name)) this.held.delete(name);
    const output = new Set<string>();
    for (const name of logical) {
      if (!this.held.has(name)) this.held.set(name, at);
      const button = sampleLogicalControllerButton(
        name,
        this.held.get(name)!,
        at,
        this.profile.turboHz,
        this.options.turboTargets,
      );
      if (button) {
        output.add(button);
        this.levels.set(
          button,
          Math.max(this.levels.get(button) ?? 0, magnitudes.get(name) ?? 1),
        );
      }
    }
    const merged = mergeLogicalButtons(output);
    for (const key of this.levels.keys())
      if (!merged.has(key)) this.levels.delete(key);
    return merged;
  }
}

// Browser ownership is independent of keyboard/pointer sources. Call sample
// from the game's existing input tick; this creates no extra game clock.
export class PhysicalController {
  get values() {
    return this.sampler.values;
  }
  private sampler: GamepadSampler;
  private focused = true;
  private disposed = false;
  get cancelVersion() {
    return this.sampler.cancelVersion;
  }
  constructor(profile: ControllerProfile, options: ControllerOptions = {}) {
    this.sampler = new GamepadSampler(profile, options);
    this.focused =
      typeof document === "undefined" || (document.hasFocus?.() ?? true);
    window.addEventListener("blur", this.blur);
    window.addEventListener("focus", this.focus);
    window.addEventListener("gamepaddisconnected", this.clear);
    document.addEventListener("visibilitychange", this.visibility);
  }
  private blur = () => {
    this.focused = false;
    this.clear();
  };
  private focus = () => {
    this.focused = true;
    this.clear();
  };
  private visibility = () => {
    if (document.hidden) this.clear();
  };
  clear = () => this.sampler.clear();
  configure(profile: ControllerProfile) {
    this.sampler.configure(profile);
  }
  sample(at: number, enabled = true, navigationRoot?: HTMLElement) {
    const target = document.activeElement;
    const blocked =
      target instanceof HTMLElement &&
      (!!target.closest(
        'input:not([type="checkbox"]):not([type="radio"]):not([type="number"]):not([type="range"]),textarea,[contenteditable]:not([contenteditable="false"])',
      ) ||
        (!!target.closest(
          'input[type="checkbox"],input[type="radio"],input[type="number"],input[type="range"],select,[role="dialog"]',
        ) &&
          !navigationRoot?.contains(target)));
    const allowed =
      enabled &&
      !this.disposed &&
      this.focused &&
      !document.hidden &&
      !blocked &&
      ![...document.querySelectorAll('dialog[open],[aria-modal="true"]')].some(
        (dialog) =>
          dialog.getClientRects().length > 0 &&
          !dialog.closest("[inert]") &&
          !navigationRoot?.contains(dialog),
      );
    return this.sampler.sample(readGamepads().pads, at, allowed);
  }
  dispose() {
    this.disposed = true;
    this.clear();
    window.removeEventListener("blur", this.blur);
    window.removeEventListener("focus", this.focus);
    window.removeEventListener("gamepaddisconnected", this.clear);
    document.removeEventListener("visibilitychange", this.visibility);
  }
}
