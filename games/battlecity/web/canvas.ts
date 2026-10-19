import { drawFrame, type Atlas } from "./assets";
import {
  POWERUP_NAMES,
  SUB_COLS,
  SUB_ROWS,
  cellAt,
  decodeBroken,
  mapById,
  type TankView,
  type View,
} from "./types";

const SIZE = 208;
const CELL = 16;
const SUB = 8;
const TEAM_COLORS = ["#e0784f", "#5aa9d6"];
const TEAM_DARK = ["#7f3f2a", "#2c617f"];
const RING = new Set([
  "5,0",
  "7,0",
  "5,1",
  "6,1",
  "7,1",
  "5,12",
  "7,12",
  "5,11",
  "6,11",
  "7,11",
]);

const lerp = (a: number, b: number, t: number) => a + (b - a) * t;

function interpolatedTank(
  cur: TankView,
  prev: TankView | undefined,
  alpha: number,
  viewer: number,
): TankView {
  if (!prev || cur.seat === viewer) return cur;
  if (Math.abs(cur.x - prev.x) > 40 || Math.abs(cur.y - prev.y) > 40) return cur;
  return {
    ...cur,
    x: lerp(prev.x, cur.x, alpha),
    y: lerp(prev.y, cur.y, alpha),
  };
}

function drawFallbackTerrain(
  ctx: CanvasRenderingContext2D,
  code: number,
  x: number,
  y: number,
  w: number,
  h: number,
) {
  switch (code) {
    case 1:
      ctx.fillStyle = "#8e442d";
      ctx.fillRect(x, y, w, h);
      ctx.fillStyle = "rgba(238,170,120,0.28)";
      ctx.fillRect(x + 1, y + 1, w - 2, 2);
      ctx.fillRect(x + 1, y + 4, w - 2, 2);
      break;
    case 2:
      ctx.fillStyle = "#87919d";
      ctx.fillRect(x, y, w, h);
      ctx.fillStyle = "rgba(255,255,255,0.22)";
      ctx.fillRect(x + 1, y + 1, w - 2, 2);
      break;
    case 3:
      ctx.fillStyle = "#235f86";
      ctx.fillRect(x, y, w, h);
      ctx.strokeStyle = "rgba(130,205,235,0.55)";
      ctx.beginPath();
      ctx.moveTo(x + 1, y + h / 2);
      ctx.lineTo(x + w - 1, y + h / 2);
      ctx.stroke();
      break;
    case 5:
      ctx.fillStyle = "#a9c9d8";
      ctx.fillRect(x, y, w, h);
      ctx.strokeStyle = "rgba(255,255,255,0.45)";
      ctx.strokeRect(x + 0.5, y + 0.5, w - 1, h - 1);
      break;
    default:
      ctx.fillStyle = "#11161c";
      ctx.fillRect(x, y, w, h);
  }
}

function drawFallbackTank(
  ctx: CanvasRenderingContext2D,
  tank: TankView,
  team: number,
) {
  const x = tank.x;
  const y = tank.y;
  const color = TEAM_COLORS[team] ?? "#ddd";
  const dark = TEAM_DARK[team] ?? "#555";
  ctx.save();
  ctx.translate(x + 8, y + 8);
  ctx.rotate((tank.dir * Math.PI) / 2);
  ctx.translate(-8, -8);
  ctx.fillStyle = dark;
  ctx.fillRect(1, 1, 3, 14);
  ctx.fillRect(12, 1, 3, 14);
  for (let i = 1; i < 14; i += 4) {
    ctx.fillStyle = "#d9d4c3";
    ctx.fillRect(1, i, 3, 1);
    ctx.fillRect(12, i, 3, 1);
  }
  ctx.fillStyle = color;
  ctx.fillRect(4, 3, 8, 10);
  ctx.fillStyle = dark;
  ctx.fillRect(5, 4, 6, 3);
  ctx.fillStyle = color;
  ctx.fillRect(7, 0, 2, 6);
  ctx.fillStyle = "#f5e5c8";
  ctx.fillRect(6, 6, 4, 4);
  ctx.fillStyle = "#1a1d20";
  ctx.fillRect(7, 7, 2, 2);
  if (tank.maxHp > 1 && tank.hp < tank.maxHp) {
    ctx.fillStyle = "rgba(0,0,0,0.35)";
    ctx.fillRect(4, 3, 8, 10 - (10 * tank.hp) / tank.maxHp);
  }
  ctx.restore();
  ctx.fillStyle = "rgba(255,255,255,0.85)";
  ctx.font = "6px monospace";
  ctx.fillText(String((tank.seat % 3) + 1), x + 1, y + 6);
}

