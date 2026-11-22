import {
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type PointerEvent as ReactPointerEvent,
  type RefObject,
} from "react";
import { defaultChatAnchor } from "./chat-anchor";

type Point = { x: number; y: number };
type Bounds = { left: number; right: number; top: number; bottom: number };
const touchSelector = ".touch-controller-overlay button, .touch-controller-overlay [data-controller-dpad], [data-touch-control], [data-touch-protected]";
function boundsFor(root: HTMLElement): Bounds {
  const viewport = window.visualViewport,
    style = getComputedStyle(root);
  const margin = (name: string) =>
    Math.max(12, parseFloat(style.getPropertyValue(name)) || 0);
  const left = viewport?.offsetLeft ?? 0,
    top = viewport?.offsetTop ?? 0;
  const right = left + (viewport?.width ?? innerWidth),
    bottom = top + (viewport?.height ?? innerHeight);
  const toolbar = root
    .querySelector(".game-fullscreen-toolbar")
    ?.getBoundingClientRect();
  const toolbarBottom =
    toolbar && toolbar.top <= top + 12 && toolbar.bottom > top
      ? toolbar.bottom + 8
      : top;
  return {
    left: left + margin("padding-left"),
    right: right - margin("padding-right"),
    top: Math.max(top + margin("padding-top"), toolbarBottom),
    bottom: bottom - margin("padding-bottom"),
  };
}
const clamp = (
  point: Point,
  bounds: Bounds,
  width: number,
  height: number,
): Point => ({
  x: Math.max(bounds.left, Math.min(point.x, bounds.right - width)),
  y: Math.max(
    Math.min(bounds.top, bounds.bottom - height),
    Math.min(point.y, bounds.bottom - height),
  ),
});

