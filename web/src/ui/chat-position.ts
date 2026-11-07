import {
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type PointerEvent as ReactPointerEvent,
  type RefObject,
} from "react";

type Point = { x: number; y: number };
type Bounds = { left: number; right: number; top: number; bottom: number };
function boundsFor(root: HTMLElement): Bounds {
  const rect = root.getBoundingClientRect(),
    viewport = window.visualViewport,
    style = getComputedStyle(root);
  const margin = (name: string) =>
    Math.max(12, parseFloat(style.getPropertyValue(name)) || 0);
  const left = viewport?.offsetLeft ?? 0,
    top = viewport?.offsetTop ?? 0;
  const toolbar = root
    .querySelector(".game-fullscreen-toolbar")
    ?.getBoundingClientRect();
  return {
    left: Math.max(rect.left, left) + margin("padding-left"),
    right:
      Math.min(rect.right, left + (viewport?.width ?? innerWidth)) -
      margin("padding-right"),
    top: Math.max(
      rect.top + margin("padding-top"),
      top + 12,
      toolbar ? toolbar.bottom + 8 : 0,
    ),
    bottom:
      Math.min(rect.bottom, top + (viewport?.height ?? innerHeight)) -
      margin("padding-bottom"),
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
    [panelStyle, setPanelStyle] = useState<CSSProperties>();
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
    if (!target || !toggle.current) return;
    const bounds = boundsFor(target),
      rect = toggle.current.getBoundingClientRect();
    let anchor = {
      left: rect.left,
      top: rect.top,
      right: rect.right,
      bottom: rect.bottom,
    };
    if (point.current) {
      const next = clamp(point.current, bounds, rect.width, rect.height);
      place(next);
      anchor = {
        left: next.x,
        top: next.y,
        right: next.x + rect.width,
        bottom: next.y + rect.height,
      };
    }
    if (!open || !panel.current) return;
    const width = Math.max(1, Math.min(360, bounds.right - bounds.left));
    const above = anchor.top - bounds.top - 8,
      below = bounds.bottom - anchor.bottom - 8;
    const upward = above >= below,
      slot = upward ? above : below;
    const ideal = Math.max(1, Math.min(500, bounds.bottom - bounds.top));
    const height = Math.min(ideal, slot >= 220 ? slot : ideal);
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
    if (!target) return;
    const resized = () => measure.current(),
      cancel = () => finish();
    const observer = new ResizeObserver(resized);
    observer.observe(target);
    window.addEventListener("resize", resized);
    window.addEventListener("blur", cancel);
    window.visualViewport?.addEventListener("resize", resized);
    window.visualViewport?.addEventListener("scroll", resized);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", resized);
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
      target && position
        ? { left: position.x, top: position.y, right: "auto", bottom: "auto" }
        : undefined,
    panelStyle: target && open ? panelStyle : undefined,
    draggable: !!target,
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
