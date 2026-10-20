import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
} from "react";
import {
  Bomb,
  ChevronLeft,
  ChevronRight,
  Crosshair,
  FlipHorizontal,
} from "lucide-react";
import type { BoardProps } from "../../../web/src/game-api/module";
import {
  cellSize,
  clamp,
  explosion,
  mapById,
  maps,
  sourceList,
  statusText,
  tankInfo,
  tankPortraits,
  tanks as tankCatalog,
  tankName,
  teamColors,
  type Arc,
  type Crater,
  type Event as GameEvent,
  type MapInfo,
  type Shot,
  type Tank,
  type View,
} from "./types";
import {
  aimFromPoint,
  carveCraters,
  decodeMask,
  pointerWorld,
  previewPath,
  stepPower,
  type Camera,
} from "./aim";

type ImageMap = Record<string, HTMLImageElement | null>;

const ART_SCALE = 2.4;
const MOBILE_VIEW = 560;
const MID_VIEW = 900;
const DESKTOP_VIEW = 1200;

const imageCache = new Map<string, Promise<HTMLImageElement | null>>();

function loadFirst(sources: string[]): Promise<HTMLImageElement | null> {
  if (!sources.length) return Promise.resolve(null);
  const key = sources.join("|");
  const existing = imageCache.get(key);
  if (existing) return existing;
  const promise = new Promise<HTMLImageElement | null>((resolve) => {
    const img = new Image();
    img.decoding = "async";
    let index = 0;
    const fail = () => {
      index++;
      if (index >= sources.length) {
        resolve(null);
        return;
      }
      img.src = sources[index];
    };
    img.onload = () => resolve(img);
    img.onerror = fail;
    img.src = sources[0];
  });
  imageCache.set(key, promise);
  return promise;
}

function useImage(sources: string[]): HTMLImageElement | null {
  const key = sources.join("|");
  const [image, setImage] = useState<HTMLImageElement | null>(null);
  useEffect(() => {
    let live = true;
    if (!sources.length) {
      setImage(null);
      return;
    }
    loadFirst(sources).then((img) => {
      if (live) setImage(img);
    });
    return () => {
      live = false;
    };
  }, [key]);
  return image;
}

function useImages(list: string[][]): ImageMap {
  const keys = list.map((sources) => sources.join("|"));
  const [images, setImages] = useState<ImageMap>({});
  useEffect(() => {
    let live = true;
    const pending = list.map((sources, i) => loadFirst(sources).then((img) => [keys[i], img] as const));
    Promise.all(pending).then((entries) => {
      if (!live) return;
      const next: ImageMap = {};
      for (const [key, img] of entries) next[key] = img;
      setImages(next);
    });
    return () => {
      live = false;
    };
  }, [keys.join(";")]);
  return images;
}

function assetMap(base: string, map: MapInfo) {
  return {
    bg: map.bg ? sourceList(base, map.bg) : [],
    fg: sourceList(base, map.fg),
    fg2: map.fg2 ? sourceList(base, map.fg2) : [],
  };
}

type Animation = {
  shot: Shot;
  start: number;
  duration: number;
  impactAt: number;
  beforeCraters: Crater[];
  beforeTanks: Tank[];
  beforeEvents: GameEvent[];
  impactPlayed: boolean;
};

const FALLBACK_BG = "#2c3948";

function useReducedMotion() {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    const media = window.matchMedia("(prefers-reduced-motion: reduce)");
    const update = () => setReduced(media.matches);
    update();
    media.addEventListener?.("change", update);
    return () => media.removeEventListener?.("change", update);
  }, []);
  return reduced;
}

function serverNow(clock: BoardProps<View>["clock"]) {
  return clock.serverNow + Math.max(0, performance.now() - clock.receivedAt);
}

function fmtSeconds(ms: number) {
  return Math.max(0, Math.ceil(ms / 1000));
}

function eventName(view: View, seat: number) {
  return view.tanks[seat]?.seat === seat ? `P${seat + 1}` : `P${seat + 1}`;
}

function pathPoint(arc: Arc, progress: number) {
  const path = arc.path;
  if (!path.length) return { x: 0, y: 0, angle: 0 };
  if (path.length === 1) return { x: path[0].x, y: path[0].y, angle: 0 };
  const lengths: number[] = [];
  let total = 0;
  for (let i = 1; i < path.length; i++) {
    const d = Math.hypot(path[i].x - path[i - 1].x, path[i].y - path[i - 1].y);
    lengths.push(d);
    total += d;
  }
  let target = clamp(progress, 0, 1) * total;
  for (let i = 1; i < path.length; i++) {
    if (target <= lengths[i - 1] || i === path.length - 1) {
      const prev = path[i - 1];
      const next = path[i];
      const t = lengths[i - 1] > 0 ? clamp(target / lengths[i - 1], 0, 1) : 0;
      return {
        x: prev.x + (next.x - prev.x) * t,
        y: prev.y + (next.y - prev.y) * t,
        angle: Math.atan2(next.y - prev.y, next.x - prev.x),
      };
    }
    target -= lengths[i - 1];
  }
  const last = path[path.length - 1];
  return { x: last.x, y: last.y, angle: 0 };
}

