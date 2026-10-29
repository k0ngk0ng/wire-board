import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { ReactNode, PointerEvent as ReactPointerEvent } from "react";
import {
  worldBounds,
  fitCamera,
  fillCamera,
  clampCamera,
  worldToScreen,
  screenToWorld,
  isWorldOverview,
  detailCameraAt,
  panCamera,
  zoomCamera,
  focusCamera,
  cameraWindow,
  dragMoved,
} from "./camera";
import type { Camera, Viewport } from "./camera";
import "./world-camera.css";
import {
  CombatTimeline,
  CombatPlayback,
  FLIGHT_MS,
  activeEffects,
  dueSoundEvents,
} from "./combat-timeline";
import {
  assetURL,
  enemyPoint,
  objectivePoint,
  freshEvents,
  blastImpactTargets,
  combatMotion,
} from "./presentation";
import type { CombatStatus } from "./combat-timeline";
import type { EventCursor } from "./presentation";
import type { Enemy, Event, View, MapData, Point, Objective } from "./types";
import atlasLayout from "../assets/public/starfort/v1/atlas.json";
import worldAtlasLayout from "../assets/public/starfort/world-v2/atlas.json";
import { shotPose, workerPose } from "./animation";
import {
  drawCrater,
  drawRepairTool,
  drawCombatAccent,
  drawIceWall,
} from "./combat-art";
import { battlefieldPalette as palette } from "./palette";

type Atlas = {
  frames: Record<string, { x: number; y: number; w: number; h: number }>;
  meta: { image: string; width: number; height: number };
};
type Art = {
  atlas: Atlas | null;
  sheet: HTMLImageElement | null;
  map: HTMLImageElement | null;
  worldAtlas: Atlas | null;
  worldSheet: HTMLImageElement | null;
};
type Effect = Event & { born: number; impact?: Point; impactTargetId?: number };
const colors = ["#b34f2d", "#386da7", "#8a5196", "#4f7641"];
const towerNames: Record<string, string> = {
  arrow: "箭塔",
  cannon: "炮塔",
  frost: "冰塔",
  arcane: "法师塔",
};
const towerColors: Record<string, string> = palette.tower;
const outlinedFrames = new WeakMap<
  HTMLImageElement,
  Map<string, HTMLCanvasElement>
>();
const outlineMargin = 6;

function outlinedFrame(
  sheet: HTMLImageElement,
  frame: Atlas["frames"][string],
  name: string,
) {
  let frames = outlinedFrames.get(sheet);
  if (!frames) {
    frames = new Map();
    outlinedFrames.set(sheet, frames);
  }
  const cached = frames.get(name);
  if (cached) return cached;
  const bitmap = document.createElement("canvas");
  bitmap.width = frame.w + outlineMargin * 2;
  bitmap.height = frame.h + outlineMargin * 2;
  const ctx = bitmap.getContext("2d");
  if (!ctx) return null;
  // Rasterize the alpha silhouette once per atlas frame. The original image
  // keeps its exact size/anchor; the thin contour does not affect hit targets.
  for (let index = 0; index < 8; index++) {
    const angle = (index * Math.PI) / 4;
    ctx.drawImage(
      sheet,
      frame.x,
      frame.y,
      frame.w,
      frame.h,
      outlineMargin + Math.cos(angle) * 4,
      outlineMargin + Math.sin(angle) * 4,
      frame.w,
      frame.h,
    );
  }
  ctx.globalCompositeOperation = "source-in";
  ctx.fillStyle = palette.unitOutline;
  ctx.fillRect(0, 0, bitmap.width, bitmap.height);
  ctx.globalCompositeOperation = "source-over";
  ctx.drawImage(
    sheet,
    frame.x,
    frame.y,
    frame.w,
    frame.h,
    outlineMargin,
    outlineMargin,
    frame.w,
    frame.h,
  );
  frames.set(name, bitmap);
  return bitmap;
}
// Canvas artwork and its accessible hit target share the same dimensions.
function towerDimensions(zoom: number) {
  const width = Math.max(66, 42 / zoom);
  return { width, height: (width * 224) / 192 };
}
function loadImage(
  src: string,
  fallback: string,
  signal: AbortSignal,
): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    const abort = () => {
      image.src = "";
      reject(new Error("aborted"));
    };
    signal.addEventListener("abort", abort, { once: true });
    const clean = () => signal.removeEventListener("abort", abort);
    image.onload = () => {
      clean();
      resolve(image);
    };
    image.onerror = () => {
      if (signal.aborted) {
        clean();
        return;
      }
      if (image.getAttribute("src") !== fallback) {
        image.src = fallback;
        return;
      }
      clean();
      reject(new Error("image unavailable"));
    };
    image.src = src;
  });
}
function sprite(
  ctx: CanvasRenderingContext2D,
  art: Art,
  name: string,
  x: number,
  y: number,
  width: number,
  height: number,
  anchor = "bottom",
): boolean {
  const worldFrame = art.worldAtlas?.frames[name];
  const frame = worldFrame ?? art.atlas?.frames[name];
  const sheet = worldFrame ? art.worldSheet : art.sheet;
  if (!sheet || !frame) return false;
  if (/^(tower|enemy|base)-/.test(name)) {
    const outlined = outlinedFrame(sheet, frame, name);
    if (outlined) {
      const marginX = (outlineMargin * width) / frame.w;
      const marginY = (outlineMargin * height) / frame.h;
      ctx.drawImage(
        outlined,
        x - width / 2 - marginX,
        y - (anchor === "center" ? height / 2 : height) - marginY,
        width + marginX * 2,
        height + marginY * 2,
      );
      return true;
    }
  }
  ctx.drawImage(
    sheet,
    frame.x,
    frame.y,
    frame.w,
    frame.h,
    x - width / 2,
    y - (anchor === "center" ? height / 2 : height),
    width,
    height,
  );
  return true;
}
function circle(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  r: number,
  fill: string,
  stroke?: string,
  strokeWidth = 2,
) {
  ctx.beginPath();
  ctx.arc(x, y, r, 0, Math.PI * 2);
  ctx.fillStyle = fill;
  ctx.fill();
  if (stroke) {
    ctx.strokeStyle = stroke;
    ctx.lineWidth = strokeWidth;
    ctx.stroke();
  }
}
function linePath(ctx: CanvasRenderingContext2D, path: View["map"]["path"]) {
  ctx.beginPath();
  path.forEach((p, i) => (i ? ctx.lineTo(p.x, p.y) : ctx.moveTo(p.x, p.y)));
}
function fallbackTower(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  size: number,
  kind: string,
) {
  ctx.strokeStyle = palette.unitOutline;
  ctx.lineWidth = size * 0.035;
  ctx.lineJoin = "round";
  ctx.fillStyle = "#86704b";
  ctx.fillRect(x - size * 0.35, y - size * 0.34, size * 0.7, size * 0.35);
  ctx.strokeRect(x - size * 0.35, y - size * 0.34, size * 0.7, size * 0.35);
  ctx.fillStyle = towerColors[kind];
  ctx.fillRect(x - size * 0.25, y - size * 0.8, size * 0.5, size * 0.6);
  ctx.strokeRect(x - size * 0.25, y - size * 0.8, size * 0.5, size * 0.6);
  ctx.beginPath();
  ctx.moveTo(x - size * 0.4, y - size * 0.7);
  ctx.lineTo(x, y - size);
  ctx.lineTo(x + size * 0.4, y - size * 0.7);
  ctx.closePath();
  ctx.fill();
  ctx.stroke();
}
function fallbackEnemy(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  size: number,
  kind: string,
) {
  circle(
    ctx,
    x,
    y - size * 0.4,
    size * 0.4,
    kind === "runner" ? "#d68b3f" : kind === "boss" ? "#b16a9e" : "#83b449",
    palette.unitOutline,
    size * 0.035,
  );
  circle(ctx, x - size * 0.12, y - size * 0.45, size * 0.045, "#18332d");
  circle(ctx, x + size * 0.12, y - size * 0.45, size * 0.045, "#18332d");
}

