import { useEffect, useRef, useState, type MutableRefObject } from "react";
import type { GameClock } from "../../../web/src/game-api/module";
import { automaticGear } from "./chase-camera";
import { predictCar } from "./prediction";
import { angleDelta, clamp } from "./presentation";
import { remotePose } from "./remote-pose";
import { sceneTime } from "./timeline";
import { seatColors, type Car, type View } from "./types";
import type { Drive } from "./input";
import type { SceneFrame } from "./chase-renderer";
import type { ChasePose } from "./chase-camera";
type Engine = {
  mode: string;
  resize: () => void;
  render: (f: SceneFrame) => ChasePose | null;
  dispose: () => void;
  stats: { calls: number; triangles: number };
};
export type TrackCanvasProps = {
  data: View;
  clock: GameClock;
  seat: number;
  assets: string;
  input: MutableRefObject<Drive>;
  driving: boolean;
  reduced: boolean;
  watch: number;
  rear: boolean;
  onOpenMap: () => void;
};
export function TrackCanvas(props: TrackCanvasProps) {
  const holder = useRef<HTMLDivElement>(null),
    mini = useRef<HTMLCanvasElement>(null),
    latest = useRef(props);
  latest.current = props;
  const [mode, setMode] = useState("loading"),
    [failure, setFailure] = useState(""),
    [forceFallback, setForceFallback] = useState(false);
  const frames = useRef({ previous: props.data, current: props.data });
  const own = useRef<Car | null>(null),
    correction = useRef({ x: 0, y: 0, angle: 0 });
  const identity = `${props.data.epoch}:${props.seat}:${props.clock.epoch ?? 0}`;
  useEffect(() => {
    own.current = null;
    correction.current = { x: 0, y: 0, angle: 0 };
    frames.current = { previous: props.data, current: props.data };
  }, [identity]);
  useEffect(() => {
    frames.current = { previous: frames.current.current, current: props.data };
    const c = props.data.cars.find((v) => v.seat === props.seat);
    if (c) {
      const age = clamp(
          props.clock.serverNow +
            performance.now() -
            props.clock.receivedAt -
            props.data.serverAt,
          0,
          200,
        ),
        predicted = props.driving
          ? predictCar(
              c,
              props.input.current,
              props.data.track,
              age,
              props.clock.serverNow,
            )
          : { ...c };
      const old = own.current;
      correction.current =
        old &&
        Math.hypot(old.x - predicted.x, old.y - predicted.y) < 150 &&
        old.resetUntil === predicted.resetUntil
          ? {
              x: old.x - predicted.x,
              y: old.y - predicted.y,
              angle: angleDelta(predicted.angle, old.angle),
            }
          : { x: 0, y: 0, angle: 0 };
      own.current = predicted;
    }
  }, [props.data, props.clock, props.driving, props.seat]);
  useEffect(() => {
    const host = holder.current;
    if (!host) return;
    let disposed = false,
      engine: Engine | undefined,
      raf = 0,
      last = performance.now(),
      framesCount = 0,
      statsAt = last;
    let canvas = document.createElement("canvas");
    canvas.className = "racing-chase";
    canvas.setAttribute("aria-label", "车后追逐视角");
    canvas.tabIndex = 0;
    host.prepend(canvas);
    const observer = new ResizeObserver(() => engine?.resize());
    observer.observe(host);
    const init = async () => {
      try {
        if (forceFallback) throw Error("compatibility renderer");
        const { ChaseRenderer } = await import("./chase-renderer");
        if (disposed) return;
        engine = new ChaseRenderer(canvas, latest.current.data.track);
        setMode("webgl");
      } catch (error) {
        if (disposed) return;
        const oldCanvas = canvas;
        oldCanvas.remove();
        try {
          if (oldCanvas.dataset.engine)
            oldCanvas
              .getContext("webgl2")
              ?.getExtension("WEBGL_lose_context")
              ?.loseContext();
        } catch {}
        canvas = document.createElement("canvas");
        canvas.className = "racing-chase";
        canvas.setAttribute("aria-label", "透视驾驶视角");
        canvas.tabIndex = 0;
        host.prepend(canvas);
        try {
          const { ChaseFallback } = await import("./chase-fallback");
          if (disposed) return;
          engine = new ChaseFallback(canvas, latest.current.data.track);
          setMode("canvas");
        } catch (e) {
          setFailure("画面加载失败，请重新载入。");
          return;
        }
      }
      const draw = (now: number) => {
        if (disposed) return;
        const p = latest.current,
          d = p.data,
          dt = Math.min(0.05, Math.max(0, (now - last) / 1000));
        last = now;
        if (document.hidden) {
          raf = requestAnimationFrame(draw);
          return;
        }
        const serverNow = p.clock.serverNow + now - p.clock.receivedAt,
          age = serverNow - d.serverAt,
          visual = sceneTime(serverNow, p.driving),
          moving = d.phase === "racing" || d.phase === "finishing";
        const cars = d.cars.map((c) => {
          if (c.seat === p.seat && p.driving && moving && !c.finishAt) {
            let value = own.current ?? { ...c };
            if (age < 250)
              value = predictCar(
                value,
                p.input.current,
                d.track,
                dt * 1000,
                serverNow,
              );
            own.current = value;
            const k = Math.exp(-dt * 13);
            correction.current.x *= k;
            correction.current.y *= k;
            correction.current.angle *= k;
            return {
              ...value,
              x: value.x + correction.current.x,
              y: value.y + correction.current.y,
              angle: value.angle + correction.current.angle,
            };
          }
          const old =
            frames.current.previous.cars.find((v) => v.seat === c.seat) ?? c;
          return moving
            ? remotePose({
                older: old,
                current: c,
                track: d.track,
                previousAt: frames.current.previous.serverAt,
                currentAt: d.serverAt,
                visualAt: visual,
                driving: p.driving,
              })
            : { ...c };
        });
        const follow = p.seat >= 0 ? p.seat : p.watch,
          car = cars.find((c) => c.seat === follow) ?? cars[0];
        const pose = engine?.render({
          cars,
          follow,
          rear: p.rear,
          reduced: p.reduced,
          now,
        });
        if (car) {
          canvas.dataset.snapshotAt = String(d.serverAt);
          const other = cars.find((c) => c.seat !== follow);
          canvas.dataset.otherX = String(other?.x ?? 0);
          canvas.dataset.otherY = String(other?.y ?? 0);
          canvas.dataset.renderX = String(car.x);
          canvas.dataset.renderY = String(car.y);
          canvas.dataset.angle = String(car.angle);
          canvas.dataset.rear = String(p.rear);
          canvas.dataset.renderer = engine?.mode ?? "";
          canvas.dataset.camera = JSON.stringify(pose);
          canvas.dataset.speed = String(car.speed);
          host.style.setProperty(
            "--race-speed",
            String(Math.min(1, car.speed / 310)),
          );
          const speed = host.querySelector("[data-racing-speed]"),
            gear = host.querySelector("[data-racing-gear]");
          if (speed) speed.textContent = String(Math.round(car.speed * 0.36));
          if (gear) gear.textContent = `D${automaticGear(car.speed)}`;
        }
        const m = mini.current,
          ctx = m?.getContext("2d");
        if (m && ctx) {
          const w = m.width,
            h = m.height,
            pad = 10,
            s = Math.min(
              (w - pad * 2) / d.track.width,
              (h - pad * 2) / d.track.height,
            );
          ctx.clearRect(0, 0, w, h);
          ctx.save();
          ctx.translate(
            (w - d.track.width * s) / 2,
            (h - d.track.height * s) / 2,
          );
          ctx.scale(s, s);
          ctx.beginPath();
          d.track.points.forEach((p, i) =>
            i ? ctx.lineTo(p.x, p.y) : ctx.moveTo(p.x, p.y),
          );
          ctx.closePath();
          ctx.strokeStyle = "#8799a9";
          ctx.lineWidth = 3 / s;
          ctx.stroke();
          for (const c of cars) {
            ctx.fillStyle =
              c.seat === follow
                ? "#ffffff"
                : seatColors[c.seat % seatColors.length];
            ctx.beginPath();
            ctx.arc(c.x, c.y, (c.seat === follow ? 4 : 3) / s, 0, Math.PI * 2);
            ctx.fill();
          }
          ctx.restore();
        }
        framesCount++;
        if (now - statsAt >= 1000) {
          canvas.dataset.fps = String(
            Math.round((framesCount * 1000) / (now - statsAt)),
          );
          canvas.dataset.drawCalls = String(engine?.stats.calls ?? 0);
          framesCount = 0;
          statsAt = now;
        }
        raf = requestAnimationFrame(draw);
      };
      raf = requestAnimationFrame(draw);
    };
    void init();
    const lost = (e: Event) => {
      e.preventDefault();
      setMode("loading");
      setForceFallback(true);
    };
    host.addEventListener("webglcontextlost", lost, true);
    return () => {
      disposed = true;
      cancelAnimationFrame(raf);
      observer.disconnect();
      host.removeEventListener("webglcontextlost", lost, true);
      engine?.dispose();
      canvas.remove();
    };
  }, [props.data.epoch, forceFallback]);
  return (
    <div className="racing-chase-wrap" ref={holder} data-racing-renderer={mode}>
      {mode === "loading" && (
        <span className="racing-scene-loading" role="status">
          正在进入赛道…
        </span>
      )}
      {failure && (
        <div className="racing-scene-loading" role="alert">
          {failure}
          <button onClick={() => location.reload()}>重新载入</button>
        </div>
      )}
      <button
        className="racing-route-button"
        onClick={props.onOpenMap}
        aria-label="查看赛道地图"
      >
        <canvas ref={mini} width={160} height={112} aria-hidden="true" />
        <span>赛道图</span>
      </button>
      <div className="racing-speedometer" aria-label="自动挡速度表">
        <span data-racing-speed>0</span>
        <small>km/h</small>
        <strong data-racing-gear>D1</strong>
        <em>自动挡</em>
      </div>
      {props.rear && <span className="racing-rear-indicator">向后看</span>}
      {mode === "canvas" && (
        <span className="racing-render-note">简化画面</span>
      )}
    </div>
  );
}
