type Point = { x: number; y: number };
export type AttackEffect = { kind: string; shape?: string; direction?: number; context?: { AbilityID?: string }; origin: Point; position: Point; angle: number; travelled: number; radius: number; maxRadius?: number; halfAngle: number; width: number; ageMS: number; delayMS: number; durationMS: number };
type Options = { color?: string; opacity?: number; particles?: boolean; now?: number; motionNow?: number };
const turn = Math.PI * 2;
const rgba = (red: number, green: number, blue: number, alpha: number) => `rgba(${red},${green},${blue},${alpha})`;

function paintArc(context: CanvasRenderingContext2D, effect: AttackEffect, options: Options) {
  const first = effect.angle - effect.halfAngle;
  const last = effect.angle + effect.halfAngle;
  const outer: Point[] = [];
  const inner: Point[] = [];
  for (let index = 0; index <= 18; index++) {
    const progress = index / 18;
    const angle = first + (last - first) * progress;
    const width = 2 + Math.sin(progress * Math.PI) * 8;
    const radii = [effect.radius + width, Math.max(1, effect.radius - width * 0.35)];
    for (const [target, radius] of [[outer, radii[0]], [inner, radii[1]]] as const) target.push({ x: effect.position.x + Math.cos(angle) * radius, y: effect.position.y + Math.sin(angle) * radius });
  }
  const path = () => {
    context.beginPath();
    context.moveTo(outer[0].x, outer[0].y);
    for (const point of outer.slice(1)) context.lineTo(point.x, point.y);
    for (const point of [...inner].reverse()) context.lineTo(point.x, point.y);
    context.closePath();
  };
  const hex = (options.color || "#9933cc").replace("#", "");
  const red = Number.parseInt(hex.slice(0, 2), 16) || 153;
  const green = Number.parseInt(hex.slice(2, 4), 16) || 51;
  const blue = Number.parseInt(hex.slice(4, 6), 16) || 204;
  context.save();
  context.globalAlpha = options.opacity ?? 1;
  context.globalCompositeOperation = "source-over";
  context.lineCap = "round";
  context.shadowBlur = 4;
  context.shadowColor = rgba(red, green, blue, 0.8);
  context.fillStyle = rgba(red, green, blue, 0.28);
  path();
  context.fill();
  context.shadowBlur = 2;
  context.fillStyle = rgba(red, green, blue, 0.9);
  path();
  context.fill();
  context.shadowBlur = 0;
  context.strokeStyle = rgba(red, green, blue, 0.95);
  context.lineWidth = 2;
  path();
  context.stroke();
  context.strokeStyle = rgba(Math.min(255, red + 35), Math.min(255, green + 35), Math.min(255, blue + 35), 0.95);
  context.lineWidth = 1;
  path();
  context.stroke();
  if (options.particles !== false) for (let index = 0; index < 8; index++) {
    const angle = first + (last - first) * (index / 7);
    const pulse = 1 + Math.sin((options.now ?? Date.now()) * 0.01 + index * 1.7) * 0.25;
    context.beginPath();
    context.arc(effect.position.x + Math.cos(angle) * effect.radius, effect.position.y + Math.sin(angle) * effect.radius, (1.5 + index % 3 * 0.7) * pulse, 0, turn);
    context.fillStyle = rgba(220, 160, 255, 0.75);
    context.fill();
  }
  context.restore();
}

function gradient(context: CanvasRenderingContext2D, coordinates: [number, number, number, number], stops: [number, number, number, number, number][]) {
  const result = context.createLinearGradient(...coordinates);
  for (const [position, red, green, blue, alpha] of stops) result.addColorStop(position, rgba(red, green, blue, alpha));
  return result;
}

