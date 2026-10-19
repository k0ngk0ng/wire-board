import type { View } from "./types";

export const CELL = 24;
export const COLS = 15;
export const ROWS = 13;
export const WIDTH = COLS * CELL;
export const HEIGHT = ROWS * CELL;

export type DrawPoint = { x: number; y: number };

type Theme = {
  sky: string;
  ground: string;
  groundAlt: string;
  path: string;
  wall: string;
  wallTop: string;
  crate: string;
  crateDark: string;
  accent: string;
};

const themes: Record<string, Theme> = {
  community10: {
    sky: "#c8d9c1",
    ground: "#a9c29b",
    groundAlt: "#9db78f",
    path: "#d8ceb0",
    wall: "#8d8171",
    wallTop: "#b0705e",
    crate: "#b98f5b",
    crateDark: "#8f6a3e",
    accent: "#e9dfbf",
  },
  waterpark: {
    sky: "#bcd7dd",
    ground: "#9bc4c7",
    groundAlt: "#8db8bd",
    path: "#e2d5b4",
    wall: "#6f8f98",
    wallTop: "#9b7d67",
    crate: "#c09a68",
    crateDark: "#94724a",
    accent: "#e7f2ef",
  },
  forest: {
    sky: "#c5d7bb",
    ground: "#92af87",
    groundAlt: "#85a279",
    path: "#c3b58f",
    wall: "#6c7e62",
    wallTop: "#7d6554",
    crate: "#b38a54",
    crateDark: "#876438",
    accent: "#dde7c7",
  },
  desert: {
    sky: "#e1d2b4",
    ground: "#cfb583",
    groundAlt: "#c3a875",
    path: "#ead9b6",
    wall: "#9a7b5b",
    wallTop: "#a66b4e",
    crate: "#b98d56",
    crateDark: "#8d6638",
    accent: "#f3e4c3",
  },
};

const themeFor = (id: string): Theme => themes[id] ?? themes.community10;

function roundRect(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  w: number,
  h: number,
  r: number,
) {
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.arcTo(x + w, y, x + w, y + h, r);
  ctx.arcTo(x + w, y + h, x, y + h, r);
  ctx.arcTo(x, y + h, x, y, r);
  ctx.arcTo(x, y, x + w, y, r);
  ctx.closePath();
}

