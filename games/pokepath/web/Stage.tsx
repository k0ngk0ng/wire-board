import { forwardRef, useEffect, useImperativeHandle, useRef } from "react";
import type { Assets, Point, View } from "./types";
import { interpolateBattle, paintBattle, pokemonAt, type BattlePresentation } from "./battle-renderer";
import { attachMapViewport } from "./map-viewport.js";
import type { PredictionFrame } from "./prediction";
import { siteAllowsDeployment, siteAt, type PlacementQuery, type RangeQuery } from "./placement";

export type StageHandle = { paint: (view: View, metadata?: PredictionFrame<View>) => void; focus: () => void };

export const Stage = forwardRef<StageHandle, { view: View; assets: Assets; selected?: number; placing?: boolean; allRanges?: boolean; placementPreview?: PlacementQuery; placementRange?: RangeQuery; onPointerPoint?: (point: Point) => void; onPointerCancel?: () => void; onHover?: (id?: number) => void; onPoint: (point: Point, fromController?: boolean, touch?: boolean) => void; onReady: () => void }>(function Stage({ view, assets, selected, placing = false, allRanges = false, placementPreview, placementRange, onPointerPoint, onPointerCancel, onHover, onPoint, onReady }, ref) {
  const root = useRef<HTMLDivElement>(null);
  const wrapper = useRef<HTMLDivElement>(null);
  const canvas = useRef<HTMLCanvasElement>(null);
  const presentation = useRef<BattlePresentation>({ clock: performance.now(), animations: new Map() });
  const keyboardCursor = useRef<Point | undefined>(undefined);
  const placementPointer = useRef<Point | undefined>(undefined);
  const drawCursor = (context: CanvasRenderingContext2D) => {
    const point = keyboardCursor.current;
    if (!point) return;
    context.save(); context.setTransform(1, 0, 0, 1, 0, 0); context.strokeStyle = "#fff7a4"; context.lineWidth = 2; context.beginPath(); context.arc(point.x, point.y, 10, 0, Math.PI * 2); context.moveTo(point.x - 15, point.y); context.lineTo(point.x + 15, point.y); context.moveTo(point.x, point.y - 15); context.lineTo(point.x, point.y + 15); context.stroke(); context.restore();
  };
  const last = useRef<{ view: View; metadata?: PredictionFrame<View> }>({ view });
  const current = useRef({ view, assets, selected, placing, allRanges, placementPreview, placementRange, onPointerPoint, onPointerCancel, onHover, onPoint, onReady });
  current.current = { view, assets, selected, placing, allRanges, placementPreview, placementRange, onPointerPoint, onPointerCancel, onHover, onPoint, onReady };
  const render = (context: CanvasRenderingContext2D, snapshot: View, metadata?: PredictionFrame<View>) => {
    const state = current.current;
    const visual = metadata && metadata.phase !== "reset" ? interpolateBattle(snapshot, metadata.previousView, metadata.interpolation) : snapshot;
    if (!state.placing) placementPointer.current = undefined;
    const preview = state.placing && state.selected !== undefined ? state.placementPreview?.(snapshot, state.selected) : undefined;
    const point = keyboardCursor.current ?? placementPointer.current;
    const site = preview && point ? siteAt(snapshot.tiles, point, preview.tileSize) : undefined;
    const footprint = site && state.selected !== undefined && siteAllowsDeployment(preview?.sites[site.id]) ? state.placementRange?.(snapshot, state.selected, site.id) : undefined;
    presentation.current.deployment = preview ? { ...preview, point, footprint } : undefined;
    presentation.current.allRanges = state.allRanges;
    paintBattle(context, visual, state.assets, state.selected, state.placing, presentation.current);
    drawCursor(context);
  };
  useImperativeHandle(ref, () => ({
    paint(next, metadata) {
      last.current = { view: next, metadata };
      const context = canvas.current?.getContext("2d");
      if (!context) return;
      if (metadata?.phase === "reset") presentation.current.resetEvents = true;
      presentation.current.clock = Math.max(presentation.current.clock, metadata?.nativeAt ?? performance.now());
      render(context, next, metadata);
    },
    focus() {
      canvas.current?.focus({ preventScroll: true });
      wrapper.current?.scrollIntoView({ block: "nearest", behavior: "smooth" });
    },
  }), []);
  useEffect(() => {
    if (!root.current || !canvas.current || !wrapper.current) return;
    const controller = attachMapViewport({ canvas: canvas.current, wrapper: wrapper.current, initialMode: "fill" });
    const cursorPoint = () => {
      if (keyboardCursor.current) return keyboardCursor.current;
      const state = current.current;
      const value = last.current.view;
      const tower = value.towers.find(tower => tower.pokemonID === state.selected);
      const preview = state.placing && state.selected !== undefined ? state.placementPreview?.(value, state.selected) : undefined;
      const tile = preview ? value.tiles.find(site => siteAllowsDeployment(preview.sites[site.id])) : value.tiles.find(site => !value.towers.some(tower => tower.tileID === site.id));
      const offset = (preview?.tileSize ?? 24) / 2;
      return tower ? { x: tower.x, y: tower.y } : tile ? { x: tile.x + offset, y: tile.y + offset } : { x: canvas.current!.width / 2, y: canvas.current!.height / 2 };
    };
    const reveal = () => {
      const point = keyboardCursor.current;
      if (!point) return;
      const client = controller.pointToClient(point.x, point.y);
      const element = wrapper.current!;
      const bounds = element.getBoundingClientRect();
      const ratio = { x: bounds.width / element.clientWidth, y: bounds.height / element.clientHeight };
      element.scrollLeft += (client.x < bounds.left + 24 ? client.x - bounds.left - 24 : client.x > bounds.right - 24 ? client.x - bounds.right + 24 : 0) / ratio.x;
      element.scrollTop += (client.y < bounds.top + 24 ? client.y - bounds.top - 24 : client.y > bounds.bottom - 24 ? client.y - bounds.bottom + 24 : 0) / ratio.y;
    };
    const redraw = () => {
      const context = canvas.current?.getContext("2d");
      if (!context) return;
      const { view, metadata } = last.current;
      render(context, view, metadata);
    };
    const setCursor = (point: Point) => {
      keyboardCursor.current = point;
      canvas.current!.dataset.controllerX = String(point.x);
      canvas.current!.dataset.controllerY = String(point.y);
      current.current.onHover?.(pokemonAt(last.current.view, point)?.id);
      reveal(); redraw();
    };
    const capture = (event: Event) => {
      if (event.target === canvas.current && event.type === "controller-direction") {
        const direction = (event as CustomEvent).detail?.direction;
        const offset = ({ navUp: [0, -24], navDown: [0, 24], navLeft: [-24, 0], navRight: [24, 0] } as Record<string, number[]>)[direction];
        if (!offset) return;
        event.preventDefault(); current.current.onPointerCancel?.();
        const point = cursorPoint();
        setCursor({ x: Math.max(12, Math.min(canvas.current!.width - 12, point.x + offset[0])), y: Math.max(12, Math.min(canvas.current!.height - 12, point.y + offset[1])) });
        return;
      }
      if (event.target === canvas.current && event.type === "controller-reveal") { event.preventDefault(); setCursor(cursorPoint()); return; }
      if (["pointerdown", "pointermove", "wheel"].includes(event.type) && keyboardCursor.current) { keyboardCursor.current = undefined; delete canvas.current!.dataset.controllerX; delete canvas.current!.dataset.controllerY; redraw(); }

      if (["pointerdown", "pointercancel", "lostpointercapture", "pointerleave", "wheel"].includes(event.type)) current.current.onPointerCancel?.();
      if (["pointercancel", "lostpointercapture", "pointerleave", "wheel"].includes(event.type) && placementPointer.current) { placementPointer.current = undefined; redraw(); }
      if (controller.handleEvent(event)) return;
      if (event.type === "pointermove" && event.target === canvas.current) {
        const pointer = event as PointerEvent;
        const point = controller.logicalPoint(pointer.clientX, pointer.clientY);
        if (pointer.pointerType === "mouse" && pointer.buttons === 0) {
          current.current.onPointerPoint?.(point);
          if (current.current.placing) { placementPointer.current = point; redraw(); }
        }
        current.current.onHover?.(pokemonAt(last.current.view, point)?.id);
      }
      if (event.type === "pointerleave") current.current.onHover?.();
      if (event.type === "click" && event.target === canvas.current) {
        const pointer = event as PointerEvent;
        const fromController = pointer.detail === 0;
        current.current.onPoint(fromController ? cursorPoint() : controller.logicalPoint(pointer.clientX, pointer.clientY), fromController, pointer.pointerType === "touch");
      }
    };
    const element = root.current;
    const events = ["controller-direction", "controller-reveal", "click", "pointerdown", "pointermove", "pointerup", "pointercancel", "lostpointercapture", "pointerleave", "wheel"];
    for (const name of events) element.addEventListener(name, capture, { capture: true, passive: false });
    controller.update();
    return () => {
      for (const name of events) element.removeEventListener(name, capture, true);
      controller.destroy();
    };
  }, []);
  useEffect(() => {
    const node = canvas.current;
    const context = node?.getContext("2d");
    if (!node || !context) return;
    const width = assets.route?.width ?? 720;
    const height = assets.route?.height ?? 624;
    if (node.width !== width) node.width = width;
    if (node.height !== height) node.height = height;
    if (last.current.view.epoch !== view.epoch || last.current.view.route !== view.route || last.current.view.clock < view.clock) last.current = { view };
    const latest = last.current;
    presentation.current.clock = Math.max(presentation.current.clock, performance.now());
    render(context, latest.view, latest.metadata);
    if (assets.ready) {
      const frame = requestAnimationFrame(() => current.current.onReady());
      return () => cancelAnimationFrame(frame);
    }
  }, [view, assets, selected, placing, allRanges]);
  return <div className="pokepath-stage" ref={root}>
    <div className="pokepath-camera" data-controller-group="board" ref={wrapper}><canvas ref={canvas} width={assets.route?.width ?? 720} height={assets.route?.height ?? 624} aria-label="巡径守卫战场地图" role="button" tabIndex={0} data-pokepath-canvas /></div>
    <div className="pokepath-map-tools" data-game-controls>
      <button type="button" data-map-zoom-out aria-label="缩小地图">−</button>
      <button type="button" data-map-zoom-in aria-label="放大地图">＋</button>
      <button type="button" data-map-fit aria-label="查看完整地图">全图</button>
      <button type="button" data-map-reset aria-label="恢复原始比例">原寸</button>
    </div>
  </div>;
});