function drawBase(
  ctx: CanvasRenderingContext2D,
  atlas: Atlas | null,
  team: number,
  alive: boolean,
) {
  const x = 6 * CELL;
  const y = (team === 0 ? 0 : 12) * CELL;
  const frame = !alive
    ? "base-destroyed"
    : team === 0
      ? "base-top"
      : "base-bottom";
  if (drawFrame(ctx, atlas, frame, x, y, CELL, CELL)) return;
  if (!alive) {
    ctx.fillStyle = "#2a1b18";
    ctx.fillRect(x + 2, y + 2, 12, 12);
    ctx.strokeStyle = "#a05238";
    ctx.beginPath();
    ctx.moveTo(x + 3, y + 3);
    ctx.lineTo(x + 13, y + 13);
    ctx.moveTo(x + 13, y + 3);
    ctx.lineTo(x + 3, y + 13);
    ctx.stroke();
    return;
  }
  ctx.fillStyle = team === 0 ? "#e8a16d" : "#86c4e2";
  ctx.fillRect(x + 2, y + 2, 12, 12);
  ctx.fillStyle = "#171b20";
  ctx.beginPath();
  ctx.moveTo(x + 4, y + 12);
  ctx.lineTo(x + 8, y + 3);
  ctx.lineTo(x + 12, y + 12);
  ctx.closePath();
  ctx.fill();
  ctx.fillStyle = team === 0 ? "#fff1d8" : "#e6f6ff";
  ctx.fillRect(x + 7, y + 6, 2, 4);
}

function drawPowerUp(
  ctx: CanvasRenderingContext2D,
  atlas: Atlas | null,
  type: string,
  x: number,
  y: number,
) {
  if (drawFrame(ctx, atlas, `powerup-${type}`, x, y, CELL, CELL)) return;
  ctx.fillStyle = "rgba(245,231,185,0.95)";
  ctx.fillRect(x + 2, y + 2, 12, 12);
  ctx.strokeStyle = "#2b2420";
  ctx.strokeRect(x + 2.5, y + 2.5, 11, 11);
  ctx.fillStyle = "#2b2420";
  ctx.font = "bold 8px monospace";
  const letter: Record<string, string> = {
    star: "S",
    grenade: "G",
    helmet: "H",
    shovel: "D",
    tank: "T",
    clock: "C",
  };
  ctx.fillText(letter[type] ?? "?", x + 5, y + 11);
  ctx.font = "5px monospace";
  ctx.fillText(POWERUP_NAMES[type] ?? "", x + 1, y + 16);
}