export function useChatPosition(
  toggle: RefObject<HTMLButtonElement | null>,
  panel: RefObject<HTMLElement | null>,
  target: HTMLElement | null | undefined,
  open: boolean,
) {
  const [position, setPosition] = useState<Point>(),
    [dragging, setDragging] = useState(false),
    [panelStyle, setPanelStyle] = useState<CSSProperties>(),
    [compact, setCompact] = useState(false);
  const point = useRef<Point | undefined>(undefined),
    ignoreClick = useRef(false);
  const active = useRef<
    | {
        id: number;
        start: Point;
        origin: Point;
        moved: boolean;
        element: HTMLButtonElement;
      }
    | undefined
  >(undefined);
  const finish = () => {
    const drag = active.current;
    if (!drag) return;
    active.current = undefined;
    ignoreClick.current = drag.moved || ignoreClick.current;
    setDragging(false);
    if (drag.element.hasPointerCapture(drag.id))
      drag.element.releasePointerCapture(drag.id);
  };
  const place = (next: Point) => {
    point.current = next;
    setPosition((previous) =>
      previous?.x === next.x && previous.y === next.y ? previous : next,
    );
  };
  const measure = useRef(() => {});
  measure.current = () => {
    if (!toggle.current) return;
    const surface = target ?? document.body;
    const viewport = window.visualViewport;
    if (viewport && (!(viewport.width > 0) || !(viewport.height > 0))) return;
    const bounds = boundsFor(surface),
      rect = toggle.current.getBoundingClientRect();
    const obstacles = window.matchMedia("(pointer: coarse)").matches
      ? Array.from(surface.querySelectorAll<HTMLElement>(touchSelector)).filter(element => element.getClientRects().length > 0).map(element => element.getBoundingClientRect())
      : [];
    if (!target && !obstacles.length) { setPosition(undefined); setPanelStyle(undefined); setCompact(false); return; }
    for (const picture of surface.querySelectorAll<HTMLElement>(".touch-controller-picture")) {
      if (getComputedStyle(picture).getPropertyValue("--touch-picture-gutters").trim() === "1") {
        const canvas = picture.querySelector("canvas");
        if (canvas) obstacles.push(canvas.getBoundingClientRect());
      }
    }
    const preferred = (target ? point.current : undefined) ?? (obstacles.length
      ? defaultChatAnchor(bounds, rect.width, rect.height, obstacles)
      : { x: bounds.right - rect.width, y: bounds.bottom - rect.height });
    const next = clamp(preferred, bounds, rect.width, rect.height);
    setPosition((previous) =>
      previous?.x === next.x && previous.y === next.y ? previous : next,
    );
    const anchor = {
      left: next.x,
      top: next.y,
      right: next.x + rect.width,
      bottom: next.y + rect.height,
    };
    const tight = open && bounds.bottom - bounds.top < 280;
    setCompact(tight);
    if (!open || !panel.current) return;
    const width = Math.max(1, Math.min(360, bounds.right - bounds.left));
    const above = anchor.top - bounds.top - 8,
      below = bounds.bottom - anchor.bottom - 8;
    const upward = above >= below,
      slot = upward ? above : below;
    const ideal = Math.max(1, Math.min(500, bounds.bottom - bounds.top));
    const height = tight ? ideal : Math.min(ideal, slot >= 180 ? slot : ideal);
    const left = Math.max(
      bounds.left,
      Math.min(anchor.right - width, bounds.right - width),
    );
    const top = Math.max(
      bounds.top,
      Math.min(
        upward ? anchor.top - height - 8 : anchor.bottom + 8,
        bounds.bottom - height,
      ),
    );
    setPanelStyle({
      position: "fixed",
      left,
      top,
      right: "auto",
      bottom: "auto",
      width,
      height,
      maxHeight: height,
      overflowY: height < 220 ? "auto" : undefined,
    });
  };
  useLayoutEffect(() => {
    measure.current();
  }, [target, open, position]);
  useLayoutEffect(() => {
    const resized = () => measure.current(),
      cancel = () => finish();
    const observer = new ResizeObserver(resized);
    observer.observe(target ?? document.body);
    const controls = new MutationObserver(records => {
      if (records.some(record => [...record.addedNodes, ...record.removedNodes].some(node => node instanceof Element && (node.matches(touchSelector) || node.querySelector(touchSelector))))) resized();
    });
    controls.observe(target ?? document.body, { childList: true, subtree: true });
    window.addEventListener("resize", resized);
    window.addEventListener("scroll", resized, true);
    window.addEventListener("blur", cancel);
    window.visualViewport?.addEventListener("resize", resized);
    window.visualViewport?.addEventListener("scroll", resized);
    return () => {
      observer.disconnect();
      controls.disconnect();
      window.removeEventListener("resize", resized);
      window.removeEventListener("scroll", resized, true);
      window.removeEventListener("blur", cancel);
      window.visualViewport?.removeEventListener("resize", resized);
      window.visualViewport?.removeEventListener("scroll", resized);
      finish();
    };
  }, [target]);
  const onPointerDown = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (active.current) {
      ignoreClick.current = true;
      event.preventDefault();
      event.stopPropagation();
      return;
    }
    ignoreClick.current = false;
    if (!target || (event.pointerType === "mouse" && event.button !== 0))
      return;
    const rect = event.currentTarget.getBoundingClientRect();
    active.current = {
      id: event.pointerId,
      start: { x: event.clientX, y: event.clientY },
      origin: { x: rect.x, y: rect.y },
      moved: false,
      element: event.currentTarget,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
    event.stopPropagation();
  };
  const onPointerMove = (event: ReactPointerEvent<HTMLButtonElement>) => {
    const drag = active.current;
    if (!drag || drag.id !== event.pointerId || !target) return;
    const dx = event.clientX - drag.start.x,
      dy = event.clientY - drag.start.y;
    if (!drag.moved && dx * dx + dy * dy < 36) return;
    drag.moved = true;
    setDragging(true);
    const rect = event.currentTarget.getBoundingClientRect();
    place(
      clamp(
        { x: drag.origin.x + dx, y: drag.origin.y + dy },
        boundsFor(target),
        rect.width,
        rect.height,
      ),
    );
    event.preventDefault();
    event.stopPropagation();
  };
  const end = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (active.current?.id !== event.pointerId) return;
    if (active.current.moved) event.preventDefault();
    event.stopPropagation();
    finish();
  };
  return {
    style:
      position
        ? { left: position.x, top: position.y, right: "auto", bottom: "auto" }
        : undefined,
    panelStyle: open ? panelStyle : undefined,
    compact: open && compact,
    draggable: !!target && !compact,
    dragging,
    handlers: {
      onPointerDown,
      onPointerMove,
      onPointerUp: end,
      onPointerCancel: end,
      onLostPointerCapture: end,
    },
    consumeClick: (detail: number) => {
      const consume = detail !== 0 && ignoreClick.current;
      ignoreClick.current = false;
      return consume;
    },
  };
}
