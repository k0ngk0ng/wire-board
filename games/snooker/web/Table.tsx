import { useGameController } from "../../../web/src/controllers/use-game-controller";
import { pullPower, cueTip } from "./cue-input";
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
  controllerUser: string;
  controllerScope: string;
  controlsEnabled: boolean;
  onControllerError: (message: string) => void;
  spin: { top: number; side: number };
  onSpin: (value: { top: number; side: number }) => void;
  onTogglePlace: () => void;
  onReady?: () => void;
  onLoadError?: (message: string) => void;
  balls: Ball[];
  angle: number;
  power: number;
  pullBack?: number;
  canAim: boolean;
  finished: boolean;
  assist: boolean;
  placing: boolean;
  assets: string;
  flash?: { ids: number[]; intensity: number };
  strikeCue?: { pos: Vec; angle: number; power: number; offset: number };
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
    if (p.strikeCue) ctx.translate(p.strikeCue.offset, 0);
    if (!p.strikeCue) ctx.translate(-(p.pullBack ?? 0), 0);
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
    canvas = useRef<HTMLCanvasElement>(null),
    preview = useRef<HTMLCanvasElement>(null);
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
      mode: "pan" | "place" | "aim" | "shot" | "cancel";
    } | null>(null);
  const [movingView, setMovingView] = useState(false),
    [touchInput, setTouchInput] = useState(
      () => matchMedia("(pointer: coarse)").matches,
    ),
    [charging, setCharging] = useState(false),
    [chargePower, setChargePower] = useState(0.02);
  const current = useRef(props);
  current.current = props;
  const charge = useRef<{
    origin?: Vec;
    power: number;
    limit: number;
    angle: number;
    pointer: number | "controller";
    slider?: boolean;
  } | null>(null);
  const cancelCharge = () => {
    charge.current = null;
    current.current.onPower(0);
    setCharging(false);
    if (gesture.current?.mode === "shot") gesture.current.mode = "cancel";
  };
  const startCharge = (
    pointer: number | "controller",
    angle: number,
    origin?: Vec,
  ) => {
    cancelCharge();
    charge.current = {
      angle,
      pointer,
      origin,
      power: 0,
      limit: Math.max(
        80,
        Math.min(140, Math.min(size.width, size.height) * 0.35),
      ),
    };
    setChargePower(0);
    setCharging(true);
  };
  const updatePull = (point: Vec) => {
    const c = charge.current;
    if (!c?.origin) return;
    c.power = pullPower(
      c.origin,
      point,
      c.angle,
      cam.current.portrait,
      c.limit,
    );
    setChargePower(c.power);
    current.current.onPower(c.power);
  };
  const updateSlider = (x: number) => {
    const c = charge.current;
    if (!c?.slider || !c.origin) return;
    // Relative distance: touching the far end must never jump to full power.
    c.power = pullPower(c.origin, { x, y: 0 }, Math.PI, false, c.limit);
    setChargePower(c.power);
    current.current.onPower(c.power);
  };
  const releaseCharge = (pointer: number | "controller") => {
    const c = charge.current;
    if (!c || c.pointer !== pointer) return;
    cancelCharge();
    if (c.power >= 0.02 && current.current.canAim && !current.current.placing)
      current.current.onShoot(c.angle, c.power);
  };
  const [placementCursor, setPlacementCursor] = useState<Vec>({
    x: BAULK - 0.15,
    y: H / 2,
  });
  const cursor = useRef(placementCursor);
  const menuOpen = useRef(false);
  const previous = useRef(new Set<string>());
  const cancelled = useRef(0);
  const sampledAt = useRef(0);
  const pump = useRef<(at: number) => void>(() => {});
  const menuRoot = () => {
    if (!current.current.controlsEnabled) return null;
    const board = wrap.current?.closest<HTMLElement>(".snooker-board");
    if (!board) return null;
    const surface =
      board.closest<HTMLElement>("[data-fullscreen-board]") ?? board;
    const dialog = [
      ...surface.querySelectorAll<HTMLElement>('[role="dialog"],dialog[open]'),
    ]
      .filter((n) => n.getClientRects().length && !n.closest("[inert]"))
      .at(-1);
    if (dialog) return dialog;
    const active = document.activeElement;
    const control =
      active instanceof Element &&
      board.contains(active) &&
      active.closest("button,summary,input,select") &&
      !active.matches(".snooker-power-pull");
    return !current.current.canAim || menuOpen.current || control
      ? board
      : null;
  };
  const controller = useGameController({
    gameId: "snooker",
    userId: props.controllerUser,
    scope: props.controllerScope,
    root: wrap,
    enabled: () => current.current.controlsEnabled,
    menu: menuRoot,
    menuActions: {
      left: "navLeft",
      right: "navRight",
      up: "navUp",
      down: "navDown",
      fire: "confirm",
      menu: "cancel",
    },
    changed: () => pump.current(performance.now()),
    error: props.onControllerError,
  });
  const updateCursor = (point: Vec) => {
    let x = Math.min(0, point.x - BAULK),
      y = point.y - H / 2;
    const radius = Math.hypot(x, y);
    if (radius > D) {
      x *= D / radius;
      y *= D / radius;
    }
    cursor.current = { x: BAULK + x, y: H / 2 + y };
    setPlacementCursor(cursor.current);
  };
  useEffect(() => {
    const cue = current.current.balls.find((b) => b.id === 0);
    updateCursor(cue && !cue.down ? cue.pos : { x: BAULK - 0.15, y: H / 2 });
  }, [props.gestureKey, props.placing]);
  useEffect(() => {
    const board = wrap.current?.closest<HTMLElement>(".snooker-board");
    const back = (event: Event) => {
      if (
        event.target instanceof Element &&
        event.target.closest('[role="dialog"],dialog')
      )
        return;
      event.preventDefault();
      event.stopPropagation();
      menuOpen.current = false;
      cancelCharge();
      canvas.current?.focus({ preventScroll: true });
    };
    board?.addEventListener("controller-cancel", back);
    return () => board?.removeEventListener("controller-cancel", back);
  }, []);
  pump.current = (at: number) => {
    const device = controller.current;
    if (!device) return;
    const dt = Math.min(
      0.05,
      Math.max(0, sampledAt.current ? (at - sampledAt.current) / 1000 : 1 / 60),
    );
    sampledAt.current = at;
    const actions = device.sample(at),
      p = current.current;
    const interrupted = cancelled.current !== device.cancelVersion;
    cancelled.current = device.cancelVersion;
    if (
      interrupted &&
      !actions.has("fire") &&
      charge.current?.pointer === "controller"
    )
      cancelCharge();
    const edge = (a: string) => actions.has(a) && !previous.current.has(a);
    if (menuRoot() || !p.controlsEnabled || !p.canAim) {
      if (charge.current?.pointer === "controller") cancelCharge();
      previous.current = actions;
      return;
    }
    if (edge("menu") || edge("nextGroup") || edge("previousGroup")) {
      menuOpen.current = true;
      cancelCharge();
      wrap.current
        ?.closest(".snooker-board")
        ?.querySelector<HTMLElement>(
          ".snooker-controls summary,.snooker-decision button:not(:disabled),.snooker-secondary button",
        )
        ?.focus();
      previous.current = actions;
      return;
    }
    if (edge("cancel")) {
      cancelCharge();
      if (p.placing) p.onTogglePlace();
      previous.current = actions;
      return;
    }
    if (edge("place")) {
      cancelCharge();
      p.onTogglePlace();
    }
    const level = (a: string) => device.values.get(a) ?? 0;
    const dx = level("right") - level("left"),
      dy = level("down") - level("up");
    const precision = actions.has("fine") ? 0.1 : 1;
    if (p.placing) {
      if (dx || dy)
        updateCursor({
          x:
            cursor.current.x +
            (cam.current.portrait ? dy : dx) * dt * 0.35 * precision,
          y:
            cursor.current.y +
            (cam.current.portrait ? -dx : dy) * dt * 0.35 * precision,
        });
      if (edge("reset")) updateCursor({ x: BAULK - 0.15, y: H / 2 });
      if (edge("fire")) p.onPlace(cursor.current);
    } else {
      if (actions.has("spin")) {
        if (dx || dy) {
          p.onSpin(
            cueTip(
              p.spin.side + dx * dt * 0.7 * precision,
              p.spin.top - dy * dt * 0.7 * precision,
            ),
          );
          const panel = wrap.current
            ?.closest(".snooker-board")
            ?.querySelector<HTMLDetailsElement>(".snooker-spin");
          if (panel) panel.open = true;
        }
        if (edge("reset")) p.onSpin({ top: 0, side: 0 });
      } else {
        let angle = p.angle;
        if (dx) angle += ((dx * dt * Math.PI) / 3) * precision;
        if (edge("reset")) angle = 0;
        if (
          !charge.current &&
          !actions.has("fire") &&
          (edge("up") || edge("down"))
        )
          angle =
            (actions.has("up") ? -Math.PI / 2 : Math.PI / 2) -
            (cam.current.portrait ? Math.PI / 2 : 0);
        if (angle !== p.angle) {
          p.onAim(angle);
          if (charge.current?.pointer === "controller")
            charge.current.angle = angle;
        }
      }
      if (edge("fire") && !charge.current) startCharge("controller", p.angle);
      if (
        charge.current?.pointer === "controller" &&
        dy &&
        !actions.has("spin")
      ) {
        const value = Math.max(
          0,
          Math.min(1, charge.current.power - dy * dt * 0.5 * precision),
        );
        charge.current.power = value;
        setChargePower(value);
        p.onPower(value);
      }
      if (previous.current.has("fire") && !actions.has("fire") && !interrupted)
        releaseCharge("controller");
    }
    previous.current = actions;
  };
  useEffect(() => {
    let frame = 0;
    const sample = (at: number) => {
      pump.current(at);
      frame = requestAnimationFrame(sample);
    };
    frame = requestAnimationFrame(sample);
    return () => cancelAnimationFrame(frame);
  }, []);
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
    document.addEventListener("fullscreenchange", cancelCharge);
    return () => {
      current.current.onPower(0);
      charge.current = null;
      window.removeEventListener("blur", cancelCharge);
      window.removeEventListener("keydown", escape);
      document.removeEventListener("visibilitychange", hidden);
      document.removeEventListener("fullscreenchange", cancelCharge);
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
      img.crossOrigin = "anonymous";
      const path = `/snooker/v1/${name}.webp`;
      img.onload = () => {
        if (active) setArt((old) => ({ ...old, [name]: img }));
      };
      let local = !props.assets;
      img.onerror = () => {
        if (!local) {
          local = true;
          img.src = path;
        } else if (active) props.onLoadError?.("球桌图片加载失败，请重试");
      };
      img.src = props.assets ? props.assets.replace(/\/$/, "") + path : path;
    }
    return () => {
      active = false;
    };
  }, [props.assets]);
  useEffect(() => {
    if (Object.keys(art).length === 9) props.onReady?.();
  }, [art, props.onReady]);
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
    const width = Math.round(size.width * ratio),
      height = Math.round(size.height * ratio);
    if (el.width !== width) el.width = width;
    if (el.height !== height) el.height = height;
    ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
    ctx.clearRect(0, 0, size.width, size.height);
    const pull = charge.current;
    const pullPixels =
      pull && pull.power > 0
        ? 6 + ((pull.power - 0.02) / 0.98) * (pull.limit - 6)
        : 0;
    drawScene(
      ctx,
      camera,
      { ...props, pullBack: pullPixels / scaleOf(camera) },
      art,
    );
    if (props.placing) {
      const point = worldToScreen(placementCursor, camera);
      ctx.save();
      ctx.beginPath();
      ctx.arc(
        point.x,
        point.y,
        Math.max(8, R * scaleOf(camera)),
        0,
        Math.PI * 2,
      );
      ctx.fillStyle = "#fff9debb";
      ctx.fill();
      ctx.lineWidth = 2;
      ctx.strokeStyle = "#142f25";
      ctx.setLineDash([3, 2]);
      ctx.stroke();
      ctx.restore();
    }
    // The precision view has its own canvas outside the playing surface.
    // Its reserved layout space never covers a pocket or consumes table input.
    const detail = preview.current;
    if (!detail) return;
    const lensCtx = detail.getContext("2d")!;
    const edge = detail.clientWidth || 64;
    const detailSize = Math.round(edge * ratio);
    if (detail.width !== detailSize) detail.width = detailSize;
    if (detail.height !== detailSize) detail.height = detailSize;
    lensCtx.setTransform(ratio, 0, 0, ratio, 0, 0);
    lensCtx.fillStyle = "#254f40";
    lensCtx.fillRect(0, 0, edge, edge);
    const cue = props.balls.find((b) => b.id === 0);
    if (props.canAim && cue && !cue.down && !props.placing) {
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
      const base: Camera = {
        width: edge,
        height: edge,
        portrait: size.portrait,
        zoom: 1,
        pan: { x: 0, y: 0 },
      };
      const lens: Camera = { ...base, zoom: 700 / scaleOf(base) };
      const loc = worldToScreen(focus, lens);
      lens.pan = { x: edge / 2 - loc.x, y: edge / 2 - loc.y };
      drawScene(lensCtx, lens, { ...props, placing: false }, art);
    } else {
      lensCtx.fillStyle = "#fff7db";
      lensCtx.font = "12px sans-serif";
      lensCtx.textAlign = "center";
      lensCtx.fillText(
        props.placing ? "摆放白球" : "等待瞄准",
        edge / 2,
        edge / 2 + 4,
      );
    }
  }, [
    props.balls,
    props.flash,
    props.strikeCue,
    props.angle,
    props.power,
    props.canAim,
    props.finished,
    props.assist,
    props.placing,
    placementCursor,
    pose,
    size,
    art,
  ]);
  const point = (e: React.PointerEvent) => {
    const r = e.currentTarget.getBoundingClientRect();
    return { x: e.clientX - r.left, y: e.clientY - r.top };
  };
  const touchControls = touchInput || size.portrait;
  const sliderEnabled = props.canAim && !props.placing && !movingView;
  return (
    <div className="snooker-viewport">
      {!props.finished && (
        <div className="snooker-aim-tools">
          <div className="snooker-aim-preview">
            <canvas ref={preview} role="img" aria-label="瞄准细节放大视图" />
          </div>
          <div className="snooker-aim-readout">
            <strong>
              {touchControls ? "拖动球桌瞄准 · 滑动力度后松手击球" : "瞄准细节"}
            </strong>
            {charging && !touchControls && (
              <div
                className="snooker-charge"
                role="progressbar"
                aria-label="击球力度"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={Math.round(chargePower * 100)}
              >
                <span>拉杆 {Math.round(chargePower * 100)}% · 松开击球</span>
                <div>
                  <i style={{ width: `${chargePower * 100}%` }} />
                </div>
              </div>
            )}
            {!charging && props.flash && (
              <span className="snooker-respot-note" role="status">
                已复位：{props.flash.ids.map(ballName).join("、")}
              </span>
            )}
            {!charging && !props.flash && !touchControls && (
              <span>
                {movingView
                  ? "移动视图中，拖动只平移球桌"
                  : props.placing
                    ? "在 D 区点击摆放白球"
                    : props.strikeCue
                      ? "正在出杆…"
                      : props.canAim
                        ? "瞄准后向后拉杆，松开击球"
                        : "等待下一次瞄准"}
              </span>
            )}
            {touchControls && (
              <button
                type="button"
                role="slider"
                data-controller-ignore=""
                className={`snooker-power-pull${charging ? " snooker-charge" : ""}`}
                aria-label="击球力度，向右滑动增加力度，拉回起点取消，松开击球"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={charging ? Math.round(chargePower * 100) : 0}
                disabled={!sliderEnabled}
                onBlur={cancelCharge}
                onPointerDown={(e) => {
                  if (!sliderEnabled || e.button !== 0) return;
                  if (!e.isPrimary || charge.current || pointers.current.size) {
                    cancelCharge();
                    return;
                  }
                  e.currentTarget.focus({ preventScroll: true });
                  e.currentTarget.setPointerCapture(e.pointerId);
                  startCharge(e.pointerId, props.angle, { x: e.clientX, y: 0 });
                  charge.current!.slider = true;
                  charge.current!.limit = Math.max(
                    80,
                    Math.min(140, e.currentTarget.clientWidth - 30),
                  );
                }}
                onPointerMove={(e) => {
                  if (charge.current?.pointer === e.pointerId)
                    updateSlider(e.clientX);
                }}
                onPointerUp={(e) => {
                  if (charge.current?.pointer === e.pointerId) {
                    updateSlider(e.clientX);
                    releaseCharge(e.pointerId);
                  }
                  if (e.currentTarget.hasPointerCapture(e.pointerId))
                    e.currentTarget.releasePointerCapture(e.pointerId);
                }}
                onPointerCancel={cancelCharge}
                onLostPointerCapture={(e) => {
                  if (charge.current?.pointer === e.pointerId) cancelCharge();
                }}
              >
                <i
                  aria-hidden="true"
                  style={{ width: `${charging ? chargePower * 100 : 0}%` }}
                />
                <span>
                  {!sliderEnabled
                    ? props.placing
                      ? "先在 D 区摆放白球"
                      : movingView
                        ? "移动视图中"
                        : "等待击球"
                    : charging
                      ? `力度 ${Math.round(chargePower * 100)}% · 拉回取消`
                      : "向右滑动调力度 →"}
                </span>
              </button>
            )}
          </div>
        </div>
      )}
      <div
        className="snooker-table"
        ref={wrap}
        data-zoom={pose.zoom.toFixed(3)}
        data-angle={props.angle}
        data-charging={charging}
        data-charge-power={charging ? chargePower : 0}
        data-placing={props.placing}
        data-cursor-x={placementCursor.x}
        data-cursor-y={placementCursor.y}
        data-spin-top={props.spin.top}
        data-spin-side={props.spin.side}
        data-respot={props.flash?.ids.join(",") ?? ""}
      >
        <canvas
          ref={canvas}
          data-controller-ignore=""
          tabIndex={0}
          aria-label="斯诺克球桌，触屏点击或拖动瞄准，通过力度条击球；鼠标移动瞄准，按住向后拉杆，松开击球；按住击球键调整力度，松开击球；Esc 取消"
          aria-disabled={!props.canAim}
          data-angle={props.angle}
          data-cue-offset={props.strikeCue?.offset}
          onBlur={cancelCharge}
          onPointerDown={(e) => {
            if (e.button !== 0) return;
            if (e.pointerType === "touch") setTouchInput(true);
            if (charge.current?.slider) cancelCharge();
            const p = point(e);
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
                    ? e.pointerType === "touch"
                      ? "aim"
                      : "shot"
                    : "cancel";
            if ((mode === "shot" || mode === "aim") && cue) {
              const world = screenToWorld(p, cam.current);
              const angle = Math.atan2(
                world.y - cue.pos.y,
                world.x - cue.pos.x,
              );
              props.onAim(angle);
              if (mode === "shot") startCharge(e.pointerId, angle, p);
            }
            gesture.current = { start: p, dragged: false, mode };
          }}
          onPointerMove={(e) => {
            const old = pointers.current.get(e.pointerId);
            if (!old) {
              const cue = props.balls.find((b) => b.id === 0);
              if (
                e.pointerType === "mouse" &&
                e.buttons === 0 &&
                !pointers.current.size &&
                !charge.current &&
                !movingView &&
                props.canAim &&
                !props.placing &&
                cue &&
                !cue.down
              ) {
                const world = screenToWorld(point(e), cam.current);
                props.onAim(
                  Math.atan2(world.y - cue.pos.y, world.x - cue.pos.x),
                );
              }
              return;
            }
            const before = [...pointers.current.values()],
              p = point(e);
            pointers.current.set(e.pointerId, p);
            const after = [...pointers.current.values()];
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
              updatePull(p);
            } else if (g.mode === "aim" && props.canAim && !props.placing) {
              const cue = props.balls.find((b) => b.id === 0);
              if (cue && !cue.down) {
                const world = screenToWorld(p, cam.current);
                props.onAim(
                  Math.atan2(world.y - cue.pos.y, world.x - cue.pos.x),
                );
              }
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
            if (g?.mode === "shot") {
              updatePull(p);
              releaseCharge(e.pointerId);
            } else {
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
      </div>
      <div className="snooker-camera">
        <span>
          {movingView
            ? "拖动平移 · 滚轮／双指缩放"
            : touchControls
              ? "点按或拖动瞄准 · 力度条松手击球"
              : "移动瞄准 · 向后拉杆 · 松开击球 · Esc 取消"}
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
