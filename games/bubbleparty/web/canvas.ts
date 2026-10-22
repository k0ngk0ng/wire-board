import type { Player, View } from "./types";
import {
  avatarDeathDuration,
  drawAvatarArt,
  drawAvatarFallback,
} from "./avatar-art.ts";
import {
  frameRect,
  getSprite,
  spriteFrames,
  type SpriteName,
} from "./sprites.ts";

import {
  characterDeathDuration,
  drawCharacterArt,
  drawCharacterFallback,
} from "./character-art.ts";
import { drawMountedAvatar, drawMountItem, mountName } from "./mount-art.ts";
import {
  classicTerrain,
  classicObjectEntries,
  classicObjectOpacity,
  drawClassicGround,
  drawClassicObject,
  type ClassicImages,
} from "./classic-terrain.ts";

export const CELL = 24;
export const COLS = 15;
export const ROWS = 13;
export const WIDTH = COLS * CELL;
export const HEIGHT = ROWS * CELL;
// Room for tall characters above the first row; physics coordinates stay on
// the original 15x13 grid and sprites remain anchored to their feet.
export const DRAW_WIDTH = WIDTH + 24;
export const DRAW_HEIGHT = HEIGHT + 52;
export type DrawPoint = { x: number; y: number; move?: string; dir?: string };
const TAU = Math.PI * 2;
export type DrawDiagnostics = {
  terrain: "classic" | "legacy";
  nativeObjectsDrawn: number;
  nativeObjectsFallback: number;
  playersNative: number;
  playersFallback: number;
  mountedNative: number;
  mountedFallback: number;
  mountItemsNative: number;
  movingBoxes: {
    x: number;
    y: number;
    toX: number;
    toY: number;
    objectId: number;
  }[];
};

export function visibleBoxMotions(
  view: Pick<View, "boxMotions"> & Partial<Pick<View, "cells">>,
  now: number,
) {
  return (view.boxMotions ?? [])
    .filter(
      (m) =>
        m.endAt > now &&
        m.endAt > m.at &&
        (!view.cells || view.cells[m.y * COLS + m.x] === 4),
    )
    .map((m) => {
      const t = Math.max(0, Math.min(1, (now - m.at) / (m.endAt - m.at)));
      return {
        x: m.fromX + (m.x - m.fromX) * t,
        y: m.fromY + (m.y - m.fromY) * t,
        toX: m.x,
        toY: m.y,
        objectId: m.objectId,
      };
    });
}

function mountFallback(
  ctx: CanvasRenderingContext2D,
  kind: string,
  x: number,
  y: number,
  scale = 1,
) {
  ctx.save();
  ctx.translate(x, y);
  ctx.scale(scale, scale);
  const air = kind.startsWith("airship"),
    owl = kind === "owl",
    rail = kind === "railcar";
  ctx.fillStyle = air
    ? "#a8c4d3"
    : owl
      ? "#a77947"
      : rail
        ? "#c69a58"
        : kind.startsWith("octopus")
          ? "#b986b1"
          : "#4c985e";
  ctx.strokeStyle = "#294b47";
  ctx.lineWidth = 1.5;
  ctx.beginPath();
  ctx.ellipse(0, -5, air ? 18 : 15, air ? 6 : 10, 0, 0, TAU);
  ctx.fill();
  ctx.stroke();
  if (rail) {
    circle(ctx, -9, 2, 4, "#40505b");
    circle(ctx, 9, 2, 4, "#40505b");
  } else if (owl) {
    circle(ctx, -5, -10, 4, "#fff2c9");
    circle(ctx, 5, -10, 4, "#fff2c9");
    circle(ctx, -5, -10, 1.5, "#293940");
    circle(ctx, 5, -10, 1.5, "#293940");
  } else if (!air) {
    circle(ctx, -12, 2, 3, "#b8c781");
    circle(ctx, 12, 2, 3, "#b8c781");
  }
  ctx.restore();
}