export function drawBoard(
  ctx: CanvasRenderingContext2D,
  view: View,
  display: Map<number, DrawPoint>,
  now: number,
  reduced: boolean,
) {
  const theme = themeFor(view.mapId);
  ctx.clearRect(0, 0, WIDTH, HEIGHT);
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  const dpr = 2;
  ctx.scale(dpr, dpr);
  const bg = ctx.createLinearGradient(0, 0, 0, HEIGHT);
  bg.addColorStop(0, theme.sky);
  bg.addColorStop(1, theme.groundAlt);
  ctx.fillStyle = bg;
  ctx.fillRect(0, 0, WIDTH, HEIGHT);

  for (let y = 0; y < ROWS; y++) {
    for (let x = 0; x < COLS; x++) {
      const px = x * CELL;
      const py = y * CELL;
      ctx.fillStyle = (x + y) % 2 === 0 ? theme.ground : theme.groundAlt;
      ctx.globalAlpha = 0.55;
      ctx.fillRect(px, py, CELL, CELL);
      ctx.globalAlpha = 1;
      if (view.mapId === "community10" && (y === 4 || y === 8) && x > 0 && x < COLS - 1) {
        ctx.fillStyle = theme.path;
        ctx.globalAlpha = 0.32;
        ctx.fillRect(px, py + CELL - 5, CELL, 5);
        ctx.globalAlpha = 1;
      }
    }
  }

  for (let y = 0; y < ROWS; y++) {
    for (let x = 0; x < COLS; x++) {
      const value = view.cells[y * COLS + x];
      const px = x * CELL;
      const py = y * CELL;
      if (value === 1) {
        ctx.fillStyle = theme.wall;
        roundRect(ctx, px + 1, py + 3, CELL - 2, CELL - 3, 3);
        ctx.fill();
        ctx.fillStyle = theme.wallTop;
        roundRect(ctx, px, py, CELL, 8, 3);
        ctx.fill();
        ctx.fillStyle = "rgba(255,255,255,.15)";
        ctx.fillRect(px + 3, py + 11, 5, CELL - 16);
        if (view.mapId === "community10") {
          ctx.fillStyle = theme.accent;
          ctx.fillRect(px + 14, py + 13, 7, 8);
        }
      } else if (value === 2) {
        ctx.fillStyle = theme.crate;
        roundRect(ctx, px + 2, py + 2, CELL - 4, CELL - 4, 3);
        ctx.fill();
        ctx.strokeStyle = theme.crateDark;
        ctx.lineWidth = 2;
        ctx.stroke();
        ctx.beginPath();
        ctx.moveTo(px + 5, py + 5);
        ctx.lineTo(px + CELL - 5, py + CELL - 5);
        ctx.moveTo(px + CELL - 5, py + 5);
        ctx.lineTo(px + 5, py + CELL - 5);
        ctx.stroke();
      }
    }
  }

  if (view.sudden && view.safeInset > 0) {
    const inset = view.safeInset * CELL;
    ctx.fillStyle = "rgba(33,45,43,.48)";
    ctx.fillRect(0, 0, WIDTH, inset);
    ctx.fillRect(0, HEIGHT - inset, WIDTH, inset);
    ctx.fillRect(0, inset, inset, HEIGHT - inset * 2);
    ctx.fillRect(WIDTH - inset, inset, inset, HEIGHT - inset * 2);
    ctx.strokeStyle = "rgba(255,220,140,.8)";
    ctx.lineWidth = 2;
    ctx.setLineDash([6, 5]);
    ctx.strokeRect(inset, inset, WIDTH - inset * 2, HEIGHT - inset * 2);
    ctx.setLineDash([]);
  }

  for (const item of view.items) {
    const px = item.x * CELL + CELL / 2;
    const py = item.y * CELL + CELL / 2 + Math.sin(now / 280 + item.id) * 1.5;
    const colors: Record<string, string> = {
      speed: "#6fa6c7",
      bomb: "#b58ab8",
      range: "#d09a5d",
      shield: "#7fae83",
    };
    ctx.fillStyle = colors[item.type] ?? "#9aa";
    ctx.beginPath();
    ctx.arc(px, py, 7, 0, Math.PI * 2);
    ctx.fill();
    ctx.strokeStyle = "rgba(255,255,255,.7)";
    ctx.lineWidth = 1.5;
    ctx.stroke();
    ctx.fillStyle = "#fff";
    ctx.font = "bold 8px system-ui, sans-serif";
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    const letter: Record<string, string> = {
      speed: "速",
      bomb: "弹",
      range: "程",
      shield: "盾",
    };
    ctx.fillText(letter[item.type] ?? "?", px, py + 0.5);
  }

  for (const bomb of view.bombs) {
    const px = bomb.x * CELL + CELL / 2;
    const py = bomb.y * CELL + CELL / 2;
    const left = Math.max(0, bomb.explodeAt - now);
    const pulse = reduced ? 1 : 1 + Math.sin((bomb.explodeAt - now) / 80) * 0.08;
    ctx.fillStyle = "rgba(55,91,114,.28)";
    ctx.beginPath();
    ctx.ellipse(px, py + 9, 9, 3, 0, 0, Math.PI * 2);
    ctx.fill();
    ctx.fillStyle = left < 500 ? "#d58c88" : "#7fb9d1";
    ctx.strokeStyle = "#476d83";
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.arc(px, py - 1, 8 * pulse, 0, Math.PI * 2);
    ctx.fill();
    ctx.stroke();
    ctx.fillStyle = "rgba(255,255,255,.65)";
    ctx.beginPath();
    ctx.arc(px - 3, py - 4, 2.2, 0, Math.PI * 2);
    ctx.fill();
    ctx.strokeStyle = "#8a714b";
    ctx.beginPath();
    ctx.moveTo(px + 5, py - 8);
    ctx.quadraticCurveTo(px + 9, py - 13, px + 7, py - 16);
    ctx.stroke();
  }

  for (const blast of view.blasts) {
    const px = blast.x * CELL + CELL / 2;
    const py = blast.y * CELL + CELL / 2;
    const life = Math.max(0, Math.min(1, (blast.endAt - now) / 420));
    ctx.globalAlpha = 0.35 + life * 0.4;
    ctx.fillStyle = "#cfe9ee";
    ctx.beginPath();
    ctx.arc(px, py, 4 + (1 - life) * 8, 0, Math.PI * 2);
    ctx.fill();
    ctx.strokeStyle = "#79adc1";
    ctx.lineWidth = 2;
    ctx.stroke();
    ctx.globalAlpha = 1;
  }

  for (const p of view.players) {
    const pos = display.get(p.seat) ?? { x: p.x, y: p.y };
    const px = pos.x * CELL + CELL / 2;
    const py = pos.y * CELL + CELL / 2;
    if (!p.alive) {
      ctx.globalAlpha = 0.35;
      ctx.strokeStyle = "#5b4e48";
      ctx.lineWidth = 3;
      ctx.beginPath();
      ctx.moveTo(px - 6, py - 6);
      ctx.lineTo(px + 6, py + 6);
      ctx.moveTo(px + 6, py - 6);
      ctx.lineTo(px - 6, py + 6);
      ctx.stroke();
      ctx.globalAlpha = 1;
      continue;
    }
    const color = p.team === 0 ? "#b85f52" : "#5d8cae";
    const dark = p.team === 0 ? "#703b34" : "#35566e";
    ctx.fillStyle = "rgba(46,61,56,.26)";
    ctx.beginPath();
    ctx.ellipse(px, py + 10, 11, 4, 0, 0, Math.PI * 2);
    ctx.fill();
    ctx.fillStyle = color;
    ctx.strokeStyle = dark;
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.arc(px, py - 2, 14, 0, Math.PI * 2);
    ctx.fill();
    ctx.stroke();
    ctx.fillStyle = "#f7f1e2";
    ctx.beginPath();
    ctx.arc(px - 4.2, py - 5, 2, 0, Math.PI * 2);
    ctx.arc(px + 4.2, py - 5, 2, 0, Math.PI * 2);
    ctx.fill();
    ctx.strokeStyle = dark;
    ctx.lineWidth = 1.4;
    ctx.beginPath();
    ctx.arc(px, py + 1, 5, 0.15, Math.PI - 0.15);
    ctx.stroke();
    if (p.shield) {
      ctx.strokeStyle = "rgba(238,221,139,.9)";
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.arc(px, py - 2, 18, 0, Math.PI * 2);
      ctx.stroke();
    }
    ctx.fillStyle = "rgba(30,45,40,.72)";
    ctx.beginPath();
    ctx.arc(px + 12, py - 14, 7, 0, Math.PI * 2);
    ctx.fill();
    ctx.fillStyle = "#fff";
    ctx.font = "bold 8px system-ui, sans-serif";
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.fillText(String(p.seat + 1), px + 12, py - 14);
  }

  if (view.phase === "countdown") {
    ctx.fillStyle = "rgba(24,45,43,.28)";
    ctx.fillRect(0, 0, WIDTH, HEIGHT);
  }
}

export function drawPreview(
  ctx: CanvasRenderingContext2D,
  cells: number[],
  mapId: string,
) {
  const theme = themeFor(mapId);
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.clearRect(0, 0, WIDTH, HEIGHT);
  ctx.fillStyle = theme.ground;
  ctx.fillRect(0, 0, WIDTH, HEIGHT);
  for (let y = 0; y < ROWS; y++) {
    for (let x = 0; x < COLS; x++) {
      const v = cells[y * COLS + x];
      const px = x * CELL;
      const py = y * CELL;
      if (v === 1) {
        ctx.fillStyle = theme.wall;
        ctx.fillRect(px + 1, py + 1, CELL - 2, CELL - 2);
        ctx.fillStyle = theme.wallTop;
        ctx.fillRect(px, py, CELL, 7);
      } else if (v === 2) {
        ctx.fillStyle = theme.crate;
        ctx.fillRect(px + 3, py + 3, CELL - 6, CELL - 6);
      }
    }
  }
}
