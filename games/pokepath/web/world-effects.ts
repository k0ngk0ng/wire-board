export type PersistentZone = { kind: string; x: number; y: number; radius: number; innerRadius?: number; magma?: boolean; smoke?: boolean; phase?: string; ageMS: number; lifeMS: number };
type Options = { opacity?: number; particles?: boolean; motionNow?: number };
type Point = { x: number; y: number };

const turn = Math.PI * 2;
const color = (red: number, green: number, blue: number, alpha: number) => `rgba(${red},${green},${blue},${alpha})`;
const at = (angle: number, radius: number): Point => ({ x: Math.cos(angle) * radius, y: Math.sin(angle) * radius });
const fraction = (value: number) => value - Math.floor(value);
const noise = (seed: number) => fraction(Math.sin(seed * 12.9898) * 43758.5453);
let smokeTexture: HTMLCanvasElement | OffscreenCanvas | undefined;

export function paintShadowExplosion(context: CanvasRenderingContext2D, effect: { x: number; y: number; radius: number; ageMS: number; durationMS: number }): boolean {
  const progress = Math.min(1, effect.ageMS / effect.durationMS);
  const radius = effect.radius * (0.25 + (1 - Math.pow(1 - progress, 3)) * 0.9);
  const alpha = 1 - progress;
  const x = effect.x, y = effect.y + 8;
  context.save();
  const fill = context.createRadialGradient(x, y, 0, x, y, radius);
  for (const [position, red, green, blue, opacity] of [[0, 0, 0, 0, alpha * 0.95], [0.35, 8, 3, 14, alpha * 0.8], [0.7, 35, 10, 50, alpha * 0.35], [1, 60, 15, 80, 0]]) fill.addColorStop(position, color(red, green, blue, opacity));
  context.fillStyle = fill;
  context.beginPath();
  context.ellipse(x, y, radius, radius * 0.42, 0, 0, turn);
  context.fill();
  context.strokeStyle = color(70, 25, 95, alpha * 0.45);
  context.lineWidth = 2.5 * (1 - progress * 0.5);
  context.beginPath();
  context.ellipse(x, y, radius * 0.9, radius * 0.36, 0, 0, turn);
  context.stroke();
  for (let index = 0; index < 8; index++) {
    const angle = index / 8 * Math.PI * 2 + index * 0.35;
    const distance = radius * (0.35 + progress * 0.7);
    context.strokeStyle = color(12, 4, 20, alpha * 0.7);
    context.lineWidth = 2 + Math.sin(index) * 0.5;
    context.beginPath();
    context.moveTo(x + Math.cos(angle) * radius * 0.15, y + Math.sin(angle) * radius * 0.06);
    context.quadraticCurveTo(x + Math.cos(angle + 0.25) * distance * 0.55, y + Math.sin(angle + 0.25) * distance * 0.2, x + Math.cos(angle) * distance, y + Math.sin(angle) * distance * 0.35);
    context.stroke();
  }
  context.restore();
  return true;
}

function smokeImage() {
  if (smokeTexture) return smokeTexture;
  const canvas = typeof OffscreenCanvas === "undefined" ? document.createElement("canvas") : new OffscreenCanvas(96, 96);
  canvas.width = 96;
  canvas.height = 96;
  const context = canvas.getContext("2d") as CanvasRenderingContext2D;
  const shade = context.createRadialGradient(48, 48, 2, 48, 48, 96 * 0.48);
  for (const [position, red, green, blue, alpha] of [[0, 145, 145, 150, 0.75], [0.35, 120, 120, 126, 0.58], [0.7, 90, 90, 98, 0.30], [1, 60, 60, 68, 0]]) shade.addColorStop(position, color(red, green, blue, alpha));
  context.fillStyle = shade;
  context.beginPath();
  context.arc(48, 48, 96 * 0.46, 0, turn);
  context.fill();
  context.globalCompositeOperation = "source-over";
  for (const [x, y, radius] of [[-18, -8, 20], [14, -12, 18], [20, 10, 21], [-12, 17, 18], [0, 0, 24]]) {
    context.fillStyle = color(115, 115, 122, 0.10);
    context.beginPath();
    context.arc(48 + x, 48 + y, radius, 0, turn);
    context.fill();
  }
  smokeTexture = canvas;
  return canvas;
}

