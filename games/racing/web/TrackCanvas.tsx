import { MAX_ROAD_SPEED } from "./dynamics";
import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type MutableRefObject,
} from "react";
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
import { ClassicRenderer } from "./classic-renderer";
import type { ClassicScenePack } from "./classic-loading";
import { classicPropWorld, type ClassicRuleTrack } from "./classic-road";
import { ClassicGauges, classicGaugeArc } from "./ClassicGauges";
import { classicGaugeReading } from "./classic-gauges";
import { classicVehicle } from "./classic-motion";
import {
  classicSeatColor,
  minimapBounds,
  minimapTransform,
  orderedMinimapCars,
} from "./classic-minimap";
type Engine = {
  mode: string;
  resize: () => void;
  render: (f: SceneFrame) => ChasePose | null;
  dispose: () => void;
  stats: { calls: number; triangles: number };
};
export type TrackCanvasProps = {
  classicAssets?: ClassicScenePack;
  data: View;
  clock: GameClock;
  seat: number;
  assets: string;
  onInputFrame?: () => void;
  onReady?: () => void;
  onLoadError?: (error: string) => void;
  input: MutableRefObject<Drive>;
  driving: boolean;
  reduced: boolean;
  watch: number;
  rear: boolean;
  cameraMode?: number;
  drawDistance?: number;
  mirrorVisible?: boolean;
  hudVisible?: boolean;
  viewScale?: number;
  onOpenMap: () => void;
};
export function TrackCanvas(props: TrackCanvasProps) {
  const holder = useRef<HTMLDivElement>(null),
    mini = useRef<HTMLCanvasElement>(null),
    latest = useRef(props);
  latest.current = props;
  const propWorld = useMemo(() => {
    const catalog = props.classicAssets?.propCatalog;
    if (!catalog) return undefined;
    return classicPropWorld(
      catalog,
      props.data.props ?? {
        catalog: catalog.catalogHash,
        broken: btoa(String.fromCharCode(0).repeat(Math.ceil(catalog.props.length / 8))),
      },
      props.classicAssets!.track as ClassicRuleTrack,
    );
  }, [
    props.classicAssets,
    props.data.props?.catalog,
    props.data.props?.broken,
  ]);
  const confirmedProps = useRef(propWorld);
  confirmedProps.current = propWorld;
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
              confirmedProps.current,
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
    let ready = false;
    const miniRoute = new Path2D();
    const miniBounds = minimapBounds(props.data.track.points);
    props.data.track.points.forEach((point, index) =>
      index
        ? miniRoute.lineTo(point.x, point.y)
        : miniRoute.moveTo(point.x, point.y),
    );
    miniRoute.closePath();
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
        const pack = latest.current.classicAssets;
        if (pack) {
          engine = await ClassicRenderer.create(
            canvas,
            latest.current.data.track,
            pack.files,
            { timeOfDay: pack.timeOfDay, vehicles: pack.vehicles },
          );
          if (disposed) {
            engine.dispose();
            return;
          }
        } else {
          const { ChaseRenderer } = await import("./chase-renderer");
          if (disposed) return;
          engine = new ChaseRenderer(canvas, latest.current.data.track);
        }
        setMode("webgl");
      } catch (error) {
        if (disposed) return;
        if (latest.current.classicAssets) {
          const message = "三维画面未能初始化，请重新载入。";
          setFailure(message);
          latest.current.onLoadError?.(message);
          return;
        }
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
          latest.current.onLoadError?.("画面加载失败，请重新载入。");
          return;
        }
      }
      const draw = (now: number) => {
        if (disposed) return;
        latest.current.onInputFrame?.();
        const p = latest.current,
          d = p.data,
          dt = Math.min(0.05, Math.max(0, (now - last) / 1000));
        last = now;
        if (document.hidden) {
          raf = requestAnimationFrame(draw);
          return;
        }
        const serverNow =
            p.clock.serverNow + (p.clock.paused ? 0 : now - p.clock.receivedAt),
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
                confirmedProps.current,
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
                props: confirmedProps.current,
              })
            : { ...c };
        });
        const follow = p.seat >= 0 ? p.seat : p.watch,
          car = cars.find((c) => c.seat === follow) ?? cars[0];
        const pose = engine?.render({
          cars,
          follow,
          rear: p.rear,
          cameraMode: p.cameraMode,
          drawDistance: p.drawDistance,
          mirrorVisible: p.mirrorVisible,
          reduced: p.reduced,
          now,
          propWorld: confirmedProps.current,
          propEvents: d.events,
          serverNow,
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
          canvas.dataset.viewScale = String(p.viewScale ?? 1);
          canvas.dataset.renderer = engine?.mode ?? "";
          canvas.dataset.camera = JSON.stringify(pose);
          canvas.dataset.speed = String(car.speed);
          host.style.setProperty(
            "--race-speed",
            String(Math.min(1, car.speed / MAX_ROAD_SPEED)),
          );
          const reading = classicGaugeReading(
            d.physics === 3 ? car : {...car,gear:car.gear || automaticGear(car.speed)},
            classicVehicle(car.vehicle ?? "p911").redlineRPM,
          );
          for (const [selector,value] of [
            ["[data-racing-speed]",String(reading.speed)],
            ["[data-racing-rpm]",String(reading.rpm)],
            ["[data-racing-gear]",reading.gear],
          ]) host.querySelectorAll(selector).forEach(element => { element.textContent = value; });
          if (d.physics === 3) {
            host.querySelectorAll("[data-racing-speed-needle]").forEach(needle => {
              needle.setAttribute("transform",`rotate(${reading.speedAngle} 80 80)`);
              needle.setAttribute("data-value",String(reading.speed));
            });
            host.querySelectorAll("[data-racing-rpm-needle]").forEach(needle => {
              needle.setAttribute("transform",`rotate(${reading.rpmAngle} 80 80)`);
              needle.setAttribute("data-value",String(reading.rpm));
            });
            host.querySelectorAll("[data-racing-redline]").forEach(redline => redline.setAttribute("d",classicGaugeArc(reading.redlineAngle)));
            host.querySelectorAll(".racing-gauges").forEach(gauges => gauges.setAttribute("aria-label", `速度 ${reading.speed} 千米每小时，转速 ${reading.rpm} 转每分钟，挡位 ${reading.gear}`));
          }
        }
        const m = mini.current,
          ctx = m?.getContext("2d");
        if (m && ctx) {
          const rect = m.getBoundingClientRect();
          const dpr = Math.min(2, window.devicePixelRatio || 1);
          const cssW = rect.width > 10 ? rect.width : 208;
          const cssH = rect.height > 10 ? rect.height : 148;
          const backingW = Math.max(1, Math.round(cssW * dpr));
          const backingH = Math.max(1, Math.round(cssH * dpr));
          if (m.width !== backingW || m.height !== backingH) {
            m.width = backingW;
            m.height = backingH;
          }
          const compactMini = cssW < 130;
          const pad = compactMini ? 9 : 13;
          const tf = minimapTransform(miniBounds, cssW, cssH, pad);
          ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
          ctx.clearRect(0, 0, cssW, cssH);
          ctx.save();
          ctx.translate(tf.offsetX, tf.offsetY);
          ctx.scale(tf.scale, tf.scale);
          ctx.lineJoin = "round";
          ctx.lineCap = "round";
          ctx.strokeStyle = "rgba(4, 12, 22, 0.88)";
          ctx.lineWidth = (compactMini ? 5.6 : 6.8) / tf.scale;
          ctx.stroke(miniRoute);
          ctx.strokeStyle = "#cde0ec";
          ctx.lineWidth = (compactMini ? 2.8 : 3.5) / tf.scale;
          ctx.stroke(miniRoute);
          if (d.track.points.length >= 2) {
            const p0 = d.track.points[0];
            const p1 = d.track.points[1];
            const len = Math.hypot(p1.x - p0.x, p1.y - p0.y) || 1;
            const nx = -(p1.y - p0.y) / len;
            const ny = (p1.x - p0.x) / len;
            const tick = (compactMini ? 4.5 : 6) / tf.scale;
            ctx.beginPath();
            ctx.moveTo(p0.x - nx * tick, p0.y - ny * tick);
            ctx.lineTo(p0.x + nx * tick, p0.y + ny * tick);
            ctx.strokeStyle = "#ffffff";
            ctx.lineWidth = (compactMini ? 2 : 2.5) / tf.scale;
            ctx.stroke();
          }
          for (const c of orderedMinimapCars(cars, follow)) {
            const isSelf = c.seat === follow;
            const color = classicSeatColor(c.seat);
            const radius = (isSelf ? (compactMini ? 5.8 : 7.0) : compactMini ? 4.4 : 5.4) / tf.scale;
            if (isSelf) {
              ctx.beginPath();
              ctx.arc(c.x, c.y, radius + 2.8 / tf.scale, 0, Math.PI * 2);
              ctx.fillStyle = "rgba(4, 12, 22, 0.94)";
              ctx.fill();
              ctx.beginPath();
              ctx.arc(c.x, c.y, radius + 1.6 / tf.scale, 0, Math.PI * 2);
              ctx.fillStyle = "#ffffff";
              ctx.fill();
              ctx.beginPath();
              ctx.arc(c.x, c.y, radius, 0, Math.PI * 2);
              ctx.fillStyle = color;
              ctx.fill();
              const tip = (compactMini ? 10.2 : 12.2) / tf.scale;
              const base = (compactMini ? 4.2 : 5.2) / tf.scale;
              const wing = (compactMini ? 3.4 : 4.2) / tf.scale;
              const cos = Math.cos(c.angle);
              const sin = Math.sin(c.angle);
              ctx.beginPath();
              ctx.moveTo(c.x + cos * tip, c.y + sin * tip);
              ctx.lineTo(c.x + cos * base - sin * wing, c.y + sin * base + cos * wing);
              ctx.lineTo(c.x + cos * base + sin * wing, c.y + sin * base - cos * wing);
              ctx.closePath();
              ctx.fillStyle = "#ffffff";
              ctx.fill();
              ctx.lineWidth = 1.2 / tf.scale;
              ctx.strokeStyle = "#06111c";
              ctx.stroke();
            } else {
              ctx.beginPath();
              ctx.arc(c.x, c.y, radius, 0, Math.PI * 2);
              ctx.fillStyle = color;
              ctx.fill();
              ctx.lineWidth = (compactMini ? 1.6 : 2) / tf.scale;
              ctx.strokeStyle = "#06111c";
              ctx.stroke();
            }
          }
          ctx.restore();
        }
        if (!ready) {
          ready = true;
          latest.current.onReady?.();
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
      if (latest.current.classicAssets) {
        cancelAnimationFrame(raf);
        const message = "三维画面已中断，请重新载入。";
        setFailure(message);
        latest.current.onLoadError?.(message);
        return;
      }
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
    <div
      className="racing-chase-wrap"
      ref={holder}
      data-racing-renderer={mode}
      style={
        props.classicAssets
          ? {
              width: `${(props.viewScale ?? 1) * 100}%`,
              height: `calc(var(--racing-view-height) * ${props.viewScale ?? 1})`,
              margin: "0 auto",
            }
          : undefined
      }
    >
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
        hidden={props.hudVisible === false}
        onClick={props.onOpenMap}
        aria-label="查看赛道地图"
      >
        <canvas ref={mini} width={416} height={296} aria-hidden="true" />
        <span>赛道图</span>
      </button>
      {props.data.physics === 3 ? <ClassicGauges hidden={props.hudVisible === false} /> : <div
        hidden={props.hudVisible === false}
        className="racing-speedometer"
        aria-label="自动挡速度表"
      >
        <span data-racing-speed>0</span>
        <small>km/h</small>
        <strong data-racing-gear>1</strong>
        <em>自动挡</em>
      </div>}
      {props.rear && <span className="racing-rear-indicator">向后看</span>}
      {mode === "canvas" && (
        <span className="racing-render-note">简化画面</span>
      )}
    </div>
  );
}