function drawTerrain(
  canvas: HTMLCanvasElement,
  map: MapInfo,
  fg: HTMLImageElement | null,
  fg2: HTMLImageElement | null,
  craters: Crater[],
) {
  const ctx = canvas.getContext("2d");
  if (!ctx) return;
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  ctx.globalCompositeOperation = "source-over";
  if (fg) ctx.drawImage(fg, 0, map.offY);
  if (fg2) {
    ctx.globalAlpha = 0.55;
    ctx.drawImage(fg2, 0, map.offY);
    ctx.globalAlpha = 1;
  }
  ctx.globalCompositeOperation = "destination-out";
  for (const c of craters) {
    ctx.beginPath();
    ctx.arc(c.x, c.y, c.r, 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.globalCompositeOperation = "source-over";
}

function drawTank(
  ctx: CanvasRenderingContext2D,
  view: View,
  tank: Tank,
  img: HTMLImageElement | null,
  info: { sprites: Record<string, { ax: number; ay: number; w: number; h: number; url: string }> } | undefined,
  now: number,
  anim: Animation | null,
) {
  if (!tank.selected) return;
  const sprite = info?.sprites.idle;
  const die = info?.sprites.die;
  const useDie = !tank.alive && die;
  const source = useDie ? img : img;
  if (!source) {
    ctx.fillStyle = teamColors[tank.team] ?? "#888";
    ctx.fillRect(tank.x - 14, tank.y - 28, 28, 26);
    return;
  }
  const spriteMeta = useDie ? die : sprite;
  if (!spriteMeta) return;
  const current = tank.current && view.phase === "playing";
  const hitRecent =
    anim && now >= anim.impactAt && now < anim.impactAt + 260 && anim.shot.blasts.some((b) => b.victim === tank.seat);
  ctx.save();
  // Team marker under the tank.
  ctx.globalAlpha = useDie ? 0.55 : 1;
  ctx.fillStyle = teamColors[tank.team] ?? "#888";
  ctx.beginPath();
  ctx.ellipse(tank.x, tank.y + 1, 34, 10, 0, 0, Math.PI * 2);
  ctx.fill();
  if (current) {
    ctx.strokeStyle = "#f4dc8a";
    ctx.lineWidth = 2.5;
    ctx.beginPath();
    ctx.ellipse(tank.x, tank.y + 1, 42 + Math.sin(now / 180) * 2, 12, 0, 0, Math.PI * 2);
    ctx.stroke();
  }
  ctx.translate(tank.x, tank.y);
  ctx.scale(tank.facing < 0 ? -1 : 1, 1);
  const artScale = useDie ? ART_SCALE * 0.78 : ART_SCALE;
  ctx.drawImage(source, -spriteMeta.ax * artScale, -spriteMeta.ay * artScale, source.width * artScale, source.height * artScale);
  if (hitRecent) {
    ctx.globalCompositeOperation = "source-atop";
    ctx.fillStyle = "rgba(255,255,255,.65)";
    ctx.fillRect(-spriteMeta.ax * artScale, -spriteMeta.ay * artScale, source.width * artScale, source.height * artScale);
  }
  ctx.restore();
  if (!useDie) {
    const width = 74;
    const x = tank.x - width / 2;
    const y = tank.y - 104;
    ctx.fillStyle = "rgba(12,18,24,.82)";
    ctx.fillRect(x - 2, y - 2, width + 4, 10);
    ctx.fillStyle = teamColors[tank.team] ?? "#888";
    ctx.fillRect(x, y, width * clamp(tank.hp / Math.max(1, tank.maxHp), 0, 1), 6);
    ctx.fillStyle = "#f7f3e6";
    ctx.font = "bold 13px system-ui, sans-serif";
    ctx.textAlign = "center";
    ctx.lineWidth = 3;
    ctx.strokeStyle = "rgba(5,15,25,.9)";
    ctx.strokeText(`P${tank.seat + 1} ${tank.hp}`, tank.x, y - 7);
    ctx.fillText(`P${tank.seat + 1} ${tank.hp}`, tank.x, y - 7);
    if (tank.statuses?.length) {
      ctx.fillStyle = "#f0d58c";
      ctx.font = "11px system-ui, sans-serif";
      const status = tank.statuses.map(statusText).join(" ");
      ctx.strokeText(status, tank.x, y - 21);
      ctx.fillText(status, tank.x, y - 21);
    }
  }
}

function Board({ room, data, act, busy, assets, sound, audio, clock }: BoardProps<View>) {
  const tanks: Tank[] = Array.isArray(data?.tanks) ? data.tanks : [];
  const craters: Crater[] = Array.isArray(data?.craters) ? data.craters : [];
  const events: GameEvent[] = Array.isArray(data?.events) ? data.events : [];
  const log: string[] = Array.isArray(data?.log) ? data.log : [];
  const phase = data?.phase ?? "";
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const terrainRef = useRef<HTMLCanvasElement | null>(null);
  const cameraRef = useRef<Camera>({ x: 0, y: 0, worldViewW: DESKTOP_VIEW, worldViewH: 600 });
  const pointerType = useRef("mouse");
  const [size, setSize] = useState({ w: 960, h: 560 });
  const [anim, setAnim] = useState<Animation | null>(null);
  const [angle, setAngle] = useState(45);
  const [power, setPower] = useState(65);
  const [facing, setFacing] = useState(1);
  const [weapon, setWeapon] = useState<"primary" | "special">("primary");
  const [now, setNow] = useState(Date.now());
  const lastView = useRef<View | null>(null);
  const seenShot = useRef<number | null>(null);
  const seenEpoch = useRef<string>("");
  const lastEvent = useRef<number>(0);
  const eventEpoch = useRef<string>("");
  const eventReady = useRef(false);
  const lastTurn = useRef<string>("");

  const reducedMotion = useReducedMotion();
  const map = mapById.get(data.mapId ?? "") ?? maps[0];
  const mapAssets = useMemo(() => assetMap(assets, map), [assets, map]);
  const bgImage = useImage(mapAssets.bg);
  const fgImage = useImage(mapAssets.fg);
  const fg2Image = useImage(mapAssets.fg2);
  const explosionImage = useImage(sourceList(assets, explosion.url));
  const tankKeys = useMemo(
    () =>
      tankCatalog.flatMap((info) => [
        sourceList(assets, info.sprites.idle?.url ?? ""),
        sourceList(assets, info.sprites.die?.url ?? ""),
        info.bullet ? sourceList(assets, info.bullet.url) : [],
      ]),
    [assets],
  );
  const images = useImages(tankKeys);

  const currentTank = tanks[data.current] ?? tanks[data.you] ?? tanks.find((t) => t.alive);
  const myTurn = phase === "playing" && data.you >= 0 && data.you === data.current && !busy;
  const canAct = myTurn && !anim;
  const own = tanks[data.you];
  const myInfo = tankInfo(own?.tankId ?? 0);
  const maxAngle = Math.max(5, myInfo?.angle ?? 45);
  const baseMask = useMemo(() => decodeMask(map.mask), [map]);
  const craterSignature = useMemo(
    () => craters.map((crater) => `${Math.round(crater.x)},${Math.round(crater.y)},${Math.round(crater.r)}`).join(";"),
    [craters],
  );
  const terrainMask = useMemo(
    () => carveCraters(baseMask, map.maskCols, map.maskRows, cellSize, craters),
    [baseMask, map, craterSignature],
  );
  const aimPreview = useMemo(() => {
    if (!canAct || phase !== "playing" || !own?.alive || !own.selected) return [];
    return previewPath({
      width: map.width,
      height: map.height,
      mask: terrainMask,
      maskCols: map.maskCols,
      maskRows: map.maskRows,
      cell: cellSize,
      startX: own.x,
      startY: own.y,
      angle,
      power,
      facing,
      rangeScale: (myInfo?.range ?? 50) / 50,
      wind: data.wind,
      limit: 220,
    });
  }, [
    angle,
    canAct,
    data.wind,
    facing,
    map,
    myInfo?.range,
    own,
    phase,
    power,
    terrainMask,
  ]);

  // Keep the control values legal when the selected tank changes.
  useEffect(() => {
    const key = `${data.epoch}:${data.turn}:${data.current}`;
    if (lastTurn.current === key) return;
    lastTurn.current = key;
    if (data.you >= 0 && data.current === data.you && phase === "playing") {
      const self = tanks[data.you];
      const foes = tanks.filter((t) => t.alive && t.team !== self.team);
      const aimAt = foes.sort(
        (a, b) => Math.abs(a.x - self.x) - Math.abs(b.x - self.x),
      )[0];
      setFacing(aimAt && aimAt.x < self.x ? -1 : 1);
      setAngle(Math.min(maxAngle, 45));
      setPower(65);
      setWeapon("primary");
    }
  }, [data.epoch, data.turn, data.current, data.you, data.phase, tanks, maxAngle]);

  // Detect a new shot and animate it once. On a fresh mount the newest shot is
  // treated as already played, so refresh/reconnect never replays old actions.
  useEffect(() => {
    if (seenEpoch.current !== data.epoch) {
      seenEpoch.current = data.epoch;
      lastView.current = data;
      seenShot.current = data.lastShot?.id ?? null;
      setAnim(null);
      return;
    }
    const shot = data.lastShot;
    if (!shot) {
      lastView.current = data;
      return;
    }
    if (seenShot.current === null) {
      seenShot.current = shot.id;
      lastView.current = data;
      return;
    }
    if (shot.id <= seenShot.current) {
      lastView.current = data;
      return;
    }
    const before = lastView.current ?? data;
    if (reducedMotion) {
      seenShot.current = shot.id;
      lastView.current = data;
      setAnim(null);
      return;
    }
    const path = shot.arcs[0]?.path ?? [];
    let length = 0;
    for (let i = 1; i < path.length; i++) length += Math.hypot(path[i].x - path[i - 1].x, path[i].y - path[i - 1].y);
    const duration = clamp(length * 4.5, 520, 2200);
    seenShot.current = shot.id;
    setAnim({
      shot,
      start: performance.now(),
      duration,
      impactAt: duration * 0.86,
      beforeCraters: before.craters ?? [],
      beforeTanks: before.tanks,
      beforeEvents: before.events,
      impactPlayed: false,
    });
    lastView.current = data;
  }, [data, reducedMotion]);

  // Update the displayed clock and finish animations.
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 120);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    if (!anim) return;
    const total = anim.duration + 900;
    const timer = window.setTimeout(() => setAnim(null), total);
    return () => window.clearTimeout(timer);
  }, [anim]);

  // Audio: keep one cursor so refresh does not replay. Music follows the map.
  const musicStarted = useRef("");
  useEffect(() => {
    if (!sound || !map.music) return;
    const key = `${data.epoch}:${map.id}`;
    if (musicStarted.current === key) return;
    musicStarted.current = key;
    void audio.music(sourceList(assets, map.music));
  }, [audio, assets, data.epoch, map.id, map.music, sound]);

  useEffect(() => {
    if (!sound) {
      lastEvent.current = Math.max(0, ...events.map((e) => e.id));
      eventReady.current = true;
      eventEpoch.current = data.epoch;
      return;
    }
    if (!eventReady.current || eventEpoch.current !== data.epoch) {
      lastEvent.current = Math.max(0, ...events.map((e) => e.id));
      eventEpoch.current = data.epoch;
      eventReady.current = true;
      return;
    }
    const fresh = events.filter((e) => e.id > lastEvent.current);
    lastEvent.current = Math.max(lastEvent.current, ...events.map((e) => e.id));
    for (const event of fresh) {
      const sources =
        event.type === "shot"
          ? sourceList(assets, "/fortressblue/v2/audio/fire.mp3")
          : event.type === "hit" || event.type === "explode"
            ? sourceList(assets, "/fortressblue/v2/audio/explode.mp3")
            : event.type === "death"
              ? sourceList(assets, "/fortressblue/v2/audio/hit.mp3")
              : event.type === "turn"
                ? sourceList(assets, "/fortressblue/v2/audio/turn.mp3")
                : event.type === "finish"
                  ? sourceList(assets, "/fortressblue/v2/audio/win.mp3")
                  : [];
      if (sources.length) void audio.playWithFallback(sources);
    }
  }, [audio, assets, events, sound]);

  // Canvas size follows its container; the world camera uses CSS pixels.
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const resize = () => {
      const rect = canvas.getBoundingClientRect();
      const dpr = Math.min(2, window.devicePixelRatio || 1);
      const w = Math.max(320, Math.round(rect.width));
      const h = Math.max(220, Math.round(rect.height));
      canvas.width = Math.round(w * dpr);
      canvas.height = Math.round(h * dpr);
      setSize({ w, h });
    };
    resize();
    const observer = new ResizeObserver(resize);
    observer.observe(canvas);
    return () => observer.disconnect();
  }, []);

  const displayCraters = useMemo(() => {
    if (!anim) return craters ?? [];
    const elapsed = now - anim.start;
    if (elapsed < anim.impactAt) return anim.beforeCraters;
    return craters ?? [];
  }, [anim, craters, now]);

  // Terrain is cached and only repainted when the map or crater set changes.
  const terrainKey = `${map.id}:${displayCraters.map((c) => `${Math.round(c.x)},${Math.round(c.y)},${Math.round(c.r)}`).join(";")}`;
  const terrainKeyRef = useRef("");
  useEffect(() => {
    const canvas = terrainRef.current;
    if (!canvas) return;
    if (terrainKeyRef.current === terrainKey) return;
    terrainKeyRef.current = terrainKey;
    drawTerrain(canvas, map, fgImage, fg2Image, displayCraters);
  }, [terrainKey, map, fgImage, fg2Image, displayCraters]);

  const draw = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    const dpr = Math.min(2, window.devicePixelRatio || 1);
    const cw = size.w;
    const ch = size.h;
    const mobile = cw < 620;
    const worldViewW = Math.min(map.width, mobile ? MOBILE_VIEW : cw < 1000 ? MID_VIEW : DESKTOP_VIEW);
    const scale = cw / worldViewW;
    const worldViewH = ch / scale;
    const focus = anim
      ? pathPoint(anim.shot.arcs[0], clamp((now - anim.start) / Math.max(1, anim.impactAt), 0, 1))
      : currentTank ?? { x: map.width / 2, y: map.height * 0.7 };
    const camX = clamp(
      focus.x - worldViewW / 2,
      0,
      Math.max(0, map.width - worldViewW),
    );
    const camY = clamp(
      focus.y - worldViewH * 0.58,
      0,
      Math.max(0, map.height - worldViewH),
    );
    cameraRef.current = { x: camX, y: camY, worldViewW, worldViewH };
    canvas.dataset.cameraX = String(camX);
    canvas.dataset.cameraY = String(camY);
    canvas.dataset.viewW = String(worldViewW);
    canvas.dataset.viewH = String(worldViewH);
    canvas.dataset.scale = String(scale);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.fillStyle = FALLBACK_BG;
    ctx.fillRect(0, 0, cw, ch);
    ctx.save();
    ctx.setTransform(dpr * scale, 0, 0, dpr * scale, -camX * dpr * scale, -camY * dpr * scale);
    if (bgImage) ctx.drawImage(bgImage, 0, 0, map.width, map.height);
    else {
      ctx.fillStyle = "#50657a";
      ctx.fillRect(0, 0, map.width, map.height);
    }
    if (terrainRef.current) ctx.drawImage(terrainRef.current, 0, 0);
    else if (fgImage) ctx.drawImage(fgImage, 0, map.offY);
    if (canAct && aimPreview.length > 1) {
      ctx.save();
      ctx.strokeStyle = "rgba(255,232,150,.62)";
      ctx.lineWidth = 1.6 / scale;
      ctx.setLineDash([3 / scale, 7 / scale]);
      ctx.beginPath();
      ctx.moveTo(aimPreview[0].x, aimPreview[0].y);
      for (const point of aimPreview.slice(1)) ctx.lineTo(point.x, point.y);
      ctx.stroke();
      ctx.setLineDash([]);
      const impact = aimPreview[aimPreview.length - 1];
      ctx.strokeStyle = "rgba(255,240,180,.92)";
      ctx.lineWidth = 1.5 / scale;
      ctx.beginPath();
      ctx.arc(impact.x, impact.y, 9, 0, Math.PI * 2);
      ctx.moveTo(impact.x - 15, impact.y);
      ctx.lineTo(impact.x + 15, impact.y);
      ctx.moveTo(impact.x, impact.y - 15);
      ctx.lineTo(impact.x, impact.y + 15);
      ctx.stroke();
      ctx.restore();
    }
    const renderTanks = anim && now < anim.impactAt ? anim.beforeTanks : tanks;
    for (const tank of renderTanks) {
      const info = tankInfo(tank.tankId);
      const sprite = tank.alive ? info?.sprites.idle : info?.sprites.die;
      const key = sprite ? sourceList(assets, sprite.url).join("|") : "";
      drawTank(ctx, data, tank, images[key] ?? null, info, now, anim);
    }
    if (anim) {
      const elapsed = now - anim.start;
      if (elapsed <= anim.impactAt) {
        const p = pathPoint(anim.shot.arcs[0], elapsed / Math.max(1, anim.impactAt));
        const shooter = tankInfo(tanks[anim.shot.seat]?.tankId ?? 0);
        const bullet = shooter?.bullet;
        if (bullet) {
          const img = images[sourceList(assets, bullet.url).join("|")];
          if (img) {
            ctx.save();
            ctx.translate(p.x, p.y);
            ctx.rotate(Math.abs(p.angle) > Math.PI / 2 ? p.angle + Math.PI : p.angle);
            const s = 1.0;
            ctx.drawImage(img, -img.width * s / 2, -img.height * s / 2, img.width * s, img.height * s);
            ctx.restore();
          } else {
            ctx.fillStyle = "#ffe7a2";
            ctx.beginPath();
            ctx.arc(p.x, p.y, 4, 0, Math.PI * 2);
            ctx.fill();
          }
        }
      } else {
        const impact = anim.shot.blasts[0] ?? { x: anim.shot.arcs[0]?.impact.x ?? 0, y: anim.shot.arcs[0]?.impact.y ?? 0 };
        const progress = clamp((elapsed - anim.impactAt) / 900, 0, 1);
        const frame = Math.min(explosion.count - 1, Math.floor(progress * explosion.count));
        if (explosionImage) {
          const sheet = explosionImage;
          const cellW = explosion.cellW;
          const cellH = explosion.cellH;
          const sx = frame * cellW;
          const sy = 0;
          ctx.globalAlpha = 1 - progress * 0.35;
          const blastRadius = Math.max(32, anim.shot.blasts[0]?.r ?? 48);
          const drawW = blastRadius * 2.6;
          const drawH = drawW * (cellH / cellW);
          ctx.drawImage(sheet, sx, sy, cellW, cellH, impact.x - drawW / 2, impact.y - drawH * 0.5, drawW, drawH);
          ctx.globalAlpha = 1;
        }
      }
    }
    ctx.restore();

    if (phase === "playing" && own?.selected) {
      ctx.save();
      for (const tank of renderTanks) {
        if (!tank.alive || tank.team === own.team) continue;
        const sx = (tank.x - camX) * scale;
        const sy = (tank.y - camY) * scale;
        if (sx >= 14 && sx <= cw - 14 && sy >= 14 && sy <= ch - 14) continue;
        const ex = clamp(sx, 18, cw - 18);
        const ey = clamp(sy, 18, ch - 18);
        ctx.save();
        ctx.translate(ex, ey);
        ctx.rotate(Math.atan2(sy - ey, sx - ex));
        ctx.fillStyle = teamColors[tank.team] ?? "#fff";
        ctx.beginPath();
        ctx.moveTo(10, 0);
        ctx.lineTo(-4, -6);
        ctx.lineTo(-4, 6);
        ctx.closePath();
        ctx.fill();
        ctx.restore();
        ctx.fillStyle = "#102230";
        ctx.font = "bold 11px system-ui, sans-serif";
        ctx.textAlign = "center";
        ctx.fillText(`P${tank.seat + 1}`, ex, ey + 4);
      }
      ctx.restore();
    }

    // Screen-space minimap and wind bracket.
    const mmW = mobile ? 108 : 176;
    const mmH = Math.round((mmW * map.height) / map.width);
    const mmX = cw - mmW - 10;
    const mmY = 10;
    ctx.save();
    ctx.fillStyle = "rgba(8,16,26,.68)";
    ctx.fillRect(mmX - 3, mmY - 3, mmW + 6, mmH + 6);
    if (bgImage) ctx.drawImage(bgImage, mmX, mmY, mmW, mmH);
    else ctx.fillStyle = "#24394d", ctx.fillRect(mmX, mmY, mmW, mmH);
    if (terrainRef.current) ctx.drawImage(terrainRef.current, mmX, mmY, mmW, mmH);
    for (const tank of renderTanks) {
      if (!tank.selected || !tank.alive) continue;
      const x = mmX + (tank.x / map.width) * mmW;
      const y = mmY + (tank.y / map.height) * mmH;
      ctx.fillStyle = teamColors[tank.team] ?? "#fff";
      ctx.beginPath();
      ctx.arc(x, y, tank.current ? 4 : 3, 0, Math.PI * 2);
      ctx.fill();
      if (tank.current) {
        ctx.strokeStyle = "#ffe6a1";
        ctx.lineWidth = 1.5;
        ctx.stroke();
      }
    }
    const viewX = mmX + (camX / map.width) * mmW;
    const viewW = (worldViewW / map.width) * mmW;
    const viewY = mmY + (camY / map.height) * mmH;
    const viewH = (worldViewH / map.height) * mmH;
    ctx.strokeStyle = "rgba(255,236,164,.8)";
    ctx.lineWidth = 1;
    ctx.strokeRect(viewX, viewY, viewW, viewH);
    ctx.restore();
  }, [aimPreview, anim, assets, bgImage, canAct, currentTank, data, explosionImage, fgImage, images, map, now, own, size, terrainRef]);

  useEffect(() => {
    draw();
  }, [draw]);

  useEffect(() => {
    if (!anim) return;
    let raf = 0;
    const loop = () => {
      draw();
      if (performance.now() < anim.start + anim.duration + 920) raf = requestAnimationFrame(loop);
    };
    raf = requestAnimationFrame(loop);
    return () => cancelAnimationFrame(raf);
  }, [anim, draw]);

  const aimRef = useRef({ angle, power, facing });
  useEffect(() => {
    aimRef.current = { angle, power, facing };
  }, [angle, facing, power]);

  const sendFire = useCallback(
    (aim?: { angle: number; facing: number }) => {
      if (!canAct || phase !== "playing") return;
      const current = aim ?? aimRef.current;
      void act({
        type: "fire",
        epoch: data.epoch,
        turn: data.turn,
        angle: clamp(current.angle, 0, maxAngle),
        power: clamp(aimRef.current.power, 20, 100),
        facing: current.facing,
        weapon,
      });
    },
    [act, canAct, data.epoch, data.turn, maxAngle, phase, weapon],
  );

  const move = useCallback(
    (dir: number) => {
      if (!canAct || phase !== "playing") return;
      void act({ type: "move", epoch: data.epoch, turn: data.turn, dir });
    },
    [act, canAct, data.epoch, data.turn, phase],
  );

  const toggleFacing = useCallback(() => {
    const next = aimRef.current.facing > 0 ? -1 : 1;
    aimRef.current = { ...aimRef.current, facing: next };
    setFacing(next);
  }, []);

  const aimAtEvent = useCallback(
    (event: ReactPointerEvent<HTMLCanvasElement>) => {
      pointerType.current = event.pointerType;
      if (!canAct || phase !== "playing" || !own) return;
      const rect = event.currentTarget.getBoundingClientRect();
      const point = pointerWorld(
        event.clientX,
        event.clientY,
        rect,
        cameraRef.current,
      );
      const next = aimFromPoint(
        { x: own.x, y: own.y },
        point,
        maxAngle,
        26,
      );
      const angleValue = Math.round(next.angle);
      aimRef.current = {
        ...aimRef.current,
        angle: angleValue,
        facing: next.facing,
      };
      setAngle(angleValue);
      setFacing(next.facing);
    },
    [canAct, maxAngle, own, phase],
  );

  // Native wheel listener: React's root wheel listener is passive, so the
  // page would scroll while the player tries to tune power.
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const onWheel = (event: WheelEvent) => {
      if (!canAct || phase !== "playing") return;
      event.preventDefault();
      const next = stepPower(aimRef.current.power, event.deltaY);
      aimRef.current = { ...aimRef.current, power: next };
      setPower(next);
    };
    canvas.addEventListener("wheel", onWheel, { passive: false });
    return () => canvas.removeEventListener("wheel", onWheel);
  }, [canAct, phase]);

  const choose = useCallback(
    (tankId: number) => {
      if (phase !== "select" || !data.canSelect || busy) return;
      void act({ type: "select", epoch: data.epoch, tank: tankId });
    },
    [act, busy, data.canSelect, data.epoch, data.phase],
  );

  // Keyboard controls follow the original arcade layout: arrows move and aim,
  // while Z/X tune power. The game claims browser keys only during its turn.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (!canAct || phase !== "playing") return;
      const target = event.target as HTMLElement | null;
      const tag = target?.tagName ?? "";
      if (
        target?.isContentEditable ||
        tag === "TEXTAREA" ||
        (tag === "INPUT" && (target as HTMLInputElement).type !== "range")
      ) {
        return;
      }
      const key = event.key.toLowerCase();
      const claimed = [
        "arrowup",
        "arrowdown",
        "arrowleft",
        "arrowright",
        " ",
        "enter",
        "w",
        "s",
        "a",
        "d",
        "q",
        "e",
        "z",
        "x",
      ];
      if (claimed.includes(key)) event.preventDefault();
      if (key === "w" || key === "arrowup") {
        const next = clamp(aimRef.current.angle + 2, 0, maxAngle);
        aimRef.current = { ...aimRef.current, angle: next };
        setAngle(next);
      }
      if (key === "s" || key === "arrowdown") {
        const next = clamp(aimRef.current.angle - 2, 0, maxAngle);
        aimRef.current = { ...aimRef.current, angle: next };
        setAngle(next);
      }
      if (key === "x") {
        const next = stepPower(aimRef.current.power, -1);
        aimRef.current = { ...aimRef.current, power: next };
        setPower(next);
      }
      if (key === "z") {
        const next = stepPower(aimRef.current.power, 1);
        aimRef.current = { ...aimRef.current, power: next };
        setPower(next);
      }
      if (key === "arrowleft" || key === "a") void move(-1);
      if (key === "arrowright" || key === "d") void move(1);
      if (key === "q" || key === "e") {
        setWeapon((current) => (current === "primary" ? "special" : "primary"));
      }
      if (key === " " || key === "enter") sendFire();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [canAct, maxAngle, move, phase, sendFire]);

  const remaining = phase === "playing" ? data.deadline - serverNow(clock) : 0;
  const selectRemaining = phase === "select" ? data.selectUntil - serverNow(clock) : 0;
  const activeTank = tanks[data.current];
  const hpPercent = own ? clamp((own.hp / Math.max(1, own.maxHp)) * 100, 0, 100) : 0;
  const selectedIds = tanks.filter((t) => t.selected).map((t) => t.tankId);

  return (
    <div
      className="fortressblue-board"
      data-fortressblue-phase={data.phase}
      data-fortressblue-turn={data.turn}
      data-fortressblue-shot={data.lastShot?.id ?? 0}
      data-fortressblue-animating={anim ? "1" : "0"}
      data-fortressblue-craters={craters.length}
      data-fortressblue-epoch={data.epoch}
      data-fortressblue-map={map.id}
      data-fortressblue-my-turn={canAct ? "1" : "0"}
      data-fortressblue-angle={Math.round(angle)}
      data-fortressblue-power={Math.round(power)}
      data-fortressblue-facing={facing}
      data-fortressblue-preview={aimPreview.length}
      data-fortressblue-art-scale={ART_SCALE}
    >
      <canvas
        ref={canvasRef}
        className={`fb-canvas ${canAct ? "aiming" : ""}`}
        data-fortressblue-canvas
        aria-label={`${map.name} 战场：鼠标瞄准，滚轮调整力度，点击开火`}
        onPointerMove={aimAtEvent}
        onPointerDown={(event) => {
          pointerType.current = event.pointerType;
          if (event.pointerType !== "touch") aimAtEvent(event);
        }}
        onClick={() => {
          if (pointerType.current === "touch") return;
          sendFire();
        }}
      />
      <canvas ref={terrainRef} className="fb-terrain" width={map.width} height={map.height} aria-hidden="true" />
      <div className="fb-top">
        <div className="fb-mapname">
          <strong>{map.name}</strong>
          <span>{map.nameEn}</span>
        </div>
        <div className="fb-turninfo">
          {phase === "select" ? (
            <span>选择坦克 · {fmtSeconds(selectRemaining)}s</span>
          ) : (
            <>
              <span>第 {data.turn} 回合</span>
              <span className={remaining <= 5000 ? "fb-timer urgent" : "fb-timer"} data-fortressblue-timer>
                {fmtSeconds(remaining)}s
              </span>
              <span className="fb-wind" title="风向">
                {data.wind === 0 ? "风 0" : `风 ${data.wind > 0 ? "→" : "←"} ${Math.abs(data.wind)}`}
              </span>
              {data.sudden && <span className="fb-sudden">突然死亡</span>}
            </>
          )}
        </div>
      </div>
      <div className="fb-active">
        {phase === "playing" && activeTank ? (
          <>
            <i style={{ background: teamColors[activeTank.team] }} />
            <strong>{room.seats[activeTank.seat]?.name ?? `P${activeTank.seat + 1}`}</strong>
            <span>{tankName(activeTank.tankId)}</span>
            {activeTank.seat === data.you ? <em>你的回合</em> : <em>等待行动</em>}
          </>
        ) : (
          <span>{phase === "select" ? "选择你的坦克" : "对局结束"}</span>
        )}
      </div>

      {phase === "select" && (
        <div className="fb-draft" role="dialog" aria-label="选择坦克">
          <div className="fb-draft-head">
            <div>
              <h2>选择坦克</h2>
              <p>全部选定或 {fmtSeconds(selectRemaining)} 秒后自动开局，可重复选择同款坦克。</p>
            </div>
            <span className="fb-draft-count">
              {selectedIds.length}/{tanks.length}
            </span>
          </div>
          <div className="fb-tank-grid">
            {tankCatalog.map((info) => {
              const portrait = tankPortraits[String(info.id)] ?? info.sprites.idle;
              const count = selectedIds.filter((id) => id === info.id).length;
              const mine = data.selectedTank === info.id;
              return (
                <button
                  type="button"
                  key={info.id}
                  className={`fb-tank-card ${mine ? "selected" : ""} ${count ? "taken" : ""}`}
                  disabled={!data.canSelect || busy}
                  onClick={() => choose(info.id)}
                  data-fortressblue-tank={info.id}
                >
                  <img
                    src={sourceList(assets, portrait?.url ?? "")[0]}
                    alt=""
                    onError={(event) => {
                      const img = event.currentTarget;
                      const sources = sourceList(assets, portrait?.url ?? "");
                      const idx = Number(img.dataset.fallback ?? "0");
                      if (idx + 1 < sources.length) {
                        img.dataset.fallback = String(idx + 1);
                        img.src = sources[idx + 1];
                      }
                    }}
                  />
                  <span className="fb-card-name">{info.name}</span>
                  <span className="fb-card-stats">
                    HP {info.hp} · 攻 {info.primary} · 角 {info.angle}
                  </span>
                  <span className="fb-card-skill">{info.skillName ? `${info.skillName}特殊弹` : "高伤特殊弹"}</span>
                  {count > 0 && <b className="fb-taken">{count > 1 ? `×${count}` : "已选"}</b>}
                </button>
              );
            })}
          </div>
        </div>
      )}

      <div className="fb-controls">
        <div className="fb-resource">
          <span>HP</span>
          <div className="fb-hpbar">
            <i style={{ width: `${hpPercent}%`, background: own ? teamColors[own.team] : "#888" }} />
          </div>
          <b>{own?.hp ?? "—"}/{own?.maxHp ?? "—"}</b>
        </div>
        <div className="fb-knobs">
          <div className="fb-dial" aria-hidden="true">
            <i style={{ transform: `rotate(${facing > 0 ? 90 - angle : angle - 90}deg)` }} />
            <b>{Math.round(angle)}°</b>
            <span>{facing > 0 ? "右" : "左"}</span>
          </div>
          <div className="fb-knob-sliders">
            <label>
              <span>角度 <b>{Math.round(angle)}°</b></span>
              <input
                aria-label="角度"
                type="range"
                min={0}
                max={maxAngle}
                value={angle}
                disabled={!canAct}
                onChange={(event) => {
                  const next = Number(event.target.value);
                  aimRef.current = { ...aimRef.current, angle: next };
                  setAngle(next);
                }}
              />
            </label>
            <label>
              <span>力度 <b>{Math.round(power)}</b></span>
              <input
                aria-label="力度"
                type="range"
                min={20}
                max={100}
                value={power}
                disabled={!canAct}
                onChange={(event) => {
                  const next = Number(event.target.value);
                  aimRef.current = { ...aimRef.current, power: next };
                  setPower(next);
                }}
              />
            </label>
          </div>
        </div>
        <div className="fb-actions">
          <button
            type="button"
            className="fb-move"
            onClick={() => move(-1)}
            disabled={!canAct || !data.canMove}
            aria-label="向左移动"
          >
            <ChevronLeft size={22} />
          </button>
          <button
            type="button"
            className="fb-facing"
            onClick={toggleFacing}
            disabled={!canAct}
            aria-label="切换朝向"
          >
            <FlipHorizontal size={18} />
            <span>{facing > 0 ? "朝右" : "朝左"}</span>
          </button>
          <button
            type="button"
            className="fb-move"
            onClick={() => move(1)}
            disabled={!canAct || !data.canMove}
            aria-label="向右移动"
          >
            <ChevronRight size={22} />
          </button>
          <button
            type="button"
            className={weapon === "special" ? "fb-special active" : "fb-special"}
            onClick={() => setWeapon((v) => (v === "primary" ? "special" : "primary"))}
            disabled={!canAct}
            aria-label="切换弹种"
          >
            <Bomb size={18} />
            <span>{weapon === "special" ? `特殊弹 ${own?.special ?? 0}` : "主炮"}</span>
          </button>
          <button
            type="button"
            className="fb-fire"
            onClick={() => sendFire()}
            disabled={!canAct || !data.canFire}
            aria-label="开火"
          >
            <Crosshair size={20} />
            开火
          </button>
        </div>
        <div className="fb-tip">
          {phase === "playing"
            ? canAct
              ? `移动力 ${Math.round(own?.fuel ?? 0)} · 鼠标瞄准／滚轮力度／点击开火 · ←→ 移动 · ↑↓ 角度 · Z/X 力度 · Q/E 弹种 · 空格开火`
              : "等待其他玩家行动"
            : phase === "select"
              ? "选择坦克后自动开始"
              : "对局结束"}
        </div>
      </div>
      <div className="fb-live" aria-live="polite">
        {log.slice(-1)[0]}
      </div>
    </div>
  );
}

export { Board };
