import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import type { PointerEvent as ReactPointerEvent } from "react";

export function useRailMapControls({
  aspect = 1744 / 1125,
  minMobileWidth = 620,
} = {}) {
  const viewport = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ width: 0, height: 0, mobile: false });
  const [zoom, setZoom] = useState(1);
  const [dragging, setDragging] = useState(false);
  const drag = useRef<{
    id: number;
    x: number;
    y: number;
    left: number;
    top: number;
  } | null>(null);
  const suppressClick = useRef(false);
  const pending = useRef<{ x: number; y: number; u: number; v: number } | null>(
    null,
  );
  const baseWidth = size.mobile
    ? Math.max(minMobileWidth, size.width)
    : Math.min(size.width, size.height * aspect);
  const width = baseWidth * zoom;
  const height = width / aspect;
  const left = Math.max(0, (size.width - width) / 2);
  const top = Math.max(0, (size.height - height) / 2);

  useLayoutEffect(() => {
    const el = viewport.current;
    if (!el) return;
    const observer = new ResizeObserver(() => {
      setSize({
        width: el.clientWidth,
        height: el.clientHeight,
        mobile: window.innerWidth <= 850,
      });
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const zoomAt = useCallback(
    (next: number, x?: number, y?: number) => {
      const el = viewport.current;
      if (!el || !width) return;
      next = Math.min(3, Math.max(1, next));
      if (next === zoom) return;
      x ??= el.clientWidth / 2;
      y ??= el.clientHeight / 2;
      pending.current = {
        x,
        y,
        u: (el.scrollLeft + x - left) / width,
        v: (el.scrollTop + y - top) / height,
      };
      setZoom(next);
    },
    [zoom, width, height, left, top],
  );

  useLayoutEffect(() => {
    const el = viewport.current,
      anchor = pending.current;
    if (el && anchor) {
      el.scrollLeft = anchor.u * width + left - anchor.x;
      el.scrollTop = anchor.v * height + top - anchor.y;
      pending.current = null;
    }
  }, [width, height, left, top]);

  useEffect(() => {
    const el = viewport.current;
    if (!el) return;
    const wheel = (event: WheelEvent) => {
      event.preventDefault();
      const rect = el.getBoundingClientRect();
      const delta =
        event.deltaY *
        (event.deltaMode === 1
          ? 16
          : event.deltaMode === 2
            ? el.clientHeight
            : 1);
      zoomAt(
        zoom * Math.exp(-Math.max(-300, Math.min(300, delta)) * 0.002),
        event.clientX - rect.left,
        event.clientY - rect.top,
      );
    };
    el.addEventListener("wheel", wheel, { passive: false });
    return () => el.removeEventListener("wheel", wheel);
  }, [zoom, zoomAt]);

  const endDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (drag.current?.id !== event.pointerId) return;
    drag.current = null;
    setDragging(false);
    if (event.currentTarget.hasPointerCapture(event.pointerId))
      event.currentTarget.releasePointerCapture(event.pointerId);
  };

  return {
    viewport,
    zoom,
    zoomAt,
    dragging,
    canvasStyle: {
      width: Math.max(size.width, width),
      height: Math.max(size.height, height),
      position: "relative" as const,
    },
    mapStyle: {
      width,
      height,
      left,
      top,
      position: "absolute" as const,
      minWidth: 0,
      minHeight: 0,
    },
    handlers: {
      onPointerDown(event: ReactPointerEvent<HTMLDivElement>) {
        suppressClick.current = false;
        if (event.pointerType !== "mouse" || event.button !== 0) return;
        drag.current = {
          id: event.pointerId,
          x: event.clientX,
          y: event.clientY,
          left: event.currentTarget.scrollLeft,
          top: event.currentTarget.scrollTop,
        };
      },
      onPointerMove(event: ReactPointerEvent<HTMLDivElement>) {
        const start = drag.current;
        if (!start || start.id !== event.pointerId) return;
        if (!event.buttons) {
          endDrag(event);
          return;
        }
        const dx = event.clientX - start.x,
          dy = event.clientY - start.y;
        if (!suppressClick.current && Math.hypot(dx, dy) < 5) return;
        suppressClick.current = true;
        setDragging(true);
        event.currentTarget.setPointerCapture(event.pointerId);
        event.currentTarget.scrollLeft = start.left - dx;
        event.currentTarget.scrollTop = start.top - dy;
        event.preventDefault();
      },
      onPointerUp: endDrag,
      onPointerCancel: endDrag,
      onLostPointerCapture: endDrag,
      onPointerLeave(event: ReactPointerEvent<HTMLDivElement>) {
        if (!event.currentTarget.hasPointerCapture(event.pointerId))
          endDrag(event);
      },
      onClickCapture(event: React.MouseEvent<HTMLDivElement>) {
        if (suppressClick.current && event.detail !== 0) {
          event.preventDefault();
          event.stopPropagation();
          suppressClick.current = false;
        }
      },
      onDragStart(event: React.DragEvent<HTMLDivElement>) {
        event.preventDefault();
      },
    },
  };
}
