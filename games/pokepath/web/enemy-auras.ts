import type { Point } from "./types";

type Stop = readonly [number, string, number, number];
type AuraStyle = { radius: number; core: number; period: number; amplitude?: number; absolute?: boolean; fill: Stop[]; outline?: readonly [string, number, number] };

const styles: Record<string, AuraStyle> = {
  flameBody: { radius: 120, core: .2, period: 500, absolute: true, fill: [[0, "255,75,25", .16, .06], [.7, "255,55,15", .08, .04], [1, "255,45,10", .03, 0]], outline: ["255,75,35", .45, .2] },
  tailWind: { radius: 75, core: .2, period: 500, absolute: true, fill: [[0, "220,250,255", .08, .05], [.75, "220,250,255", .2, .08], [1, "220,250,255", 0, 0]] },
  snowCloak: { radius: 150, core: .2, period: 600, amplitude: .8, fill: [[0, "180,220,255", .35, .15], [.7, "180,220,255", .15, .1], [1, "180,220,255", .08, 0]], outline: ["200,230,255", .55, 0] },
  ingrain: { radius: 140, core: .15, period: 700, fill: [[0, "80,255,120", .3, .15], [.65, "60,220,90", .18, .08], [1, "60,220,90", .03, 0]], outline: ["120,255,160", .55, .2] },
  static: { radius: 115, core: .2, period: 200, fill: [[0, "255,255,180", .55, .25], [.6, "255,240,120", .25, .15], [1, "255,240,120", .08, 0]], outline: ["255,240,120", .9, 0] }
};
const circle = Math.PI * 2;
const rgba = (rgb: string, alpha: number) => `rgba(${rgb},${alpha})`;
const at = ({ x, y }: Point, angle: number, radius: number): Point => ({ x: x + Math.cos(angle) * radius, y: y + Math.sin(angle) * radius });

export function enemyAuraReach(passive?: string) {
  return passive ? (styles[passive]?.radius ?? 0) + 10 : 0;
}

function dot(ctx: CanvasRenderingContext2D, position: Point, size: number, fill: string) {
  ctx.fillStyle = fill;
  ctx.beginPath();
  ctx.arc(position.x, position.y, size, 0, circle);
  ctx.fill();
}

function movingWind(ctx: CanvasRenderingContext2D, center: Point, radius: number, now: number) {
  ctx.strokeStyle = "rgba(230,250,255,0.75)";
  ctx.lineWidth = 2.5;
  ctx.lineCap = "round";
  for (let index = 0; index < 3; index++) {
    const angle = index / 3 * circle + now / 900;
    const start = at(center, angle, radius);
    const end = at(center, angle + .9, radius);
    const control = at(center, angle + .45, radius * 1.25);
    ctx.beginPath();
    ctx.moveTo(start.x, start.y);
    ctx.quadraticCurveTo(control.x, control.y, end.x, end.y);
    ctx.stroke();
  }
}

function embers(ctx: CanvasRenderingContext2D, center: Point, now: number) {
  ctx.globalAlpha = .7 + .2 * Math.sin(now / 300);
  for (let index = 0; index < 500; index++) {
    const point = at(center, Math.random() * circle, 70 + Math.random() * 50);
    const size = 2 + Math.random() * 3;
    dot(ctx, point, size, `rgb(255,${Math.floor(100 + 155 * Math.random())},0)`);
  }
}

function pollen(ctx: CanvasRenderingContext2D, center: Point, radius: number, now: number) {
  for (let index = 0; index < 40; index++) {
    const seed = index * 987.123;
    const phase = now * .0015 + seed;
    const point = at(center, seed * 17 % circle, seed * 53 % radius);
    point.y -= Math.sin(phase) * 6;
    dot(ctx, point, 1.5 + seed * 13 % 2.5, rgba("180,255,180", .45 + .35 * Math.sin(phase * 2)));
  }
  for (let index = 0; index < 12; index++) dot(ctx, at(center, now / 1800 + index * (circle / 12), radius), 2.5, "rgba(220,255,220,0.8)");
}

function sparks(ctx: CanvasRenderingContext2D, center: Point, radius: number, now: number) {
  ctx.strokeStyle = rgba("255,255,200", .85 + .15 * Math.sin(now / 100));
  ctx.lineWidth = 1.2;
  for (let index = 0; index < 22; index++) {
    const point = at(center, Math.random() * circle, radius * (.55 + Math.random() * .45));
    const length = 14 + Math.random() * 10;
    const segments = 3 + Math.floor(Math.random() * 3);
    let { x, y } = point;
    ctx.beginPath();
    ctx.moveTo(x, y);
    for (let segment = 0; segment < segments; segment++) {
      x += (Math.random() - .5) * length;
      y += (Math.random() - .5) * length;
      ctx.lineTo(x, y);
    }
    ctx.stroke();
    dot(ctx, point, 1.5, "rgba(255,240,120,0.95)");
  }
}

export function paintEnemyAura(ctx: CanvasRenderingContext2D, position: Point, passive: string | undefined, now: number, particles: boolean) {
  const style = passive && styles[passive];
  if (!style) return;
  const { x, y } = position;
  const { radius } = style;
  const pulse = .5 + (style.amplitude ?? .5) * Math.sin(now / style.period);
  ctx.save();
  if (style.absolute) ctx.globalAlpha = 1;
  const gradient = ctx.createRadialGradient(x, y, radius * style.core, x, y, radius);
  for (const [offset, color, alpha, gain] of style.fill) gradient.addColorStop(offset, rgba(color, alpha + pulse * gain));
  ctx.beginPath();
  ctx.arc(x, y, radius, 0, circle);
  ctx.fillStyle = gradient;
  ctx.fill();
  if (style.outline) {
    const [color, alpha, gain] = style.outline;
    ctx.strokeStyle = rgba(color, alpha + pulse * gain);
    ctx.lineWidth = 2;
    ctx.stroke();
  }
  if (passive === "tailWind") movingWind(ctx, position, radius * .65, now);
  else if (particles) {
    if (passive === "flameBody") embers(ctx, position, now);
    else if (passive === "ingrain") pollen(ctx, position, radius, now);
    else if (passive === "static") sparks(ctx, position, radius, now);
    else if (passive === "snowCloak") for (let index = 0; index < 50; index++) dot(ctx, at(position, Math.random() * circle, Math.random() * radius), 1 + Math.random() * 2, "rgba(210,240,255,0.8)");
  }
  ctx.restore();
}
