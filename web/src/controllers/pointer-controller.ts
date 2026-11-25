import { useEffect, useRef, useState, type PointerEvent } from "react";
import type { ControllerProfile } from "./input";

export type ControllerSink = {
  pressPointer(id: number, buttons: readonly string[]): void;
  releasePointer(id: number, cancelled: boolean): void;
};
type OwnedPointer = { buttons: readonly string[]; sink: ControllerSink };
type TouchContact = { dpad: boolean; button: string; face: boolean };

const faceButtons = new Set(["a", "b", "c", "x", "y", "z", "turboA", "turboB"]);
const coverageSamples = [
  [0, 0],
  [-5000, 0],
  [5000, 0],
  [0, -5000],
  [0, 5000],
  [-3500, -3500],
  [3500, -3500],
  [-3500, 3500],
  [3500, 3500],
  [-8000, 0],
  [8000, 0],
  [0, -8000],
  [0, 8000],
  [-5600, -5600],
  [5600, -5600],
  [-5600, 5600],
  [5600, 5600],
];

function insideEllipse(
  x: number,
  y: number,
  radiusX: number,
  radiusY: number,
): boolean {
  const horizontalRadius = Math.round(radiusX);
  const verticalRadius = Math.round(radiusY);
  if (horizontalRadius <= 0 || verticalRadius <= 0) return false;
  const horizontal = Math.round((x * 10000) / horizontalRadius);
  const vertical = Math.round((y * 10000) / verticalRadius);
  return horizontal * horizontal + vertical * vertical <= 100000000;
}

function contactButtons(root: HTMLElement, contact: Touch): string[] {
  const x = Math.round(contact.clientX * 1000);
  const y = Math.round(contact.clientY * 1000);
  const radiusX = Number.isFinite(contact.radiusX)
    ? Math.round(contact.radiusX * 1000)
    : 0;
  const radiusY = Number.isFinite(contact.radiusY)
    ? Math.round(contact.radiusY * 1000)
    : 0;
  const result: string[] = [];
  for (const button of root.querySelectorAll<HTMLButtonElement>(
    '[data-controller-button="a"],[data-controller-button="b"],[data-controller-button="c"]',
  )) {
    if (button.disabled) continue;
    const rect = button.getBoundingClientRect();
    const width = Math.round(rect.width * 1000);
    const height = Math.round(rect.height * 1000);
    if (width <= 0 || height <= 0) continue;
    const top = Math.round(rect.top * 1000);
    const bottom = Math.round(rect.bottom * 1000);
    if (y < top || y > bottom) continue;
    const centerX = Math.round((rect.left + rect.width / 2) * 1000);
    const centerY = Math.round((rect.top + rect.height / 2) * 1000);
    const unknown = radiusX <= 1000 || radiusY <= 1000;
    let hits = 0;
    if (!unknown) {
      for (const [sampleX, sampleY] of coverageSamples) {
        const pointX = centerX + Math.round((width * sampleX) / 20000);
        const pointY = centerY + Math.round((height * sampleY) / 20000);
        if (insideEllipse(pointX - x, pointY - y, radiusX, radiusY)) hits++;
      }
    }
    if (
      insideEllipse(
        x - centerX,
        y - centerY,
        Math.round((width * (unknown ? 70 : 50)) / 100),
        Math.round((height * (unknown ? 65 : 50)) / 100),
      ) ||
      hits >= 9
    )
      result.push(button.dataset.controllerButton!);
  }
  return result;
}

function directionAt(
  element: HTMLElement,
  clientX: number,
  clientY: number,
): string[] {
  const rect = element.getBoundingClientRect();
  const widthMilli = Math.round(rect.width * 1000);
  const heightMilli = Math.round(rect.height * 1000);
  if (
    widthMilli <= 0 ||
    heightMilli <= 0 ||
    !Number.isFinite(clientX) ||
    !Number.isFinite(clientY)
  )
    return [];
  const x = Math.round(
    ((2 * Math.round((clientX - rect.left) * 1000) - widthMilli) * 10000) /
      widthMilli,
  );
  const y = Math.round(
    ((2 * Math.round((clientY - rect.top) * 1000) - heightMilli) * 10000) /
      heightMilli,
  );
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
  return (
    element.dataset.controllerCombo?.split(" ") ?? [
      element.dataset.controllerButton ?? "",
    ]
  );
}

function isFaceControl(element: HTMLElement): boolean {
  return (
    !!element.dataset.controllerCombo ||
    faceButtons.has(element.dataset.controllerButton ?? "")
  );
}