function smoke(context: CanvasRenderingContext2D, zone: PersistentZone, alpha: number, now: number) {
  const progress = Math.min(1, zone.ageMS / zone.lifeMS);
  const radius = zone.radius * (0.35 + progress * 0.65);
  const fade = Math.min(1, progress * 6);
  const time = now * 0.001;
  const texture = smokeImage();
  const seedBase = Math.abs(Math.sin(zone.x * 12.9898 + zone.y * 78.233) * 43758.5453) % 1000;
  for (let index = 0; index < 6; index++) {
    const seed = seedBase + index * 17.31;
    const angle = index / 6 * Math.PI * 2 + Math.sin(seed) * 0.5;
    const distance = radius * (0.10 + (Math.sin(seed * 4.1) * 0.5 + 0.5) * 0.35);
    const phase = time * (0.7 + (Math.sin(seed * 2.7) * 0.5 + 0.5) * 0.5) + seed;
    const rise = (Math.sin(phase) * 0.5 + 0.5) * radius * 0.65;
    const x = Math.cos(angle) * distance + Math.sin(time * 1.1 + seed) * radius * 0.10 + Math.sin(time * 0.47 + seed * 2) * radius * 0.06;
    const y = Math.sin(angle) * distance - rise + Math.cos(time * 0.7 + seed) * radius * 0.035;
    const size = radius * (0.28 + (0.55 + (Math.sin(seed * 5.3) * 0.5 + 0.5) * 0.35) * 0.18);
    const pulse = 0.92 + Math.sin(time * 1.4 + seed) * 0.08;
    context.globalAlpha = alpha * fade * (0.75 + Math.sin(time * 0.8 + seed * 1.7) * 0.15);
    context.drawImage(texture, x - size * pulse, y - size * pulse, size * 2 * pulse, size * 2 * pulse);
  }
  context.globalAlpha = alpha * fade * 0.38;
  const core = radius * 1.15;
  context.drawImage(texture, -core, -core, core * 2, core * 2);
  context.globalAlpha = alpha * 0.45 * (0.85 + Math.sin(time * 2) * 0.08);
  context.strokeStyle = color(170, 170, 178, 1);
  context.lineWidth = 2;
  context.beginPath();
  context.arc(0, 0, radius, 0, turn);
  context.stroke();
  context.globalAlpha = alpha * 0.18;
  context.lineWidth = 4;
  context.beginPath();
  context.arc(0, 0, radius + 2, 0, turn);
  context.stroke();
  context.globalAlpha = 1;
}

function shadowTag(context: CanvasRenderingContext2D, zone: PersistentZone, now: number) {
  const radius = Math.min(zone.radius, zone.radius * zone.ageMS / 2000);
  const progress = radius / zone.radius;
  const pulse = 1 + Math.sin(now / 180) * 0.035;
  const width = radius * pulse;
  const height = radius * 0.42 * pulse;
  const x = zone.x;
  const y = zone.y + 8;
  context.save();
  const shade = context.createRadialGradient(x, y, 0, x, y, width);
  shade.addColorStop(0, color(2, 1, 4, 0.95 + progress * 0.03));
  shade.addColorStop(0.45, color(7, 3, 12, 0.9 + progress * 0.05));
  shade.addColorStop(0.75, color(15, 6, 25, 0.7 + progress * 0.08));
  shade.addColorStop(1, color(35, 10, 50, 0));
  context.fillStyle = shade;
  context.beginPath();
  context.ellipse(x, y, width, height, 0, 0, turn);
  context.fill();
  context.beginPath();
  for (let index = 0; index <= 32; index++) {
    const angle = index / 32 * Math.PI * 2;
    const wave = Math.sin(angle * 3 + now / 260) * 0.06 + Math.sin(angle * 7 - now / 330) * 0.035;
    const radiusX = width * (0.93 + wave);
    const radiusY = height * (0.93 + wave);
    const point = { x: x + Math.cos(angle) * radiusX, y: y + Math.sin(angle) * radiusY };
    if (index === 0) context.moveTo(point.x, point.y);
    else context.lineTo(point.x, point.y);
  }
  context.closePath();
  context.strokeStyle = color(45, 15, 65, 0.3 + progress * 0.15);
  context.lineWidth = 2;
  context.stroke();
  for (let ring = 0; ring < 3; ring++) {
    const phase = (zone.ageMS / 650 + ring / 3) % 1;
    const size = width * (0.25 + phase * 0.7);
    context.strokeStyle = color(65, 25, 90, (1 - phase) * 0.16);
    context.lineWidth = 1.2;
    context.beginPath();
    context.ellipse(x, y, size, size * 0.38, 0, 0, turn);
    context.stroke();
  }
  for (let index = 0; index < 6; index++) {
    const angle = index / 6 * Math.PI * 2 + Math.sin(now / 900 + index) * 0.12;
    const length = width * (0.10 + Math.sin(now / 350 + index * 2) * 0.05);
    const first = { x: x + Math.cos(angle) * width * 0.55, y: y + Math.sin(angle) * height * 0.55 };
    const last = { x: first.x + Math.cos(angle) * length, y: first.y + Math.sin(angle) * length * 0.45 };
    context.strokeStyle = color(15, 5, 25, 0.35 + progress * 0.15);
    context.lineWidth = 2;
    context.beginPath();
    context.moveTo(first.x, first.y);
    context.quadraticCurveTo(first.x + Math.cos(angle + 0.5) * length * 0.5, first.y + Math.sin(angle + 0.5) * length * 0.25, last.x, last.y);
    context.stroke();
  }
  context.restore();
}

