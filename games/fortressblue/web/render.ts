import animationData from "../data/animations.json" with { type: "json" };
import {
  cellSize,
  explosion,
  tankInfo,
  teamColors,
  type MapInfo,
  type Tank,
  type View,
  type Crater,
} from "./types";
import { decodeMask, carveCraters, solidAt, type Camera } from "./aim";
import { arcAt, fallAt, renderableShot } from "./timeline";
import { supportSlope, selectSlopeFrame } from "./slope";
import { tankPose } from "./animation";
export type Frame = {
  atlas: number;
  x: number;
  y: number;
  w: number;
  h: number;
  ax: number;
  ay: number;
  sourceIndex: number;
  facing: string;
};
type Action = {
  left: number[];
  right: number[];
  durationMs: number;
  loop: boolean;
  slopeVariants?: number;
};
type Animation = {
  atlases: string[];
  frames: Frame[];
  actions: Record<string, Action>;
};
export const animations = animationData.tanks as unknown as Record<
  string,
  Animation
>;
export type Images = Record<string, HTMLImageElement | null>;
const collisionMasks = new Map<string, Uint8Array>();
const terrainMasks = new WeakMap<View, Uint8Array>();
export type Motion = {
  fromX: number;
  fromY: number;
  x: number;
  y: number;
  at: number;
};
export function paintTerrain(
  canvas: HTMLCanvasElement,
  map: MapInfo,
  fg: HTMLImageElement | null,
  fg2: HTMLImageElement | null,
  craters: Crater[],
) {
  canvas.width = map.width;
  canvas.height = map.height;
  const ctx = canvas.getContext("2d");
  if (!ctx) return;
  ctx.imageSmoothingEnabled = false;
  if (fg) ctx.drawImage(fg, 0, map.offY);
  else {
    // The collision mask remains visible while artwork is loading or unavailable.
    const bits = decodeMask(map.mask);
    ctx.fillStyle = "#887d57";
    for (let y = 0; y < map.maskRows; y++)
      for (let x = 0; x < map.maskCols; x++) {
        const i = y * map.maskCols + x;
        if ((bits[i >> 3] & (1 << (7 - (i & 7)))) !== 0)
          ctx.fillRect(x * cellSize, y * cellSize, cellSize, cellSize);
      }
  }
  if (fg2) ctx.drawImage(fg2, 0, map.offY);
  ctx.globalCompositeOperation = "destination-out";
  for (const c of craters) {
    ctx.beginPath();
    ctx.arc(c.x, c.y, c.r, 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.globalCompositeOperation = "source-over";
  canvas.dataset.artReady = fg ? "1" : "0";
}
function sprite(
  ctx: CanvasRenderingContext2D,
  tank: Tank,
  action: string,
  elapsed: number,
  images: Images,
  slope: number,
) {
  const sheet = animations[String(tank.tankId)];
  const clip = sheet?.actions[action] ?? sheet?.actions.idle;
  const indices = tank.facing < 0 ? clip?.left : clip?.right;
  if (!sheet || !clip || !indices?.length) return null;
  const progress = clip.loop
    ? (Math.max(0, elapsed) % Math.max(1, clip.durationMs)) /
      Math.max(1, clip.durationMs)
    : Math.min(0.9999, Math.max(0, elapsed) / Math.max(1, clip.durationMs));
  const frame = selectSlopeFrame(
    sheet.frames,
    indices[Math.floor(progress * indices.length)],
    slope,
    tank.facing,
    clip.slopeVariants ?? 1,
  );
  if (!frame) return null;
  const image = images[sheet.atlases[frame.atlas]];
  if (!image) return null;
  ctx.drawImage(
    image,
    frame.x,
    frame.y,
    frame.w,
    frame.h,
    tank.x - frame.ax,
    tank.y - frame.ay,
    frame.w,
    frame.h,
  );
  return frame;
}
export function miniRect(width: number, map: MapInfo) {
  const w = width < 500 ? 90 : 140;
  return { x: width - w - 10, y: 10, w, h: (w * map.height) / map.width };
}
export function drawScene(input: {
  canvas: HTMLCanvasElement;
  terrain: HTMLCanvasElement | null;
  map: MapInfo;
  view: View;
  camera: Camera;
  images: Images;
  bg: HTMLImageElement | null;
  now: number;
  angle: number;
  facing: number;
  canAim: boolean;
  names: string[];
  motions: Map<number, Motion>;
  reduced: boolean;
}) {
  const {
    canvas,
    terrain,
    map,
    view,
    camera: c,
    images,
    bg,
    now,
    angle,
    facing,
    canAim,
    names,
    motions,
    reduced,
  } = input;
  const ctx = canvas.getContext("2d");
  if (!ctx) return;
  const rect = canvas.getBoundingClientRect(),
    w = rect.width,
    h = rect.height,
    dpr = canvas.width / Math.max(1, w),
    scale = w / c.worldViewW;
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, w, h);
  ctx.imageSmoothingEnabled = false;
  const sky = ctx.createLinearGradient(0, 0, 0, h);
  sky.addColorStop(0, "#33556e");
  sky.addColorStop(1, "#b4d0cf");
  ctx.fillStyle = sky;
  ctx.fillRect(0, 0, w, h);
  if (bg) {
    // Background has its own parallax, but shares the map's original scale.
    const bw = Math.max(w, bg.width * scale),
      bh = Math.max(h, bg.height * scale);
    ctx.drawImage(
      bg,
      -(c.x / Math.max(1, map.width - c.worldViewW)) * (bw - w),
      -(c.y / Math.max(1, map.height - c.worldViewH)) * (bh - h),
      bw,
      bh,
    );
  }
  ctx.save();
  ctx.scale(scale, scale);
  ctx.translate(-c.x, -c.y);
  if (terrain) ctx.drawImage(terrain, 0, 0);
  let mask = terrainMasks.get(view);
  if (!mask) {
    let base = collisionMasks.get(map.id);
    if (!base) {
      base = decodeMask(map.mask);
      collisionMasks.set(map.id, base);
    }
    mask = carveCraters(
      base,
      map.maskCols,
      map.maskRows,
      cellSize,
      view.craters ?? [],
    );
    terrainMasks.set(view, mask);
  }
  const terrainSolid = (x: number, y: number) =>
    solidAt(
      mask!,
      map.maskCols,
      map.maskRows,
      cellSize,
      x,
      y,
      map.width,
      map.height,
    );
  const hazards = (view.hazards ?? []).filter(
    (h) => now >= h.at && now < h.endAt,
  );
  if (hazards.length) {
    const supported = terrainSolid;
    for (const hazard of hazards) {
      for (let x = hazard.x - hazard.r; x <= hazard.x + hazard.r; x += 9) {
        let ground: number | null = null;
        // Search only the supporting surface near this hazard, not another level.
        for (let y = hazard.y - 20; y <= hazard.y + 20; y += 2) {
          if (supported(x, y) && !supported(x, y - 3)) {
            ground = y;
            break;
          }
        }
        if (ground === null) continue;
        const edge = Math.max(
          0.2,
          1 - Math.abs(x - hazard.x) / Math.max(1, hazard.r),
        );
        const wave = reduced ? 0.5 : 0.5 + 0.5 * Math.sin(now / 170 + x);
        ctx.save();
        ctx.globalAlpha = 0.55 + edge * 0.3;
        if (hazard.kind === "fire") {
          const height = 7 + edge * (7 + wave * 7);
          ctx.fillStyle = "#ff8b3e";
          ctx.beginPath();
          ctx.moveTo(x - 5, ground);
          ctx.quadraticCurveTo(
            x - 5,
            ground - height * 0.7,
            x + 1,
            ground - height,
          );
          ctx.quadraticCurveTo(x + 6, ground - height * 0.5, x + 5, ground);
          ctx.closePath();
          ctx.fill();
          ctx.fillStyle = "#ffec9b";
          ctx.beginPath();
          ctx.ellipse(x, ground - 3, 2, 4 + wave * 2, 0, 0, Math.PI * 2);
          ctx.fill();
        } else {
          ctx.fillStyle = "#b8e481";
          ctx.beginPath();
          ctx.ellipse(
            x,
            ground - 5 - wave * 4,
            8,
            4 + edge * 3,
            0,
            0,
            Math.PI * 2,
          );
          ctx.fill();
          ctx.fillStyle = "#e0fac4";
          ctx.beginPath();
          ctx.arc(x + 3, ground - 11 - wave * 5, 1.5, 0, Math.PI * 2);
          ctx.fill();
        }
        ctx.restore();
      }
    }
  }
  canvas.dataset.renderedHazards = JSON.stringify(
    hazards.map((h) => ({ id: h.id, kind: h.kind, x: h.x, y: h.y, r: h.r })),
  );
  const shot = renderableShot(view.lastShot);
  let projectileCount = 0;
  const renderedTanks: {
    seat: number;
    x: number;
    y: number;
    hp: number;
    alive: boolean;
    action: string;
    screenX: number;
    screenY: number;
    screenWidth: number;
    screenHeight: number;
    artReady: boolean;
    slope: number;
    sourceIndex?: number;
    supportLeft: number | null;
    supportRight: number | null;
    screenLeft: number;
    screenTop: number;
  }[] = [];
  const renderedProjectiles: { x: number; y: number }[] = [];
  const renderedFeedback: {
    type: string;
    target: number;
    amount: number;
    at: number;
    x: number;
    y: number;
    screenX: number;
    screenY: number;
  }[] = [];
  for (const original of view.tanks) {
    if (!original.selected) continue;
    const tank = { ...original };
    const fall = view.resolution?.falls?.find((f) => f.seat === tank.seat);
    const motion = motions.get(tank.seat);
    const actions = animations[String(tank.tankId)]?.actions;
    const damage = [...(view.events ?? [])]
      .reverse()
      .find(
        (e) =>
          (e.type === "hit" || e.type === "status") &&
          e.target === tank.seat &&
          e.at <= now,
      );
    const death = !tank.alive
      ? [...(view.events ?? [])]
          .reverse()
          .find((e) => e.type === "death" && e.target === tank.seat)
      : undefined;
    const item = [...(view.events ?? [])]
      .reverse()
      .find((e) => e.type === "item" && e.seat === tank.seat && e.at <= now);
    const { action, elapsed } = tankPose({
      alive: tank.alive,
      seat: tank.seat,
      now,
      shot,
      fall,
      motionAt: motion?.at,
      damageAt: damage?.at,
      itemAt: item?.at,
      deathAt: death?.at,
      actions,
      reduced,
    });
    if (action === "fall" && fall) Object.assign(tank, fallAt(fall, now));
    else if (action === "move" && motion) {
      const t = Math.max(0, (now - motion.at) / 180);
      tank.x = motion.fromX + (motion.x - motion.fromX) * t;
      tank.y = motion.fromY + (motion.y - motion.fromY) * t;
    }
    if (canAim && tank.seat === view.you) tank.facing = facing;
    const support = supportSlope(tank, terrainSolid);
    const drawn = sprite(ctx, tank, action, elapsed, images, support.slope);
    renderedTanks.push({
      seat: tank.seat,
      x: tank.x,
      y: tank.y,
      hp: tank.hp,
      alive: tank.alive,
      action,
      screenX: (tank.x - c.x) * scale,
      screenY: (tank.y - c.y) * scale,
      screenWidth: (drawn?.w ?? 40) * scale,
      screenHeight: (drawn?.h ?? 22) * scale,
      artReady: drawn !== null,
      slope: support.slope,
      sourceIndex: drawn?.sourceIndex,
      supportLeft: support.left,
      supportRight: support.right,
      screenLeft: (tank.x - (drawn?.ax ?? 20) - c.x) * scale,
      screenTop: (tank.y - (drawn?.ay ?? 22) - c.y) * scale,
    });
    if (!drawn) {
      ctx.fillStyle = teamColors[tank.team] ?? "#d7e3ed";
      ctx.strokeStyle = "#071c30";
      ctx.lineWidth = 2;
      ctx.fillRect(tank.x - 18, tank.y - 20, 36, 16);
      ctx.strokeRect(tank.x - 18, tank.y - 20, 36, 16);
      ctx.fillStyle = "#102638";
      ctx.fillRect(tank.x - 20, tank.y - 8, 40, 8);
    }
    if (!tank.alive) continue;
    if (tank.current) {
      ctx.fillStyle = "#ffcf52";
      ctx.strokeStyle = "#4b2611";
      ctx.lineWidth = 1.5;
      ctx.beginPath();
      ctx.moveTo(tank.x - 7, tank.y - 63);
      ctx.lineTo(tank.x + 7, tank.y - 63);
      ctx.lineTo(tank.x, tank.y - 54);
      ctx.closePath();
      ctx.fill();
      ctx.stroke();
    }
    // Labels stay legible at zoom levels that still show the native tank art.
    ctx.save();
    ctx.translate(tank.x, tank.y + 5);
    ctx.scale(1 / scale, 1 / scale);
    ctx.fillStyle = "#081b2a";
    ctx.fillRect(-23, 0, 46, 6);
    ctx.fillStyle = tank.team === 0 ? "#ff8c78" : "#8ac5ff";
    ctx.fillRect(
      -22,
      1,
      44 * Math.max(0, tank.hp / Math.max(1, tank.maxHp)),
      4,
    );
    ctx.font = "bold 10px system-ui";
    ctx.textAlign = "center";
    ctx.lineWidth = 3;
    ctx.strokeStyle = "#081b2a";
    ctx.strokeText(names[tank.seat] ?? "玩家", 0, 18);
    ctx.fillStyle = "#fff7e3";
    ctx.fillText(names[tank.seat] ?? "玩家", 0, 18);
    ctx.restore();
    if (canAim && tank.seat === view.you) {
      const rad = (angle * Math.PI) / 180;
      ctx.strokeStyle = "#ffed9e";
      ctx.lineWidth = 2 / scale;
      ctx.beginPath();
      ctx.moveTo(tank.x + facing * 8, tank.y - 26);
      ctx.lineTo(
        tank.x + facing * (8 + Math.cos(rad) * 42),
        tank.y - 26 - Math.sin(rad) * 42,
      );
      ctx.stroke();
    }
  }
  if (shot) {
    const bullet = tankInfo(
      view.tanks.find((t) => t.seat === shot.seat)?.tankId ?? 0,
    )?.bullet;
    for (const arc of shot.arcs) {
      const p = arcAt(
        { ...arc, impactAt: arc.impactAt ?? shot.impactAt },
        shot.at,
        now,
        shot.launchAt ?? shot.at,
      );
      if (p) {
        projectileCount++;
        renderedProjectiles.push({ x: p.x, y: p.y });
        ctx.save();
        ctx.translate(p.x, p.y);
        ctx.rotate(p.angle);
        const img = bullet ? images[bullet.url] : null;
        if (img && bullet)
          ctx.drawImage(img, -bullet.ax, -bullet.ay, bullet.w, bullet.h);
        else {
          ctx.fillStyle = "#ffe28d";
          ctx.strokeStyle = "#80462c";
          ctx.lineWidth = 2;
          ctx.beginPath();
          ctx.ellipse(0, 0, 7, 5, 0, 0, Math.PI * 2);
          ctx.fill();
          ctx.stroke();
        }
        if (shot.weapon === "healing") {
          ctx.strokeStyle = "#b1ffce";
          ctx.lineWidth = 3;
          ctx.beginPath();
          ctx.arc(0, 0, 17, 0, Math.PI * 2);
          ctx.stroke();
          ctx.fillStyle = "#e0ffe9";
          ctx.fillRect(-6, -2, 12, 4);
          ctx.fillRect(-2, -6, 4, 12);
        }
        ctx.restore();
      }
    }
    for (const blast of shot.blasts ?? []) {
      const event = [...view.events]
        .reverse()
        .find(
          (e) =>
            e.type === "explode" &&
            Math.abs((e.x ?? 0) - blast.x) < 2 &&
            Math.abs((e.y ?? 0) - blast.y) < 2,
        );
      const at = event?.at ?? shot.impactAt ?? 0,
        elapsed = now - at;
      if (elapsed < 0 || elapsed > 700) continue;
      const img = images[explosion.url];
      if (blast.kind === "heal") {
        ctx.strokeStyle = "#a9ffd1";
        ctx.fillStyle = "#9cffc12e";
        ctx.lineWidth = 3;
        ctx.beginPath();
        ctx.arc(
          blast.x,
          blast.y,
          Math.max(8, blast.r * (reduced ? 1 : 0.5 + elapsed / 1400)),
          0,
          Math.PI * 2,
        );
        ctx.fill();
        ctx.stroke();
        ctx.fillStyle = "#d1ffe3";
        ctx.fillRect(blast.x - 10, blast.y - 3, 20, 6);
        ctx.fillRect(blast.x - 3, blast.y - 10, 6, 20);
      } else if (img && !reduced) {
        const frame = Math.min(
          explosion.count - 1,
          Math.floor((elapsed / 700) * explosion.count),
        );
        const size = Math.max(70, blast.r * 2);
        ctx.drawImage(
          img,
          frame * explosion.cellW,
          0,
          explosion.cellW,
          explosion.cellH,
          blast.x - size / 2,
          blast.y - size / 2,
          size,
          size,
        );
      } else {
        ctx.strokeStyle = "#ffeab1";
        ctx.lineWidth = 4;
        ctx.beginPath();
        ctx.arc(
          blast.x,
          blast.y,
          Math.max(5, blast.r * (0.3 + elapsed / 700)),
          0,
          Math.PI * 2,
        );
        ctx.stroke();
      }
    }
  }
  for (const event of view.events ?? []) {
    const elapsed = now - event.at;
    if (
      (event.type !== "hit" &&
        event.type !== "heal" &&
        event.type !== "status") ||
      !event.amount ||
      elapsed < 0 ||
      elapsed > 1000
    )
      continue;
    const feedbackX = event.x ?? 0;
    const feedbackY = (event.y ?? 0) - 30 - (reduced ? 0 : elapsed * 0.025);
    renderedFeedback.push({
      type: event.type,
      target: event.target,
      amount: event.amount,
      at: event.at,
      x: feedbackX,
      y: feedbackY,
      screenX: (feedbackX - c.x) * scale,
      screenY: (feedbackY - c.y) * scale,
    });
    ctx.save();
    ctx.translate(feedbackX, feedbackY);
    ctx.scale(1 / scale, 1 / scale);
    ctx.font = "bold 18px system-ui";
    ctx.textAlign = "center";
    ctx.lineWidth = 3;
    const healing = event.type === "heal";
    const text = `${healing ? "+" : "−"}${event.amount}`;
    ctx.strokeStyle = healing ? "#123b29" : "#301b13";
    ctx.strokeText(text, 0, 0);
    ctx.fillStyle = healing ? "#c5ffdd" : "#fff1ba";
    ctx.fillText(text, 0, 0);
    ctx.restore();
  }
  ctx.restore();
  const mm = miniRect(w, map);
  ctx.fillStyle = "#0b2035";
  ctx.fillRect(mm.x - 3, mm.y - 3, mm.w + 6, mm.h + 6);
  ctx.fillStyle = "#7f9eaa";
  ctx.fillRect(mm.x, mm.y, mm.w, mm.h);
  if (terrain) ctx.drawImage(terrain, mm.x, mm.y, mm.w, mm.h);
  for (const tank of view.tanks) {
    if (!tank.selected || !tank.alive) continue;
    ctx.fillStyle = tank.team === 0 ? "#ff806e" : "#6bbdff";
    ctx.beginPath();
    ctx.arc(
      mm.x + (tank.x / map.width) * mm.w,
      mm.y + (tank.y / map.height) * mm.h,
      3,
      0,
      Math.PI * 2,
    );
    ctx.fill();
  }
  ctx.strokeStyle = "#fff1a8";
  ctx.lineWidth = 1.5;
  ctx.strokeRect(
    mm.x + (c.x / map.width) * mm.w,
    mm.y + (Math.max(0, c.y) / map.height) * mm.h,
    (c.worldViewW / map.width) * mm.w,
    (Math.max(0, Math.min(map.height, c.y + c.worldViewH) - Math.max(0, c.y)) /
      map.height) *
      mm.h,
  );
  canvas.dataset.renderedTanks = JSON.stringify(renderedTanks);
  canvas.dataset.renderedProjectiles = JSON.stringify(renderedProjectiles);
  canvas.dataset.renderedFeedback = JSON.stringify(renderedFeedback);
  canvas.dataset.terrainRevision = String(view.craters?.length ?? 0);
  canvas.dataset.renderedAt = now.toFixed(2);
  canvas.dataset.minimap = JSON.stringify(mm);
  canvas.dataset.projectiles = String(projectileCount);
  canvas.dataset.cameraX = c.x.toFixed(2);
  canvas.dataset.cameraY = c.y.toFixed(2);
  canvas.dataset.worldViewWidth = c.worldViewW.toFixed(2);
}