const tileSprites: Record<number, SpriteName> = {
  2: "hedge",
  3: "tree",
  4: "houseRed",
  5: "houseBlue",
  6: "houseYellow",
  7: "brickRed",
  8: "brickOrange",
  9: "crate",
  10: "houseRed",
};
const teamColors = ["#b52e35", "#175fac"];

function circle(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  r: number,
  fill: string,
  stroke?: string,
) {
  ctx.beginPath();
  ctx.arc(x, y, r, 0, TAU);
  ctx.fillStyle = fill;
  ctx.fill();
  if (stroke) {
    ctx.strokeStyle = stroke;
    ctx.lineWidth = 1.5;
    ctx.stroke();
  }
}
function sprite(
  ctx: CanvasRenderingContext2D,
  name: SpriteName,
  x: number,
  y: number,
  width: number,
  height: number,
  col = 0,
  row = 0,
) {
  const image = getSprite(name);
  if (!image) return false;
  const f = frameRect(name, col, row);
  ctx.drawImage(image, f.sx, f.sy, f.sw, f.sh, x, y, width, height);
  return true;
}
function tileCode(view: View, i: number) {
  return (
    view.tiles?.[i] ??
    (view.cells[i] === 1 || view.cells[i] === 3
      ? 10
      : view.cells[i] === 2 || view.cells[i] === 4
        ? 9
        : 0)
  );
}
function ground(ctx: CanvasRenderingContext2D, view: View) {
  ctx.fillStyle = "#82b747";
  ctx.fillRect(0, 0, WIDTH, HEIGHT);
  for (let y = 0; y < ROWS; y++)
    for (let x = 0; x < COLS; x++) {
      const road =
        tileCode(view, y * COLS + x) === 1 ||
        (view.mapId === "community10" && x >= 6 && x <= 8);
      const px = x * CELL,
        py = y * CELL;
      const col = road ? 2 : (x + y) % 2;
      if (!sprite(ctx, "ground", px, py, CELL, CELL, col)) {
        ctx.fillStyle = road ? "#969a98" : (x + y) % 2 ? "#8dbf51" : "#a1ca63";
        ctx.fillRect(px, py, CELL, CELL);
        ctx.strokeStyle = road ? "#777e7b" : "#739845";
        ctx.lineWidth = 0.5;
        ctx.strokeRect(px, py, CELL, CELL);
      }
      if (road && x === 7 && y % 3 === 0) {
        ctx.fillStyle = "#f2f0db";
        ctx.fillRect(px + 10, py + 6, 3, 10);
      }
    }
}
function obstacle(
  ctx: CanvasRenderingContext2D,
  code: number,
  x: number,
  y: number,
) {
  const name = tileSprites[code];
  if (!name) return;
  const px = x * CELL,
    foot = (y + 1) * CELL;
  // Fruit herbs keep their original silhouette; depth, not flattening the
  // image, lets a character move behind their foliage.
  const height = spriteFrames[name].height * 0.6;
  if (sprite(ctx, name, px, foot - height, CELL, height)) return;
  if (code === 2 || code === 3) {
    ctx.fillStyle = "#62512d";
    ctx.fillRect(px + 10, foot - 12, 4, 10);
    circle(ctx, px + 12, foot - 13, code === 2 ? 9 : 11, "#3b833d", "#20572b");
  } else if ((code >= 4 && code <= 6) || code === 10) {
    ctx.fillStyle = "#f1dfad";
    ctx.fillRect(px + 2, foot - 19, 20, 19);
    ctx.fillStyle = code === 5 ? "#427eb8" : code === 6 ? "#dfae32" : "#bb5140";
    ctx.beginPath();
    ctx.moveTo(px, foot - 17);
    ctx.lineTo(px + 12, foot - 27);
    ctx.lineTo(px + 24, foot - 17);
    ctx.fill();
    ctx.fillStyle = "#36483c";
    ctx.fillRect(px + 9, foot - 10, 6, 10);
  } else {
    ctx.fillStyle = code === 7 ? "#d45e3c" : "#d5a14d";
    ctx.fillRect(px + 1, foot - 23, 22, 22);
    ctx.strokeStyle = "#735429";
    ctx.lineWidth = 2;
    ctx.strokeRect(px + 2, foot - 22, 20, 20);
    if (code === 9) {
      ctx.beginPath();
      ctx.moveTo(px + 4, foot - 20);
      ctx.lineTo(px + 20, foot - 4);
      ctx.moveTo(px + 20, foot - 20);
      ctx.lineTo(px + 4, foot - 4);
      ctx.stroke();
    }
  }
}
function water(
  ctx: CanvasRenderingContext2D,
  view: View,
  now: number,
  reduced: boolean,
) {
  const live = view.blasts.filter((b) => b.at <= now && b.endAt > now);
  const cells = new Set(live.map((b) => `${b.x},${b.y}`));
  for (const b of live) {
    const px = b.x * CELL,
      py = b.y * CELL;
    const up = cells.has(`${b.x},${b.y - 1}`),
      down = cells.has(`${b.x},${b.y + 1}`);
    const left = cells.has(`${b.x - 1},${b.y}`),
      right = cells.has(`${b.x + 1},${b.y}`);
    const center = (up || down) && (left || right);
    const age = Math.max(0, now - b.at);
    const frame = reduced ? 8 : 6 + Math.min(7, Math.floor(age / 55));
    const row = center
      ? 4
      : up && down
        ? 0
        : left && right
          ? 2
          : down
            ? 0
            : up
              ? 1
              : right
                ? 2
                : 3;
    const col = center ? (reduced ? 1 : Math.floor(age / 70) % 4) : frame;
    // Every confirmed blast occupies the full cell; adjacent arms meet without gaps.
    ctx.fillStyle = "#a4eaf8";
    ctx.fillRect(px + 5, py + 5, 14, 14);
    if (up) ctx.fillRect(px + 5, py, 14, 12);
    if (down) ctx.fillRect(px + 5, py + 12, 14, 12);
    if (left) ctx.fillRect(px, py + 5, 12, 14);
    if (right) ctx.fillRect(px + 12, py + 5, 12, 14);
    if (!sprite(ctx, "blast", px, py, CELL, CELL, col, row)) {
      ctx.strokeStyle = "#2373ab";
      ctx.lineWidth = 1.5;
      ctx.strokeRect(px + 5, py + 5, 14, 14);
      ctx.fillStyle = "#f1ffff";
      ctx.fillRect(px + 9, py + 3, 6, 18);
      ctx.fillRect(px + 3, py + 9, 18, 6);
    }
  }
}
function drawPlayer(
  ctx: CanvasRenderingContext2D,
  p: Player,
  view: View,
  pos: DrawPoint,
  now: number,
  reduced: boolean,
  assets: string,
  diagnostics: DrawDiagnostics,
) {
  const px = pos.x * CELL + CELL / 2,
    py = pos.y * CELL + CELL / 2;
  const color = teamColors[p.team] ?? teamColors[0];
  const death = [...view.events]
    .reverse()
    .find((e) => e.type === "eliminate" && e.seat === p.seat && e.at <= now);
  const deathAge = death ? now - death.at : Infinity;
  if (
    !p.alive &&
    (reduced ||
      deathAge >
        ((view.rulesVersion ?? 0) >= 3
          ? characterDeathDuration(p.character)
          : avatarDeathDuration(p.avatar)))
  ) {
    circle(ctx, px, py, 8, p.team === 1 ? "#d8e8f5" : "#efd7d5", color);
    ctx.strokeStyle = color;
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(px - 3, py - 3);
    ctx.lineTo(px + 3, py + 3);
    ctx.moveTo(px + 3, py - 3);
    ctx.lineTo(px - 3, py + 3);
    ctx.stroke();
    return;
  }
  ctx.fillStyle = "#46643a66";
  ctx.beginPath();
  ctx.ellipse(px, py + 9, 13, 4, 0, 0, TAU);
  ctx.fill();
  ctx.strokeStyle = p.seat === view.you ? "#fff9cb" : color;
  ctx.lineWidth = p.seat === view.you ? 2.5 : 2;
  ctx.beginPath();
  ctx.ellipse(px, py + 8, 14, 5, 0, 0, TAU);
  ctx.stroke();
  const trapped = p.alive && p.trappedUntil > 0;
  const move = pos.move ?? p.applied.move;
  const moving = move !== "none" && move !== "" && !trapped;
  const original = (view.rulesVersion ?? 0) >= 3;
  const pose = {
    character: p.character,
    avatar: p.avatar,
    team: p.team,
    dir: pos.dir ?? (moving ? move : p.dir),
    moving,
    trapped,
    dead: !p.alive,
    deathAge,
    now,
    reduced,
  };
  const mounted = original && !!p.mount && p.alive && !trapped;
  let drawn = false;
  if (mounted) {
    drawn = drawMountedAvatar(
      ctx,
      assets,
      { ...pose, mount: p.mount! },
      px,
      py + 12,
    );
    if (drawn) diagnostics.mountedNative++;
    else {
      diagnostics.mountedFallback++;
      mountFallback(ctx, p.mount!, px, py + 12);
    }
  }
  if (!drawn) {
    const foot = py + 12 - (mounted ? 12 : 0);
    drawn = original
      ? drawCharacterArt(ctx, assets, pose, px, foot)
      : drawAvatarArt(ctx, assets, pose, px, foot);
    if (!drawn) {
      if (original) drawCharacterFallback(ctx, p.character, p.team, px, foot);
      else drawAvatarFallback(ctx, p.avatar, p.team, px, foot);
    }
  }
  if (drawn) diagnostics.playersNative++;
  else diagnostics.playersFallback++;

  if (trapped) {
    const bw = 42,
      bx = px - bw / 2,
      by = py - 27;
    if (
      !sprite(
        ctx,
        "bigBubble",
        bx,
        by,
        bw,
        bw,
        reduced ? 3 : 2 + (Math.floor(now / 180) % 2),
      )
    ) {
      circle(ctx, px, py - 5, 20, "#9cdcf366", "#1167a5");
    }
    const text = `${Math.max(1, Math.ceil((p.trappedUntil - now) / 1000))}`;
    ctx.fillStyle = "#143a56";
    ctx.fillRect(px - 10, py - 30, 20, 13);
    ctx.fillStyle = "#ffffff";
    ctx.font = "bold 10px system-ui, sans-serif";
    ctx.textAlign = "center";
    ctx.fillText(text, px, py - 20);
  }
  if (p.shield || p.invulnerableUntil > now) {
    ctx.strokeStyle = "#fff2a3";
    ctx.lineWidth = 2.5;
    ctx.beginPath();
    ctx.ellipse(px, py - 6, 19, 22, 0, 0, TAU);
    ctx.stroke();
  }
  const rescue = view.events.find(
    (e) =>
      e.type === "rescue" &&
      e.seat === p.seat &&
      e.at <= now &&
      now - e.at < 650,
  );
  if (rescue) {
    ctx.strokeStyle = "#ffffff";
    ctx.lineWidth = 2;
    const radius = reduced ? 18 : 13 + (now - rescue.at) / 45;
    ctx.beginPath();
    ctx.arc(px, py, radius, 0, TAU);
    ctx.stroke();
  }
}