function paintDune(context: CanvasRenderingContext2D, effect: AttackEffect, options: Options) {
  const now = options.motionNow ?? performance.now();
  const half = effect.width / 2;
  const trail = 145;
  const crest = 38;
  const wobble = Math.sin(now * 0.004 + effect.origin.x * 0.025 + effect.origin.y * 0.015) * 4;
  const outline = () => {
    context.beginPath();
    context.moveTo(-trail, -half * 0.72);
    for (const [controlX, controlY, endX, endY] of [[-trail * 0.62, -half * 1.03, -trail * 0.3, -half * 0.88], [-trail * 0.04, -half * 0.82, crest * 0.58 + wobble, -half * 0.42], [crest + wobble, -half * 0.12, crest + wobble, 0], [crest + wobble, half * 0.12, crest * 0.58 + wobble, half * 0.42], [-trail * 0.04, half * 0.82, -trail * 0.3, half * 0.88], [-trail * 0.62, half * 1.03, -trail, half * 0.72]]) context.quadraticCurveTo(controlX, controlY, endX, endY);
    context.closePath();
  };
  context.save();
  context.globalAlpha *= (options.opacity ?? 1) * 0.75;
  context.translate(effect.position.x, effect.position.y);
  context.rotate(Math.atan2(Math.sin(effect.angle), Math.cos(effect.angle)));
  context.fillStyle = gradient(context, [-trail, 0, crest + 15, 0], [[0, 75, 43, 20, 0], [0.3, 75, 43, 20, 0.16], [0.8, 75, 43, 20, 0.24], [1, 75, 43, 20, 0]]);
  context.beginPath();
  context.ellipse(-trail * 0.4, 5, trail * 0.58, half * 1.05, 0, 0, turn);
  context.fill();
  const body = gradient(context, [-trail, 0, crest + 20, 0], [[0, 145, 91, 43, 0.12], [0.18, 164, 107, 51, 0.55], [0.45, 197, 145, 75, 0.9], [0.75, 228, 184, 105, 0.97], [1, 255, 225, 165, 0.98]]);
  outline();
  context.fillStyle = body;
  context.fill();
  context.save();
  outline();
  context.clip();
  context.fillStyle = gradient(context, [0, -half, 0, half], [[0, 103, 62, 28, 0.45], [0.22, 160, 104, 46, 0.12], [0.5, 255, 224, 153, 0.25], [0.78, 158, 99, 42, 0.12], [1, 91, 54, 24, 0.38]]);
  context.fillRect(-trail - 2, -half - 2, trail + crest + 25, half * 2 + 4);
  const ripple = effect.travelled * 1.4 % 30;
  for (let index = 0; index < 6; index++) {
    const x = -trail + (index * 30 + ripple) % trail;
    const progress = (x + trail) / trail;
    const width = half * (0.2 + progress * 0.52);
    const curve = Math.sin(now * 0.003 + index * 1.8) * 4;
    for (const [offset, height, color, lineWidth] of [[5, width, rgba(116, 70, 31, 0.3), 3], [3, width * 0.94, rgba(255, 229, 170, 0.38), 1.3]] as const) {
      context.beginPath();
      context.moveTo(x - offset, -height);
      context.quadraticCurveTo(x + curve + 5, 0, x - offset, height);
      context.strokeStyle = color;
      context.lineWidth = lineWidth;
      context.stroke();
    }
  }
  context.restore();
  context.strokeStyle = rgba(135, 79, 34, 0.75);
  context.lineWidth = 7;
  context.stroke();
  context.strokeStyle = rgba(255, 233, 183, 0.95);
  context.lineWidth = 2.5;
  context.stroke();
  for (let index = 0; index < 7; index++) {
    const pulse = (Math.sin(now * 0.008 + index * 1.7) + 1) * 0.5;
    const x = crest * 0.2 + index * 4 + wobble * 0.4;
    const y = Math.sin(index * 1.2 + now * 0.002) * half * 0.16;
    context.beginPath();
    context.arc(x, y, 0.8 + pulse * 1.5, 0, turn);
    context.fillStyle = rgba(255, 244, 207, 0.25 + pulse * 0.65);
    context.fill();
  }
  if (options.particles !== false) for (let index = 0; index < 18; index++) {
    const phase = now * 0.003 + index * 2.399;
    const travel = (now * 0.035 + index * 17) % 60 / 60;
    const x = crest + 4 + travel * (25 + index % 4 * 5) + Math.sin(phase) * 5;
    const y = Math.sin(phase * 1.3) * half * (0.35 + index % 4 * 0.15);
    const alpha = (1 - travel) * (0.25 + 0.45 * (0.5 + 0.5 * Math.sin(phase * 1.7)));
    context.beginPath();
    context.arc(x, y, 0.8 + index * 7 % 5 * 0.45, 0, turn);
    context.fillStyle = rgba(255, 218, 145, alpha);
    context.fill();
  }
  context.restore();
}

