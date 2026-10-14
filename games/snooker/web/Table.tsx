import { useEffect, useRef, useState } from "react";
import type { Ball, Vec } from "./types";
import { ballInk, valueOf, ballName } from "./types";
import {
  W,
  H,
  R,
  BAULK,
  D,
  scaleOf,
  screenToWorld,
  worldToScreen,
  zoomAt,
  clampPan,
  type Camera,
} from "./presentation";

type Props = {
  balls: Ball[];
  angle: number;
  power: number;
  canAim: boolean;
  assist: boolean;
  placing: boolean;
  assets: string;
  flash?: { ids: number[]; intensity: number };
  strikeCue?: { pos: Vec; angle: number; power: number; progress: number };
  gestureKey: string;
  onShoot: (angle: number, power: number) => void;
  onPower: (power: number) => void;
  onAim: (angle: number) => void;
  onPlace: (point: Vec) => void;
};
const spots = [
  { x: BAULK, y: H / 2 + D },
  { x: BAULK, y: H / 2 - D },
  { x: BAULK, y: H / 2 },
  { x: W / 2, y: H / 2 },
  { x: W * 0.75, y: H / 2 },
  { x: W - 0.324, y: H / 2 },
];
const pockets = [
  { x: 0, y: 0 },
  { x: W / 2, y: -0.026 },
  { x: W, y: 0 },
  { x: 0, y: H },
  { x: W / 2, y: H + 0.026 },
  { x: W, y: H },
];
function aimEnd(balls: Ball[], angle: number): { point: Vec; hit?: Ball } {
  const cue = balls.find((b) => b.id === 0)!,
    dir = { x: Math.cos(angle), y: Math.sin(angle) };
  let length = 5,
    hit: Ball | undefined;
  for (const b of balls) {
    if (b.down || b.id === 0) continue;
    const dx = cue.pos.x - b.pos.x,
      dy = cue.pos.y - b.pos.y,
      dot = dx * dir.x + dy * dir.y,
      c = dx * dx + dy * dy - 4 * R * R,
      disc = dot * dot - c;
    if (dot < 0 && disc >= 0) {
      const t = -dot - Math.sqrt(disc);
      if (t >= 0 && t < length) {
        length = t;
        hit = b;
      }
    }
  }
  for (const [p, d, bound] of [
    [cue.pos.x, dir.x, dir.x > 0 ? W - R : R],
    [cue.pos.y, dir.y, dir.y > 0 ? H - R : R],
  ]) {
    if (Math.abs(d) > 1e-9) {
      const t = (bound - p) / d;
      if (t > 0 && t < length) {
        length = t;
        hit = undefined;
      }
    }
  }
  return {
    point: { x: cue.pos.x + dir.x * length, y: cue.pos.y + dir.y * length },
    hit,
  };
}
function drawScene(
  ctx: CanvasRenderingContext2D,
  c: Camera,
  p: Props,
  art: Record<string, HTMLImageElement>,
) {
  const k = scaleOf(c);
  ctx.save();
  ctx.translate(c.width / 2 + c.pan.x, c.height / 2 + c.pan.y);
  if (c.portrait) ctx.rotate(Math.PI / 2);
  ctx.scale(k, k);
  ctx.translate(-W / 2, -H / 2);
  ctx.fillStyle = "#593f2e";
  ctx.beginPath();
  ctx.roundRect(-0.15, -0.15, W + 0.3, H + 0.3, 0.095);
  ctx.fill();
  ctx.strokeStyle = "#a78760";
  ctx.lineWidth = 0.012;
  ctx.stroke();
  const felt = ctx.createLinearGradient(0, 0, W, H);
  felt.addColorStop(0, "#316a54");
  felt.addColorStop(1, "#1e4f40");
  ctx.fillStyle = felt;
  ctx.fillRect(0, 0, W, H);
  if (art.cloth) ctx.drawImage(art.cloth, 0, 0, W, H);
  ctx.strokeStyle = "#dce3bd99";
  ctx.lineWidth = 0.003;
  ctx.beginPath();
  ctx.moveTo(BAULK, 0);
  ctx.lineTo(BAULK, H);
  ctx.stroke();
  ctx.beginPath();
  ctx.arc(BAULK, H / 2, D, Math.PI / 2, (3 * Math.PI) / 2);
  ctx.stroke();
  for (const spot of spots) {
    ctx.fillStyle = "#c9d1ad";
    ctx.beginPath();
    ctx.arc(spot.x, spot.y, 0.004, 0, Math.PI * 2);
    ctx.fill();
  }
  const mouth = 0.076,
    mid = 0.082;
  ctx.fillStyle = "#407961";
  for (const y of [-0.063, H]) {
    ctx.fillRect(mouth, y, W / 2 - mid - mouth, 0.063);
    ctx.fillRect(W / 2 + mid, y, W / 2 - mid - mouth, 0.063);
  }
  for (const x of [-0.063, W]) ctx.fillRect(x, mouth, 0.063, H - 2 * mouth);
  for (const pocket of pockets) {
    ctx.fillStyle = "#15241f";
    ctx.beginPath();
    ctx.arc(pocket.x, pocket.y, 0.071, 0, Math.PI * 2);
    ctx.fill();
    ctx.fillStyle = "#090f0d";
    ctx.beginPath();
    ctx.arc(pocket.x, pocket.y, 0.05, 0, Math.PI * 2);
    ctx.fill();
  }
  // The ball diameter follows the actual table geometry. Precision is supplied
  // by zoom and the magnifier rather than by changing the collision diameter.
  for (const b of p.balls) {
    if (b.down) continue;
    if (p.flash?.ids.includes(b.id) && p.flash.intensity > 0) {
      ctx.strokeStyle = `rgba(251,223,136,${p.flash.intensity})`;
      ctx.lineWidth = Math.max(0.002, 1.5 / k);
      ctx.beginPath();
      ctx.arc(
        b.pos.x,
        b.pos.y,
        R + 0.016 * (1 - p.flash.intensity),
        0,
        Math.PI * 2,
      );
      ctx.stroke();
    }
    ctx.fillStyle = "#0005";
    ctx.beginPath();
    ctx.ellipse(
      b.pos.x + 0.008,
      b.pos.y + 0.01,
      R * 1.05,
      R * 0.92,
      0,
      0,
      Math.PI * 2,
    );
    ctx.fill();
    const sprite = art[`ball-${valueOf(b.id)}`];
    if (sprite) {
      ctx.drawImage(sprite, b.pos.x - R, b.pos.y - R, R * 2, R * 2);
      ctx.strokeStyle = "#ece8c2";
      ctx.lineWidth = Math.max(0.0008, 0.8 / k);
      ctx.beginPath();
      ctx.arc(b.pos.x, b.pos.y, R, 0, Math.PI * 2);
      ctx.stroke();
      continue;
    }
    const ink = ballInk[valueOf(b.id)],
      light = ctx.createRadialGradient(
        b.pos.x - R * 0.35,
        b.pos.y - R * 0.4,
        R * 0.04,
        b.pos.x,
        b.pos.y,
        R * 1.18,
      );
    light.addColorStop(0, b.id === 21 ? "#657275" : "#fff9e9");
    light.addColorStop(0.26, ink);
    light.addColorStop(0.83, ink);
    light.addColorStop(1, "#172d2c");
    ctx.fillStyle = light;
    ctx.beginPath();
    ctx.arc(b.pos.x, b.pos.y, R, 0, Math.PI * 2);
    ctx.fill();
    ctx.strokeStyle = "#152c2666";
    ctx.lineWidth = 0.001;
    ctx.stroke();
  }
  const cue = p.strikeCue
    ? { pos: p.strikeCue.pos, down: false }
    : p.balls.find((b) => b.id === 0);
  if (cue && !cue.down && (p.canAim || p.strikeCue) && !p.placing) {
    const dir = { x: Math.cos(p.angle), y: Math.sin(p.angle) };
    if (p.assist && !p.strikeCue) {
      const end = aimEnd(p.balls, p.angle);
      ctx.strokeStyle = "#f7efd7bf";
      ctx.lineWidth = Math.max(0.003, 1 / k);
      ctx.setLineDash([0.025, 0.018]);
      ctx.beginPath();
      ctx.moveTo(cue.pos.x, cue.pos.y);
      ctx.lineTo(end.point.x, end.point.y);
      ctx.stroke();
      ctx.setLineDash([]);
      ctx.beginPath();
      ctx.arc(end.point.x, end.point.y, R, 0, Math.PI * 2);
      ctx.stroke();
      if (end.hit) {
        const dx = end.hit.pos.x - end.point.x,
          dy = end.hit.pos.y - end.point.y,
          d = Math.hypot(dx, dy);
        ctx.beginPath();
        ctx.moveTo(end.hit.pos.x, end.hit.pos.y);
        ctx.lineTo(
          end.hit.pos.x + (dx / d) * 0.2,
          end.hit.pos.y + (dy / d) * 0.2,
        );
        ctx.stroke();
      }
    }
    ctx.save();
    ctx.translate(cue.pos.x, cue.pos.y);
    ctx.rotate(p.strikeCue?.angle ?? p.angle);
    if (p.strikeCue)
      ctx.translate(
        0.028 + p.strikeCue.progress * 0.035 * p.strikeCue.power,
        0,
      );
    if (!p.strikeCue) ctx.translate(-p.power * 0.16, 0);
    ctx.lineCap = "round";
    ctx.strokeStyle = "#dfc591";
    ctx.lineWidth = 0.012;
    ctx.beginPath();
    ctx.moveTo(-R - 0.028, 0);
    ctx.lineTo(-0.73, 0);
    ctx.stroke();
    ctx.strokeStyle = "#442e25";
    ctx.lineWidth = 0.02;
    ctx.beginPath();
    ctx.moveTo(-0.46, 0);
    ctx.lineTo(-0.83, 0);
    ctx.stroke();
    ctx.strokeStyle = "#88bec3";
    ctx.lineWidth = 0.012;
    ctx.beginPath();
    ctx.moveTo(-R - 0.028, 0);
    ctx.lineTo(-R - 0.036, 0);
    ctx.stroke();
    ctx.restore();
  }
  if (p.placing) {
    ctx.fillStyle = "#d2deb320";
    ctx.beginPath();
    ctx.arc(BAULK, H / 2, D, Math.PI / 2, (3 * Math.PI) / 2);
    ctx.closePath();
    ctx.fill();
  }
  ctx.restore();
}
export function Table(props: Props) {
  const wrap = useRef<HTMLDivElement>(null),
    canvas = useRef<HTMLCanvasElement>(null);
  const [size, setSize] = useState({
      width: 600,
      height: 350,
      portrait: false,
    }),
    [pose, setPose] = useState({ zoom: 1, pan: { x: 0, y: 0 } }),
    [art, setArt] = useState<Record<string, HTMLImageElement>>({});
  const camera: Camera = { ...size, ...pose },
    cam = useRef(camera);
  cam.current = camera;
  const pointers = useRef(new Map<number, Vec>()),
    gesture = useRef<{
      start: Vec;
      dragged: boolean;
      mode: "pan" | "place" | "shot" | "cancel";
    } | null>(null);
  const [movingView, setMovingView] = useState(false),
    [charging, setCharging] = useState(false),
    [chargePower, setChargePower] = useState(0.02);
  const current = useRef(props);
  current.current = props;
  const charge = useRef<{
    start: number;
    angle: number;
    pointer: number | "keyboard";
  } | null>(null);
  const frame = useRef(0);
  const strength = (elapsed: number) => {
    const t = Math.max(0, elapsed - 120) % 3200;
    return 0.02 + 0.98 * (1 - Math.abs(t - 1600) / 1600);
  };
  const cancelCharge = () => {
    charge.current = null;
    cancelAnimationFrame(frame.current);
    setCharging(false);
    if (gesture.current?.mode === "shot") gesture.current.mode = "cancel";
  };
  const startCharge = (pointer: number | "keyboard", angle: number) => {
    cancelCharge();
    charge.current = { start: performance.now(), angle, pointer };
    setCharging(true);
    const tick = () => {
      const c = charge.current;
      if (!c) return;
      const power = strength(performance.now() - c.start);
      setChargePower(power);
      current.current.onPower(power);
      frame.current = requestAnimationFrame(tick);
    };
    tick();
  };
  const releaseCharge = (pointer: number | "keyboard") => {
    const c = charge.current;
    if (!c || c.pointer !== pointer) return;
    const elapsed = performance.now() - c.start;
    const power = strength(elapsed);
    cancelCharge();
    // Brief taps may aim, but must not accidentally shoot while touching UI.
    if (elapsed >= 120 && current.current.canAim && !current.current.placing)
      current.current.onShoot(c.angle, power);
  };
  useEffect(() => {
    cancelCharge();
    gesture.current = null;
    pointers.current.clear();
  }, [props.gestureKey, props.canAim, props.placing, size.width, size.height]);
  useEffect(() => {
    const hidden = () => {
      if (document.hidden) cancelCharge();
    };
    const escape = (e: KeyboardEvent) => {
      if (e.key === "Escape" && charge.current) {
        e.preventDefault();
        cancelCharge();
      }
    };
    window.addEventListener("blur", cancelCharge);
    window.addEventListener("keydown", escape);
    document.addEventListener("visibilitychange", hidden);
    return () => {
      cancelAnimationFrame(frame.current);
      charge.current = null;
      window.removeEventListener("blur", cancelCharge);
      window.removeEventListener("keydown", escape);
      document.removeEventListener("visibilitychange", hidden);
    };
  }, []);
  useEffect(() => {
    const el = wrap.current!;
    const observer = new ResizeObserver(() =>
      setSize({
        width: el.clientWidth,
        height: el.clientHeight,
        portrait: innerWidth <= 600,
      }),
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    let active = true;
    for (const name of [
      "cloth",
      ...Array.from({ length: 8 }, (_, i) => `ball-${i}`),
    ]) {
      const img = new Image();
      const path = `/snooker/v1/${name}.webp`;
      img.onload = () => {
        if (active) setArt((old) => ({ ...old, [name]: img }));
      };
      let local = !props.assets;
      img.onerror = () => {
        if (!local) {
          local = true;
          img.src = path;
        }
      };
      img.src = props.assets ? props.assets.replace(/\/$/, "") + path : path;
    }
    return () => {
      active = false;
    };
  }, [props.assets]);
  const changeCamera = (c: Camera) => {
    cancelCharge();
    cam.current = c;
    setPose({ zoom: c.zoom, pan: c.pan });
  };
  useEffect(() => {
    const el = canvas.current!;
    const wheel = (e: WheelEvent) => {
      e.preventDefault();
      const r = el.getBoundingClientRect();
      changeCamera(
        zoomAt(cam.current, cam.current.zoom * Math.exp(-e.deltaY * 0.0015), {
          x: e.clientX - r.left,
          y: e.clientY - r.top,
        }),
      );
    };
    el.addEventListener("wheel", wheel, { passive: false });
    return () => el.removeEventListener("wheel", wheel);
  }, []);
  useEffect(() => {
    const el = canvas.current!,
      ctx = el.getContext("2d")!;
    const ratio = Math.min(2, devicePixelRatio || 1);
    el.width = Math.round(size.width * ratio);
    el.height = Math.round(size.height * ratio);
    ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
    ctx.clearRect(0, 0, size.width, size.height);
    drawScene(ctx, camera, props, art);
    const cue = props.balls.find((b) => b.id === 0);
    if (props.canAim && cue && !cue.down && !props.placing) {
      const radius = 51,
        cx = size.width - 60,
        cy = 60;
      ctx.save();
      ctx.beginPath();
      ctx.arc(cx, cy, radius, 0, Math.PI * 2);
      ctx.clip();
      ctx.fillStyle = "#254f40";
      ctx.fillRect(cx - radius, cy - radius, radius * 2, radius * 2);
      const end = aimEnd(props.balls, props.angle);
      const focus = end.hit
        ? {
            x: (end.point.x + end.hit.pos.x) / 2,
            y: (end.point.y + end.hit.pos.y) / 2,
          }
        : {
            x: cue.pos.x + Math.cos(props.angle) * 0.025,
            y: cue.pos.y + Math.sin(props.angle) * 0.025,
          };
      const lens: Camera = {
        ...camera,
        zoom: 700 / scaleOf({ ...camera, zoom: 1 }),
        pan: { x: 0, y: 0 },
      };
      const loc = worldToScreen(focus, lens);
      lens.pan = { x: cx - loc.x, y: cy - loc.y };
      drawScene(ctx, lens, { ...props, placing: false }, art);
      ctx.restore();
      ctx.strokeStyle = "#ded1a9";
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.arc(cx, cy, radius, 0, Math.PI * 2);
      ctx.stroke();
    }
  }, [
    props.balls,
    props.flash,
    props.strikeCue,
    props.angle,
    props.power,
    props.canAim,
    props.assist,
    props.placing,
    pose,
    size,
    art,
  ]);
  const point = (e: React.PointerEvent) => {
    const r = e.currentTarget.getBoundingClientRect();
    return { x: e.clientX - r.left, y: e.clientY - r.top };
  };
  return (
    <div className="snooker-viewport">
      <div
        className="snooker-table"
        ref={wrap}
        data-zoom={pose.zoom.toFixed(3)}
        data-respot={props.flash?.ids.join(",") ?? ""}
      >
        <canvas
          ref={canvas}
          tabIndex={0}
          aria-label="斯诺克球桌，按住瞄准蓄力，松开击球；空格蓄力，方向键瞄准，Esc 取消"
          aria-disabled={!props.canAim}
          data-angle={props.angle}
          onBlur={cancelCharge}
          onKeyDown={(e) => {
            if (
              e.key === " " &&
              !e.repeat &&
              props.canAim &&
              !props.placing &&
              !movingView
            ) {
              e.preventDefault();
              startCharge("keyboard", props.angle);
            } else if (
              [
                "ArrowLeft",
                "ArrowRight",
                "ArrowUp",
                "ArrowDown",
                "Home",
              ].includes(e.key) &&
              props.canAim
            ) {
              e.preventDefault();
              const angle =
                e.key === "Home"
                  ? 0
                  : e.key === "ArrowUp"
                    ? -Math.PI / 2
                    : e.key === "ArrowDown"
                      ? Math.PI / 2
                      : props.angle +
                        ((e.key === "ArrowLeft" ? -1 : 1) *
                          (e.shiftKey ? 0.1 : 1) *
                          Math.PI) /
                          180;
              props.onAim(angle);
              if (charge.current) charge.current.angle = angle;
            }
          }}
          onKeyUp={(e) => {
            if (e.key === " ") {
              e.preventDefault();
              releaseCharge("keyboard");
            }
          }}
          onPointerDown={(e) => {
            if (e.button !== 0) return;
            const p = point(e);
            if (
              props.canAim &&
              !props.placing &&
              Math.hypot(p.x - (size.width - 60), p.y - 60) < 53
            )
              return;
            e.currentTarget.focus({ preventScroll: true });
            pointers.current.set(e.pointerId, p);
            e.currentTarget.setPointerCapture(e.pointerId);
            if (pointers.current.size > 1) {
              cancelCharge();
              gesture.current = { start: p, dragged: true, mode: "pan" };
              return;
            }
            const cue = props.balls.find((b) => b.id === 0);
            const mode =
              movingView || !props.canAim
                ? "pan"
                : props.placing
                  ? "place"
                  : cue && !cue.down
                    ? "shot"
                    : "cancel";
            if (mode === "shot" && cue) {
              const world = screenToWorld(p, cam.current);
              const angle = Math.atan2(
                world.y - cue.pos.y,
                world.x - cue.pos.x,
              );
              props.onAim(angle);
              startCharge(e.pointerId, angle);
            }
            gesture.current = { start: p, dragged: false, mode };
          }}
          onPointerMove={(e) => {
            const old = pointers.current.get(e.pointerId);
            if (!old) return;
            const before = [...pointers.current.values()],
              p = point(e);
            pointers.current.set(e.pointerId, p);
            const after = [...pointers.current.values()];
            if (p.x < 0 || p.y < 0 || p.x > size.width || p.y > size.height) {
              cancelCharge();
              if (gesture.current) gesture.current.mode = "cancel";
              return;
            }
            if (after.length === 2) {
              cancelCharge();
              const dist = (a: Vec[]) =>
                  Math.hypot(a[1].x - a[0].x, a[1].y - a[0].y),
                mid = (a: Vec[]) => ({
                  x: (a[0].x + a[1].x) / 2,
                  y: (a[0].y + a[1].y) / 2,
                });
              const a = mid(before),
                b = mid(after),
                next = zoomAt(
                  cam.current,
                  (cam.current.zoom * dist(after)) / Math.max(1, dist(before)),
                  a,
                );
              changeCamera(
                clampPan({
                  ...next,
                  pan: { x: next.pan.x + b.x - a.x, y: next.pan.y + b.y - a.y },
                }),
              );
              return;
            }
            const g = gesture.current;
            if (!g) return;
            if (Math.hypot(p.x - g.start.x, p.y - g.start.y) > 5)
              g.dragged = true;
            if (g.mode === "shot" && charge.current) {
              const cue = props.balls.find((b) => b.id === 0)!;
              const world = screenToWorld(p, cam.current);
              const angle = Math.atan2(
                world.y - cue.pos.y,
                world.x - cue.pos.x,
              );
              charge.current.angle = angle;
              props.onAim(angle);
            } else if (g.mode === "pan" && g.dragged) {
              changeCamera(
                clampPan({
                  ...cam.current,
                  pan: {
                    x: cam.current.pan.x + p.x - old.x,
                    y: cam.current.pan.y + p.y - old.y,
                  },
                }),
              );
            }
          }}
          onPointerUp={(e) => {
            if (!pointers.current.has(e.pointerId)) return;
            const p = point(e),
              g = gesture.current;
            pointers.current.delete(e.pointerId);
            const inside =
              p.x >= 0 && p.y >= 0 && p.x <= size.width && p.y <= size.height;
            if (g?.mode === "shot" && inside) releaseCharge(e.pointerId);
            else {
              cancelCharge();
              if (g?.mode === "place" && !g.dragged && inside && props.canAim)
                props.onPlace(screenToWorld(p, cam.current));
            }
            if (!pointers.current.size) gesture.current = null;
            if (e.currentTarget.hasPointerCapture(e.pointerId))
              e.currentTarget.releasePointerCapture(e.pointerId);
          }}
          onPointerCancel={(e) => {
            cancelCharge();
            pointers.current.delete(e.pointerId);
            gesture.current = null;
          }}
          onLostPointerCapture={(e) => {
            if (pointers.current.has(e.pointerId)) {
              cancelCharge();
              pointers.current.delete(e.pointerId);
              gesture.current = null;
            }
          }}
        />
        {charging && (
          <div
            className="snooker-charge"
            role="progressbar"
            aria-label="击球力度"
            aria-valuemin={2}
            aria-valuemax={100}
            aria-valuenow={Math.round(chargePower * 100)}
          >
            <span>力度 {Math.round(chargePower * 100)}% · 松开击球</span>
            <div>
              <i style={{ width: `${chargePower * 100}%` }} />
            </div>
          </div>
        )}
        {props.flash && (
          <span className="snooker-respot-note" role="status">
            已复位：{props.flash.ids.map(ballName).join("、")}
          </span>
        )}
      </div>
      <div className="snooker-camera">
        <span>
          {movingView
            ? "拖动平移 · 滚轮／双指缩放"
            : "按住瞄准蓄力 · 松开击球 · 移出球桌或 Esc 取消"}
        </span>
        <button
          aria-pressed={movingView}
          onClick={() => {
            cancelCharge();
            setMovingView((v) => !v);
          }}
        >
          移动视图
        </button>
        <button
          onClick={() =>
            changeCamera(
              zoomAt(cam.current, pose.zoom * 1.25, {
                x: size.width / 2,
                y: size.height / 2,
              }),
            )
          }
          aria-label="放大球桌"
        >
          ＋
        </button>
        <button
          onClick={() =>
            changeCamera(
              zoomAt(cam.current, pose.zoom / 1.25, {
                x: size.width / 2,
                y: size.height / 2,
              }),
            )
          }
          aria-label="缩小球桌"
        >
          −
        </button>
        <button
          onClick={() =>
            changeCamera({ ...cam.current, zoom: 1, pan: { x: 0, y: 0 } })
          }
        >
          复位视图
        </button>
      </div>
    </div>
  );
}