export function drawBoard(
  ctx: CanvasRenderingContext2D,
  view: View,
  display: Map<number, DrawPoint>,
  now: number,
  reduced: boolean,
  assets = "",
  classicImages: ClassicImages = {},
): DrawDiagnostics {
  const native = classicTerrain(view.mapId);
  const movingBoxes = visibleBoxMotions(view, now);
  const diagnostics: DrawDiagnostics = {
    terrain: native ? "classic" : "legacy",
    nativeObjectsDrawn: 0,
    nativeObjectsFallback: 0,
    playersNative: 0,
    playersFallback: 0,
    mountedNative: 0,
    mountedFallback: 0,
    mountItemsNative: 0,
    movingBoxes,
  };
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.clearRect(0, 0, ctx.canvas.width, ctx.canvas.height);
  ctx.scale(ctx.canvas.width / DRAW_WIDTH, ctx.canvas.height / DRAW_HEIGHT);
  ctx.fillStyle = "#9db78f";
  ctx.fillRect(0, 0, DRAW_WIDTH, DRAW_HEIGHT);
  ctx.translate(12, 44);
  ctx.imageSmoothingEnabled = true;
  ctx.globalAlpha = 1;
  if (
    !drawClassicGround(ctx, view, view.mapId, classicImages, now, {
      cellSize: CELL,
      reduced,
    })
  )
    ground(ctx, view);
  for (const item of view.items) {
    const x = item.x * CELL,
      y = item.y * CELL;
    if (mountName(item.type)) {
      if (drawMountItem(ctx, assets, item.type, x + CELL / 2, y + CELL))
        diagnostics.mountItemsNative++;
      else mountFallback(ctx, item.type, x + CELL / 2, y + CELL - 3, 0.7);
      continue;
    }
    // Saved matches may still contain the old passive shield. It has no
    // verified original sprite: keep a shield-shaped fallback, not a text coin.
    if (item.type === "shield") {
      ctx.beginPath();
      ctx.moveTo(x + 12, y + 2);
      ctx.lineTo(x + 21, y + 5);
      ctx.lineTo(x + 20, y + 13);
      ctx.quadraticCurveTo(x + 19, y + 18, x + 12, y + 22);
      ctx.quadraticCurveTo(x + 5, y + 18, x + 4, y + 13);
      ctx.lineTo(x + 3, y + 5);
      ctx.closePath();
      ctx.fillStyle = "#b9dcec";
      ctx.fill();
      ctx.strokeStyle = "#28566d";
      ctx.lineWidth = 2;
      ctx.stroke();
      ctx.beginPath();
      ctx.moveTo(x + 12, y + 5);
      ctx.lineTo(x + 12, y + 18);
      ctx.strokeStyle = "#f4fbff";
      ctx.stroke();
      continue;
    }
    const name: SpriteName | undefined = (
      { bomb: "itemBomb", speed: "itemSpeed", range: "itemRange" } as Record<
        string,
        SpriteName
      >
    )[item.type];
    if (
      !name ||
      !sprite(
        ctx,
        name,
        x - 1,
        y - 3,
        26,
        28,
        reduced ? 1 : Math.floor(now / 180) % 3,
      )
    ) {
      circle(ctx, x + 12, y + 12, 9, "#fff1c0", "#80572c");
      ctx.fillStyle = "#3e4e33";
      ctx.textAlign = "center";
      ctx.font = "bold 10px system-ui, sans-serif";
      ctx.fillText(
        (
          {
            needle: "针",
            speed: "速",
            bomb: "泡",
            range: "水",
          } as Record<string, string>
        )[item.type] ?? "宝",
        x + 12,
        y + 16,
      );
    }
  }
  water(ctx, view, now, reduced);
  const objects: { y: number; draw: () => void }[] = [];
  const movingTargets = new Set(movingBoxes.map((m) => `${m.toX},${m.toY}`));
  if (native) {
    const positions = view.players
      .filter((p) => p.alive)
      .map((p) => display.get(p.seat) ?? { x: p.px ?? p.x, y: p.py ?? p.y });
    for (const object of classicObjectEntries(view, view.mapId)) {
      if (movingTargets.has(`${object.x},${object.y}`)) continue;
      objects.push({
        y: object.depth,
        draw: () => {
          const ok = drawClassicObject(
            ctx,
            object.id,
            object.x,
            object.y,
            classicImages,
            now,
            {
              cellSize: CELL,
              reduced,
              opacity: classicObjectOpacity(
                object.id,
                object.x,
                object.y,
                positions,
              ),
            },
          );
          if (ok) diagnostics.nativeObjectsDrawn++;
          else diagnostics.nativeObjectsFallback++;
        },
      });
    }
  } else {
    for (let y = 0; y < ROWS; y++)
      for (let x = 0; x < COLS; x++) {
        const i = y * COLS + x,
          code = tileCode(view, i);
        if (code >= 2 && !movingTargets.has(`${x},${y}`))
          objects.push({
            y: y + (code === 2 ? 0.85 : 0.4),
            draw: () => obstacle(ctx, code, x, y),
          });
      }
  }
  for (const box of movingBoxes)
    objects.push({
      y: box.y + 0.4,
      draw: () => {
        if (native && box.objectId >= 0) {
          const ok = drawClassicObject(
            ctx,
            box.objectId,
            box.x,
            box.y,
            classicImages,
            now,
            { cellSize: CELL, reduced },
          );
          if (ok) diagnostics.nativeObjectsDrawn++;
          else diagnostics.nativeObjectsFallback++;
        } else obstacle(ctx, 9, box.x, box.y);
      },
    });
  for (const b of view.bombs)
    if (b.explodeAt > now)
      objects.push({
        y: b.y + 0.5,
        draw: () => {
          const x = b.x * CELL,
            y = b.y * CELL;
          if (
            !sprite(
              ctx,
              "bubble",
              x - 1,
              y - 2,
              26.4,
              24.6,
              reduced ? 1 : Math.floor((now - b.at) / 180) % 3,
            )
          ) {
            circle(ctx, x + 12, y + 12, 10, "#73c6eb", "#1c5c8b");
            circle(ctx, x + 8, y + 8, 3, "#e5faff");
          }
        },
      });
  for (const p of view.players) {
    const pos = display.get(p.seat) ?? { x: p.px ?? p.x, y: p.py ?? p.y };
    objects.push({
      y: pos.y + 0.6,
      draw: () =>
        drawPlayer(ctx, p, view, pos, now, reduced, assets, diagnostics),
    });
  }
  objects.sort((a, b) => a.y - b.y).forEach((o) => o.draw());
  if (view.sudden && view.safeInset > 0) {
    const inset = view.safeInset * CELL;
    ctx.fillStyle = "#19364d66";
    ctx.fillRect(0, 0, WIDTH, inset);
    ctx.fillRect(0, HEIGHT - inset, WIDTH, inset);
    ctx.fillRect(0, inset, inset, HEIGHT - inset * 2);
    ctx.fillRect(WIDTH - inset, inset, inset, HEIGHT - inset * 2);
    ctx.strokeStyle = "#fff1ad";
    ctx.lineWidth = 2;
    ctx.strokeRect(inset, inset, WIDTH - 2 * inset, HEIGHT - 2 * inset);
  }
  if (view.phase === "countdown" || view.phase === "select") {
    ctx.fillStyle = "#17395533";
    ctx.fillRect(0, 0, WIDTH, HEIGHT);
  }
  return diagnostics;
}

export function drawPreview(
  ctx: CanvasRenderingContext2D,
  cells: number[],
  mapId: string,
  tiles?: number[],
) {
  const view = { cells, mapId, tiles } as View;
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.clearRect(0, 0, ctx.canvas.width, ctx.canvas.height);
  ctx.scale(ctx.canvas.width / WIDTH, ctx.canvas.height / HEIGHT);
  ground(ctx, view);
  for (let y = 0; y < ROWS; y++)
    for (let x = 0; x < COLS; x++)
      obstacle(ctx, tileCode(view, y * COLS + x), x, y);
}