function paintWave(context: CanvasRenderingContext2D, effect: AttackEffect, options: Options) {
  if (effect.ageMS < effect.delayMS) return;
  const alpha = Math.max(0, 0.9 * (1 - effect.radius / Math.max(1, effect.maxRadius ?? 1)));
  if (alpha <= 0) return;
  const hex = (options.color || "#e85e29").replace("#", "");
  const red = Number.parseInt(hex.slice(0, 2), 16) || 255;
  const green = Number.parseInt(hex.slice(2, 4), 16) || 255;
  const blue = Number.parseInt(hex.slice(4, 6), 16) || 255;
  context.save();
  context.globalAlpha *= options.opacity ?? 1;
  context.globalCompositeOperation = "lighter";
  const layers = [[effect.radius, rgba(red, green, blue, alpha * 0.3), effect.width * 2.2], [effect.radius, rgba(red, green, blue, alpha), effect.width * 0.7], [effect.radius - effect.width * 0.25, rgba(255, 255, 255, alpha * 0.55), effect.width * 0.2]] as const;
  for (const [radius, color, width] of layers) {
    if (radius < 0 || radius === 0 && radius !== effect.radius) continue;
    context.beginPath();
    context.arc(effect.origin.x, effect.origin.y, radius, 0, turn);
    context.strokeStyle = color;
    context.lineWidth = width;
    context.stroke();
  }
  context.restore();
}

function paintBlade(context: CanvasRenderingContext2D, effect: AttackEffect, options: Options) {
  if (effect.ageMS >= effect.durationMS) return;
  const progress = effect.ageMS / effect.durationMS;
  const angle = effect.direction && effect.direction < 0 ? effect.angle + Math.PI / 2 - Math.PI * progress : effect.angle - Math.PI / 2 + Math.PI * progress;
  const x = Math.cos(angle), y = Math.sin(angle);
  const point = (distance: number, offset: number): Point => ({ x: effect.origin.x + x * distance - y * offset, y: effect.origin.y + y * distance + x * offset });
  const left = point(12, 20), right = point(12, -20), endLeft = point(effect.radius, 7), endRight = point(effect.radius, -7);
  const dark = effect.context?.AbilityID === "ceaselessEdge";
  const golden = effect.context?.AbilityID === "intrepidSword";
  const colors = dark ? ["#2b4347", "#2b4347", "rgba(43, 67, 71, 0.95)", "rgba(154, 32, 53, 0.85)", "rgba(154, 32, 53, 0)"] : golden ? ["#fff2a6", "#ff9d00", "rgba(255, 245, 180, 0.98)", "rgba(255, 157, 0, 0.85)", "rgba(255, 140, 0, 0)"] : ["#ffffff", "#b8f8ff", "rgba(255, 255, 255, 0.95)", "rgba(170, 235, 255, 0.75)", "rgba(120, 200, 255, 0)"];
  context.save();
  context.globalAlpha = (1 - progress) * (options.opacity ?? 1);
  context.lineJoin = "round";
  context.shadowColor = colors[1];
  context.shadowBlur = 4;
  const fill = context.createLinearGradient(effect.origin.x, effect.origin.y, endLeft.x, endLeft.y);
  for (const [index, position] of [0, 0.5, 1].entries()) fill.addColorStop(position, colors[index + 2]);
  context.fillStyle = fill;
  context.beginPath();
  context.moveTo(left.x, left.y);
  for (const vertex of [endLeft, endRight, right]) context.lineTo(vertex.x, vertex.y);
  context.closePath();
  context.fill();
  context.strokeStyle = colors[0];
  context.lineWidth = 4;
  for (const vertices of [[left, endLeft], [right, endRight], [endLeft, point(effect.radius + 12, 0), endRight]]) {
    context.beginPath();
    context.moveTo(vertices[0].x, vertices[0].y);
    for (const vertex of vertices.slice(1)) context.lineTo(vertex.x, vertex.y);
    context.stroke();
  }
  context.restore();
}

