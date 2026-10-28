import { useSyncExternalStore } from "react";
import type { RuntimeSnapshot, RuntimeSnapshotHeader } from "./runtime-bridge";

// Both public life summaries and the fullscreen HUD follow the painted state.
// Server snapshots may be up to one small presentation buffer ahead.
const values = new Map<string, RuntimeSnapshot>();
const listeners = new Map<string, Set<() => void>>();
export const runtimeVisibleKey = (roomID: string, epoch: string) =>
  `${roomID}:${epoch}`;
export const readRuntimeVisible = (key: string) => values.get(key);
export function publishRuntimeVisible(key: string, value?: RuntimeSnapshot) {
  if (value) values.set(key, value);
  else values.delete(key);
  for (const listener of listeners.get(key) ?? []) listener();
}
export function useRuntimeVisible(
  key: string,
  fallback: RuntimeSnapshotHeader,
) {
  return useSyncExternalStore(
    (listener) => {
      let group = listeners.get(key);
      if (!group) {
        group = new Set();
        listeners.set(key, group);
      }
      group.add(listener);
      return () => {
        group!.delete(listener);
        if (!group!.size) listeners.delete(key);
      };
    },
    () => values.get(key) ?? fallback,
    () => fallback,
  );
}
