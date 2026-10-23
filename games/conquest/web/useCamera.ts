import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
} from "react";
import {
  fitCamera,
  locateCamera,
  mapPoint,
  panCamera,
  pinchCamera,
  zoomCamera,
  type Camera,
  type Point,
  type Size,
} from "./camera";

export function useCamera(map: Size) {
  const viewport = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState<Size>({ width: 1, height: 1 });
  const [camera, setCamera] = useState<Camera>({ x: 0, y: 0, scale: 1 });
  const current = useRef(camera),
    dimensions = useRef(size);
  const points = useRef(new Map<number, Point>());
  const anchor = useRef<{ camera: Camera; points: Point[] } | null>(null);
  const blockedClick = useRef(false);
  const update = useCallback((next: Camera) => {
    current.current = next;
    setCamera(next);
  }, []);
  const reset = useCallback(
    () => update(fitCamera(map, dimensions.current)),
    [map.width, map.height, update],
  );
  const focus = useCallback(
    (point: Point) => {
      const fit = fitCamera(map, dimensions.current);
      const c =
        current.current.scale < fit.scale * 2.5
          ? { ...current.current, scale: fit.scale * 2.5 }
          : current.current;
      update(locateCamera(c, point, map, dimensions.current));
    },
    [map.width, map.height, update],
  );
  const zoom = useCallback(
    (factor: number, point?: Point) =>
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
      ),
    [map.width, map.height, update],
  );
  const locate = useCallback(
    (point: Point) =>
      update(locateCamera(current.current, point, map, dimensions.current)),
    [map.width, map.height, update],
  );
  useEffect(() => {
    const element = viewport.current;
    if (!element) return;
    let initialized = false;
    const observer = new ResizeObserver(() => {
      const next = { width: element.clientWidth, height: element.clientHeight };
      if (next.width < 1 || next.height < 1) return;
      const prior = dimensions.current,
        centre = mapPoint(current.current, {
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
    return () => observer.disconnect();
  }, [map.width, map.height, update]);
  useEffect(() => {
    const element = viewport.current;
    if (!element) return;
    function wheel(event: WheelEvent) {
      event.preventDefault();
      const bounds = element!.getBoundingClientRect();
      const delta =
        event.deltaY *
        (event.deltaMode === 1
          ? 16
          : event.deltaMode === 2
            ? bounds.height
            : 1);
      zoom(Math.exp(-Math.max(-400, Math.min(400, delta)) * 0.002), {
        x: event.clientX - bounds.left,
        y: event.clientY - bounds.top,
      });
    }
    const release = () => {
      points.current.clear();
      anchor.current = null;
      blockedClick.current = true;
    };
    const visibility = () => {
      if (document.hidden) release();
    };
    element.addEventListener("wheel", wheel, { passive: false });
    window.addEventListener("blur", release);
    document.addEventListener("visibilitychange", visibility);
    return () => {
      element.removeEventListener("wheel", wheel);
      window.removeEventListener("blur", release);
      document.removeEventListener("visibilitychange", visibility);
      release();
    };
  }, [zoom]);
  const local = (event: ReactPointerEvent<HTMLDivElement>) => {
    const r = event.currentTarget.getBoundingClientRect();
    return { x: event.clientX - r.left, y: event.clientY - r.top };
  };
  const reanchor = () => {
    anchor.current = points.current.size
      ? { camera: current.current, points: [...points.current.values()] }
      : null;
  };
  const finish = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!points.current.delete(event.pointerId)) return;
    if (event.currentTarget.hasPointerCapture(event.pointerId))
      event.currentTarget.releasePointerCapture(event.pointerId);
    reanchor();
  };
  return {
    viewport,
    camera,
    size,
    reset,
    focus,
    zoom,
    locate,
    canClick: (detail: number) => detail === 0 || !blockedClick.current,
    ratio: camera.scale / fitCamera(map, size).scale,
    handlers: {
      onPointerDown(event: ReactPointerEvent<HTMLDivElement>) {
        if (event.pointerType === "mouse" && event.button !== 0) return;
        if (points.current.size === 0) blockedClick.current = false;
        points.current.set(event.pointerId, local(event));
        if (points.current.size > 1) blockedClick.current = true;
        reanchor();
      },
      onPointerMove(event: ReactPointerEvent<HTMLDivElement>) {
        if (!points.current.has(event.pointerId)) return;
        if (event.pointerType === "mouse" && event.buttons === 0) {
          finish(event);
          return;
        }
        points.current.set(event.pointerId, local(event));
        const saved = anchor.current,
          latest = [...points.current.values()];
        if (!saved) return;
        const dx = latest[0].x - saved.points[0].x,
          dy = latest[0].y - saved.points[0].y;
        if (
          latest.length < 2 &&
          !blockedClick.current &&
          Math.hypot(dx, dy) < 5
        )
          return;
        blockedClick.current = true;
        event.currentTarget.setPointerCapture(event.pointerId);
        event.preventDefault();
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
        );
      },
      onPointerUp: finish,
      onPointerCancel(event: ReactPointerEvent<HTMLDivElement>) {
        blockedClick.current = true;
        finish(event);
      },
      onLostPointerCapture: finish,
      onPointerLeave(event: ReactPointerEvent<HTMLDivElement>) {
        if (!event.currentTarget.hasPointerCapture(event.pointerId))
          finish(event);
      },
      onDragStart(event: React.DragEvent<HTMLDivElement>) {
        event.preventDefault();
      },
    },
  };
}
