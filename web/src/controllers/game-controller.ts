import {
  KeyboardController,
  actionsForKey,
  type ControllerModel,
  type ControllerProfile,
} from "./input";
import { PhysicalController, mergeLogicalButtons } from "./gamepad";
import { ControllerNavigation } from "./navigation";

export type GameControllerContext = {
  enabled: () => boolean;
  menu?: () => HTMLElement | null;
  menuActions?: Record<string, string>;
  changed: () => void;
  keyboardTarget?: (target: EventTarget | null) => boolean;
};

// Samples only when the owning game asks. Key edges also notify the game's
// existing input path immediately, so short taps cannot disappear between frames.
export class GameController {
  private levels = new Map<string, number>();
  get values(): ReadonlyMap<string, number> {
    return this.levels;
  }
  private root: HTMLElement;
  private context: GameControllerContext;
  private profile: ControllerProfile;
  private keyboard: KeyboardController;
  private physical: PhysicalController;
  private order = new Set<string>();
  private navigation?: ControllerNavigation;
  private menuRoot: HTMLElement | null = null;
  private focused = document.hasFocus();
  private disposed = false;
  private cancellations = 0;
  get cancelVersion() {
    return this.cancellations + this.physical.cancelVersion;
  }
  constructor(
    root: HTMLElement,
    profile: ControllerProfile,
    model: ControllerModel,
    context: GameControllerContext,
  ) {
    this.root = root;
    this.context = context;
    this.profile = profile;
    this.keyboard = new KeyboardController(profile, model.turboTargets ?? {});
    this.physical = new PhysicalController(profile, model);
    window.addEventListener("keydown", this.down, true);
    window.addEventListener("keyup", this.up, true);
    window.addEventListener("blur", this.blur);
    window.addEventListener("focus", this.focus);
    document.addEventListener("visibilitychange", this.visibility);
    document.addEventListener("focusin", this.focusIn);
  }
  configure(profile: ControllerProfile) {
    if (this.order.size) this.cancellations++;
    this.profile = profile;
    this.keyboard.configure(profile);
    this.physical.configure(profile);
    this.order.clear();
    this.navigation?.clear();
    this.context.changed();
  }
  clear() {
    this.levels.clear();
    if (this.order.size || this.keyboard.sample(performance.now()).size)
      this.cancellations++;
    this.keyboard.clear();
    this.physical.clear();
    this.order.clear();
    this.navigation?.clear();
  }
  private available() {
    const menu = this.context.menu?.() ?? null;
    return (
      !this.disposed &&
      this.focused &&
      !document.hidden &&
      (this.context.enabled() || !!menu) &&
      ![...document.querySelectorAll('dialog[open],[aria-modal="true"]')].some(
        (el) =>
          el.getClientRects().length &&
          !el.closest("[inert]") &&
          !menu?.contains(el),
      )
    );
  }
  private keyboardAllowed(target: EventTarget | null) {
    if (!this.available()) return false;
    if (
      !this.context.menu?.() &&
      this.context.keyboardTarget &&
      !this.context.keyboardTarget(target)
    )
      return false;
    if (!(target instanceof Element)) return true;
    if (
      target.closest(
        'input,textarea,select,[contenteditable]:not([contenteditable="false"])',
      )
    )
      return false;
    return (
      target === document.body ||
      this.root.contains(target) ||
      (target.matches("[data-fullscreen-board]") && target.contains(this.root))
    );
  }
  private down = (event: KeyboardEvent) => {
    if (
      !this.keyboardAllowed(event.target) ||
      event.defaultPrevented ||
      event.isComposing ||
      ((event.ctrlKey || event.metaKey || event.altKey) &&
        !/^(Control|Alt|Meta)/.test(event.code))
    )
      return;
    if (!actionsForKey(this.profile, event.code).length) return;
    event.preventDefault();
    event.stopImmediatePropagation();
    this.keyboard.press(event.code, performance.now(), event.repeat);
    this.context.changed();
  };
  private up = (event: KeyboardEvent) => {
    this.keyboard.release(event.code);
    if (actionsForKey(this.profile, event.code).length) this.context.changed();
  };
  private blur = () => {
    this.focused = false;
    this.clear();
    this.context.changed();
  };
  private focus = () => {
    this.focused = true;
    this.clear();
    this.context.changed();
  };
  private visibility = () => {
    if (document.hidden) {
      this.clear();
      this.context.changed();
    }
  };
  private focusIn = () => {
    if (!this.keyboardAllowed(document.activeElement)) {
      if (this.keyboard.sample(performance.now()).size) this.cancellations++;
      this.keyboard.clear();
      this.context.changed();
    }
  };
  sample(at: number): Set<string> {
    this.levels.clear();
    if (!this.available()) {
      this.clear();
      return new Set();
    }
    const menu = this.context.menu?.() ?? null;
    if (menu !== this.menuRoot) {
      this.navigation?.dispose();
      this.navigation = menu ? new ControllerNavigation(menu) : undefined;
      this.menuRoot = menu;
      this.clear();
    }
    if (!this.keyboardAllowed(document.activeElement)) {
      if (this.keyboard.sample(at).size) this.cancellations++;
      this.keyboard.clear();
    }
    const keyboard = this.keyboard.sample(at);
    const actions = mergeLogicalButtons(
      keyboard,
      this.physical.sample(at, true, menu ?? undefined),
    );
    if (menu) {
      this.navigation!.sample(
        new Set([...actions].map((a) => this.context.menuActions?.[a] ?? a)),
        at,
      );
      return new Set();
    }
    for (const action of actions)
      this.levels.set(
        action,
        Math.max(
          keyboard.has(action) ? 1 : 0,
          this.physical.values.get(action) ?? 0,
        ),
      );
    for (const action of this.order)
      if (!actions.has(action)) this.order.delete(action);
    for (const action of actions) this.order.add(action);
    return new Set(this.order);
  }
  dispose() {
    this.disposed = true;
    this.clear();
    this.navigation?.dispose();
    this.physical.dispose();
    window.removeEventListener("keydown", this.down, true);
    window.removeEventListener("keyup", this.up, true);
    window.removeEventListener("blur", this.blur);
    window.removeEventListener("focus", this.focus);
    document.removeEventListener("visibilitychange", this.visibility);
    document.removeEventListener("focusin", this.focusIn);
  }
}

export function lastDirection(
  actions: Iterable<string>,
): "none" | "up" | "down" | "left" | "right" {
  let direction: ReturnType<typeof lastDirection> = "none";
  for (const action of actions)
    if (
      action === "up" ||
      action === "down" ||
      action === "left" ||
      action === "right"
    )
      direction = action;
  return direction;
}
