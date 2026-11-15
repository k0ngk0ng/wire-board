import { useLayoutEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import type { MapData } from "./types";
import { worldToScreen } from "./camera";
import type { Camera, Viewport } from "./camera";
import { noticePosition } from "./wave-notices";

export function MapNotices({ map, camera, viewport, children }: {
  map: MapData; camera: Camera; viewport: Viewport; children: ReactNode;
}) {
  const node = useRef<HTMLDivElement>(null);
  const [measure, setMeasure] = useState({ width: 0, height: 0, avoid: [] as { left: number; top: number; right: number; bottom: number }[] });
  useLayoutEffect(() => {
    const element = node.current, field = element?.closest(".sf-battlefield"), board = element?.closest(".starfort-board");
    if (!element || !field || !board) return;
    const controls = [...board.querySelectorAll(".sf-topbar, .sf-map-navigation")];
    const update = () => {
      const origin = field.getBoundingClientRect(), size = element.getBoundingClientRect();
      const avoid = controls.map((control) => { const r = control.getBoundingClientRect(); return { left: r.left - origin.left - 6, right: r.right - origin.left + 6, top: r.top - origin.top - 6, bottom: r.bottom - origin.top + 6 }; });
      const next = { width: size.width, height: size.height, avoid };
      setMeasure((previous) => JSON.stringify(previous) === JSON.stringify(next) ? previous : next);
    };
    const observer = new ResizeObserver(update);
    for (const target of [element, field, ...controls]) observer.observe(target);
    update();
    return () => observer.disconnect();
  }, [children, viewport.width, viewport.height]);
  const paths = map.routes?.map((route) => route.points) ?? [map.path];
  const roads = paths.map((path) => path.map((point) => worldToScreen(point, camera, viewport)));
  const position = noticePosition(viewport, measure, measure.avoid, roads, Math.max(8, 34 * camera.zoom));
  return <div ref={node} className="sf-wave-notices" data-sf-notices style={{ ...position, visibility: measure.width && viewport.width ? "visible" : "hidden" }}>{children}</div>;
}