const regionColors: Record<string, string> = palette.terrain;
function drawLandscape(
  ctx: CanvasRenderingContext2D,
  art: Art,
  map: MapData,
  scale: number,
  previous?: MapData,
  compact = false,
) {
  const regions = map.regions ?? [
    {
      id: 0,
      x: 0,
      y: 0,
      width: map.width,
      height: map.height,
      biome: "forest",
      label: "星光林地",
    },
  ];
  for (const region of regions) {
    if (previous?.regions?.some((old) => old.id === region.id)) continue;
    ctx.save();
    ctx.beginPath();
    ctx.rect(region.x, region.y, region.width, region.height);
    ctx.clip();
    ctx.fillStyle = regionColors[region.biome] ?? regionColors.forest;
    ctx.fillRect(region.x, region.y, region.width, region.height);
    ctx.save();
    ctx.globalCompositeOperation = "multiply";
    ctx.globalAlpha *= palette.textureOpacity;
    if (!art.worldSheet && art.map && region.id === 0)
      ctx.drawImage(art.map, region.x, region.y, region.width, region.height);
    for (let y = region.y; y < region.y + region.height; y += 256)
      for (let x = region.x; x < region.x + region.width; x += 256)
        sprite(ctx, art, `tile-${region.biome}`, x + 128, y + 256, 256, 256);
    ctx.restore();
    ctx.strokeStyle = palette.terrainEdge;
    ctx.lineWidth = Math.max(2, (compact ? 0.75 : 1.25) / scale);
    ctx.strokeRect(region.x, region.y, region.width, region.height);
    ctx.restore();
    // Sparse edge ornaments leave roads, pads and enemy health readable.
    if (!compact) {
      for (let i = 0; i < 6; i++) {
        const x =
          region.x +
          48 +
          ((i * 173 + region.id * 31) % Math.max(1, region.width - 96));
        const y = region.y + (i % 2 ? region.height - 20 : 56);
        if (map.pads.some((pad) => Math.hypot(pad.x - x, pad.y - y) < 80))
          continue;
        sprite(
          ctx,
          art,
          `prop-${region.biome === "canyon" ? "rock" : region.biome === "river" || region.biome === "marsh" ? "reeds" : region.biome === "snow" ? "pine" : region.biome === "ruins" ? "ruins" : "tree"}`,
          x,
          y,
          72,
          72,
        );
      }
      ctx.font = `600 ${Math.max(17, 12 / scale)}px system-ui`;
      ctx.textAlign = "left";
      ctx.fillStyle = palette.label.ink;
      ctx.lineWidth = 4;
      ctx.strokeStyle = palette.label.halo;
      ctx.strokeText(
        region.label,
        region.x + 26,
        region.y + region.height - 22,
      );
      ctx.fillText(region.label, region.x + 26, region.y + region.height - 22);
    }
  }
  for (const feature of map.features ?? []) {
    if (previous?.features?.some((old) => old.id === feature.id)) continue;
    if (feature.kind === "river") {
      ctx.fillStyle = palette.water.fill;
      ctx.fillRect(
        feature.x - feature.width / 2,
        feature.y - feature.height / 2,
        feature.width,
        feature.height,
      );
      ctx.save();
      ctx.strokeStyle = palette.water.edge;
      ctx.lineWidth = Math.max(4, 1.5 / scale);
      ctx.strokeRect(
        feature.x - feature.width / 2,
        feature.y - feature.height / 2,
        feature.width,
        feature.height,
      );
      ctx.strokeStyle = palette.water.ripple;
      ctx.lineWidth = 3;
      for (
        let y = feature.y - feature.height / 2 + 14;
        y < feature.y + feature.height / 2;
        y += 40
      ) {
        ctx.beginPath();
        ctx.moveTo(feature.x - feature.width / 2 + 10, y);
        ctx.quadraticCurveTo(
          feature.x,
          y - 12,
          feature.x + feature.width / 2 - 10,
          y,
        );
        ctx.stroke();
      }
      ctx.restore();
    }
    if (feature.kind === "highground") {
      ctx.fillStyle = palette.highground.edge;
      ctx.beginPath();
      ctx.ellipse(
        feature.x,
        feature.y + 8,
        feature.width / 2,
        feature.height / 2,
        0,
        0,
        Math.PI * 2,
      );
      ctx.fill();
      ctx.fillStyle = palette.highground.top;
      ctx.beginPath();
      ctx.ellipse(
        feature.x,
        feature.y,
        feature.width / 2,
        feature.height / 2,
        0,
        0,
        Math.PI * 2,
      );
      ctx.fill();
      ctx.strokeStyle = palette.highground.edge;
      ctx.lineWidth = Math.max(3, 1.25 / scale);
      ctx.stroke();
    }
  }
}
function drawRoutes(
  ctx: CanvasRenderingContext2D,
  map: MapData,
  scale: number,
  previous?: MapData,
  compact = false,
) {
  const routes = map.routes ?? [
    {
      id: "main",
      label: "林间入口",
      sector: 0,
      points: map.path,
      active: true,
    },
  ];
  ctx.lineJoin = "round";
  ctx.lineCap = "round";
  for (const route of routes) {
    if (previous?.routes?.some((old) => old.id === route.id)) continue;
    ctx.save();
    const gated = route.id.includes("shortcut") && !route.active;
    if (previous || !route.active) ctx.setLineDash([24, 18]);
    linePath(ctx, route.points);
    ctx.lineWidth = compact ? 20 : previous ? 22 : gated ? 28 : 64;
    ctx.strokeStyle = previous
      ? "#cbb4d2"
      : route.active
        ? "#a19161"
        : "#86906d";
    ctx.stroke();
    linePath(ctx, route.points);
    ctx.lineWidth = compact ? 14 : previous ? 14 : gated ? 20 : 53;
    ctx.strokeStyle = route.active ? "#c5b184" : "#9a9c7a";
    ctx.stroke();
    if (!compact && !previous && !gated) {
      ctx.setLineDash([]);
      for (let segment = 1; segment < route.points.length; segment++) {
        const a = route.points[segment - 1],
          b = route.points[segment];
        const length = Math.hypot(b.x - a.x, b.y - a.y),
          nx = -(b.y - a.y) / length,
          ny = (b.x - a.x) / length;
        for (let step = 0; step < length; step += 13) {
          const p = step / length,
            seed = segment * 31 + step;
          const x = a.x + (b.x - a.x) * p,
            y = a.y + (b.y - a.y) * p;
          const offset = Math.sin(seed * 2.17) * 22;
          ctx.fillStyle = step % 3 < 1 ? "#dfcd9f" : "#a69067";
          ctx.beginPath();
          ctx.ellipse(
            x + nx * offset,
            y + ny * offset,
            2 + Math.abs(Math.sin(seed)) * 2,
            1.3,
            seed,
            0,
            Math.PI * 2,
          );
          ctx.fill();
          for (const side of [-1, 1]) {
            const edge = side * (28 + Math.sin(seed) * 5),
              ex = x + nx * edge,
              ey = y + ny * edge;
            ctx.strokeStyle = side > 0 ? "#71834b" : "#879356";
            ctx.lineWidth = 2;
            ctx.beginPath();
            ctx.moveTo(ex - 2, ey);
            ctx.lineTo(ex, ey - 4 - Math.abs(Math.sin(seed)) * 3);
            ctx.lineTo(ex + 2, ey);
            ctx.stroke();
          }
        }
      }
    }
    const entry = route.points[0];
    if (entry) {
      const radius = compact ? 3.5 / scale : Math.max(15, 10 / scale);
      circle(
        ctx,
        entry.x,
        entry.y,
        radius,
        previous ? "#e7cfec" : route.active ? "#af5137" : "#687962",
        "#fff9e4",
      );
      if (!compact) {
        ctx.setLineDash([]);
        ctx.font = `bold ${Math.max(17, 12 / scale)}px system-ui`;
        const inward = entry.x > map.width * 0.75;
        ctx.textAlign = inward ? "right" : "left";
        ctx.strokeStyle = "#fff9e9";
        ctx.lineWidth = 4;
        const text = gated ? "封闭捷径" : route.label;
        ctx.strokeText(
          text,
          entry.x + (inward ? -radius - 8 : radius + 8),
          entry.y - radius,
        );
        ctx.fillStyle = "#543b2f";
        ctx.fillText(
          text,
          entry.x + (inward ? -radius - 8 : radius + 8),
          entry.y - radius,
        );
      }
    }
    ctx.restore();
  }
}
function drawOverview(
  ctx: CanvasRenderingContext2D,
  map: MapData,
  view: View,
  scale: number,
) {
  for (const pad of map.pads) {
    const tower = view.towers.find((item) => item.pad === pad.id);
    circle(
      ctx,
      pad.x,
      pad.y,
      (tower ? 3.7 : 1.6) / scale,
      tower ? colors[tower.owner % colors.length] : "#77845d",
    );
  }
  for (const enemy of view.enemies) {
    circle(
      ctx,
      enemy.x,
      enemy.y,
      (enemy.kind === "boss" ? 3.2 : 1.6) / scale,
      enemy.kind === "boss" ? "#843869" : "#a13629",
    );
  }
  for (const objective of view.world?.objectives ?? []) {
    ctx.fillStyle =
      objective.status === "failed"
        ? "#695a52"
        : objective.status === "active"
          ? "#fff4c1"
          : "#4c6b6d";
    const size = 5 / scale;
    ctx.fillRect(objective.x - size / 2, objective.y - size / 2, size, size);
  }
  const base = map.base ?? map.path[map.path.length - 1];
  if (base) {
    circle(ctx, base.x, base.y, 6 / scale, "#fff1a5", "#795626");
    ctx.font = `bold ${10 / scale}px system-ui`;
    ctx.textAlign = "center";
    ctx.strokeStyle = "#fffbea";
    ctx.lineWidth = 3 / scale;
    ctx.strokeText("星核", base.x, base.y + 18 / scale);
    ctx.fillStyle = "#4c432c";
    ctx.fillText("星核", base.x, base.y + 18 / scale);
  }
}
function drawFeatures(
  ctx: CanvasRenderingContext2D,
  art: Art,
  map: MapData,
  scale: number,
  view: View,
  previous: Map<string, Objective>,
  fraction: number,
  at: number,
  reduced: boolean,
) {
  for (const feature of map.features ?? []) {
    if (feature.kind === "river" || feature.kind === "highground") continue;
    if (
      view.world?.objectives.some(
        (objective) =>
          objective.kind === feature.kind &&
          objective.sector === feature.sector,
      )
    )
      continue;
    const name =
      feature.kind === "beacon"
        ? "watchtower"
        : feature.kind === "convoy"
          ? "cart"
          : feature.kind;
    const width =
      feature.kind === "bridge" ? feature.width : Math.max(64, 32 / scale);
    const height = feature.kind === "bridge" ? feature.height : width;
    sprite(
      ctx,
      art,
      `prop-${name}`,
      feature.x,
      feature.y + height / 2,
      width,
      height,
    );
  }
  for (const route of map.routes ?? [])
    if (
      route.id.includes("shortcut") &&
      !route.active &&
      route.points.length > 1
    ) {
      const p = route.points[1];
      sprite(ctx, art, "prop-gate", p.x, p.y + 16, 76, 76);
    }
  for (const objective of view.world?.objectives ?? []) {
    const position = objectivePoint(
      map.routes?.find((route) => route.id === objective.route)?.points ??
        map.path,
      previous.get(objective.id),
      objective,
      fraction,
      reduced,
      view.phase === "battle",
    );
    const size = Math.max(48, 30 / scale);
    const status =
      objective.status === "active"
        ? "进行中"
        : objective.status === "completed"
          ? "已完成"
          : objective.status === "failed"
            ? "未守住"
            : "可承接";
    // The convoy moves only while its authoritative side objective is active.
    const bob =
      objective.kind === "convoy" &&
      objective.status === "active" &&
      !reduced &&
      view.phase === "battle"
        ? Math.sin(at / 180) * 1.5
        : 0;
    const name =
      objective.kind === "beacon"
        ? "watchtower"
        : objective.kind === "convoy"
          ? "cart"
          : "mine";
    ctx.save();
    if (objective.status === "failed") ctx.globalAlpha = 0.55;
    sprite(ctx, art, `prop-${name}`, position.x, position.y + bob, size, size);
    ctx.restore();
    if (objective.status === "active" && objective.maxHP > 0) {
      const barWidth = Math.max(48, 32 / scale),
        barHeight = Math.max(5, 3 / scale),
        border = Math.max(1, 1 / scale);
      ctx.fillStyle = palette.health.track;
      ctx.fillRect(
        position.x - barWidth / 2 - border,
        position.y - size - barHeight - 3 - border,
        barWidth + border * 2,
        barHeight + border * 2,
      );
      ctx.fillStyle =
        objective.hp < objective.maxHP * 0.35
          ? palette.health.critical
          : palette.health.healthy;
      ctx.fillRect(
        position.x - barWidth / 2,
        position.y - size - barHeight - 3,
        barWidth * Math.max(0, Math.min(1, objective.hp / objective.maxHP)),
        barHeight,
      );
    }
    ctx.font = `600 ${Math.max(13, 10 / scale)}px system-ui`;
    ctx.textAlign = "center";
    ctx.strokeStyle = palette.label.halo;
    ctx.lineWidth = 4;
    ctx.fillStyle =
      objective.status === "failed" ? palette.label.danger : palette.label.ink;
    ctx.strokeText(
      `${objective.label} · ${status}`,
      position.x,
      position.y + 15 / scale,
    );
    ctx.fillText(
      `${objective.label} · ${status}`,
      position.x,
      position.y + 15 / scale,
    );
  }
}