function fitControls(root: HTMLElement) {
  const overlay = root.closest<HTMLElement>(".touch-controller-overlay");
  const stage = root.closest<HTMLElement>(".touch-controller-stage");
  const canvas = stage?.querySelector("canvas");
  if (!overlay) return;
  for (const name of [
    "size",
    "gap",
    "pad-size",
    "system-width",
    "system-height",
    "safe-left",
    "safe-right",
  ])
    overlay.style.removeProperty("--control-" + name);
  if (!stage || !canvas || !matchMedia("(pointer: coarse)").matches) return;
  const bounds = stage.getBoundingClientRect();
  const image = canvas.getBoundingClientRect();
  const width = Math.round(bounds.width);
  const height = Math.round(bounds.height);
  const nativeWidth = Math.round(canvas.width);
  const nativeHeight = Math.round(canvas.height);
  if (nativeWidth <= 0 || nativeHeight <= 0) return;
  const imageWidth = Math.min(
    Math.round(image.width),
    Math.floor((image.height * nativeWidth) / nativeHeight),
  );
  const leftSpace = Math.floor(
    image.left - bounds.left + (image.width - imageWidth) / 2,
  );
  const rightSpace = Math.floor(
    bounds.right - image.left - (image.width + imageWidth) / 2,
  );
  const columns = root.dataset.controllerLayout === "md" ? 3 : 2;
  if (
    width <= height ||
    leftSpace < 152 ||
    rightSpace < 20 + columns * 44 + (columns - 1) * 8
  )
    return;
  const gap = Math.min(24, Math.max(8, Math.floor((width * 15) / 800)));
  const size = Math.min(
    96,
    Math.floor((width * 3) / 40),
    Math.floor((rightSpace - 20 - (columns - 1) * gap) / columns),
  );
  const fittedGap = size >= 44 ? gap : 8;
  const fittedSize = Math.max(
    44,
    Math.min(
      96,
      Math.floor((rightSpace - 20 - (columns - 1) * fittedGap) / columns),
      Math.floor((width * 3) / 40),
    ),
  );
  const values = {
    size: fittedSize,
    gap: fittedGap,
    "pad-size": Math.min(256, Math.floor(width / 5), leftSpace - 20),
    "system-width": Math.min(125, Math.floor((leftSpace - 20 - fittedGap) / 2)),
    "system-height": Math.min(70, Math.max(44, Math.floor((width * 35) / 640))),
    "safe-left": 10,
    "safe-right": 10,
  };
  for (const [name, value] of Object.entries(values))
    overlay.style.setProperty("--control-" + name, value + "px");
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
    const root = surface.current;
    const stage = root?.closest<HTMLElement>(".touch-controller-stage");
    if (!root || !stage || mode !== "play") return;
    let frame = 0;
    const update = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => fitControls(root));
    };
    const resize = new ResizeObserver(update);
    resize.observe(stage);
    const changes = new MutationObserver(update);
    changes.observe(stage, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: ["width", "height"],
    });
    window.addEventListener("resize", update);
    update();
    return () => {
      cancelAnimationFrame(frame);
      resize.disconnect();
      changes.disconnect();
      window.removeEventListener("resize", update);
    };
  }, [mode]);
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
    for (const type of [
      "touchstart",
      "touchmove",
      "touchend",
      "touchcancel",
    ] as const)
      document.addEventListener(type, touch, { capture: true, passive: false });
    return () => {
      window.removeEventListener("blur", blur);
      window.removeEventListener("resize", blur);
      window.removeEventListener("orientationchange", blur);
      document.removeEventListener("visibilitychange", visibility);
      pointerMode.removeEventListener("change", blur);
      for (const type of [
        "touchstart",
        "touchmove",
        "touchend",
        "touchcancel",
      ] as const)
        document.removeEventListener(type, touch, true);
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
    if (
      !Number.isInteger(event.button) ||
      event.button !== 0 ||
      disabled ||
      mode !== "play" ||
      !sink
    )
      return;
    if (event.pointerType === "touch" && surface.current) return;
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    press(event.pointerId, buttons);
  };
  touchChanged.current = (event) => {
    const root = surface.current;
    const context = current.current;
    if (!root || context.disabled || context.mode !== "play" || !context.sink)
      return;
    const wasOwned = Boolean(touches.current.size);
    if (event.type === "touchstart") {
      for (const contact of Array.from(event.changedTouches)) {
        if (
          !(contact.target instanceof Element) ||
          !root.contains(contact.target)
        )
          continue;
        const button = contact.target.closest<HTMLElement>(
          "[data-controller-button],[data-controller-combo]",
        );
        const dpad = contact.target.closest<HTMLElement>(
          "[data-controller-dpad]",
        );
        const actionZone = contact.target.closest<HTMLElement>(
          "[data-controller-action-zone]",
        );
        if (
          !dpad &&
          ((!button && !actionZone) ||
            button?.matches(":disabled,[aria-disabled=true]"))
        )
          continue;
        touches.current.set(contact.identifier, {
          dpad: Boolean(dpad),
          button: button?.dataset.controllerButton ?? "",
          face: Boolean(actionZone) || (!!button && isFaceControl(button)),
        });
      }
    }
    const activeIDs = new Set(
      Array.from(event.touches, (contact) => contact.identifier),
    );
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
        const target = document
          .elementFromPoint(contact.clientX, contact.clientY)
          ?.closest<HTMLElement>(
            "[data-controller-button],[data-controller-combo]",
          );
        const button = target?.dataset.controllerButton;
        if (owned.face) buttons = contactButtons(root, contact);
        if (
          !buttons.length &&
          target &&
          root.contains(target) &&
          !target.matches(":disabled,[aria-disabled=true]") &&
          (owned.face
            ? isFaceControl(target) && !["a", "b", "c"].includes(button ?? "")
            : button === owned.button)
        )
          buttons = logicalButtons(target);
      }
      press(-contact.identifier - 1, buttons);
    }
    if ((wasOwned || touches.current.size) && event.cancelable)
      event.preventDefault();
  };
  return { active, pointers, press, end, lost, direction, down, surface };
}
