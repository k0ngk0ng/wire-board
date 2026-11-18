type Point = { x: number; y: number };
export type DirectionalParticle = { distance: number; offset: number; size: number; speed: number; wave: number; opacity: number; rotation: number };
type Effect = { origin: Point; angle: number; radius: number; halfAngle: number; ageMS: number; durationMS: number };
type Options = { ability: string; opacity?: number; particles?: boolean; now?: number; startedAt?: number; particleData?: readonly DirectionalParticle[] };

export function createDirectionalParticles(ability: string, random: () => number = Math.random): DirectionalParticle[] {
  if (ability !== "icyWind" && ability !== "dazzlingGleam") return [];
  return Array.from({ length: ability === "icyWind" ? 18 : 24 }, () => ({ distance: random(), offset: (random() * 2 - 1) * 0.85, size: 1 + random() * (ability === "icyWind" ? 2.5 : 2), speed: 0.7 + random() * 0.7, wave: random() * Math.PI * 2, opacity: 0.35 + random() * 0.4, rotation: random() * Math.PI * 2 }));
}

export function paintDirectionalAttack(context: CanvasRenderingContext2D, effect: Effect, options: Options): boolean {
  if (options.ability !== "icyWind" && options.ability !== "dazzlingGleam") return false;
  if (effect.ageMS >= effect.durationMS) return true;
  const icy = options.ability === "icyWind";
  const progress = effect.ageMS / effect.durationMS;
  const radius = effect.radius * (1 - Math.pow(1 - progress, 3));
  const leftAngle = effect.angle - effect.halfAngle, rightAngle = effect.angle + effect.halfAngle;
  const endpoint = (angle: number): Point => ({ x: effect.origin.x + Math.cos(angle) * radius, y: effect.origin.y + Math.sin(angle) * radius });
  const left = endpoint(leftAngle), right = endpoint(rightAngle);
  let alpha = 0.6;
  if (progress > 0.72) alpha *= 1 - (progress - 0.72) / 0.28;
  const opacity = options.opacity ?? 1;
  const now = options.now ?? Date.now();
  const startedAt = options.startedAt ?? now - effect.ageMS;
  const colors = icy ? ["rgba(20, 170, 255, 0.78)", "rgba(5, 130, 245, 0.68)", "rgba(0, 95, 220, 0.55)"] : ["rgba(255, 210, 255, 0.85)", "rgba(255, 130, 225, 0.68)", "rgba(225, 70, 190, 0.35)"];
  context.save();
  context.globalCompositeOperation = "lighter";
  context.globalAlpha = alpha * opacity;
  const fill = context.createRadialGradient(effect.origin.x, effect.origin.y, 0, effect.origin.x, effect.origin.y, Math.max(radius, 1));
  for (const [index, position] of [0, 0.35, 0.7, 1].entries()) fill.addColorStop(position, colors[Math.min(2, index)]);
  context.fillStyle = fill;
  context.beginPath();
  context.moveTo(effect.origin.x, effect.origin.y);
  context.lineTo(left.x, left.y);
  context.arc(effect.origin.x, effect.origin.y, radius, leftAngle, rightAngle);
  context.lineTo(effect.origin.x, effect.origin.y);
  context.closePath();
  context.fill();
  if (options.particles !== false) for (const particle of options.particleData ?? []) {
    let portion = particle.distance + progress * particle.speed;
    if (portion > 1) portion -= 1;
    const distance = radius * portion;
    const side = particle.offset * (distance * 0.72) + Math.sin(startedAt * 0.008 + particle.wave + distance * 0.025) * 4;
    const x = effect.origin.x + Math.cos(effect.angle) * distance + Math.cos(effect.angle + Math.PI / 2) * side;
    const y = effect.origin.y + Math.sin(effect.angle) * distance + Math.sin(effect.angle + Math.PI / 2) * side;
    context.globalAlpha = alpha * particle.opacity * opacity;
    context.fillStyle = icy ? "rgba(90, 190, 255, 0.8)" : "rgba(255, 225, 250, 0.9)";
    if (icy) {
      context.beginPath();
      context.arc(x, y, particle.size, 0, Math.PI * 2);
      context.fill();
      if (particle.size > 2) {
        context.strokeStyle = "rgba(220, 250, 255, 0.7)";
        context.lineWidth = 1;
        context.beginPath();
        context.moveTo(x - particle.size * 1.5, y);
        context.lineTo(x + particle.size * 1.5, y);
        context.moveTo(x, y - particle.size * 1.5);
        context.lineTo(x, y + particle.size * 1.5);
        context.stroke();
      }
    } else {
      const size = particle.size * 2;
      context.save();
      context.translate(x, y);
      context.rotate(particle.rotation + now * 0.002);
      context.beginPath();
      context.moveTo(0, -size);
      for (const [dx, dy] of [[0.35, -0.35], [1, 0], [0.35, 0.35], [0, 1], [-0.35, 0.35], [-1, 0], [-0.35, -0.35]]) context.lineTo(dx * size, dy * size);
      context.closePath();
      context.fill();
      context.restore();
    }
  }
  context.globalAlpha = alpha * 0.9 * opacity;
  context.strokeStyle = icy ? "rgba(40, 160, 255, 0.95)" : "rgba(255, 180, 245, 0.95)";
  context.lineWidth = 2.5;
  context.lineCap = "round";
  context.lineJoin = "round";
  for (const point of [left, right]) {
    context.beginPath();
    context.moveTo(effect.origin.x, effect.origin.y);
    context.lineTo(point.x, point.y);
    context.stroke();
  }
  context.strokeStyle = "rgba(130, 220, 255, 0.8)";
  context.lineWidth = 2;
  context.beginPath();
  context.arc(effect.origin.x, effect.origin.y, radius, leftAngle, rightAngle);
  context.stroke();
  context.restore();
  return true;
}
