// Real hook and real map geometry, kept out of production modules/builds.
import React, { Profiler, useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { useCamera } from "../web/useCamera";
import { viewPoint, mapPoint, locateCamera } from "../web/camera";
import world from "../maps/world.json";

const counters = {
  renders: 0,
  commits: 0,
  clicks: 0,
  pointerMoves: 0,
  touchMoves: 0,
  presses: [] as {
    pointerType: string;
    tag: string;
    region: string | null;
    label: string | null;
    trusted: boolean;
  }[],
  frames: [] as number[],
  miniExpected: null as { x: number; y: number; scale: number } | null,
};
(window as any).cameraEvidence = counters;

function MapFixture() {
  counters.renders++;
  const camera = useCamera(world) as ReturnType<typeof useCamera> & {
    sceneRef?: React.RefObject<SVGGElement | null>;
    overlayRef?: React.RefObject<SVGGElement | null>;
    miniViewportRef?: React.RefObject<SVGRectElement | null>;
  };
  const [selected, select] = useState("");
  const recorder = useRef(0);
  useEffect(() => {
    const frame = (now: number) => {
      counters.frames.push(now);
      if (counters.frames.length > 500) counters.frames.shift();
      recorder.current = requestAnimationFrame(frame);
    };
    recorder.current = requestAnimationFrame(frame);
    const p = () => counters.pointerMoves++;
    const t = () => counters.touchMoves++;
    const down = (event: PointerEvent) => {
      const target = event.target as Element;
      counters.presses.push({
        pointerType: event.pointerType,
        tag: target.tagName,
        region: target.getAttribute("data-region"),
        label: target.getAttribute("data-label"),
        trusted: event.isTrusted,
      });
    };
    document.addEventListener("pointermove", p);
    document.addEventListener("touchmove", t);
    document.addEventListener("pointerdown", down);
    return () => {
      cancelAnimationFrame(recorder.current);
      document.removeEventListener("pointermove", p);
      document.removeEventListener("touchmove", t);
      document.removeEventListener("pointerdown", down);
    };
  }, []);
  const click = (id: string, event: React.MouseEvent) => {
    if (camera.canClick(event.detail)) {
      counters.clicks++;
      select(id);
    }
  };
  const corner = mapPoint(camera.camera, { x: 0, y: 0 });
  const miniLocate = (event: React.PointerEvent<SVGSVGElement>) => {
    const r = event.currentTarget.getBoundingClientRect();
    const target = {
      x: ((event.clientX - r.left) / r.width) * world.width,
      y: ((event.clientY - r.top) / r.height) * world.height,
    };
    const current = camera.getCamera?.() ?? camera.camera;
    counters.miniExpected = locateCamera(current, target, world, camera.size);
    camera.locate(target);
  };
  return (
    <main>
      <header>
        <button onClick={() => camera.zoom(3)}>Zoom</button>
        <button onClick={camera.reset}>Reset</button>
        <button
          onClick={() =>
            camera.focus(
              world.provinces.find((p) => p.id === "fra-east")!.label,
            )
          }
        >
          Focus France
        </button>
        <output>{selected || "No selection"}</output>
      </header>
      <div
        id="camera-viewport"
        className="cq-viewport"
        ref={camera.viewport}
        {...camera.handlers}
      >
        <svg className="cq-map" width="100%" height="100%">
          <rect width="100%" height="100%" fill="#a8c6c8" />
          <g
            id="scene"
            className="cq-map-layer"
            ref={camera.sceneRef}
            transform={`translate(${camera.camera.x} ${camera.camera.y}) scale(${camera.camera.scale})`}
          >
            {world.provinces.map((p) => (
              <path
                key={p.id}
                data-region={p.id}
                d={p.path}
                fill={p.id === selected ? "#bc7563" : "#ddd9b8"}
                fillRule="evenodd"
                stroke="#526b5b"
                vectorEffect="non-scaling-stroke"
                onClick={(e) => click(p.id, e)}
              />
            ))}
          </g>
          <g id="overlay" ref={camera.overlayRef}>
            {world.provinces.map((p) => {
              const q = viewPoint(camera.camera, p.label);
              return (
                <text
                  key={p.id}
                  data-label={p.id}
                  x={q.x}
                  y={q.y}
                  fontSize="10"
                  onClick={(e) => click(p.id, e)}
                >
                  500
                </text>
              );
            })}
          </g>
        </svg>
        <svg
          id="minimap"
          data-camera-control=""
          style={{ position: "absolute", right: 6, bottom: 6 }}
          width="144"
          height="72"
          viewBox="0 0 1440 720"
          onPointerDown={(e) => {
            e.stopPropagation();
            e.currentTarget.setPointerCapture(e.pointerId);
            miniLocate(e);
          }}
          onPointerMove={(e) => {
            e.stopPropagation();
            if (e.currentTarget.hasPointerCapture(e.pointerId)) miniLocate(e);
          }}
          onPointerUp={(e) => {
            e.stopPropagation();
            if (e.currentTarget.hasPointerCapture(e.pointerId))
              e.currentTarget.releasePointerCapture(e.pointerId);
            (camera as any).settle?.();
          }}
        >
          <rect width="1440" height="720" fill="#abc" />
          <rect
            id="mini-viewport"
            ref={camera.miniViewportRef}
            x={Math.max(0, corner.x)}
            y={Math.max(0, corner.y)}
            width={Math.min(
              world.width,
              camera.size.width / camera.camera.scale,
            )}
            height={Math.min(
              world.height,
              camera.size.height / camera.camera.scale,
            )}
            fill="none"
            stroke="#124"
            strokeWidth="10"
          />
        </svg>
      </div>
      <div style={{ height: 400 }}>Outside map scroll area</div>
    </main>
  );
}
createRoot(document.getElementById("root")!).render(
  <Profiler id="camera" onRender={() => counters.commits++}>
    <MapFixture />
  </Profiler>,
);
