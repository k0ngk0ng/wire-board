import { useEffect, useRef, useState, type PointerEvent } from "react";
import type { ControllerProfile } from "./input";

export type ControllerSink = {
  pressPointer(id: number, buttons: readonly string[]): void;
  releasePointer(id: number, cancelled: boolean): void;
};
type OwnedPointer = { buttons: readonly string[]; sink: ControllerSink };

// Shared pointer ownership and cancellation for the platform controller models.
// Mapping and rapid-fire sampling remain in the input layer.
export function useControllerPointers({
  profile,
  mode,
  disabled = false,
  sink,
}: {
  profile: ControllerProfile;
  mode: "play" | "configure";
  disabled?: boolean;
  sink?: ControllerSink;
}) {
  const pointers = useRef(new Map<number, OwnedPointer>());
  const [active, setActive] = useState<Set<string>>(new Set());
  const current = useRef({ sink, disabled, mode });
  current.current = { sink, disabled, mode };
  const signature = JSON.stringify([
    profile.revision,
    profile.keyboard,
    profile.turboHz,
  ]);
  const refresh = () =>
    setActive(
      new Set([...pointers.current.values()].flatMap((p) => [...p.buttons])),
    );
  const release = (id: number, cancelled: boolean) => {
    const p = pointers.current.get(id);
    if (!p) return;
    pointers.current.delete(id);
    p.sink.releasePointer(id, cancelled);
    refresh();
  };
  const cancelAll = () => {
    for (const [id, p] of pointers.current) p.sink.releasePointer(id, true);
    pointers.current.clear();
    setActive(new Set());
  };
  useEffect(() => {
    cancelAll();
    // Only a real profile/mode/disabled change resets sources, not a fresh sink
    // object supplied by the game's normal rendering cycle.
  }, [signature, disabled, mode]);
  useEffect(() => {
    const blur = () => cancelAll();
    const visibility = () => {
      if (document.hidden) cancelAll();
    };
    window.addEventListener("blur", blur);
    document.addEventListener("visibilitychange", visibility);
    return () => {
      window.removeEventListener("blur", blur);
      document.removeEventListener("visibilitychange", visibility);
      for (const [id, p] of pointers.current) p.sink.releasePointer(id, true);
      pointers.current.clear();
    };
  }, []);
  const press = (id: number, buttons: readonly string[]) => {
    const c = current.current;
    if (c.disabled || c.mode !== "play" || !c.sink) return;
    const previous = pointers.current.get(id);
    if (previous && previous.buttons.join(",") === buttons.join(",")) return;
    const owner = previous?.sink ?? c.sink;
    pointers.current.set(id, { buttons: [...buttons], sink: owner });
    owner.pressPointer(id, buttons);
    refresh();
  };
  const end = (event: PointerEvent<HTMLElement>, cancelled: boolean) => {
    release(event.pointerId, cancelled);
    if (event.currentTarget.hasPointerCapture(event.pointerId))
      event.currentTarget.releasePointerCapture(event.pointerId);
  };
  const lost = (event: PointerEvent<HTMLElement>) => {
    if (event.target === event.currentTarget) release(event.pointerId, true);
  };
  const direction = (event: PointerEvent<HTMLElement>): string[] => {
    const r = event.currentTarget.getBoundingClientRect();
    const x = ((event.clientX - r.left) / r.width) * 2 - 1;
    const y = ((event.clientY - r.top) / r.height) * 2 - 1;
    if (
      Math.abs(x) > 1 ||
      Math.abs(y) > 1 ||
      Math.max(Math.abs(x), Math.abs(y)) < 0.22
    )
      return [];
    const buttons: string[] = [];
    if (Math.abs(y) >= Math.abs(x) * 0.5) buttons.push(y < 0 ? "up" : "down");
    if (Math.abs(x) >= Math.abs(y) * 0.5)
      buttons.push(x < 0 ? "left" : "right");
    return buttons;
  };
  const down = (
    event: PointerEvent<HTMLElement>,
    buttons: readonly string[],
  ) => {
    if (event.button !== 0 || disabled || mode !== "play" || !sink) return;
    event.preventDefault(); // Keep the game's keyboard focus while pointing.
    event.currentTarget.setPointerCapture(event.pointerId);
    press(event.pointerId, buttons);
  };
  return { active, pointers, press, end, lost, direction, down };
}
