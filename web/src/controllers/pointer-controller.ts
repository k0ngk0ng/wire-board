import { useEffect, useRef, useState, type PointerEvent } from "react";
import type { ControllerProfile } from "./input";

export type ControllerSink = {
  pressPointer(id: number, buttons: readonly string[]): void;
  releasePointer(id: number, cancelled: boolean): void;
};
type OwnedPointer = { buttons: readonly string[]; sink: ControllerSink };
type TouchContact = { dpad: boolean; button: string; face: boolean };

const faceButtons = new Set(["a", "b", "c", "x", "y", "z", "turboA", "turboB"]);

function directionAt(element: HTMLElement, clientX: number, clientY: number): string[] {
  const rect = element.getBoundingClientRect();
  const widthMilli = Math.round(rect.width * 1000);
  const heightMilli = Math.round(rect.height * 1000);
  if (widthMilli <= 0 || heightMilli <= 0 || !Number.isFinite(clientX) || !Number.isFinite(clientY)) return [];
  const x = Math.round((2 * Math.round((clientX - rect.left) * 1000) - widthMilli) * 10000 / widthMilli);
  const y = Math.round((2 * Math.round((clientY - rect.top) * 1000) - heightMilli) * 10000 / heightMilli);
  const absX = Math.abs(x);
  const absY = Math.abs(y);
  if (absX > 10000 || absY > 10000 || Math.max(absX, absY) < 2200) return [];
  const vertical = y < 0 ? "up" : "down";
  const horizontal = x < 0 ? "left" : "right";
  if (absY * 1000000 > absX * 2144507) return [vertical];
  if (absY * 1000000 < absX * 466308) return [horizontal];
  return [vertical, horizontal];
}

function logicalButtons(element: HTMLElement): string[] {
  return element.dataset.controllerCombo?.split(" ") ?? [element.dataset.controllerButton ?? ""];
}

function isFaceControl(element: HTMLElement): boolean {
  return !!element.dataset.controllerCombo || faceButtons.has(element.dataset.controllerButton ?? "");
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
    const pointerMode = window.matchMedia("(pointer: coarse)");
    pointerMode.addEventListener("change", blur);
    window.addEventListener("blur", blur);
    window.addEventListener("resize", blur);
    window.addEventListener("orientationchange", blur);
    document.addEventListener("visibilitychange", visibility);
    for (const type of ["touchstart", "touchmove", "touchend", "touchcancel"] as const) document.addEventListener(type, touch, { capture: true, passive: false });
    return () => {
      window.removeEventListener("blur", blur);
      window.removeEventListener("resize", blur);
      window.removeEventListener("orientationchange", blur);
      document.removeEventListener("visibilitychange", visibility);
      pointerMode.removeEventListener("change", blur);
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
    if (!Number.isInteger(event.button) || event.button !== 0 || disabled || mode !== "play" || !sink) return;
    if (event.pointerType === "touch" && surface.current) return;
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    press(event.pointerId, buttons);
  };
  touchChanged.current = event => {
    const root = surface.current;
    const context = current.current;
    if (!root || context.disabled || context.mode !== "play" || !context.sink) return;
    const wasOwned = Boolean(touches.current.size);
    if (event.type === "touchstart") {
      for (const contact of Array.from(event.changedTouches)) {
        if (!(contact.target instanceof Element) || !root.contains(contact.target)) continue;
        const button = contact.target.closest<HTMLElement>("[data-controller-button],[data-controller-combo]");
        const dpad = contact.target.closest<HTMLElement>("[data-controller-dpad]");
        if (!dpad && (!button || button.matches(":disabled,[aria-disabled=true]"))) continue;
        touches.current.set(contact.identifier, { dpad: Boolean(dpad), button: button?.dataset.controllerButton ?? "", face: !!button && isFaceControl(button) });
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
        const target = document.elementFromPoint(contact.clientX, contact.clientY)?.closest<HTMLElement>("[data-controller-button],[data-controller-combo]");
        const button = target?.dataset.controllerButton;
        if (target && root.contains(target) && !target.matches(":disabled,[aria-disabled=true]") && (owned.face ? isFaceControl(target) : button === owned.button)) buttons = logicalButtons(target);
      }
      press(-contact.identifier - 1, buttons);
    }
    if ((wasOwned || touches.current.size) && event.cancelable) event.preventDefault();
  };
  return { active, pointers, press, end, lost, direction, down, surface };
}
