export type ControllerProfile = {
  revision: number;
  keyboard: Record<string, string>;
  keyboardAlt?: Record<string, string[]>;
  turboHz: number;
  gamepads?: Record<string, GamepadProfile>;
  gamepadDevice?: string;
};

export type GamepadBinding =
  | { type: "button"; index: number }
  | { type: "axis"; index: number; direction: -1 | 1 };
export type GamepadProfile = {
  deadzone: number;
  bindings: Record<string, GamepadBinding[]>;
};

export type ControllerModel = {
  id: string;
  name: string;
  buttons: { id: string; label: string }[];
  defaults: ControllerProfile;
  continuous?: string[];
  gameId?: string;
  layout?: string;
  standardGamepad?: GamepadProfile["bindings"];
  turboTargets?: Record<string, string>;
};

export type ControllerOptions = Pick<
  ControllerModel,
  "standardGamepad" | "turboTargets" | "continuous"
>;

export function actionsForKey(
  profile: ControllerProfile,
  code: string,
): string[] {
  if (!code) return [];
  return Object.keys(profile.keyboard).filter(
    (name) =>
      profile.keyboard[name] === code ||
      profile.keyboardAlt?.[name]?.includes(code),
  );
}

export type ControllerSettingsData = {
  models: ControllerModel[];
  profiles: Record<string, ControllerProfile>;
};

// Keyboard, touch pads and physical controllers share logical turbo
// timing. A game only converts these logical names to its own input protocol.
export function sampleLogicalControllerButton(
  button: string,
  began: number,
  at: number,
  turboHz: number,
  targets?: Record<string, string>,
): string | undefined {
  const target =
    targets === undefined
      ? ({ turboA: "a", turboB: "b" } as Record<string, string>)[button]
      : targets[button];
  if (!target) return button;
  const period = 1000 / turboHz;
  if (Math.max(0, at - began) % period >= period / 2) return undefined;
  return target;
}

// Keyboard state is independent of physical controllers and touch pointers.
// Game adapters merge their logical outputs, never synthetic keyboard events.
export class KeyboardController {
  private profile: ControllerProfile;
  private held = new Map<string, number>();
  private actionStarts = new Map<string, number>();
  private turboTargets?: Record<string, string>;

  constructor(
    profile: ControllerProfile,
    turboTargets?: Record<string, string>,
  ) {
    this.profile = structuredClone(profile);
    this.turboTargets = turboTargets;
  }

  configure(profile: ControllerProfile) {
    this.clear();
    this.profile = structuredClone(profile);
  }

  press(code: string, at: number, repeat = false) {
    if (!code) return;
    if (repeat && !this.held.has(code)) return;
    if (!this.held.has(code)) this.held.set(code, at);
  }

  release(code: string) {
    this.held.delete(code);
    for (const action of actionsForKey(this.profile, code)) {
      const keys = [
        this.profile.keyboard[action],
        ...(this.profile.keyboardAlt?.[action] ?? []),
      ];
      if (!keys.some((key) => this.held.has(key)))
        this.actionStarts.delete(action);
    }
  }

  clear() {
    this.held.clear();
    this.actionStarts.clear();
  }

  sample(at: number): Set<string> {
    const buttons = new Set<string>();
    for (const [button, code] of Object.entries(this.profile.keyboard)) {
      const starts = [
        code,
        ...(this.profile.keyboardAlt?.[button] ?? []),
      ].flatMap((key) => (this.held.has(key) ? [this.held.get(key)!] : []));
      if (!starts.length) {
        this.actionStarts.delete(button);
        continue;
      }
      if (!this.actionStarts.has(button))
        this.actionStarts.set(button, Math.min(...starts));
      const began = this.actionStarts.get(button)!;
      const active = sampleLogicalControllerButton(
        button,
        began,
        at,
        this.profile.turboHz,
        this.turboTargets,
      );
      if (active) buttons.add(active);
    }
    return buttons;
  }
}

export function keyLabel(code: string): string {
  const labels: Record<string, string> = {
    Space: "空格",
    Enter: "Enter",
    Tab: "Tab",
    Backspace: "退格",
    ArrowUp: "↑",
    ArrowDown: "↓",
    ArrowLeft: "←",
    ArrowRight: "→",
    ShiftLeft: "左 Shift",
    ShiftRight: "右 Shift",
    ControlLeft: "左 Ctrl",
    ControlRight: "右 Ctrl",
    AltLeft: "左 Alt",
    AltRight: "右 Alt",
    BracketLeft: "[",
    BracketRight: "]",
    Semicolon: ";",
    Quote: "'",
    Comma: ",",
    Period: ".",
    Slash: "/",
    Backslash: "\\",
    Minus: "-",
    Equal: "=",
    Backquote: "`",
  };
  return labels[code] ?? code.replace(/^Key|^Digit/, "");
}
