import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
} from "react";
import {
  CAMERA_MAX_RATIO,
  fitCamera,
  focusCamera,
  locateCamera,
  mapPoint,
  panCamera,
  pinchCamera,
  zoomCamera,
  visibleCameraRect,
  revealCamera,
  restoreCamera,
  type Camera,
  type Point,
  type Size,
} from "./camera";

export type CameraFrame = { camera: Camera; size: Size; ratio: number };

// The geographical scene is not React-rendered for every input event. A frame
// changes only its transform, the overlay's relative transform and minimap rect.
// React receives the settled camera once the gesture/wheel sequence ends.
// Attach sceneRef to world-coordinate geometry and overlayRef to labels computed
// with `camera` (the settled snapshot). Nested label glyphs can counter-scale via
// --cq-gesture-inverse-scale. `getCamera()` reads the live frame for hit geometry.
// Mark embedded controls/minimaps data-camera-control; call settle() when their
// drag ends. subscribeFrame is for imperative DOM consumers, not per-frame state.
export function useCamera(map: Size) {
  const viewport = useRef<HTMLDivElement>(null);
  const sceneRef = useRef<SVGGElement>(null);
  const overlayRef = useRef<SVGGElement>(null);
  const miniViewportRef = useRef<SVGRectElement>(null);
  const [size, setSize] = useState<Size>({ width: 1, height: 1 });
  const [camera, setCamera] = useState<Camera>({ x: 0, y: 0, scale: 1 });
  const current = useRef(camera),
    settled = useRef(camera),
    dimensions = useRef(size);
  const frame = useRef<number | null>(null),
    wheelIdle = useRef<number | null>(null);
  const commitPending = useRef(false),
    alive = useRef(true),
    paints = useRef(0);
  const points = useRef(new Map<number, Point>());
  const touchIds = useRef(new Set<number>());
  const pointerId = useRef<number | null>(null);
  const anchor = useRef<{ camera: Camera; points: Point[] } | null>(null);
  const blockedClick = useRef(false);
  const listeners = useRef(new Set<(value: CameraFrame) => void>());

  const paint = useCallback(() => {
    const c = current.current,
      base = settled.current,
      d = dimensions.current;
    sceneRef.current?.setAttribute(
      "transform",
      `translate(${c.x} ${c.y}) scale(${c.scale})`,
    );
    const relative = c.scale / base.scale;
    if (overlayRef.current) {
      overlayRef.current.setAttribute(
        "transform",
        `translate(${c.x - base.x * relative} ${c.y - base.y * relative}) scale(${relative})`,
      );
      overlayRef.current.style.setProperty(
        "--cq-gesture-scale",
        String(relative),
      );
      overlayRef.current.style.setProperty(
        "--cq-gesture-inverse-scale",
        String(1 / relative),
      );
    }
    const mini = miniViewportRef.current;
    if (mini) {
      const visible = visibleCameraRect(c, map, d);
      mini.setAttribute("x", String(visible.x));
      mini.setAttribute("y", String(visible.y));
      mini.setAttribute("width", String(visible.width));
      mini.setAttribute("height", String(visible.height));
    }
    if (viewport.current) {
      viewport.current.dataset.cameraX = String(c.x);
      viewport.current.dataset.cameraY = String(c.y);
      viewport.current.dataset.cameraScale = String(c.scale);
      viewport.current.dataset.cameraPaints = String(++paints.current);
    }
    const value = {
      camera: { ...c },
      size: { ...d },
      ratio: c.scale / fitCamera(map, d).scale,
    };
    for (const listener of listeners.current) listener(value);
  }, [map.width, map.height]);

  const schedule = useCallback(
    (commit = false) => {
      commitPending.current ||= commit;
      if (frame.current !== null) return;
      frame.current = requestAnimationFrame(() => {
        frame.current = null;
        if (!alive.current) return;
        paint();
        if (commitPending.current) {
          commitPending.current = false;
          const c = current.current;
          setCamera((previous) =>
            previous.x === c.x &&
            previous.y === c.y &&
            previous.scale === c.scale
              ? previous
              : { ...c },
          );
        }
      });
    },
    [paint],
  );
  const stopWheel = useCallback(() => {
    if (wheelIdle.current !== null) window.clearTimeout(wheelIdle.current);
    wheelIdle.current = null;
  }, []);
  const update = useCallback(
    (next: Camera, commit = true) => {
      current.current = next;
      schedule(commit);
    },
    [schedule],
  );
  const reset = useCallback(() => {
    stopWheel();
    update(fitCamera(map, dimensions.current));
  }, [map.width, map.height, update, stopWheel]);
  const focus = useCallback(
    (point: Point) => {
      stopWheel();
      update(focusCamera(current.current, point, map, dimensions.current));
    },
    [map.width, map.height, update, stopWheel],
  );
  const zoom = useCallback(
    (factor: number, point?: Point) => {
      stopWheel();
      update(
        zoomCamera(
          current.current,
          factor,
          point ?? {
            x: dimensions.current.width / 2,
            y: dimensions.current.height / 2,
          },
          map,
          dimensions.current,
        ),
      );
    },
    [map.width, map.height, update, stopWheel],
  );
  const reveal = useCallback(
    (points: Point[]) => {
      stopWheel();
      update(revealCamera(current.current, points, map, dimensions.current));
    },
    [map.width, map.height, update, stopWheel],
  );
  const restore = useCallback(
    (saved: Camera, previous: Size) => {
      stopWheel();
      update(restoreCamera(saved, previous, map, dimensions.current));
    },
    [map.width, map.height, update, stopWheel],
  );
  const locate = useCallback(
    (point: Point) => {
      stopWheel();
      update(
        locateCamera(current.current, point, map, dimensions.current),
        false,
      );
      wheelIdle.current = window.setTimeout(() => {
        wheelIdle.current = null;
        schedule(true);
      }, 120);
    },
    [map.width, map.height, update, stopWheel, schedule],
  );
  const settle = useCallback(() => {
    stopWheel();
    schedule(true);
  }, [schedule, stopWheel]);
  const getCamera = useCallback(() => ({ ...current.current }), []);
  const subscribeFrame = useCallback(
    (listener: (value: CameraFrame) => void) => {
      listeners.current.add(listener);
      listener({
        camera: { ...current.current },
        size: { ...dimensions.current },
        ratio: current.current.scale / fitCamera(map, dimensions.current).scale,
      });
      return () => {
        listeners.current.delete(listener);
      };
    },
    [map.width, map.height],
  );

  useLayoutEffect(() => {
    // React has now redrawn overlay coordinates using this settled camera.
    settled.current = camera;
    paint();
  }, [camera, size, paint]);
  useEffect(() => {
    alive.current = true;
    const element = viewport.current;
    if (!element) return;
    let initialized = false;
    const observer = new ResizeObserver(() => {
      const next = { width: element.clientWidth, height: element.clientHeight };
      if (next.width < 1 || next.height < 1) return;
      const prior = dimensions.current;
      if (
        initialized &&
        next.width === prior.width &&
        next.height === prior.height
      )
        return;
      const centre = mapPoint(current.current, {
        x: prior.width / 2,
        y: prior.height / 2,
      });
      const oldFit = fitCamera(map, prior),
        nextFit = fitCamera(map, next);
      dimensions.current = next;
      setSize(next);
      update(
        initialized
          ? locateCamera(
              {
                ...current.current,
                scale: (nextFit.scale * current.current.scale) / oldFit.scale,
              },
              centre,
              map,
              next,
            )
          : nextFit,
      );
      initialized = true;
    });
    observer.observe(element);
    return () => {
      alive.current = false;
      observer.disconnect();
      stopWheel();
      if (frame.current !== null) cancelAnimationFrame(frame.current);
      frame.current = null;
      commitPending.current = false;
    };
  }, [map.width, map.height, update, stopWheel]);

  const reanchor = useCallback(() => {
    anchor.current = points.current.size
      ? { camera: { ...current.current }, points: [...points.current.values()] }
      : null;
  }, []);
  const begin = useCallback(() => {
    stopWheel();
    commitPending.current = false;
    if (!points.current.size) blockedClick.current = false;
  }, [stopWheel]);
  const advance = useCallback(() => {
    const saved = anchor.current,
      latest = [...points.current.values()];
    if (!saved || !latest.length) return false;
    const dx = latest[0].x - saved.points[0].x,
      dy = latest[0].y - saved.points[0].y;
    if (latest.length < 2 && !blockedClick.current && Math.hypot(dx, dy) < 5)
      return false;
    blockedClick.current = true;
    update(
      latest.length >= 2 && saved.points.length >= 2
        ? pinchCamera(
            saved.camera,
            { a: saved.points[0], b: saved.points[1] },
            { a: latest[0], b: latest[1] },
            map,
            dimensions.current,
          )
        : panCamera(saved.camera, dx, dy, map, dimensions.current),
      false,
    );
    return true;
  }, [map.width, map.height, update]);
  const finishPointer = useCallback(
    (id: number) => {
      if (pointerId.current !== id) return;
      pointerId.current = null;
      points.current.clear();
      reanchor();
      const element = viewport.current;
      if (element?.hasPointerCapture(id)) element.releasePointerCapture(id);
      schedule(true);
    },
    [reanchor, schedule],
  );

  useEffect(() => {
    const element = viewport.current;
    if (!element) return;
    const local = (x: number, y: number) => {
      const r = element.getBoundingClientRect();
      return { x: x - r.left, y: y - r.top };
    };
    const syncTouches = (event: TouchEvent) => {
      points.current.clear();
      for (const touch of Array.from(event.touches))
        if (touchIds.current.has(touch.identifier))
          points.current.set(
            touch.identifier,
            local(touch.clientX, touch.clientY),
          );
    };
    const touchStart = (event: TouchEvent) => {
      // TouchEvent propagation is separate from React's pointer propagation.
      // Embedded minimaps and controls own their native touch sequences.
      if ((event.target as Element | null)?.closest("[data-camera-control]"))
        return;
      if (pointerId.current !== null) finishPointer(pointerId.current);
      begin();
      for (const t of Array.from(event.changedTouches))
        touchIds.current.add(t.identifier);
      syncTouches(event);
      reanchor();
      if (points.current.size > 1) {
        blockedClick.current = true;
        if (event.cancelable) event.preventDefault();
      }
    };
    const touchMove = (event: TouchEvent) => {
      if (!touchIds.current.size) return;
      syncTouches(event);
      if (advance() && event.cancelable) event.preventDefault();
    };
    const touchEnd = (event: TouchEvent) => {
      if (!touchIds.current.size) return;
      if (event.type === "touchcancel") blockedClick.current = true;
      for (const t of Array.from(event.changedTouches))
        touchIds.current.delete(t.identifier);
      syncTouches(event);
      reanchor();
      if (blockedClick.current && event.cancelable) event.preventDefault();
      if (!points.current.size) schedule(true);
    };
    const wheel = (event: WheelEvent) => {
      if (points.current.size) return;
      event.preventDefault();
      const r = element.getBoundingClientRect();
      const delta =
        event.deltaY *
        (event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? r.height : 1);
      update(
        zoomCamera(
          current.current,
          Math.exp(-Math.max(-400, Math.min(400, delta)) * 0.002),
          { x: event.clientX - r.left, y: event.clientY - r.top },
          map,
          dimensions.current,
        ),
        false,
      );
      stopWheel();
      wheelIdle.current = window.setTimeout(() => {
        wheelIdle.current = null;
        schedule(true);
      }, 120);
    };
    const release = () => {
      stopWheel();
      const id = pointerId.current;
      pointerId.current = null;
      if (id !== null && element.hasPointerCapture(id))
        element.releasePointerCapture(id);
      points.current.clear();
      touchIds.current.clear();
      anchor.current = null;
      blockedClick.current = true;
      schedule(true);
    };
    const visibility = () => {
      if (document.hidden) release();
    };
    const pointerUp = (event: PointerEvent) => {
      if (event.pointerType !== "touch") finishPointer(event.pointerId);
    };
    element.addEventListener("touchstart", touchStart, { passive: false });
    element.addEventListener("touchmove", touchMove, { passive: false });
    element.addEventListener("touchend", touchEnd, { passive: false });
    element.addEventListener("touchcancel", touchEnd, { passive: false });
    element.addEventListener("wheel", wheel, { passive: false });
    window.addEventListener("pointerup", pointerUp);
    window.addEventListener("pointercancel", pointerUp);
    window.addEventListener("blur", release);
    document.addEventListener("visibilitychange", visibility);
    return () => {
      element.removeEventListener("touchstart", touchStart);
      element.removeEventListener("touchmove", touchMove);
      element.removeEventListener("touchend", touchEnd);
      element.removeEventListener("touchcancel", touchEnd);
      element.removeEventListener("wheel", wheel);
      window.removeEventListener("pointerup", pointerUp);
      window.removeEventListener("pointercancel", pointerUp);
      window.removeEventListener("blur", release);
      document.removeEventListener("visibilitychange", visibility);
      stopWheel();
      const id = pointerId.current;
      pointerId.current = null;
      if (id !== null && element.hasPointerCapture(id))
        element.releasePointerCapture(id);
      points.current.clear();
      touchIds.current.clear();
      anchor.current = null;
    };
  }, [
    advance,
    begin,
    finishPointer,
    map.width,
    map.height,
    reanchor,
    schedule,
    stopWheel,
    update,
  ]);

  const local = (event: ReactPointerEvent<HTMLDivElement>) => {
    const r = event.currentTarget.getBoundingClientRect();
    return { x: event.clientX - r.left, y: event.clientY - r.top };
  };
  return {
    viewport,
    sceneRef,
    overlayRef,
    miniViewportRef,
    camera,
    size,
    reset,
    focus,
    reveal,
    restore,
    zoom,
    locate,
    settle,
    getCamera,
    subscribeFrame,
    maxRatio: CAMERA_MAX_RATIO,
    canClick: (detail: number) => detail === 0 || !blockedClick.current,
    ratio: camera.scale / fitCamera(map, size).scale,
    handlers: {
      onPointerDown(event: ReactPointerEvent<HTMLDivElement>) {
        if (
          event.pointerType === "touch" ||
          touchIds.current.size ||
          (event.pointerType === "mouse" && event.button !== 0)
        )
          return;
        begin();
        pointerId.current = event.pointerId;
        points.current.set(event.pointerId, local(event));
        reanchor();
      },
      onPointerMove(event: ReactPointerEvent<HTMLDivElement>) {
        if (
          event.pointerType === "touch" ||
          pointerId.current !== event.pointerId
        )
          return;
        if (event.pointerType === "mouse" && event.buttons === 0) {
          finishPointer(event.pointerId);
          return;
        }
        points.current.set(event.pointerId, local(event));
        if (advance()) {
          event.currentTarget.setPointerCapture(event.pointerId);
          event.preventDefault();
        }
      },
      onPointerUp(event: ReactPointerEvent<HTMLDivElement>) {
        if (event.pointerType !== "touch") finishPointer(event.pointerId);
      },
      onPointerCancel(event: ReactPointerEvent<HTMLDivElement>) {
        if (event.pointerType !== "touch") {
          blockedClick.current = true;
          finishPointer(event.pointerId);
        }
      },
      onLostPointerCapture(event: ReactPointerEvent<HTMLDivElement>) {
        // A child's implicit capture is lost when this viewport takes over.
        // The bubbling child event must not remove the still-active pointer.
        if (
          event.pointerType !== "touch" &&
          event.target === event.currentTarget &&
          !event.currentTarget.hasPointerCapture(event.pointerId)
        )
          finishPointer(event.pointerId);
      },
      onDragStart(event: React.DragEvent<HTMLDivElement>) {
        event.preventDefault();
      },
    },
  };
}