function paintClaws(context: CanvasRenderingContext2D, effect: AttackEffect, options: Options) {
  const progress = effect.ageMS / effect.durationMS;
  if (progress >= 1) return;
  const empowered = effect.radius > 52;
  const scale = empowered ? 1.5 : 1;
  const opacity = options.opacity ?? 1;
  const primary = empowered ? "#d93432" : "#b9a5d6";
  const highlight = empowered ? "#f08b78" : "#e5dcf2";
  context.save();
  context.translate(effect.origin.x, effect.origin.y);
  context.rotate(effect.angle);
  context.lineCap = "round";
  context.lineJoin = "round";
  if (empowered && progress < 0.42) {
    const ring = progress / 0.42;
    context.globalAlpha = (1 - ring) * 0.4 * opacity;
    context.strokeStyle = "#c95b4b";
    context.lineWidth = 1.5 * (1 - ring) + 0.5;
    context.beginPath();
    context.arc(0, 0, effect.radius * (0.2 + ring * 0.8), 0, turn);
    context.stroke();
  }
  for (let index = -1; index <= 1; index++) {
    const slash = Math.max(0, Math.min(1, (progress - (index + 1) * 0.075) / 0.42));
    if (slash === 0) continue;
    const eased = 1 - Math.pow(1 - slash, 2);
    const offset = index * effect.radius * 0.38;
    const start = { x: -effect.radius, y: offset - 10 * scale };
    const control = { x: 0, y: offset + (index === 0 ? 4 : 7) * scale };
    const end = { x: effect.radius, y: offset + 10 * scale };
    const x = (1 - eased) ** 2 * start.x + 2 * (1 - eased) * eased * control.x + eased ** 2 * end.x;
    const y = (1 - eased) ** 2 * start.y + 2 * (1 - eased) * eased * control.y + eased ** 2 * end.y;
    const fade = Math.min(1, slash * 5, (1 - progress) * 2.2);
    const fill = context.createLinearGradient(start.x, start.y, end.x, end.y);
    fill.addColorStop(0, empowered ? "#a93d39" : "#827095");
    fill.addColorStop(0.55, primary);
    fill.addColorStop(1, highlight);
    context.globalAlpha = fade * opacity;
    context.beginPath();
    context.moveTo(start.x, start.y);
    context.quadraticCurveTo(start.x + (control.x - start.x) * eased, start.y + (control.y - start.y) * eased, x, y);
    for (const [color, width, alpha] of [[empowered ? "#732b2c" : "#6e5d80", 9 * scale, fade * opacity], [fill, 5 * scale, fade * opacity], [highlight, scale, fade * opacity * 0.65]] as const) {
      context.strokeStyle = color;
      context.lineWidth = width;
      context.globalAlpha = alpha;
      context.stroke();
    }
  }
  context.restore();
}

function paintWhip(context: CanvasRenderingContext2D, effect: AttackEffect) {
  const progress = Math.min(1, effect.ageMS / effect.durationMS);
  const fade = 1 - Math.max(0, (progress - 0.7) / 0.3);
  const start = effect.angle - effect.halfAngle;
  const sweep = effect.angle + effect.halfAngle - start;
  const path = (phase: number) => {
    const angle = start + phase * sweep;
    const dx = effect.origin.x + Math.cos(angle) * effect.radius - effect.origin.x;
    const dy = effect.origin.y + Math.sin(angle) * effect.radius - effect.origin.y;
    const distance = Math.hypot(dx, dy);
    context.beginPath();
    for (let index = 0; index <= 10; index++) {
      const portion = index / 10;
      const curve = Math.sin(portion * Math.PI) * distance * 0.15 + Math.sin(portion * Math.PI * 2.5) * distance * 0.03;
      const x = effect.origin.x + dx * portion - dy / distance * curve;
      const y = effect.origin.y + dy * portion + dx / distance * curve;
      if (index === 0) context.moveTo(x, y);
      else context.lineTo(x, y);
    }
    context.stroke();
  };
  context.save();
  context.globalCompositeOperation = "lighter";
  context.shadowColor = rgba(92, 18, 0, 0.35 * fade);
  context.strokeStyle = rgba(92, 18, 0, 0.55 * fade);
  context.lineWidth = 8;
  context.lineCap = "round";
  context.lineJoin = "round";
  path(progress);
  for (let index = 1; index <= 10; index++) {
    const portion = index / 10;
    context.strokeStyle = rgba(196, 58, 18, (1 - portion) * 0.45 * fade);
    context.lineWidth = 6 * (1 - portion * 0.5);
    path(Math.max(0, progress - portion * 0.12));
  }
  context.restore();
}

export function paintAttackModeEffect(context: CanvasRenderingContext2D, effect: AttackEffect, options: Options = {}): boolean {
  if (effect.kind === "arc") paintArc(context, effect, options);
  else if (effect.kind === "dune") paintDune(context, effect, options);
  else if (effect.kind === "wave") paintWave(context, effect, options);
  else if (effect.shape === "blade") paintBlade(context, effect, options);
  else if (effect.shape === "targetCircle") paintClaws(context, effect, options);
  else if (effect.context?.AbilityID === "fireLash") paintWhip(context, effect);
  else return false;
  return true;
}
