import type { Action } from "./types";
import type { Panel } from "./Panels";

type KeyInput = { key: string; ctrlKey?: boolean; metaKey?: boolean; altKey?: boolean; repeat?: boolean; typing?: boolean };
type Command = { type: "select" | "aim"; slot: number } | { type: "panel"; panel: Panel } | { type: "action"; action: Action } | { type: "damage" | "ranges" | "close" | "wave" } | { type: "details"; direction: number };
export function keyboardCommand(input: KeyInput, language: number): Command | undefined {
  if (input.repeat || input.typing || input.metaKey) return;
  const key = input.key.toLowerCase();
  if (input.ctrlKey) return /^[1-5]$/.test(key) ? { type: "action", action: { type: "saveTeam", slot: Number(key) - 1 } } : undefined;
  if (input.altKey) return /^[0-9]$/.test(key) ? { type: "aim", slot: (Number(key) + 9) % 10 } : undefined;
  if (/^f[1-5]$/.test(key)) return { type: "action", action: { type: "loadTeam", slot: Number(key.slice(1)) - 1 } };
  if (/^[0-9]$/.test(key)) return { type: "select", slot: (Number(key) + 9) % 10 };
  if (key === "escape") return { type: "close" };
  if (key === " " || key === "enter") return { type: "wave" };
  if (key === "shift") return { type: "action", action: { type: "speed" } };
  if (key === "x") return { type: "ranges" };
  if (key === (language === 2 ? "w" : "z")) return { type: "damage" };
  const panels: Record<string, Panel> = { [language === 2 ? "a" : "q"]: "profile", [language === 2 ? "z" : "w"]: "routes", e: "box", s: "shop" };
  if (panels[key]) return { type: "panel", panel: panels[key] };
  if (key === "arrowleft" || key === (language === 2 ? "q" : "a")) return { type: "details", direction: -1 };
  if (key === "arrowright" || key === "d") return { type: "details", direction: 1 };
}