function MapPopover({
  point,
  viewport,
  children,
}: {
  point: Point;
  viewport: Viewport;
  children: ReactNode;
}) {
  const node = useRef<HTMLDivElement>(null);
  const [height, setHeight] = useState(160);
  useLayoutEffect(() => {
    const observer = new ResizeObserver(() =>
      setHeight(node.current?.offsetHeight ?? 160),
    );
    if (node.current) observer.observe(node.current);
    return () => observer.disconnect();
  }, []);
  const width = Math.min(260, Math.max(0, viewport.width - 16));
  const topLimit = 64,
    bottomLimit = Math.max(topLimit, viewport.height - 76);
  let left = point.x + 34;
  let top = point.y - height / 2;
  if (left + width > viewport.width - 8) {
    if (point.x - 34 - width >= 8) left = point.x - 34 - width;
    else {
      left = point.x - width / 2;
      top =
        point.y + 38 + height <= bottomLimit
          ? point.y + 38
          : point.y - height - 38;
    }
  }
  left = Math.max(8, Math.min(viewport.width - width - 8, left));
  top = Math.max(topLimit, Math.min(bottomLimit - height, top));
  return (
    <div
      ref={node}
      className="sf-map-popover"
      style={{ left, top, width, maxHeight: bottomLimit - topLimit }}
      onPointerDown={(e) => e.stopPropagation()}
      onPointerUp={(e) => e.stopPropagation()}
      onClick={(e) => e.stopPropagation()}
      onWheel={(e) => e.stopPropagation()}
    >
      {children}
    </div>
  );
}
const enemyInfo: Record<string, [string, string]> = {
  slime: ["史莱姆", "普通敌人"],
  runner: ["疾行者", "移动较快"],
  swarm: ["蜂群", "成群来袭"],
  armored: ["重甲兵", "护甲降低物理伤害"],
  shield: ["护盾兵", "护盾吸收伤害"],
  elite: ["精英", "不受一箭穿心秒杀"],
  boss: ["首领", "半血进入第二阶段，加速并召唤护卫；不受一箭穿心秒杀"],
  healer: ["治疗者", "恢复附近同伴生命"],
  banner: ["旗手", "提高附近同伴移动速度"],
  splitter: ["分裂怪", "死亡后生成小怪"],
  engineer: ["工兵", "更快修补弹坑，可打开下一波捷径"],
};

