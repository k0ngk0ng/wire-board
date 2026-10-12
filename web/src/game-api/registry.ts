import type { GameModule } from "./module";

const entries = import.meta.glob<{ default: GameModule }>(
  "../../../games/*/web/module.tsx",
  { eager: true },
);
const modules = new Map<string, GameModule>();
for (const [path, entry] of Object.entries(entries).sort(([a], [b]) =>
  a.localeCompare(b),
)) {
  const module = entry.default;
  if (!module?.id || !module.name || !module.Board || modules.has(module.id)) {
    throw new Error(`无效或重复的游戏模块：${path}`);
  }
  modules.set(module.id, module);
}
export function gameModule(kind: string) {
  return modules.get(kind);
}
export function gameName(kind: string) {
  return modules.get(kind)?.name || kind;
}
export function isIndependentGame(kind: string) {
  return !!modules.get(kind) && !modules.get(kind)?.legacy;
}
