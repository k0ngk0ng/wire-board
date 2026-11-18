import { keyboardCommand } from "./keyboard-command";

export function controllerCommand(action: string): ReturnType<typeof keyboardCommand> {
  const keys: Record<string, string> = { profile: "q", routes: "w", box: "e", shop: "s", damage: "z", ranges: "x", speed: "Shift", previousDetails: "a", nextDetails: "d" };
  if (Object.hasOwn(keys, action)) return keyboardCommand({ key: keys[action] }, 8);
  if (/^select[0-9]$/.test(action)) return { type: "select", slot: Number(action.slice(6)) };
  if (/^load[0-4]$/.test(action)) return { type: "action", action: { type: "loadTeam", slot: Number(action.slice(4)) } };
}
