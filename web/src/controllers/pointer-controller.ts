import { useEffect, useRef, useState, type PointerEvent } from "react";
import type { ControllerProfile } from "./input";

export type ControllerSink = {
  pressPointer(id: number, buttons: readonly string[]): void;
  releasePointer(id: number, cancelled: boolean): void;
};
type OwnedPointer = { buttons: readonly string[]; sink: ControllerSink };
type TouchContact = { dpad: boolean; button: string };

const faceButtons = new Set(["a", "b", "c", "x", "y", "z", "turboA", "turboB"]);

function directionAt(element: HTMLElement, clientX: number, clientY: number): string[] {
  const rect = element.getBoundingClientRect();
  const x = ((clientX - rect.left) / rect.width) * 2 - 1;
  const y = ((clientY - rect.top) / rect.height) * 2 - 1;
  if (!rect.width || !rect.height || Math.abs(x) > 1 || Math.abs(y) > 1 || Math.max(Math.abs(x), Math.abs(y)) < 0.22) return [];
  const buttons: string[] = [];
  if (Math.abs(y) >= Math.abs(x) * 0.5) buttons.push(y < 0 ? "up" : "down");
  if (Math.abs(x) >= Math.abs(y) * 0.5) buttons.push(x < 0 ? "left" : "right");
  return buttons;
}

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
  const surface = useRef<HTMLDivElement>(null);
  const touches = useRef(new Map<number, TouchContact>());
  const touchChanged = useRef<(event: TouchEvent) => void>(() => {});
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
    touches.current.clear();
    setActive(new Set());
  };
  useEffect(() => {
    cancelAll();
  }, [signature, disabled, mode]);
  useEffect(() => {
    const blur = () => cancelAll();
    const visibility = () => {
      if (document.hidden) cancelAll();
    };
    const touch = (event: TouchEvent) => touchChanged.current(event);
    window.addEventListener("blur", blur);
    document.addEventListener("visibilitychange", visibility);
    for (const type of ["touchstart", "touchmove", "touchend", "touchcancel"] as const) document.addEventListener(type, touch, { capture: true, passive: false });
    return () => {
      window.removeEventListener("blur", blur);
      document.removeEventListener("visibilitychange", visibility);
      for (const type of ["touchstart", "touchmove", "touchend", "touchcancel"] as const) document.removeEventListener(type, touch, true);
      for (const [id, p] of pointers.current) p.sink.releasePointer(id, true);
      pointers.current.clear();
      touches.current.clear();
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
    return directionAt(event.currentTarget, event.clientX, event.clientY);
  };
  const down = (
    event: PointerEvent<HTMLElement>,
    buttons: readonly string[],
  ) => {
    if (event.button !== 0 || disabled || mode !== "play" || !sink) return;
    if (event.pointerType === "touch" && surface.current) return;
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    press(event.pointerId, buttons);
  };
  touchChanged.current = event => {
    const root = surface.current;
    const context = current.current;
    if (!root || context.disabled || context.mode !== "play" || !context.sink) return;
    const wasOwned = touches.current.size > 0;
    if (event.type === "touchstart") {
      for (const contact of Array.from(event.changedTouches)) {
        if (!(contact.target instanceof Element) || !root.contains(contact.target)) continue;
        const button = contact.target.closest<HTMLButtonElement>("[data-controller-button]");
        const dpad = contact.target.closest<HTMLElement>("[data-controller-dpad]");
        if (!dpad && (!button || button.disabled)) continue;
        touches.current.set(contact.identifier, { dpad: Boolean(dpad), button: button?.dataset.controllerButton ?? "" });
      }
    }
    const activeIDs = new Set(Array.from(event.touches, contact => contact.identifier));
    for (const id of touches.current.keys()) {
      if (activeIDs.has(id)) continue;
      touches.current.delete(id);
      release(-id - 1, event.type === "touchcancel");
    }
    for (const contact of Array.from(event.touches)) {
      const owned = touches.current.get(contact.identifier);
      if (!owned) continue;
      let buttons: string[] = [];
      if (owned.dpad) {
        const pad = root.querySelector<HTMLElement>("[data-controller-dpad]");
        if (pad) buttons = directionAt(pad, contact.clientX, contact.clientY);
      } else {
        const target = document.elementFromPoint(contact.clientX, contact.clientY)?.closest<HTMLButtonElement>("[data-controller-button]");
        const button = target?.dataset.controllerButton;
        if (target && root.contains(target) && !target.disabled && button && (faceButtons.has(owned.button) ? faceButtons.has(button) : button === owned.button)) buttons = [button];
      }
      press(-contact.identifier - 1, buttons);
    }
    if ((wasOwned || touches.current.size > 0) && event.cancelable) event.preventDefault();
  };
  return { active, pointers, press, end, lost, direction, down, surface };
}