function toothedPath(radius: number, height: number, count: number) {
  const path = new Path2D();
  path.moveTo(radius, 0);
  for (let index = 0; index < count; index++) {
    const first = index * Math.PI * 2 / count;
    const second = (index + 1) * Math.PI * 2 / count;
    const peak = at((first + second) / 2, radius + height);
    const base = at(second, radius);
    path.lineTo(peak.x, peak.y);
    path.lineTo(base.x, base.y);
  }
  path.closePath();
  return path;
}

function spikes(context: CanvasRenderingContext2D, radius: number, alpha: number, now: number) {
  const edge = toothedPath(radius, 6, 24);
  const shade = context.createRadialGradient(0, 0, 0, 0, 0, radius + 6);
  shade.addColorStop(0, color(60, 45, 30, 0));
  shade.addColorStop(0.8, color(80, 55, 40, alpha * 0.25));
  shade.addColorStop(1, color(45, 35, 25, alpha * 0.4));
  context.fillStyle = shade;
  context.fill(edge);
  context.strokeStyle = color(120, 95, 70, alpha);
  context.lineWidth = 2.5;
  context.stroke(edge);
  for (let index = 0; index < 12; index++) {
    const angle = index * Math.PI * 2 / 12;
    const outside = at(angle, radius - 2);
    const inside = at(angle, radius * 0.4);
    context.strokeStyle = color(180, 150, 120, alpha * (0.4 + 0.6 * Math.sin(angle + now / 300)));
    context.lineWidth = 2;
    context.beginPath();
    context.moveTo(outside.x, outside.y);
    context.lineTo(inside.x, inside.y);
    context.stroke();
  }
}

function web(context: CanvasRenderingContext2D, radius: number, alpha: number, now: number) {
  const shade = context.createRadialGradient(0, 0, 0, 0, 0, radius);
  shade.addColorStop(0, color(220, 225, 230, 0));
  shade.addColorStop(0.8, color(200, 205, 215, alpha * 0.12));
  shade.addColorStop(1, color(180, 185, 195, alpha * 0.2));
  context.beginPath();
  context.arc(0, 0, radius, 0, turn);
  context.fillStyle = shade;
  context.fill();
  const axes = Array.from({ length: 10 }, (_, index) => ({ angle: index * Math.PI * 2 / 10 + (noise(index + 10) - 0.5) * 0.06, radius: radius * (1 + (noise(index + 20) - 0.5) * 0.25) }));
  for (const [index, axis] of axes.entries()) {
    const bend = (noise(index + 30) - 0.5) * radius * 0.08;
    const middle = at(axis.angle, axis.radius * 0.5);
    const end = at(axis.angle, axis.radius);
    context.strokeStyle = color(230, 235, 240, alpha * (0.6 + 0.4 * Math.sin(axis.angle * 3 + now / 500)));
    context.lineWidth = 1.2;
    context.beginPath();
    context.moveTo(0, 0);
    context.quadraticCurveTo(middle.x - Math.sin(axis.angle) * bend, middle.y + Math.cos(axis.angle) * bend, end.x, end.y);
    context.stroke();
  }
  for (let ring = 1; ring <= 4; ring++) {
    const points = axes.map((axis, index) => ({ angle: axis.angle, radius: axis.radius * ring / 4 * (1 + (noise(ring * 100 + index) - 0.5) * 0.15) }));
    context.strokeStyle = ring === 4 ? color(200, 205, 215, alpha * 0.5) : color(210, 215, 225, alpha * (0.35 + 0.15 * Math.sin(now / 600 + ring)));
    context.lineWidth = ring === 4 ? 1.5 : 1;
    context.beginPath();
    const first = at(points[0].angle, points[0].radius);
    context.moveTo(first.x, first.y);
    for (let index = 1; index <= 10; index++) {
      const current = points[index % 10];
      const previous = points[(index - 1) % 10];
      const control = at((previous.angle + current.angle) / 2, (previous.radius + current.radius) / 2 * (1 + (noise(ring * 200 + index % 10) - 0.5) * 0.12));
      const point = at(current.angle, current.radius);
      context.quadraticCurveTo(control.x, control.y, point.x, point.y);
    }
    context.stroke();
  }
}

