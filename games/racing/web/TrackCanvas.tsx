import { useEffect, useRef, useState } from "react";
import type {
  MutableRefObject,
  PointerEvent as ReactPointerEvent,
} from "react";
import type { GameClock } from "../../../web/src/game-api/module";
import atlas from "../assets/public/racing/v1/atlas.json";
import type { Drive } from "./input";
import {
  angleDelta,
  clamp,
  extrapolateCar,
  interpolateCar,
  panBy,
  screenToWorld,
  zoomAt,
  type Camera,
} from "./presentation";
import { predictCar, roadDistance } from "./prediction";
import { sceneTime, effectAge } from "./timeline";
import {
  assetURL,
  seatColors,
  type Car,
  type Point,
  type Track,
  type View,
} from "./types";

type Art = { cars?: HTMLImageElement; scenery?: HTMLImageElement };
type Frame = { data: View; clock: GameClock; received: number };
type Props = {
  data: View;
  clock: GameClock;
  seat: number;
  assets: string;
  input: MutableRefObject<Drive>;
  driving: boolean;
  reduced: boolean;
  watch: number;
};
const palettes: Record<
  string,
  { ground: string; accent: string; edge: string }
> = {
  woodland: { ground: "#aabe8b", accent: "#75926b", edge: "#ddd3a5" },
  coast: { ground: "#e1d1a8", accent: "#7cbfca", edge: "#f4e5c5" },
  canyon: { ground: "#c89d77", accent: "#ac7a59", edge: "#e2bf94" },
};
function path(ctx: CanvasRenderingContext2D, points: Point[]) {
  ctx.beginPath();
  points.forEach((p, i) => (i ? ctx.lineTo(p.x, p.y) : ctx.moveTo(p.x, p.y)));
  ctx.closePath();
}
function dot(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  r: number,
  fill: string,
) {
  ctx.beginPath();
  ctx.arc(x, y, r, 0, 2 * Math.PI);
  ctx.fillStyle = fill;
  ctx.fill();
}
function sprite(
  ctx: CanvasRenderingContext2D,
  art: Art,
  name: string,
  x: number,
  y: number,
  w: number,
  h: number,
) {
  const f = atlas.scenery.sprites.find((f) => f.name === name);
  if (!f || !art.scenery) return false;
  ctx.drawImage(
    art.scenery,
    ...(f.rect as [number, number, number, number]),
    x - w / 2,
    y - h / 2,
    w,
    h,
  );
  return true;
}
function backdrop(track: Track, art: Art) {
  const sheet = document.createElement("canvas");
  sheet.width = track.width;
  sheet.height = track.height;
  const ctx = sheet.getContext("2d")!,
    theme = palettes[track.id] ?? palettes.woodland;
  ctx.fillStyle = theme.ground;
  ctx.fillRect(0, 0, track.width, track.height);
  // Local deterministic decoration. It never changes the physical road shape.
  for (let y = 55; y < track.height; y += 126)
    for (let x = 48; x < track.width; x += 132) {
      const n = Math.sin(x * 12.9898 + y * 78.233) * 43758.5453,
        frac = n - Math.floor(n);
      const p = { x: x + (frac - 0.5) * 85, y: y + (Math.sin(n) - 0.5) * 60 };
      if (roadDistance(track, p) < track.roadWidth * 0.5 + 62) continue;
      if (
        track.id === "coast" &&
        (x > track.width * 0.82 || y > track.height * 0.88)
      ) {
        dot(ctx, p.x, p.y, 100, theme.accent);
        continue;
      }
      ctx.globalAlpha = 0.18;
      dot(ctx, p.x, p.y, 25 + frac * 25, theme.accent);
      ctx.globalAlpha = 1;
      const name =
        track.id === "woodland"
          ? frac > 0.17
            ? "tree"
            : "rock"
          : track.id === "canyon"
            ? "rock"
            : frac > 0.6
              ? "tree"
              : "rock";
      if (!sprite(ctx, art, name, p.x, p.y, 42 + frac * 35, 42 + frac * 35))
        dot(ctx, p.x, p.y, 18, theme.accent);
    }
  ctx.lineJoin = "round";
  ctx.lineCap = "round";
  path(ctx, track.points);
  ctx.lineWidth = track.roadWidth + 24;
  ctx.strokeStyle = theme.edge;
  ctx.stroke();
  ctx.lineWidth = track.roadWidth + 11;
  ctx.strokeStyle = "#f5e8d1";
  ctx.stroke();
  ctx.save();
  ctx.setLineDash([25, 25]);
  ctx.strokeStyle = "#b8614c";
  ctx.stroke();
  ctx.restore();
  ctx.lineWidth = track.roadWidth;
  ctx.strokeStyle = "#657476";
  ctx.stroke();
  ctx.lineWidth = track.roadWidth - 9;
  ctx.strokeStyle = "#536365";
  ctx.stroke();
  ctx.lineWidth = 2;
  ctx.strokeStyle = "#c1c4b6";
  ctx.setLineDash([20, 30]);
  ctx.globalAlpha = 0.6;
  ctx.stroke();
  ctx.setLineDash([]);
  ctx.globalAlpha = 1;
  for (let i = 5; i < track.points.length; i += 10) {
    const p = track.points[i],
      n = track.points[(i + 1) % track.points.length],
      angle = Math.atan2(n.y - p.y, n.x - p.x);
    ctx.save();
    ctx.translate(p.x, p.y);
    ctx.rotate(angle);
    ctx.strokeStyle = "#a7b8b3";
    ctx.lineWidth = 3;
    ctx.beginPath();
    ctx.moveTo(-9, -8);
    ctx.lineTo(0, 0);
    ctx.lineTo(-9, 8);
    ctx.stroke();
    ctx.restore();
  }
  const a = track.points[0],
    b = track.points[1],
    angle = Math.atan2(b.y - a.y, b.x - a.x);
  ctx.save();
  ctx.translate(a.x, a.y);
  ctx.rotate(angle);
  const size = track.roadWidth / 12;
  for (let row = 0; row < 2; row++)
    for (let col = 0; col < 12; col++) {
      ctx.fillStyle = (row + col) % 2 ? "#293c3c" : "#f8edd9";
      ctx.fillRect((row - 1) * size, (col - 6) * size, size, size);
    }
  ctx.restore();
  const nx = -Math.sin(angle),
    ny = Math.cos(angle);
  sprite(
    ctx,
    art,
    "grandstand",
    a.x + nx * (track.roadWidth * 0.5 + 75),
    a.y + ny * (track.roadWidth * 0.5 + 75),
    126,
    90,
  );
  sprite(
    ctx,
    art,
    "lights",
    a.x - nx * (track.roadWidth * 0.5 + 37),
    a.y - ny * (track.roadWidth * 0.5 + 37),
    64,
    64,
  );
  return sheet;
}
function carBody(
  ctx: CanvasRenderingContext2D,
  c: Car,
  art: Art,
  own: boolean,
  scale: number,
  steer: number,
) {
  ctx.save();
  ctx.translate(c.x, c.y);
  if (own) {
    ctx.strokeStyle = "#fff9dd";
    ctx.lineWidth = 2 / scale;
    ctx.beginPath();
    ctx.arc(0, 0, 29, 0, Math.PI * 2);
    ctx.stroke();
  }
  ctx.rotate(c.angle + Math.PI / 2);
  ctx.fillStyle = "#23383c";
  for (const x of [-13, 13]) {
    ctx.save();
    ctx.translate(x, -12);
    ctx.rotate(steer * 0.36);
    ctx.fillRect(-3, -6, 6, 12);
    ctx.restore();
    ctx.fillRect(x - 3, 7, 6, 11);
  }
  ctx.shadowColor = "#21323755";
  ctx.shadowBlur = 5;
  ctx.shadowOffsetY = 3;
  if (art.cars)
    ctx.drawImage(
      art.cars,
      c.seat * 192,
      clamp(c.skin, 0, 4) * 320,
      192,
      320,
      -14,
      -24,
      28,
      48,
    );
  else {
    ctx.fillStyle = seatColors[c.seat % 6];
    ctx.beginPath();
    ctx.roundRect(-13, -23, 26, 46, 6);
    ctx.fill();
    ctx.fillStyle = "#cbe2db";
    ctx.fillRect(-9, -12, 18, 11);
    ctx.fillStyle = "#28444a";
    ctx.fillRect(-9, 9, 18, 7);
  }
  ctx.shadowColor = "transparent";
  ctx.restore();
}
export function TrackCanvas(props: Props) {
  const canvas = useRef<HTMLCanvasElement>(null),
    mini = useRef<HTMLCanvasElement>(null),
    container = useRef<HTMLDivElement>(null);
  const latest = useRef(props);
  latest.current = props;
  const frames = useRef<{ old: Frame; next: Frame } | null>(null),
    art = useRef<Art>({}),
    background = useRef<HTMLCanvasElement | null>(null);
  const camera = useRef<Camera>({
    x: props.data.cars[props.seat]?.x ?? props.data.track.width / 2,
    y: props.data.cars[props.seat]?.y ?? props.data.track.height / 2,
    scale: 1.1,
  });
  const follow = useRef(true),
    [following, setFollowing] = useState(true),
    [scaleLabel, setScaleLabel] = useState(110);
  const viewport = useRef({ width: 640, height: 420 }),
    touches = useRef(new Map<number, Point>()),
    lastPinch = useRef<{ distance: number; center: Point } | null>(null);
  const sceneKey = `${props.data.epoch}:${props.clock.epoch ?? 0}`,
    scene = useRef(sceneKey);
  const own = useRef<Car | null>(null),
    correction = useRef({ x: 0, y: 0, angle: 0 });
  const marks = useRef<{ x: number; y: number; angle: number; at: number }[]>(
      [],
    ),
    sparks = useRef<{ x: number; y: number; at: number; id: number }[]>([]),
    seen = useRef(Math.max(0, ...props.data.events.map((e) => e.id)));
  useEffect(() => {
    const frame = {
        data: props.data,
        clock: props.clock,
        received: performance.now(),
      },
      prev = frames.current?.next;
    if (scene.current !== sceneKey) {
      scene.current = sceneKey;
      frames.current = null;
      own.current = null;
      marks.current = [];
      sparks.current = [];
      seen.current = Math.max(0, ...props.data.events.map((e) => e.id));
      follow.current = true;
      setFollowing(true);
    }
    frames.current = { old: prev ?? frame, next: frame };
    const c = props.data.cars.find((c) => c.seat === props.seat);
    if (c) {
      const predicted = predictCar(
        c,
        props.input.current,
        props.data.track,
        Math.max(0, props.clock.serverNow - props.data.serverAt),
        props.clock.serverNow,
      );
      const old = own.current;
      correction.current =
        old &&
        Math.hypot(old.x - c.x, old.y - c.y) < 130 &&
        old.resetUntil === c.resetUntil
          ? {
              x: old.x - predicted.x,
              y: old.y - predicted.y,
              angle: angleDelta(predicted.angle, old.angle),
            }
          : { x: 0, y: 0, angle: 0 };
      own.current = predicted;
    }
    for (const e of props.data.events) {
      if (e.id <= seen.current) continue;
      if (e.type === "collision")
        sparks.current.push({ x: e.x, y: e.y, at: e.at, id: e.id });
      seen.current = Math.max(seen.current, e.id);
    }
  }, [props.data, props.clock, sceneKey, props.seat]);
  useEffect(() => {
    let disposed = false;
    const imgs: HTMLImageElement[] = [];
    for (const [key, file] of [
      ["cars", "cars.webp"],
      ["scenery", "scenery.webp"],
    ] as const) {
      const img = new Image();
      imgs.push(img);
      img.onload = () => {
        if (!disposed) {
          art.current = { ...art.current, [key]: img };
          background.current = null;
        }
      };
      img.onerror = () => {
        if (img.getAttribute("src") !== assetURL("", file))
          img.src = assetURL("", file);
      };
      img.src = assetURL(props.assets, file);
    }
    return () => {
      disposed = true;
      imgs.forEach((i) => {
        i.onload = null;
        i.onerror = null;
      });
    };
  }, [props.assets]);
  useEffect(() => {
    background.current = null;
  }, [props.data.track.id]);
  useEffect(() => {
    const el = canvas.current;
    if (!el) return;
    const resize = new ResizeObserver(() => {
      const r = el.getBoundingClientRect();
      viewport.current = { width: r.width, height: r.height };
      const dpr = Math.min(2, window.devicePixelRatio || 1);
      el.width = Math.round(r.width * dpr);
      el.height = Math.round(r.height * dpr);
    });
    resize.observe(el);
    const wheel = (e: WheelEvent) => {
      e.preventDefault();
      const r = el.getBoundingClientRect();
      camera.current = zoomAt(
        camera.current,
        viewport.current,
        { x: e.clientX - r.left, y: e.clientY - r.top },
        Math.exp(-e.deltaY * 0.0015),
      );
      follow.current = false;
      setFollowing(false);
      setScaleLabel(Math.round(camera.current.scale * 100));
    };
    el.addEventListener("wheel", wheel, { passive: false });
    return () => {
      resize.disconnect();
      el.removeEventListener("wheel", wheel);
    };
  }, []);
  useEffect(() => {
    let request = 0,
      last = performance.now(),
      lastMark = 0;
    const draw = (now: number) => {
      const p = latest.current,
        { data: d, seat, watch, reduced } = p,
        f = frames.current,
        el = canvas.current,
        ctx = el?.getContext("2d");
      const dt = clamp((now - last) / 1000, 0, 0.05);
      last = now;
      if (!ctx || !el || !f) {
        request = requestAnimationFrame(draw);
        return;
      }
      const serverNow = p.clock.serverNow + now - p.clock.receivedAt,
        age = serverNow - d.serverAt,
        visualAt = sceneTime(serverNow, p.driving);
      const moving = d.phase === "racing" || d.phase === "finishing";
      const cars = d.cars.map((c) => {
        if (c.seat === seat && p.driving && moving && !c.finishAt) {
          let local = own.current ?? { ...c };
          if (age < 250)
            local = predictCar(
              local,
              p.input.current,
              d.track,
              dt * 1000,
              serverNow,
            );
          const k = Math.exp(-dt * 13);
          correction.current.x *= k;
          correction.current.y *= k;
          correction.current.angle *= k;
          own.current = local;
          return {
            ...local,
            x: local.x + correction.current.x,
            y: local.y + correction.current.y,
            angle: local.angle + correction.current.angle,
          };
        }
        const older = f.old.data.cars.find((v) => v.seat === c.seat) ?? c;
        if (!moving) return { ...c };
        if (visualAt <= d.serverAt)
          return interpolateCar(
            older,
            c,
            (visualAt - f.old.data.serverAt) /
              Math.max(1, d.serverAt - f.old.data.serverAt),
          );
        return extrapolateCar(c, visualAt - d.serverAt);
      });
      const target =
        cars.find((c) => c.seat === (seat >= 0 ? seat : watch)) ?? cars[0];
      if (target && follow.current) {
        const ahead = moving ? 0.35 : 0,
          factor = reduced ? 1 : 1 - Math.exp(-dt * 8);
        camera.current.x +=
          (target.x + target.vx * ahead - camera.current.x) * factor;
        camera.current.y +=
          (target.y + target.vy * ahead - camera.current.y) * factor;
      }
      const cam = camera.current,
        v = viewport.current,
        dpr = el.width / Math.max(1, v.width);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, v.width, v.height);
      ctx.fillStyle = palettes[d.track.id]?.ground ?? "#aabe8b";
      ctx.fillRect(0, 0, v.width, v.height);
      ctx.translate(v.width / 2, v.height / 2);
      ctx.scale(cam.scale, cam.scale);
      ctx.translate(-cam.x, -cam.y);
      if (!background.current)
        background.current = backdrop(d.track, art.current);
      ctx.drawImage(background.current, 0, 0);
      if (!reduced && moving && now - lastMark > 50) {
        for (const c of cars) {
          const drift = c.seat === seat ? p.input.current.drift : c.drift;
          if (drift && c.speed > 80)
            marks.current.push({ x: c.x, y: c.y, angle: c.angle, at: now });
        }
        lastMark = now;
        marks.current = marks.current
          .filter((m) => now - m.at < 9000)
          .slice(-220);
      }
      if (!reduced) {
        for (const m of marks.current) {
          ctx.save();
          ctx.translate(m.x, m.y);
          ctx.rotate(m.angle);
          ctx.globalAlpha = 0.3 * (1 - (now - m.at) / 9000);
          ctx.fillStyle = "#293d3c";
          ctx.fillRect(-8, -11, 13, 3);
          ctx.fillRect(-8, 8, 13, 3);
          ctx.restore();
        }
      }
      for (const c of cars) {
        if (!reduced && moving && c.speed > 80 && (c.offroad || c.drift)) {
          for (let i = 0; i < 3; i++) {
            const fade = (now / 500 + i / 3) % 1;
            ctx.globalAlpha = (1 - fade) * 0.25;
            const x = c.x - Math.cos(c.angle) * (20 + fade * 32),
              y = c.y - Math.sin(c.angle) * (20 + fade * 32);
            if (
              !sprite(
                ctx,
                art.current,
                c.offroad ? "dust" : "smoke",
                x,
                y,
                15 + fade * 28,
                15 + fade * 28,
              )
            )
              dot(ctx, x, y, 8 + fade * 11, c.offroad ? "#d7c398" : "#e5e8d9");
          }
          ctx.globalAlpha = 1;
        }
        carBody(
          ctx,
          c,
          art.current,
          c.seat === seat,
          cam.scale,
          c.seat === seat ? p.input.current.steer : c.steer,
        );
      }
      sparks.current = sparks.current.filter((s) => visualAt - s.at < 550);
      if (!reduced)
        for (const s of sparks.current) {
          const elapsed = effectAge(s.at, visualAt);
          if (elapsed === null) continue;
          ctx.globalAlpha = 1 - elapsed / 550;
          sprite(
            ctx,
            art.current,
            "spark",
            s.x,
            s.y,
            25 + elapsed * 0.07,
            25 + elapsed * 0.07,
          );
          ctx.globalAlpha = 1;
        }
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      const my = cars.find((c) => c.seat === seat),
        auth = d.cars.find((c) => c.seat === seat);
      el.dataset.playerX = String(auth?.x ?? 0);
      el.dataset.playerY = String(auth?.y ?? 0);
      el.dataset.renderX = String(my?.x ?? 0);
      el.dataset.renderY = String(my?.y ?? 0);
      el.dataset.cameraX = String(cam.x);
      el.dataset.cameraY = String(cam.y);
      el.dataset.scale = String(cam.scale);
      el.dataset.cameraScale = String(cam.scale);
      el.dataset.follow = String(follow.current);
      el.dataset.renderAngle = String(my?.angle ?? 0);
      const m = mini.current,
        mc = m?.getContext("2d");
      if (m && mc) {
        const sx = m.width / d.track.width,
          sy = m.height / d.track.height;
        mc.setTransform(sx, 0, 0, sy, 0, 0);
        mc.fillStyle = "#e5e4ce";
        mc.fillRect(0, 0, d.track.width, d.track.height);
        path(mc, d.track.points);
        mc.lineWidth = d.track.roadWidth;
        mc.strokeStyle = "#5c7473";
        mc.stroke();
        mc.strokeStyle = "#274b50";
        mc.lineWidth = 2 / sx;
        mc.strokeRect(
          cam.x - v.width / (2 * cam.scale),
          cam.y - v.height / (2 * cam.scale),
          v.width / cam.scale,
          v.height / cam.scale,
        );
        for (const c of cars) {
          dot(
            mc,
            c.x,
            c.y,
            (c.seat === seat ? 4 : 3) / sx,
            seatColors[c.seat % 6],
          );
          mc.strokeStyle = "#fff7df";
          mc.lineWidth = 1 / sx;
          mc.stroke();
        }
      }
      request = requestAnimationFrame(draw);
    };
    request = requestAnimationFrame(draw);
    return () => cancelAnimationFrame(request);
  }, []);
  const point = (e: ReactPointerEvent) => {
    const r = e.currentTarget.getBoundingClientRect();
    return { x: e.clientX - r.left, y: e.clientY - r.top };
  };
  const panStart = (e: ReactPointerEvent<HTMLCanvasElement>) => {
    if (e.button !== 0) return;
    e.preventDefault();
    e.currentTarget.setPointerCapture(e.pointerId);
    touches.current.set(e.pointerId, point(e));
    lastPinch.current = null;
  };
  const panMove = (e: ReactPointerEvent<HTMLCanvasElement>) => {
    const previous = touches.current.get(e.pointerId);
    if (!previous) return;
    const p = point(e);
    touches.current.set(e.pointerId, p);
    if (touches.current.size >= 2) {
      const [a, b] = [...touches.current.values()],
        center = { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 },
        distance = Math.hypot(a.x - b.x, a.y - b.y),
        old = lastPinch.current;
      if (old) {
        camera.current = panBy(
          camera.current,
          center.x - old.center.x,
          center.y - old.center.y,
        );
        camera.current = zoomAt(
          camera.current,
          viewport.current,
          center,
          distance / Math.max(1, old.distance),
        );
        setScaleLabel(Math.round(camera.current.scale * 100));
      }
      lastPinch.current = { distance, center };
    } else
      camera.current = panBy(
        camera.current,
        p.x - previous.x,
        p.y - previous.y,
      );
    if (Math.hypot(p.x - previous.x, p.y - previous.y) > 0) {
      follow.current = false;
      setFollowing(false);
    }
  };
  const panEnd = (e: ReactPointerEvent) => {
    touches.current.delete(e.pointerId);
    lastPinch.current = null;
  };
  const miniPoint = (e: ReactPointerEvent<HTMLCanvasElement>) => {
    const r = e.currentTarget.getBoundingClientRect();
    camera.current = {
      ...camera.current,
      x: clamp((e.clientX - r.left) / r.width, 0, 1) * props.data.track.width,
      y: clamp((e.clientY - r.top) / r.height, 0, 1) * props.data.track.height,
    };
    follow.current = false;
    setFollowing(false);
  };
  const zoom = (factor: number) => {
    camera.current = zoomAt(
      camera.current,
      viewport.current,
      { x: viewport.current.width / 2, y: viewport.current.height / 2 },
      factor,
    );
    setScaleLabel(Math.round(camera.current.scale * 100));
  };
  return (
    <div className="racing-track" ref={container}>
      <canvas
        ref={canvas}
        className="racing-world"
        aria-label="赛车赛道"
        role="img"
        tabIndex={0}
        onPointerDown={panStart}
        onPointerMove={panMove}
        onPointerUp={panEnd}
        onPointerCancel={panEnd}
        onLostPointerCapture={panEnd}
      />
      <div className="racing-map-footer">
        <canvas
          ref={mini}
          className="racing-minimap"
          width={240}
          height={136}
          aria-label="赛道小地图"
          onPointerDown={(e) => {
            e.preventDefault();
            e.currentTarget.setPointerCapture(e.pointerId);
            miniPoint(e);
          }}
          onPointerMove={(e) => {
            if (e.currentTarget.hasPointerCapture(e.pointerId)) miniPoint(e);
          }}
        />
        <span className="racing-camera-hint">
          {following ? "镜头跟随赛车" : "正在查看赛道"}
          <small>拖动查看 · 滚轮／双指缩放</small>
        </span>
        <div className="racing-camera-buttons">
          <button
            type="button"
            aria-label="缩小赛道"
            onClick={() => zoom(1 / 1.2)}
          >
            −
          </button>
          <span>{scaleLabel}%</span>
          <button type="button" aria-label="放大赛道" onClick={() => zoom(1.2)}>
            ＋
          </button>
          <button
            type="button"
            aria-label="跟车"
            onClick={() => {
              follow.current = true;
              setFollowing(true);
              camera.current.scale = 1.1;
              setScaleLabel(110);
            }}
          >
            跟车
          </button>
        </div>
      </div>
    </div>
  );
}
