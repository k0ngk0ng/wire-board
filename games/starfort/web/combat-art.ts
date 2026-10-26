import type { Crater, Event } from "./types";
import { craterDepth, workerPose } from "./animation";

function ellipse(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  rx: number,
  ry: number,
  color: string,
) {
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.ellipse(x, y, rx, ry, 0, 0, Math.PI * 2);
  ctx.fill();
}
export function drawCrater(ctx: CanvasRenderingContext2D, crater: Crater) {
  const depth = craterDepth(crater.remaining, crater.total),
    r = crater.radius;
  ctx.save();
  ctx.translate(crater.x, crater.y);
  ellipse(ctx, 0, 2, r + 8, r * 0.6 + 4, "#79583c");
  ellipse(ctx, 0, 0, r + 4, r * 0.6, "#b79867");
  ellipse(
    ctx,
    0,
    2,
    r * Math.max(0.12, depth),
    r * 0.53 * Math.max(0.12, depth),
    depth > 0 ? "#493b31" : "#b39b73",
  );
  for (let i = 0; i < 11; i++) {
    const angle = i * 2.399;
    const x = Math.cos(angle) * (r + 3),
      y = Math.sin(angle) * r * 0.6;
    ellipse(ctx, x, y, 3 + (i % 3), 2, "#d1b484");
  }
  if (depth > 0) {
    ctx.strokeStyle = "#e2c69a";
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.ellipse(0, -1, r, r * 0.6, 0, Math.PI, Math.PI * 2);
    ctx.stroke();
  }
  ctx.restore();
}
export function drawRepairTool(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  index: number,
  count: number,
  at: number,
  reduced: boolean,
) {
  const pose = workerPose(index, count, at, reduced);
  ctx.save();
  ctx.translate(x + 9, y - 8);
  ctx.rotate(-0.8 + pose.swing * 0.55);
  ctx.strokeStyle = "#634735";
  ctx.lineWidth = 3;
  ctx.beginPath();
  ctx.moveTo(0, -9);
  ctx.lineTo(0, 12);
  ctx.stroke();
  ctx.fillStyle = "#b8c4c5";
  ctx.beginPath();
  ctx.moveTo(-5, 8);
  ctx.lineTo(5, 8);
  ctx.lineTo(4, 17);
  ctx.lineTo(0, 20);
  ctx.lineTo(-4, 17);
  ctx.closePath();
  ctx.fill();
  ctx.restore();
  if (!reduced && pose.swing > 0.1)
    for (let i = 0; i < 3; i++)
      ellipse(
        ctx,
        x + 8 + i * 3,
        y - 6 - pose.swing * (8 + i * 4),
        2,
        1.5,
        "#c2a06c",
      );
}
export function drawCombatAccent(
  ctx: CanvasRenderingContext2D,
  e: Event,
  age: number,
  x: number,
  y: number,
  reduced: boolean,
) {
  if (reduced || age < 0 || age > 850) return;
  const p = age / 850;
  ctx.save();
  ctx.translate(x, y);
  ctx.globalAlpha = 1 - p;
  if (e.type === "chain") {
    ctx.strokeStyle = "#d9b4ff";
    ctx.lineWidth = 3;
    ctx.shadowColor = "#a96bff";
    ctx.shadowBlur = 9;
    const dx = (e.toX ?? e.x) - e.x,
      dy = (e.toY ?? e.y) - e.y;
    ctx.beginPath();
    ctx.moveTo(0, 0);
    for (let i = 1; i < 8; i++)
      ctx.lineTo(
        (dx * i) / 8 + Math.sin(i * 9 + Math.floor(age / 70)) * 6,
        (dy * i) / 8,
      );
    ctx.lineTo(dx, dy);
    ctx.stroke();
  } else if (e.type === "frost_ring") {
    ctx.strokeStyle = "#c8f4ff";
    ctx.lineWidth = 5 * (1 - p) + 1;
    ctx.beginPath();
    ctx.ellipse(0, 0, 12 + p * 90, 8 + p * 55, 0, 0, Math.PI * 2);
    ctx.stroke();
    for (let i = 0; i < 12; i++) {
      const a = (i * Math.PI) / 6;
      ctx.fillStyle = "#e7fbff";
      ctx.fillRect(
        Math.cos(a) * (12 + p * 90),
        Math.sin(a) * (8 + p * 55),
        3,
        7,
      );
    }
  } else if (["crater", "hit", "death", "crater_filled"].includes(e.type)) {
    const cannon = e.kind === "cannon" || e.type === "crater";
    const n = cannon ? 14 : e.kind === "arrow" ? 5 : 9;
    for (let i = 0; i < n; i++) {
      const a = i * 2.399 + e.id,
        distance = (cannon ? 65 : 30) * p * (0.5 + (i % 4) / 5);
      const px = Math.cos(a) * distance,
        py =
          Math.sin(a) * distance * 0.6 -
          Math.sin(p * Math.PI) * (cannon ? 25 : 12);
      ellipse(
        ctx,
        px,
        py,
        Math.max(1, (cannon ? 5 : 3) * (1 - p)),
        Math.max(1, 3 * (1 - p)),
        cannon
          ? "#c5a277"
          : e.kind === "frost"
            ? "#d3f9ff"
            : e.kind === "arcane"
              ? "#ddb5ff"
              : "#ffe6a2",
      );
    }
    if (cannon) {
      ctx.strokeStyle = "#dfc395";
      ctx.lineWidth = 3 * (1 - p);
      ctx.beginPath();
      ctx.ellipse(0, 0, 8 + p * 48, 4 + p * 25, 0, 0, Math.PI * 2);
      ctx.stroke();
    }
  }
  ctx.restore();
}
