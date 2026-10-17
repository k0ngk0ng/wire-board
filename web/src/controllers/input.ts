export type ControllerProfile = {
  revision: number;
  keyboard: Record<string, string>;
  turboHz: number;
};

export type ControllerModel = {
  id: string;
  name: string;
  buttons: { id: string; label: string }[];
  defaults: ControllerProfile;
};

export type ControllerSettingsData = {
  models: ControllerModel[];
  profiles: Record<string, ControllerProfile>;
};

// Keyboard, touch pads and future physical controllers share logical turbo
// timing. A game only converts these logical names to its own input protocol.
export function sampleLogicalControllerButton(
  button: string,
  began: number,
  at: number,
  turboHz: number,
): string | undefined {
  if (button !== "turboA" && button !== "turboB") return button;
  const period = 1000 / turboHz;
  if (Math.max(0, at - began) % period >= period / 2) return undefined;
  return button === "turboA" ? "a" : "b";
}

// One instance belongs to one input source/profile. A physical gamepad can
// later produce the same logical button set without impersonating keyboard keys.
export class KeyboardController {
  private profile: ControllerProfile;
  private held = new Map<string, number>();

  constructor(profile: ControllerProfile) {
    this.profile = structuredClone(profile);
  }

  configure(profile: ControllerProfile) {
    this.clear();
    this.profile = structuredClone(profile);
  }

  press(code: string, at: number, repeat = false) {
    if (repeat && !this.held.has(code)) return;
    if (!this.held.has(code)) this.held.set(code, at);
  }

  release(code: string) {
    this.held.delete(code);
  }

  clear() {
    this.held.clear();
  }

  sample(at: number): Set<string> {
    const buttons = new Set<string>();
    for (const [button, code] of Object.entries(this.profile.keyboard)) {
      const began = this.held.get(code);
      if (began === undefined) continue;
      const active = sampleLogicalControllerButton(button, began, at, this.profile.turboHz);
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
