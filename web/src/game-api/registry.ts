import { LegacyCover } from "../legacy/cover";
import { createElement, useSyncExternalStore } from "react";
import type { GameModule } from "./module";
const catalogs = import.meta.glob<{ default: Omit<GameModule, "Board"> }>(
  "../../../games/*/web/catalog.tsx",
  { eager: true },
);
const runtimes = import.meta.glob<{ default: GameModule }>(
  "../../../games/*/web/module.tsx",
);
const modules = new Map<string, GameModule>();
const loaders = new Map<string, () => Promise<{ default: GameModule }>>();
const pending = new Map<string, Promise<GameModule>>();
const loaded = new Set<string>();
const listeners = new Set<() => void>();
let revision = 0;
for (const [path, entry] of Object.entries(catalogs)) {
  const meta = entry.default;
  if (!meta.id || modules.has(meta.id))
    throw new Error(`无效或重复的游戏模块：${path}`);
  modules.set(meta.id, {
    ...meta,
    ...(meta.legacy
      ? { Cover: () => createElement(LegacyCover, { kind: meta.id }) }
      : {}),
    Board: () => null,
  });
  loaders.set(meta.id, runtimes[path.replace("catalog.tsx", "module.tsx")]);
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
export function gameLoaded(kind: string) {
  return loaded.has(kind);
}
export function loadGame(kind: string): Promise<GameModule> {
  if (loaded.has(kind)) return Promise.resolve(modules.get(kind)!);
  if (!pending.has(kind)) {
    const loader = loaders.get(kind);
    if (!loader) return Promise.reject(new Error("未找到游戏界面"));
    pending.set(
      kind,
      loader()
        .then(({ default: module }) => {
          module.Cover ??= modules.get(kind)?.Cover;
          modules.set(kind, module);
          loaded.add(kind);
          revision++;
          listeners.forEach((fn) => fn());
          return module;
        })
        .catch((error) => {
          pending.delete(kind);
          throw error;
        }),
    );
  }
  return pending.get(kind)!;
}
export function useGameRegistry() {
  return useSyncExternalStore(
    (fn) => {
      listeners.add(fn);
      return () => {
        listeners.delete(fn);
      };
    },
    () => revision,
  );
}