export function drawBattlefield(
  ctx: CanvasRenderingContext2D,
  view: View,
  previous: View | null,
  alpha: number,
  atlas: Atlas | null,
  viewer: number,
  serverNow: number,
) {
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.clearRect(0, 0, SIZE, SIZE);
  ctx.imageSmoothingEnabled = false;
  const map = mapById(view.mapId);
  const broken = decodeBroken(view.broken);
  const shovel = view.shovelUntil ?? [0, 0];
  if (!map) {
    ctx.fillStyle = "#11161c";
    ctx.fillRect(0, 0, SIZE, SIZE);
    return;
  }
  for (let y = 0; y < 13; y++) {
    for (let x = 0; x < 13; x++) {
      const code = cellAt(map, x, y);
      const key = `${x},${y}`;
      const ring = RING.has(key);
      const steel = ring && serverNow < (shovel[y === 0 ? 0 : 1] ?? 0);
      for (let sy = y * 2; sy < y * 2 + 2; sy++) {
        for (let sx = x * 2; sx < x * 2 + 2; sx++) {
          const idx = sy * SUB_COLS + sx;
          if ((code === 1 || code === 2) && !steel && broken[idx]) continue;
          if (code === 4) continue; // trees are drawn above the tanks
          const frame =
            steel || code === 2
              ? "steel"
              : code === 1
                ? "brick"
                : code === 3
                  ? "water"
                  : code === 5
                    ? "ice"
                    : null;
          if (frame && drawFrame(ctx, atlas, frame, sx * SUB, sy * SUB, SUB, SUB))
            continue;
          drawFallbackTerrain(ctx, steel ? 2 : code, sx * SUB, sy * SUB, SUB, SUB);
        }
      }
    }
  }
  for (let team = 0; team < 2; team++)
    drawBase(ctx, atlas, team, view.bases[team]?.alive ?? false);

  for (const p of view.powerUps ?? [])
    drawPowerUp(ctx, atlas, p.type, p.x, p.y);

  const prevBullets = new Map(
    (previous?.bullets ?? []).map((b) => [b.id, b] as const),
  );
  for (const b of view.bullets ?? []) {
    const prev = prevBullets.get(b.id);
    let x = b.x;
    let y = b.y;
    if (prev && Math.abs(prev.x - b.x) < 24 && Math.abs(prev.y - b.y) < 24) {
      x = lerp(prev.x, b.x, alpha);
      y = lerp(prev.y, b.y, alpha);
    }
    if (!drawFrame(ctx, atlas, `bullet-${b.team === 0 ? "top" : "bottom"}`, x - 3, y - 3, 6, 6)) {
      ctx.fillStyle = TEAM_COLORS[b.team] ?? "#eee";
      ctx.beginPath();
      ctx.arc(x, y, 2.4, 0, Math.PI * 2);
      ctx.fill();
      ctx.fillStyle = "#fff7dd";
      ctx.fillRect(x - 0.8, y - 0.8, 1.6, 1.6);
    }
  }

  const prevTanks = new Map(
    (previous?.tanks ?? []).map((t) => [t.seat, t] as const),
  );
  for (const t of view.tanks) {
    if (!t.alive) continue;
    const tank = interpolatedTank(t, prevTanks.get(t.seat), alpha, viewer);
    const frame = Math.floor(serverNow / 130 + t.seat) % 2;
    const name = `tank-${t.class ?? "balanced"}-${t.team === 0 ? "top" : "bottom"}-${["up", "right", "down", "left"][t.dir]}-${frame}`;
    if (!drawFrame(ctx, atlas, name, tank.x, tank.y, CELL, CELL))
      drawFallbackTank(ctx, tank, t.team);
    if (serverNow < t.shieldUntil) {
      ctx.strokeStyle = "rgba(126,224,255,0.95)";
      ctx.lineWidth = 1.5;
      ctx.strokeRect(tank.x - 1, tank.y - 1, CELL + 2, CELL + 2);
      ctx.strokeStyle = "rgba(126,224,255,0.35)";
      ctx.beginPath();
      ctx.arc(tank.x + 8, tank.y + 8, 11, 0, Math.PI * 2);
      ctx.stroke();
    }
    if (serverNow < t.freezeUntil) {
      ctx.fillStyle = "rgba(142,205,236,0.38)";
      ctx.fillRect(tank.x, tank.y, CELL, CELL);
    }
  }

  // Trees obscure tanks and bullets, as in the source game.
  for (let y = 0; y < 13; y++) {
    for (let x = 0; x < 13; x++) {
      if (cellAt(map, x, y) !== 4) continue;
      for (let sy = y * 2; sy < y * 2 + 2; sy++) {
        for (let sx = x * 2; sx < x * 2 + 2; sx++) {
          if (drawFrame(ctx, atlas, "trees", sx * SUB, sy * SUB, SUB, SUB))
            continue;
          ctx.fillStyle = "rgba(45,102,66,0.78)";
          ctx.fillRect(sx * SUB, sy * SUB, SUB, SUB);
          ctx.fillStyle = "rgba(122,170,103,0.4)";
          ctx.fillRect(sx * SUB + 1, sy * SUB + 1, 2, 2);
          ctx.fillRect(sx * SUB + 5, sy * SUB + 4, 2, 2);
        }
      }
    }
  }

  // Recent server events drive short, non-authoritative visual effects.
  for (const e of view.events) {
    const age = serverNow - e.at;
    if (age < 0 || age > 450) continue;
    if (e.type === "kill" || e.type === "base_destroyed" || e.type === "hit") {
      const frame = Math.min(2, Math.floor(age / 140));
      if (
        e.type !== "base_destroyed" &&
        drawFrame(ctx, atlas, `explosion-${frame}`, e.x - 8, e.y - 8, 16, 16)
      ) {
        continue;
      }
      const radius = 3 + (age / 450) * (e.type === "base_destroyed" ? 18 : 10);
      ctx.strokeStyle = e.type === "base_destroyed" ? "#ffb46b" : "#ffd56e";
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.arc(e.x, e.y, radius, 0, Math.PI * 2);
      ctx.stroke();
      ctx.fillStyle = "rgba(255,190,90,0.45)";
      ctx.beginPath();
      ctx.arc(e.x, e.y, radius * 0.6, 0, Math.PI * 2);
      ctx.fill();
    } else if (e.type === "brick" || e.type === "steel") {
      ctx.fillStyle =
        e.type === "brick" ? "rgba(208,132,88,0.55)" : "rgba(220,228,235,0.5)";
      ctx.fillRect(e.x - 2, e.y - 2, 4, 4);
    }
  }

  if (view.phase === "finished") {
    ctx.fillStyle = "rgba(8,10,13,0.28)";
    ctx.fillRect(0, 0, SIZE, SIZE);
  }
}

export function drawTankIcon(
  ctx: CanvasRenderingContext2D,
  atlas: Atlas | null,
  tankClass: string,
  team: number,
) {
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.clearRect(0, 0, 64, 64);
  ctx.imageSmoothingEnabled = false;
  const name = `tank-${tankClass}-${team === 0 ? "top" : "bottom"}-up-0`;
  if (drawFrame(ctx, atlas, name, 8, 8, 48, 48)) return;
  ctx.save();
  ctx.translate(8, 8);
  ctx.scale(3, 3);
  drawFallbackTank(
    ctx,
    {
      seat: 0,
      team,
      class: tankClass as TankView["class"],
      selected: true,
      x: 0,
      y: 0,
      dir: 0,
      hp: 1,
      maxHp: 1,
      tech: 0,
      alive: true,
      respawnAt: 0,
      shieldUntil: 0,
      freezeUntil: 0,
      kills: 0,
      deaths: 0,
      seq: 0,
    },
    team,
  );
  ctx.restore();
}

export const BATTLEFIELD_SIZE = SIZE;