export function Battlefield({
  onReady,
  onLoadError,
  onEvents,
  onCombatState,
  roomID,
  data,
  assets,
  selected,
  towerPanel,
  select,
}: {
  onReady?: () => void;
  onLoadError?: (message: string) => void;
  onEvents?: (events: Event[], at: number) => void;
  onCombatState?: (state: CombatStatus) => void;
  roomID: string;
  data: View;
  assets: string;
  selected: number | null;
  towerPanel?: ReactNode;
  select: (pad: number) => void;
}) {
  const canvas = useRef<HTMLCanvasElement>(null),
    container = useRef<HTMLDivElement>(null);
  const art = useRef<Art>({
    atlas: null,
    sheet: null,
    map: null,
    worldAtlas: null,
    worldSheet: null,
  });
  const state = useRef({
    data,
    previous: new Map<number, Enemy>(),
    previousObjectives: new Map<string, Objective>(),
    destinations: new Map(data.enemies.map((enemy) => [enemy.id, enemy])),
    at: performance.now(),
    duration: 1000,
  });
  // Map objects are decoded anew with each snapshot; content, not identity,
  // invalidates the static terrain layer. Never stringify it per animation frame.
  const terrainKey = useRef("");
  const timeline = useRef(new CombatPlayback(data, state.current.at));
  const movement = useRef(new CombatTimeline(state.current));
  const soundEvents = useRef<Effect[]>([]);
  const onEventsRef = useRef(onEvents);
  onEventsRef.current = onEvents;
  const onCombatStateRef = useRef(onCombatState);
  onCombatStateRef.current = onCombatState;
  useEffect(
    () =>
      timeline.current.subscribe((status) =>
        onCombatStateRef.current?.(status),
      ),
    [],
  );
  const drawnEnemies = useRef<{ enemy: Enemy; point: Point }[]>([]);
  const inspected = useRef<number | null>(null);
  const [inspection, setInspection] = useState<{
    enemy: Enemy;
    point: Point;
  } | null>(null);
  const closeInspection = () => {
    inspected.current = null;
    setInspection(null);
  };
  useEffect(() => {
    closeInspection();
  }, [selected, roomID]);
  useEffect(() => {
    const timer = window.setInterval(() => {
      if (inspected.current === null) return;
      const current = drawnEnemies.current.find(
        (e) => e.enemy.id === inspected.current && e.enemy.hp > 0,
      );
      if (current) setInspection(current);
      else closeInspection();
    }, 100);
    return () => window.clearInterval(timer);
  }, []);
  const hoverRef = useRef<number | null>(null);
  const selectedRef = useRef(selected);
  selectedRef.current = selected;
  const cursor = useRef<EventCursor | null>(null),
    effects = useRef<Effect[]>([]);
  const [previewID, setPreviewID] = useState<string | null>(null);
  const preview = data.world?.choice?.options.find(
    (option) => option.id === previewID,
  )?.map;
  const renderMap = useRef<MapData>(preview ?? data.map);
  renderMap.current = preview ?? data.map;
  const previewRef = useRef(preview);
  previewRef.current = preview;
  terrainKey.current = JSON.stringify([data.map, preview ?? null]);
  const camera = useRef<Camera>({ x: 480, y: 240, zoom: 0.3 });
  const viewport = useRef<Viewport>({ width: 0, height: 0 });
  const cameraRoom = useRef<string | null>(null);
  const fullscreenView = useRef<{ active: boolean; normal: Camera | null }>({
    active: false,
    normal: null,
  });
  const [view, setView] = useState({
    camera: camera.current,
    viewport: viewport.current,
  });
  const pointers = useRef(new Map<number, Point>());
  const gesture = useRef({
    start: { x: 0, y: 0 },
    moved: false,
    suppressUntil: 0,
  });
  const publishCamera = (next: Camera) => {
    camera.current = next;
    setView({ camera: next, viewport: { ...viewport.current } });
  };
  const focus = (point: Point) =>
    publishCamera(
      detailCameraAt(
        camera.current,
        point,
        renderMap.current,
        viewport.current,
      ),
    );
  useEffect(() => {
    const node = container.current;
    const reveal = (event: globalThis.Event) => {
      const target = event.target;
      if (
        !(target instanceof HTMLElement) ||
        !target.hasAttribute("data-sf-pad")
      )
        return;
      const pad = state.current.data.map.pads.find(
        (p) => p.id === Number(target.dataset.sfPad),
      );
      if (!pad) return;
      event.preventDefault();
      // Center in the clear part of the battlefield, below the fixed HUD.
      focus(pad);
      node?.scrollIntoView({ block: "nearest", inline: "nearest" });
    };
    node?.addEventListener("controller-reveal", reveal);
    return () => node?.removeEventListener("controller-reveal", reveal);
  }, []);
  const navigateMinimap = (event: ReactPointerEvent<SVGSVGElement>) => {
    const svg = event.currentTarget,
      point = svg.createSVGPoint();
    point.x = event.clientX;
    point.y = event.clientY;
    const matrix = svg.getScreenCTM();
    if (matrix) {
      const world = point.matrixTransform(matrix.inverse());
      focus({ x: world.x, y: world.y });
    }
  };
  const zoom = (factor: number) =>
    publishCamera(
      zoomCamera(
        camera.current,
        factor,
        { x: viewport.current.width / 2, y: viewport.current.height / 2 },
        worldBounds(renderMap.current),
        viewport.current,
      ),
    );
  const pointerPoint = (event: ReactPointerEvent) => {
    const rect = canvas.current!.getBoundingClientRect();
    return { x: event.clientX - rect.left, y: event.clientY - rect.top };
  };
  const pointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.button !== 0) return;
    const point = pointerPoint(event);
    pointers.current.set(event.pointerId, point);
    if (pointers.current.size === 1)
      gesture.current = { ...gesture.current, start: point, moved: false };
    else gesture.current.moved = true;
    // Capture only on the canvas: pad clicks remain native accessible buttons.
    if (event.target === canvas.current)
      event.currentTarget.setPointerCapture(event.pointerId);
  };
  const pointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    const prior = pointers.current.get(event.pointerId);
    if (!prior) return;
    const point = pointerPoint(event);
    const before = [...pointers.current.values()];
    pointers.current.set(event.pointerId, point);
    if (pointers.current.size > 1) {
      const after = [...pointers.current.values()];
      const oldDistance = Math.hypot(
        before[0].x - before[1].x,
        before[0].y - before[1].y,
      );
      const distance = Math.hypot(
        after[0].x - after[1].x,
        after[0].y - after[1].y,
      );
      const midpoint = {
        x: (after[0].x + after[1].x) / 2,
        y: (after[0].y + after[1].y) / 2,
      };
      const oldMidpoint = {
        x: (before[0].x + before[1].x) / 2,
        y: (before[0].y + before[1].y) / 2,
      };
      const panned = panCamera(
        camera.current,
        { x: midpoint.x - oldMidpoint.x, y: midpoint.y - oldMidpoint.y },
        worldBounds(renderMap.current),
        viewport.current,
      );
      publishCamera(
        zoomCamera(
          panned,
          oldDistance > 0 ? distance / oldDistance : 1,
          midpoint,
          worldBounds(renderMap.current),
          viewport.current,
        ),
      );
      gesture.current.moved = true;
    } else if (
      gesture.current.moved ||
      dragMoved(gesture.current.start, point)
    ) {
      gesture.current.moved = true;
      event.currentTarget.setPointerCapture(event.pointerId);
      publishCamera(
        panCamera(
          camera.current,
          { x: point.x - prior.x, y: point.y - prior.y },
          worldBounds(renderMap.current),
          viewport.current,
        ),
      );
    }
    if (gesture.current.moved) event.preventDefault();
  };
  const pointerUp = (event: ReactPointerEvent<HTMLDivElement>) => {
    pointers.current.delete(event.pointerId);
    if (gesture.current.moved)
      gesture.current.suppressUntil = performance.now() + 300;
    if (event.currentTarget.hasPointerCapture(event.pointerId))
      event.currentTarget.releasePointerCapture(event.pointerId);
  };
  const [loaded, setLoaded] = useState(false),
    [assetError, setAssetError] = useState(false);
  useEffect(() => {
    if (loaded) onReady?.();
    if (assetError) onLoadError?.("战场图片加载失败，请重试");
  }, [loaded, assetError, onReady, onLoadError]);
  const [reduced, setReduced] = useState(
    () => window.matchMedia("(prefers-reduced-motion: reduce)").matches,
  );
  useEffect(() => {
    const query = window.matchMedia("(prefers-reduced-motion: reduce)");
    const change = () => setReduced(query.matches);
    query.addEventListener("change", change);
    return () => query.removeEventListener("change", change);
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    setLoaded(false);
    setAssetError(false);
    art.current = {
      atlas: null,
      sheet: null,
      map: null,
      worldAtlas: null,
      worldSheet: null,
    };
    const load = async () => {
      // Geometry ships with this build; only artwork is served by the CDN.
      const atlas: Atlas = atlasLayout;
      const worldURL = (base: string) =>
        `${base.replace(/\/$/, "")}/starfort/world-v2/atlas.webp`;
      const [sheet, map, worldSheet] = await Promise.all([
        loadImage(
          assetURL(assets, "atlas.webp"),
          assetURL("", "atlas.webp"),
          controller.signal,
        ),
        loadImage(
          assetURL(assets, "map.webp"),
          assetURL("", "map.webp"),
          controller.signal,
        ),
        loadImage(worldURL(assets), worldURL(""), controller.signal).catch(
          () => null,
        ),
      ]);
      if (!controller.signal.aborted) {
        art.current = {
          atlas,
          sheet,
          map,
          worldAtlas: worldAtlasLayout,
          worldSheet,
        };
        setLoaded(true);
      }
    };
    void load().catch(() => {
      if (!controller.signal.aborted) setAssetError(true);
    });
    return () => controller.abort();
  }, [assets]);
  useEffect(() => {
    const at = performance.now(),
      prior = state.current;
    const [next, batch] = freshEvents(cursor.current, roomID, data, at);
    const contiguous = !next.reset;
    cursor.current = next;
    const immediate =
      !contiguous ||
      document.hidden ||
      reduced ||
      data.phase === "paused" ||
      prior.data.phase === "paused" ||
      (prior.data.phase !== "battle" && data.phase === "battle");
    if (immediate) {
      timeline.current.reset(data, at);
      effects.current = [];
      soundEvents.current =
        reduced && contiguous && !document.hidden
          ? batch
              .filter((event) => event.type !== "shot")
              .map((event) => ({ ...event, born: at }))
          : [];
    } else {
      const impactTargets = blastImpactTargets(batch);
      const scheduled = timeline.current.push(data, batch, at).map((event) => ({
        ...event,
        impactTargetId: impactTargets.get(event.id),
      }));
      effects.current.push(...scheduled);
      soundEvents.current.push(...scheduled);
    }
    // Geometry follows the same logical interval as attacks. Health is read
    // separately so a final snapshot cannot reveal damage before impact.
    if (data.tick !== prior.data.tick || immediate) {
      const motion = immediate
        ? {
            previous: new Map<number, Enemy>(),
            destinations: new Map(
              data.enemies.map((enemy) => [enemy.id, enemy]),
            ),
          }
        : combatMotion(
            prior.data.enemies,
            prior.destinations,
            timeline.current.read(at, false).enemies,
            data.enemies,
            batch,
          );
      state.current = {
        data,
        previous: motion.previous,
        destinations: motion.destinations,
        previousObjectives:
          contiguous && !immediate
            ? new Map(
                (prior.data.world?.objectives ?? []).map((objective) => [
                  objective.id,
                  objective,
                ]),
              )
            : new Map(),
        at: immediate ? at : timeline.current.time(prior.data.elapsedMS),
        duration: Math.max(200, data.elapsedMS - prior.data.elapsedMS),
      };
      if (immediate) movement.current.reset(state.current);
      else movement.current.pushAt(state.current, state.current.at);
    } else state.current = { ...state.current, data };
    effects.current = activeEffects(effects.current, at, reduced);
  }, [data, roomID, reduced]);
  useEffect(() => {
    const reset = () => {
      cursor.current = null;
      effects.current = [];
      soundEvents.current = [];
      timeline.current.reset(state.current.data, performance.now());
      state.current = {
        ...state.current,
        previous: new Map(),
        previousObjectives: new Map(),
      };
      movement.current.reset(state.current);
    };
    document.addEventListener("visibilitychange", reset);
    return () => document.removeEventListener("visibilitychange", reset);
  }, []);
  useEffect(() => {
    const node = container.current;
    if (!node) return;
    const wheel = (event: WheelEvent) => {
      if (
        event.target instanceof Element &&
        event.target.closest(".sf-map-popover")
      )
        return;
      // Wheel input over the battlefield zooms around the pointer. Outside it,
      // the page retains normal scrolling.
      event.preventDefault();
      const rect = canvas.current!.getBoundingClientRect();
      publishCamera(
        zoomCamera(
          camera.current,
          Math.exp(
            -Math.max(
              -300,
              Math.min(
                300,
                event.deltaY *
                  (event.deltaMode === 1
                    ? 16
                    : event.deltaMode === 2
                      ? viewport.current.height
                      : 1),
              ),
            ) * 0.002,
          ),
          { x: event.clientX - rect.left, y: event.clientY - rect.top },
          worldBounds(renderMap.current),
          viewport.current,
        ),
      );
    };
    node.addEventListener("wheel", wheel, { passive: false });
    return () => node.removeEventListener("wheel", wheel);
  }, []);
  useEffect(() => {
    const node = canvas.current,
      wrap = container.current;
    if (!node || !wrap) return;
    const ctx = node.getContext("2d");
    if (!ctx) return;
    const background = document.createElement("canvas");
    const backgroundContext = background.getContext("2d");
    let backgroundKey = "",
      backgroundArt: Art | null = null;
    let lastEnemyPositions = new Map<number, Point>();
    let frame = 0,
      lastPaint = 0;
    const resize = () => {
      const width = wrap.clientWidth,
        height = wrap.clientHeight;
      if (!width || !height) return;
      const dpr = Math.min(2, window.devicePixelRatio || 1);
      viewport.current = { width, height };
      const bounds = worldBounds(renderMap.current);
      const full = !!wrap.closest("[data-room-fullscreen]");
      let next: Camera;
      if (cameraRoom.current !== roomID) {
        cameraRoom.current = roomID;
        // Returning to an expanded game starts at its base, at a readable scale.
        const starter = fillCamera(
          worldBounds({ width: 960, height: 640 }),
          viewport.current,
        );
        next = clampCamera(
          { ...starter, ...(data.map.base ?? { x: 480, y: 240 }) },
          bounds,
          viewport.current,
        );
        fullscreenView.current.normal = null;
      } else if (full && !fullscreenView.current.active) {
        fullscreenView.current.normal = { ...camera.current };
        const filled = fillCamera(
          worldBounds({ width: 960, height: 640 }),
          viewport.current,
        );
        next = clampCamera(
          {
            ...camera.current,
            zoom: Math.max(camera.current.zoom, filled.zoom),
          },
          bounds,
          viewport.current,
        );
      } else if (
        !full &&
        fullscreenView.current.active &&
        fullscreenView.current.normal
      ) {
        next = clampCamera(
          fullscreenView.current.normal,
          bounds,
          viewport.current,
        );
        fullscreenView.current.normal = null;
      } else next = clampCamera(camera.current, bounds, viewport.current);
      fullscreenView.current.active = full;
      publishCamera(next);
      node.width = Math.round(width * dpr);
      node.height = Math.round(height * dpr);
    };
    resize();
    const observer = new ResizeObserver(resize);
    observer.observe(wrap);
    const draw = (at: number) => {
      frame = requestAnimationFrame(draw);
      if (document.hidden || (reduced && at - lastPaint < 150)) return;
      lastPaint = at;
      const [sounds, futureSounds] = dueSoundEvents(soundEvents.current, at);
      soundEvents.current = futureSounds;
      if (sounds.length) onEventsRef.current?.(sounds, at);
      if (!wrap.clientWidth || !node.width) return;
      const current = movement.current.read(at),
        combat = timeline.current.read(at),
        v = {
          ...state.current.data,
          enemies: combat.enemies,
          baseHP: combat.baseHP,
          craters: combat.craters,
          iceWalls: combat.iceWalls,
          phase: combat.phase,
        },
        map = v.map,
        pixelScale = camera.current.zoom,
        dpr = node.width / viewport.current.width,
        scale = pixelScale * dpr;
      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.clearRect(0, 0, node.width, node.height);
      ctx.fillStyle = palette.terrain.forest;
      ctx.fillRect(0, 0, node.width, node.height);
      ctx.setTransform(
        scale,
        0,
        0,
        scale,
        (viewport.current.width / 2 - camera.current.x * pixelScale) * dpr,
        (viewport.current.height / 2 - camera.current.y * pixelScale) * dpr,
      );
      const overview = isWorldOverview(
        renderMap.current,
        viewport.current,
        camera.current,
      );
      const cacheKey = `${terrainKey.current}|${node.width}|${node.height}|${camera.current.x}|${camera.current.y}|${pixelScale}|${overview}`;
      if (
        backgroundContext &&
        (backgroundKey !== cacheKey || backgroundArt !== art.current)
      ) {
        background.width = node.width;
        background.height = node.height;
        backgroundContext.setTransform(1, 0, 0, 1, 0, 0);
        backgroundContext.fillStyle = palette.terrain.forest;
        backgroundContext.fillRect(0, 0, background.width, background.height);
        backgroundContext.setTransform(ctx.getTransform());
        drawLandscape(
          backgroundContext,
          art.current,
          map,
          pixelScale,
          undefined,
          overview,
        );
        if (previewRef.current) {
          backgroundContext.save();
          backgroundContext.globalAlpha = 0.58;
          drawLandscape(
            backgroundContext,
            art.current,
            previewRef.current,
            pixelScale,
            map,
            overview,
          );
          backgroundContext.restore();
        }
        drawRoutes(backgroundContext, map, pixelScale, undefined, overview);
        if (previewRef.current)
          drawRoutes(
            backgroundContext,
            previewRef.current,
            pixelScale,
            map,
            overview,
          );
        backgroundKey = cacheKey;
        backgroundArt = art.current;
      }
      if (backgroundContext) {
        ctx.save();
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        ctx.drawImage(background, 0, 0);
        ctx.restore();
      } else {
        drawLandscape(ctx, art.current, map, pixelScale, undefined, overview);
        drawRoutes(ctx, map, pixelScale, undefined, overview);
      }
      if (overview) {
        drawOverview(ctx, map, v, pixelScale);
        return;
      }
      const fraction =
        v.phase === "battle" ? (at - current.at) / current.duration : 1;
      drawFeatures(
        ctx,
        art.current,
        map,
        pixelScale,
        v,
        current.previousObjectives,
        fraction,
        at,
        reduced,
      );
      for (const crater of v.craters ?? []) drawCrater(ctx, crater);
      const effectClock =
        v.phase === "battle"
          ? at - timeline.current.time(0) - FLIGHT_MS
          : v.elapsedMS;
      const wallBirths = new Map(
        effects.current
          .filter((e) => e.type === "ice_wall" && e.iceWall && e.born <= at)
          .map((e) => [e.iceWall!.id, at - e.born]),
      );
      for (const wall of v.iceWalls ?? [])
        drawIceWall(
          ctx,
          wall,
          effectClock,
          reduced,
          wallBirths.get(wall.id) ?? null,
        );
      const rangePad = hoverRef.current ?? selectedRef.current;
      const sel = map.pads.find((p) => p.id === rangePad),
        tower = v.towers.find((t) => t.pad === rangePad);
      if (sel && tower) {
        ctx.save();
        circle(
          ctx,
          sel.x,
          sel.y,
          tower.effectiveRange ??
            v.catalog.find((c) => c.kind === tower.kind)?.range ??
            120,
          "#f5edc024",
        );
        ctx.setLineDash([8, 8]);
        ctx.strokeStyle = palette.road.edge;
        ctx.lineWidth = 2;
        ctx.stroke();
        ctx.restore();
      }
      for (const pad of map.pads) {
        const occupied = v.towers.find((t) => t.pad === pad.id);
        if (occupied) continue;
        ctx.save();
        ctx.translate(pad.x, pad.y);
        ctx.scale(1, 0.45);
        circle(
          ctx,
          0,
          0,
          22,
          palette.pad.fill,
          palette.pad.edge,
          Math.max(2, 1.2 / pixelScale),
        );
        if (selectedRef.current === pad.id) {
          ctx.strokeStyle = palette.pad.selected;
          ctx.lineWidth = 6;
          ctx.stroke();
        }
        ctx.restore();
      }
      const towersByID = new Map(v.towers.map((t) => [t.id, t]));
      const latestShots = new Map<number, Effect>();
      for (const event of effects.current)
        if (event.type === "shot" && event.towerId && event.born <= at)
          latestShots.set(event.towerId, event);
      const visiblePoint = (point: Point, margin = 100) => {
        const screen = worldToScreen(point, camera.current, viewport.current);
        return (
          screen.x >= -margin &&
          screen.x <= viewport.current.width + margin &&
          screen.y >= -margin &&
          screen.y <= viewport.current.height + margin
        );
      };
      const { width: towerSize, height: towerHeight } =
        towerDimensions(pixelScale);
      const buildings: { y: number; draw: () => void }[] = [];
      for (const t of v.towers) {
        const pad = map.pads.find((p) => p.id === t.pad);
        if (!pad || !visiblePoint(pad, towerHeight * pixelScale + 32)) continue;
        const lastShot = latestShots.get(t.id);
        const pose = shotPose(
          t.kind,
          lastShot ? at - lastShot.born : -1,
          reduced,
        );
        buildings.push({
          y: pad.y,
          draw: () => {
            const towerPoint = {
              x: pad.x - Math.cos(t.angle) * pose.recoil,
              y: pad.y + 14 - Math.sin(t.angle) * pose.recoil,
            };
            if (pose.glow > 0)
              circle(
                ctx,
                pad.x,
                pad.y - towerSize * 0.55,
                12 + pose.glow * 15,
                `rgba(255,235,160,${pose.glow * 0.6})`,
              );
            ctx.save();
            ctx.shadowColor = "#243b2d40";
            ctx.shadowBlur = 4;
            ctx.shadowOffsetY = 3;
            if (
              !sprite(
                ctx,
                art.current,
                `tower-${t.kind}-${Math.min(3, t.level)}`,
                towerPoint.x,
                towerPoint.y,
                towerSize,
                towerHeight,
              )
            )
              fallbackTower(ctx, towerPoint.x, towerPoint.y, towerSize, t.kind);
            ctx.restore();
            // The aim marker rotates from server angles; it never picks a target itself.
            if (v.phase === "battle") {
              const pivot = {
                x: towerPoint.x,
                y: towerPoint.y - towerSize * 0.59,
              };
              ctx.save();
              ctx.translate(pivot.x, pivot.y);
              ctx.rotate(t.angle);
              ctx.strokeStyle =
                t.kind === "frost"
                  ? "#d4f5ff"
                  : t.kind === "arcane"
                    ? "#f1d7ff"
                    : "#ffebaa";
              ctx.lineWidth = 4;
              ctx.beginPath();
              ctx.moveTo(8, 0);
              ctx.lineTo(19, 0);
              ctx.stroke();
              ctx.restore();
            }
            if (t.level > 1) {
              ctx.font = `bold ${Math.max(13, 9 / pixelScale)}px system-ui`;
              ctx.textAlign = "center";
              ctx.lineWidth = 3;
              ctx.strokeStyle = "#263c32";
              ctx.strokeText(
                `★${t.level}`,
                towerPoint.x,
                towerPoint.y - towerSize * 0.93,
              );
              ctx.fillStyle = "#fff2b8";
              ctx.fillText(
                `★${t.level}`,
                towerPoint.x,
                towerPoint.y - towerSize * 0.93,
              );
            }
          },
        });
      }
      const endpoint = map.path[map.path.length - 1] ?? {
        x: map.width,
        y: map.height / 2,
      };
      const bx = map.base?.x ?? Math.min(map.width - 55, endpoint.x - 65),
        by = (map.base?.y ?? endpoint.y) + 35;
      buildings.push({
        y: by - 35,
        draw: () => {
          const baseSize = Math.max(105, 38 / pixelScale);
          const damaged = v.baseHP < v.baseMaxHP * 0.5;
          if (
            !sprite(
              ctx,
              art.current,
              `base-${damaged ? "damaged" : "intact"}`,
              bx,
              by,
              baseSize,
              baseSize,
            )
          ) {
            ctx.fillStyle = damaged ? "#b19291" : "#a5bacc";
            ctx.fillRect(bx - 40, by - 76, 80, 76);
            ctx.strokeStyle = palette.unitOutline;
            ctx.lineWidth = Math.max(3, 1.2 / pixelScale);
            ctx.strokeRect(bx - 40, by - 76, 80, 76);
            circle(ctx, bx, by - 72, 18, "#f8e7a1", palette.unitOutline);
          }
          if (!reduced && v.phase !== "lost") {
            const pulse = 0.3 + 0.12 * Math.sin(at / 700);
            circle(
              ctx,
              bx,
              by - baseSize * 0.62,
              baseSize * 0.13,
              `rgba(255,239,151,${pulse})`,
            );
          }
          ctx.font = `bold ${Math.max(15, 10 / pixelScale)}px system-ui`;
          ctx.textAlign = "center";
          ctx.strokeStyle = palette.label.halo;
          ctx.lineWidth = 4;
          ctx.strokeText("星核", bx, by + 15);
          ctx.fillStyle = palette.label.ink;
          ctx.fillText("星核", bx, by + 15);
        },
      });
      buildings.sort((a, b) => a.y - b.y).forEach((item) => item.draw());
      drawnEnemies.current = [];
      const enemyPositions = new Map<number, Point>();
      for (const enemy of v.enemies) {
        const destination = current.destinations.get(enemy.id);
        const position = enemyPoint(
          map.routes?.find((route) => route.id === enemy.route)?.points ??
            map.path,
          current.previous.get(enemy.id),
          destination ?? enemy,
          fraction,
          reduced,
        );
        const crater = enemy.repairing
          ? v.craters?.find((c) => c.id === enemy.repairing && c.remaining > 0)
          : undefined;
        const workerIndex = Math.max(0, crater?.workers.indexOf(enemy.id) ?? 0);
        const workerCount = Math.max(1, crater?.workers.length ?? 0);
        if (crater) {
          const pose = workerPose(
            workerIndex,
            workerCount,
            at,
            reduced || v.phase !== "battle",
          );
          position.x = crater.x + pose.x;
          position.y = crater.y + pose.y;
        }
        enemyPositions.set(enemy.id, position);
        if (!visiblePoint(position, Math.max(64 * pixelScale, 64))) continue;
        if (inspected.current === enemy.id) {
          ctx.save();
          ctx.lineWidth = 2 / pixelScale;
          circle(
            ctx,
            position.x,
            position.y,
            24 / pixelScale,
            "#fff8cf22",
            "#fff4b0",
          );
          ctx.restore();
        }
        const frozen = enemy.frostbound || enemy.slowUntil > v.tick;
        if (enemy.kind === "boss" && (enemy.phase ?? 1) > 1) {
          const size = Math.max(100, 60 / pixelScale);
          sprite(
            ctx,
            art.current,
            "effect-boss-aura",
            position.x,
            position.y,
            size,
            size,
            "center",
          );
        }
        const size = Math.max(
          enemy.kind === "boss" ? 64 : 42,
          (enemy.kind === "boss" ? 40 : 32) / pixelScale,
        );
        drawnEnemies.current.push({
          enemy,
          point: worldToScreen(
            { x: position.x, y: position.y - size * 0.3 },
            camera.current,
            viewport.current,
          ),
        });
        const moving = !reduced && v.phase === "battle" && !crater;
        const pose = moving ? Math.floor(at / 160 + enemy.id) % 3 : 0;
        const bob = moving
          ? Math.sin(at / (enemy.kind === "runner" ? 90 : 140) + enemy.id) * 1.5
          : 0;
        const arrival =
          crater && !reduced
            ? effects.current.find(
                (e) =>
                  e.type === "repair_start" &&
                  e.targetId === enemy.id &&
                  e.born <= at,
              )
            : undefined;
        const falling = arrival
          ? Math.sin(Math.min(1, (at - arrival.born) / 420) * Math.PI)
          : 0;
        const x = position.x,
          y = position.y + bob + falling * 9;
        ctx.save();
        ctx.translate(x, y);
        ctx.scale(1, 0.32);
        circle(ctx, 0, 3, size * 0.3, "#43593750");
        ctx.restore();
        if (frozen) {
          ctx.save();
          ctx.translate(x, y);
          ctx.scale(1, 0.38);
          circle(ctx, 0, 0, size * 0.5, "#bce9ff60", "#3e809e");
          ctx.restore();
        }
        if (
          !sprite(
            ctx,
            art.current,
            `enemy-${enemy.kind}-${pose}`,
            x,
            y + size * 0.2,
            size,
            size,
          )
        )
          fallbackEnemy(ctx, x, y + size * 0.15, size, enemy.kind);
        if (enemy.frostbound) {
          ctx.save();
          ctx.strokeStyle = "#ecfbff";
          ctx.lineWidth = Math.max(1.5, 1 / pixelScale);
          ctx.beginPath();
          for (let i = 0; i < 3; i++) {
            const fx = x + (i - 1) * size * 0.19,
              fy = y + size * 0.13;
            ctx.moveTo(fx, fy - 7);
            ctx.lineTo(fx, fy + 2);
            ctx.moveTo(fx - 4, fy - 4);
            ctx.lineTo(fx + 4, fy - 1);
          }
          ctx.stroke();
          ctx.restore();
        }
        if (crater)
          drawRepairTool(
            ctx,
            x,
            y,
            workerIndex,
            workerCount,
            at,
            reduced || v.phase !== "battle",
          );
        if (["healer", "banner", "engineer"].includes(enemy.kind)) {
          const label =
            enemy.kind === "healer"
              ? "治疗"
              : enemy.kind === "banner"
                ? "加速"
                : "破门";
          ctx.font = `bold ${Math.max(11, 9 / pixelScale)}px system-ui`;
          ctx.textAlign = "center";
          ctx.lineWidth = 3;
          ctx.strokeStyle = palette.label.halo;
          ctx.fillStyle = palette.label.danger;
          ctx.strokeText(label, x, y + size * 0.4);
          ctx.fillText(label, x, y + size * 0.4);
        }
        if (enemy.kind === "boss" && (enemy.phase ?? 1) > 1) {
          ctx.font = `bold ${Math.max(14, 11 / pixelScale)}px system-ui`;
          ctx.textAlign = "center";
          ctx.lineWidth = 4;
          ctx.strokeStyle = "#fff5e9";
          ctx.fillStyle = "#802f50";
          ctx.strokeText("狂暴阶段", x, y + size * 0.4);
          ctx.fillText("狂暴阶段", x, y + size * 0.4);
        }
        const barWidth = size * 0.66,
          barHeight = Math.max(4, 2.5 / pixelScale),
          barY = y - size * 0.75,
          border = Math.max(1, 1 / pixelScale),
          health = Math.max(
            0,
            Math.min(
              1,
              (enemy.health ?? enemy.hp) /
                Math.max(1, enemy.maxHealth ?? enemy.maxHP),
            ),
          );
        ctx.fillStyle = palette.health.track;
        ctx.fillRect(
          x - barWidth / 2 - border,
          barY - border,
          barWidth + border * 2,
          barHeight + border * 2,
        );
        ctx.fillStyle =
          health < 0.35 ? palette.health.critical : palette.health.healthy;
        ctx.fillRect(x - barWidth / 2, barY, barWidth * health, barHeight);
      }
      effects.current = activeEffects(effects.current, at, reduced);
      // One explosion represents the whole blast. Keep every projectile and
      // authoritative impact, but bound decorative bursts in dense battles.
      for (const event of effects.current) {
        if (event.born <= at && event.impactTargetId)
          event.impact ??= enemyPositions.get(event.impactTargetId) ??
            lastEnemyPositions.get(event.impactTargetId) ?? {
              x: event.x,
              y: event.y,
            };
      }
      const accentIDs = new Set<number>();
      const accentTypes = new Set([
        "hit",
        "death",
        "crater",
        "crater_filled",
        "ice_wall",
        "frostbound",
        "chain",
        "frost_ring",
      ]);
      for (
        let i = effects.current.length - 1;
        i >= 0 && accentIDs.size < 48;
        i--
      ) {
        const event = effects.current[i];
        if (
          event.born <= at &&
          accentTypes.has(event.type) &&
          visiblePoint(event.impact ?? event)
        )
          accentIDs.add(event.id);
      }
      for (const e of effects.current) {
        const elapsed = at - e.born,
          t = elapsed / 900;
        if (elapsed < 0) continue;
        if (reduced) {
          if (elapsed < 250 && (e.type === "base_hit" || e.type === "build"))
            circle(ctx, e.x, e.y, 16, "#fff4c880");
          continue;
        }
        const accent = {
          ...e,
          kind: towersByID.get(e.towerId ?? 0)?.kind ?? e.kind,
        };
        if (accentIDs.has(e.id))
          drawCombatAccent(
            ctx,
            accent,
            elapsed,
            e.impact?.x ?? e.x,
            e.impact?.y ?? e.y,
            reduced,
          );
        ctx.save();
        if (e.type === "shot" && elapsed < FLIGHT_MS) {
          const target = e.targetId
            ? enemyPositions.get(e.targetId)
            : undefined;
          const p = Math.min(1, elapsed / FLIGHT_MS),
            tx = target?.x ?? e.toX ?? e.x,
            ty = target?.y ?? e.toY ?? e.y;
          const x = e.x + (tx - e.x) * p,
            y =
              e.y +
              (ty - e.y) * p -
              (e.kind === "cannon" ? Math.sin(p * Math.PI) * 45 : 0);
          ctx.strokeStyle =
            e.kind === "frost"
              ? "#b9efffaa"
              : e.kind === "arcane"
                ? "#d2a0ffbb"
                : e.skill === "focus"
                  ? "#fff4bbee"
                  : "#ead4a477";
          ctx.lineWidth = ["focus", "execute", "heart", "volley"].includes(
            e.skill ?? "",
          )
            ? 5
            : 3;
          ctx.beginPath();
          const tail = Math.max(0, p - 0.22);
          ctx.moveTo(
            e.x + (tx - e.x) * tail,
            e.y +
              (ty - e.y) * tail -
              (e.kind === "cannon" ? Math.sin(tail * Math.PI) * 45 : 0),
          );
          ctx.lineTo(x, y);
          ctx.stroke();
          ctx.translate(x, y);
          ctx.rotate(Math.atan2(ty - e.y, tx - e.x));
          if (
            !sprite(
              ctx,
              art.current,
              `projectile-${e.kind}`,
              0,
              0,
              24,
              24,
              "center",
            )
          )
            circle(ctx, 0, 0, 5, towerColors[e.kind ?? "arrow"] ?? "#f7e7a0");
        } else if (
          [
            "hit",
            "death",
            "base_hit",
            "heal",
            "summon",
            "boss_phase",
            "shortcut",
          ].includes(e.type)
        ) {
          e.impact ??= (e.targetId
            ? (enemyPositions.get(e.targetId) ??
              lastEnemyPositions.get(e.targetId))
            : undefined) ?? { x: e.x, y: e.y };
          ctx.globalAlpha = 1 - t;
          const name =
            e.type === "heal"
              ? "frost"
              : e.type === "boss_phase"
                ? "arcane"
                : e.type === "base_hit"
                  ? "burst"
                  : e.kind === "frost"
                    ? "frost"
                    : e.kind === "arcane"
                      ? "arcane"
                      : "burst";
          const size = (e.type === "death" ? 38 : 24) + t * 26;
          if (
            !sprite(
              ctx,
              art.current,
              `effect-${name}`,
              e.impact.x,
              e.impact.y,
              size,
              size,
              "center",
            )
          )
            circle(ctx, e.impact.x, e.impact.y, 8 + t * 15, "#ffefa870");
        } else if (
          [
            "build",
            "upgrade",
            "sell",
            "gift",
            "objective",
            "boon",
            "expand",
          ].includes(e.type)
        ) {
          ctx.globalAlpha = 1 - t;
          ctx.lineWidth = 4;
          ctx.strokeStyle = e.type === "sell" ? "#fff0b0" : "#f6ffdf";
          ctx.beginPath();
          ctx.ellipse(e.x, e.y, 18 + t * 42, 8 + t * 18, 0, 0, Math.PI * 2);
          ctx.stroke();
          if (e.type === "upgrade") {
            ctx.font = "bold 22px system-ui";
            ctx.textAlign = "center";
            ctx.fillStyle = "#fff8c8";
            ctx.fillText("✦", e.x, e.y - 25 - t * 30);
          }
        }
        ctx.restore();
      }
      lastEnemyPositions = enemyPositions;
    };
    frame = requestAnimationFrame(draw);
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, [
    data.map.width,
    data.map.height,
    preview?.width,
    preview?.height,
    roomID,
    reduced,
  ]);
  const bounds = worldBounds(preview ?? data.map);
  const overview = isWorldOverview(
    preview ?? data.map,
    view.viewport,
    view.camera,
  );
  const windowRect = cameraWindow(view.camera, {
    width: Math.max(1, view.viewport.width),
    height: Math.max(1, view.viewport.height),
  });
  const routes = data.map.routes ?? [
    {
      id: "main",
      label: "林间入口",
      sector: 0,
      points: data.map.path,
      active: true,
    },
  ];
  const base = data.map.base ??
    data.map.path[data.map.path.length - 1] ?? { x: 860, y: 340 };
  return (
    <section className="sf-world-camera" aria-label="战场与地图导航">
      <div className="sf-map-tools">
        <div
          className="sf-camera-toolbar"
          data-controller-group="camera"
          role="group"
          aria-label="地图视角"
        >
          <button
            className="sf-secondary"
            data-sf-zoom-out
            aria-label="缩小地图"
            onClick={() => zoom(1 / 1.3)}
          >
            −
          </button>
          <output aria-label="地图缩放">
            {Math.round(view.camera.zoom * 100)}%
          </output>
          <button
            className="sf-secondary"
            data-sf-zoom-in
            aria-label="放大地图"
            onClick={() => zoom(1.3)}
          >
            ＋
          </button>
          <button
            className="sf-secondary"
            data-sf-camera-base
            onClick={() => {
              const starter = fitCamera(
                worldBounds({ width: 960, height: 640 }),
                viewport.current,
              );
              publishCamera(
                focusCamera(
                  {
                    ...camera.current,
                    zoom: Math.max(camera.current.zoom, starter.zoom),
                  },
                  base,
                  bounds,
                  viewport.current,
                ),
              );
            }}
          >
            回到星核
          </button>
          <button
            className="sf-secondary"
            data-sf-fit
            onClick={() => publishCamera(fitCamera(bounds, viewport.current))}
          >
            全图
          </button>
        </div>
        <details className="sf-entry-menu">
          <summary>入口</summary>
          <div className="sf-entry-list" aria-label="进攻入口">
            {routes.map((route) => {
              const enemies = data.enemies.filter(
                (enemy) => (enemy.route ?? "main") === route.id,
              ).length;
              return (
                <button
                  key={route.id}
                  className="sf-secondary"
                  data-sf-entry={route.id}
                  data-active={route.active}
                  onClick={() => focus(route.points[0] ?? base)}
                >
                  <span aria-hidden="true">{route.active ? "●" : "○"}</span>{" "}
                  {route.label}
                  {enemies > 0
                    ? ` · ${enemies} 敌人`
                    : route.active
                      ? " · 出怪"
                      : " · 休整"}
                </button>
              );
            })}
          </div>
        </details>
        {!!data.world?.choice?.options.some((option) => option.map) && (
          <details className="sf-preview-menu">
            <summary>扩展预览</summary>
            <div
              className="sf-map-previews"
              role="group"
              aria-label="扩展地图预览"
            >
              <span>虚线为新道路：</span>
              <button
                className="sf-secondary"
                aria-pressed={!preview}
                onClick={() => setPreviewID(null)}
              >
                当前战场
              </button>
              {data.world.choice.options
                .filter((option) => option.map)
                .map((option) => (
                  <button
                    className="sf-secondary"
                    key={option.id}
                    aria-pressed={previewID === option.id}
                    data-sf-preview={option.id}
                    onClick={() => {
                      setPreviewID(option.id);
                      publishCamera(
                        fitCamera(worldBounds(option.map!), viewport.current),
                      );
                    }}
                  >
                    预览{option.label}
                  </button>
                ))}
            </div>
          </details>
        )}
      </div>
      <div
        className={`sf-battlefield sf-world-viewport ${overview ? "sf-world-overview" : ""}`}
        ref={container}
        data-assets-ready={loaded}
        data-overview={overview}
        onPointerDown={pointerDown}
        onPointerMove={pointerMove}
        onPointerUp={pointerUp}
        onPointerCancel={pointerUp}
        onLostPointerCapture={pointerUp}
        onPointerLeave={(event) => {
          if (!event.currentTarget.hasPointerCapture(event.pointerId))
            pointerUp(event);
        }}
        onClick={(event) => {
          if (performance.now() < gesture.current.suppressUntil) return;
          if (
            event.target === canvas.current ||
            event.target === container.current
          ) {
            const rect = canvas.current!.getBoundingClientRect();
            const x = event.clientX - rect.left,
              y = event.clientY - rect.top;
            const hit = [...drawnEnemies.current]
              .reverse()
              .find((e) => Math.hypot(e.point.x - x, e.point.y - y) <= 24);
            if (hit) {
              inspected.current = hit.enemy.id;
              setInspection(hit);
              return;
            }
            closeInspection();
          }
          if (
            !isWorldOverview(
              renderMap.current,
              viewport.current,
              camera.current,
            ) ||
            performance.now() < gesture.current.suppressUntil
          )
            return;
          event.preventDefault();
          event.stopPropagation();
          const rect = canvas.current!.getBoundingClientRect();
          const point = screenToWorld(
            { x: event.clientX - rect.left, y: event.clientY - rect.top },
            camera.current,
            viewport.current,
          );
          publishCamera(
            detailCameraAt(
              camera.current,
              point,
              renderMap.current,
              viewport.current,
            ),
          );
        }}
        onClickCapture={(event) => {
          if (performance.now() < gesture.current.suppressUntil) {
            event.preventDefault();
            event.stopPropagation();
          }
        }}
      >
        <canvas
          ref={canvas}
          data-starfort-canvas
          data-enemies={data.enemies.length}
          data-overview={overview}
          data-world-width={data.map.width}
          data-world-height={data.map.height}
          data-zoom={view.camera.zoom}
          data-camera-x={view.camera.x}
          data-camera-y={view.camera.y}
          role="img"
          aria-label={`星垒战场，第 ${data.wave} 波，${data.enemies.length} 名敌人，${data.towers.length} 座防御塔。拖动或双指缩放查看地图，空地可建造，点击塔身可升级，所有按钮均可用键盘选择。`}
        />
        <div data-controller-group="board" style={{ display: "contents" }}>
          {data.map.pads.map((pad) => {
            const tower = data.towers.find((t) => t.pad === pad.id);
            const point = worldToScreen(pad, view.camera, view.viewport);
            const dimensions = towerDimensions(view.camera.zoom);
            const width = dimensions.width * view.camera.zoom;
            const height = dimensions.height * view.camera.zoom;
            const bottom = point.y + 14 * view.camera.zoom;
            return (
              <button
                key={pad.id}
                data-sf-pad={pad.id}
                data-pad={pad.id}
                hidden={overview}
                className={`sf-pad ${tower ? "sf-pad-occupied" : ""} ${selected === pad.id ? "sf-pad-selected" : ""}`}
                style={{
                  left: point.x,
                  zIndex: tower
                    ? 1 +
                      [...data.map.pads]
                        .sort((a, b) => a.y - b.y)
                        .findIndex((p) => p.id === pad.id)
                    : 0,
                  top: tower ? bottom : point.y,
                  ...(tower ? { width, height } : {}),
                }}
                aria-label={`建造点 ${pad.id + 1}${pad.terrain === "highground" ? "，高台" : ""}${tower ? `，${towerNames[tower.kind]} ${tower.level} 级，玩家 ${tower.owner + 1}` : "，空地"}`}
                aria-pressed={selected === pad.id}
                onPointerEnter={(event) => {
                  if (event.pointerType === "mouse" && tower)
                    hoverRef.current = pad.id;
                }}
                onPointerLeave={() => {
                  hoverRef.current = null;
                }}
                onFocus={() => {
                  if (tower) hoverRef.current = pad.id;
                }}
                onBlur={() => {
                  hoverRef.current = null;
                }}
                onClick={() => {
                  closeInspection();
                  select(pad.id);
                }}
              >
                {!tower && <span>{pad.id + 1}</span>}
              </button>
            );
          })}
        </div>
        {inspection ? (
          <MapPopover point={inspection.point} viewport={view.viewport}>
            <section
              className="sf-enemy-info"
              data-sf-enemy-info
              aria-label="敌人信息"
            >
              <header>
                <strong>
                  {enemyInfo[inspection.enemy.kind]?.[0] ??
                    inspection.enemy.kind}{" "}
                  · Lv.{inspection.enemy.level ?? data.wave}
                </strong>
                <button
                  data-sf-enemy-close
                  aria-label="关闭敌人信息"
                  onClick={closeInspection}
                >
                  ×
                </button>
              </header>
              <p>
                生命{" "}
                {Math.ceil(
                  inspection.enemy.health ??
                    inspection.enemy.hp + inspection.enemy.shield,
                )}{" "}
                /{" "}
                {Math.ceil(
                  inspection.enemy.maxHealth ??
                    inspection.enemy.maxHP + inspection.enemy.shield,
                )}
              </p>
              <p>
                基础速度 {Math.round(inspection.enemy.speed)}
                {inspection.enemy.frostbound ? " · 永久减速 30%" : ""}
                {inspection.enemy.slowUntil > data.tick
                  ? " · 临时减速 45%"
                  : ""}
                {inspection.enemy.repairing ? " · 填坑中" : ""}
              </p>
              <p>{enemyInfo[inspection.enemy.kind]?.[1]}</p>
            </section>
          </MapPopover>
        ) : towerPanel &&
          selected !== null &&
          data.map.pads.some((p) => p.id === selected) ? (
          <MapPopover
            point={worldToScreen(
              data.map.pads.find((p) => p.id === selected)!,
              view.camera,
              view.viewport,
            )}
            viewport={view.viewport}
          >
            {towerPanel}
          </MapPopover>
        ) : null}
        {overview && (
          <span className="sf-overview-hint" role="status">
            总览 · 点选区域放大
          </span>
        )}
        {!loaded && (
          <span className="sf-loading" role="status">
            {assetError
              ? "画面素材暂未加载，已使用简化图形。"
              : "正在点亮星垒…"}
          </span>
        )}
        {data.phase === "paused" && (
          <div className="sf-paused-overlay">
            <strong>守望暂停</strong>
            <span>全员同意后继续</span>
          </div>
        )}
      </div>
      <div className="sf-map-navigation">
        <svg
          className="sf-minimap"
          data-sf-minimap
          viewBox={`${bounds.x} ${bounds.y} ${bounds.width} ${bounds.height}`}
          role="img"
          aria-label="战场小地图，点击或拖动定位"
          onPointerDown={(event) => {
            if (event.button !== 0) return;
            event.preventDefault();
            event.currentTarget.setPointerCapture(event.pointerId);
            navigateMinimap(event);
          }}
          onPointerMove={(event) => {
            if (event.currentTarget.hasPointerCapture(event.pointerId))
              navigateMinimap(event);
          }}
          onPointerUp={(event) => {
            if (event.currentTarget.hasPointerCapture(event.pointerId))
              event.currentTarget.releasePointerCapture(event.pointerId);
          }}
          onPointerCancel={(event) => {
            if (event.currentTarget.hasPointerCapture(event.pointerId))
              event.currentTarget.releasePointerCapture(event.pointerId);
          }}
        >
          <rect
            x={bounds.x}
            y={bounds.y}
            width={bounds.width}
            height={bounds.height}
            fill="#d8dfbd"
          />
          {(data.map.regions ?? []).map((region) => (
            <rect
              key={region.id}
              x={region.x}
              y={region.y}
              width={region.width}
              height={region.height}
              fill={regionColors[region.biome] ?? regionColors.forest}
            />
          ))}
          {(preview?.routes ?? routes).map((route) => (
            <polyline
              key={route.id}
              points={route.points.map((p) => `${p.x},${p.y}`).join(" ")}
              fill="none"
              stroke={route.active ? "#805438" : "#64705d"}
              strokeWidth={14}
              strokeDasharray={route.active ? undefined : "30 20"}
            />
          ))}
          {data.towers.map((tower) => {
            const pad = data.map.pads.find((p) => p.id === tower.pad);
            return pad ? (
              <circle
                key={tower.id}
                cx={pad.x}
                cy={pad.y}
                r={13}
                fill={colors[tower.owner % colors.length]}
              />
            ) : null;
          })}
          {data.enemies.map((enemy) => (
            <circle
              key={enemy.id}
              cx={enemy.x}
              cy={enemy.y}
              r={10}
              fill="#982d38"
            />
          ))}
          <circle
            cx={base.x}
            cy={base.y}
            r={22}
            fill="#fff2a2"
            stroke="#664816"
            strokeWidth={7}
          />
          <rect
            data-sf-minimap-window
            x={windowRect.x}
            y={windowRect.y}
            width={windowRect.width}
            height={windowRect.height}
            fill="#ffffff18"
            stroke="#263e54"
            strokeWidth={8}
            vectorEffect="non-scaling-stroke"
            style={{ strokeWidth: 2 }}
          />
        </svg>
        {preview && (
          <p>
            <strong>扩展方案预览</strong>
          </p>
        )}
      </div>
    </section>
  );
}