function magma(context: CanvasRenderingContext2D, radius: number, innerRadius: number, alpha: number, now: number, particles: boolean) {
  const outer = toothedPath(radius, 9, 20);
  const inside = innerRadius > 0 ? toothedPath(innerRadius, -9 * 0.7, 14) : undefined;
  const band = new Path2D();
  band.addPath(outer);
  if (inside) band.addPath(inside);
  const pulse = 0.5 + 0.5 * Math.sin(now / 500);
  context.save();
  context.globalCompositeOperation = "lighter";
  const shade = context.createRadialGradient(0, 0, innerRadius, 0, 0, radius + 9);
  shade.addColorStop(0, color(255, 210, 80, alpha * (0.08 + pulse * 0.01)));
  shade.addColorStop(0.5, color(255, 120, 25, alpha * 0.2));
  shade.addColorStop(1, color(170, 30, 10, alpha * 0.3));
  context.fillStyle = shade;
  context.fill(band, "evenodd");
  context.strokeStyle = color(255, 150, 40, alpha);
  context.lineWidth = 2.5;
  context.stroke(outer);
  if (inside) {
    context.strokeStyle = color(255, 170 + Math.floor(pulse * 20), 60, alpha * (0.8 + pulse * 0.15));
    context.stroke(inside);
  }
  if (particles) for (let index = 0; index < 14; index++) {
    const seed = index * 12.9898;
    const sample = (offset: number) => fraction(Math.sin(seed + offset) * 43758.5453);
    const cycle = 1200 + sample(1) * 600;
    const phase = (now + sample(2) * cycle) % cycle / cycle;
    const point = at(sample(0) * Math.PI * 2, innerRadius + Math.max(1, radius - innerRadius) * (0.15 + phase * 0.8));
    context.fillStyle = color(255, 190 + Math.floor(sample(4) * 60), 90, Math.sin(phase * Math.PI) * alpha);
    context.beginPath();
    context.arc(point.x, point.y, 1.5 + sample(3) * 1.5, 0, turn);
    context.fill();
  }
  context.restore();
}

export function paintPersistentZone(context: CanvasRenderingContext2D, zone: PersistentZone, now: number, options: Options = {}): boolean {
  if (!["spikes", "web", "magmaSpikes", "smoke", "shadowTag"].includes(zone.kind)) return false;
  if (zone.kind === "shadowTag") {
    if (zone.phase === "travel") return false;
    if (zone.ageMS < 2000 && zone.radius > 0) shadowTag(context, zone, options.motionNow ?? performance.now());
    return true;
  }
  if (zone.ageMS >= zone.lifeMS || zone.radius <= 0) return true;
  const remaining = Math.max(0, (zone.lifeMS - zone.ageMS) / zone.lifeMS);
  const alpha = Math.min(Math.min(1, (1 - remaining) / 0.15), remaining > 0.1 ? 1 : remaining / 0.1);
  context.save();
  context.globalAlpha *= options.opacity ?? 1;
  context.translate(zone.x, zone.y);
  if (zone.kind === "smoke" || zone.smoke) {
    smoke(context, zone, alpha * (options.opacity ?? 1), now);
    context.restore();
    return true;
  }
  const scale = 1 + Math.sin(now / 800) * 0.01;
  context.scale(scale, scale);
  context.rotate(now / 6000 % turn);
  if (zone.kind === "web") web(context, zone.radius, alpha, now);
  else if (zone.kind === "magmaSpikes" || zone.magma) magma(context, zone.radius, zone.innerRadius ?? 0, alpha, now, options.particles !== false);
  else spikes(context, zone.radius, alpha, now);
  context.restore();
  return true;
}

export function paintSingularityOverlay(context: CanvasRenderingContext2D, zone: PersistentZone): boolean {
  if (zone.kind !== "singularity") return false;
  const circle = (radius: number) => { context.beginPath(); context.arc(zone.x, zone.y, radius, 0, turn); };
  if (zone.phase === "waiting") {
    const pulse = 1 + Math.sin(zone.ageMS * 0.012) * 0.12;
    context.save();
    context.globalCompositeOperation = "lighter";
    circle(12 * pulse);
    context.fillStyle = color(210, 100, 255, 0.20);
    context.fill();
    circle(18 * pulse);
    context.strokeStyle = color(245, 180, 255, 0.45);
    context.lineWidth = 2;
    context.stroke();
    context.restore();
    if (zone.ageMS >= 1000) {
      context.save();
      context.globalCompositeOperation = "lighter";
      circle(zone.radius);
      context.fillStyle = color(210, 100, 255, 0.18);
      context.fill();
      context.restore();
    }
  } else if (zone.phase === "pulling") {
    const progress = Math.min(1, zone.ageMS / 450);
    context.save();
    context.globalCompositeOperation = "lighter";
    circle(zone.radius * (1 - progress * 0.35));
    context.strokeStyle = color(210, 100, 255, 0.45 * (1 - progress));
    context.lineWidth = 4;
    context.stroke();
    context.restore();
  }
  return true;
}
