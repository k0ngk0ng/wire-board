type Point = { x: number; y: number };
export type BeamEffect = { from: Point; to: Point; extendedTo: Point; width: number; alpha: number };
type Options = { ability: string; item?: string; color?: string; opacity?: number; particles?: boolean; now?: number };
const turn = Math.PI * 2;
const rgba = (red: number, green: number, blue: number, alpha: number) => `rgba(${red},${green},${blue},${alpha})`;

function strokeLine(context: CanvasRenderingContext2D, from: Point, to: Point) {
  context.beginPath();
  context.moveTo(from.x, from.y);
  context.lineTo(to.x, to.y);
  context.stroke();
}

function paintAnchor(context: CanvasRenderingContext2D, beam: BeamEffect) {
  context.globalCompositeOperation = "source-over";
  const origin = { x: beam.from.x, y: beam.from.y - 18 };
  const points = Array.from({ length: 11 }, (_, index) => {
    const progress = index / 10;
    return { x: origin.x + (beam.to.x - origin.x) * progress, y: origin.y + (beam.to.y - origin.y) * progress + Math.sin(progress * Math.PI) * 10 };
  });
  const width = beam.width * 0.85, height = beam.width * 1.6;
  for (let index = 0; index < 10; index++) {
    const first = points[index], second = points[index + 1];
    context.save();
    context.translate((first.x + second.x) / 2, (first.y + second.y) / 2);
    context.rotate(Math.atan2(second.y - first.y, second.x - first.x) + (index % 2 === 0 ? 0 : Math.PI / 2));
    for (const [x, y, rx, ry, start, end, color, lineWidth] of [[1, 1.5, height / 2, width / 2, 0, turn, rgba(0, 0, 0, 0.35 * beam.alpha), width * 0.55], [0, 0, height / 2, width / 2, 0, turn, rgba(124, 96, 76, 0.95 * beam.alpha), width * 0.5], [0, -width * 0.12, height / 2 * 0.85, width / 2 * 0.4, Math.PI, turn, rgba(180, 152, 130, 0.6 * beam.alpha), width * 0.18]] as const) {
      context.beginPath();
      context.ellipse(x, y, rx, ry, 0, start, end);
      context.strokeStyle = color;
      context.lineWidth = lineWidth;
      context.stroke();
    }
    context.restore();
  }
  context.save();
  context.translate(beam.to.x, beam.to.y);
  context.beginPath();
  context.arc(1, 1.5, beam.width * 0.8, 0, turn);
  context.fillStyle = rgba(0, 0, 0, 0.3 * beam.alpha);
  context.fill();
  context.beginPath();
  context.arc(0, 0, beam.width * 0.8, 0, turn);
  const gradient = context.createRadialGradient(-beam.width * 0.3, -beam.width * 0.3, 0, 0, 0, beam.width * 0.8);
  gradient.addColorStop(0, rgba(168, 138, 116, 0.95 * beam.alpha));
  gradient.addColorStop(1, rgba(90, 68, 54, 0.95 * beam.alpha));
  context.fillStyle = gradient;
  context.fill();
  context.strokeStyle = rgba(60, 45, 36, 0.9 * beam.alpha);
  context.lineWidth = 1.5;
  context.stroke();
  context.restore();
}

function paintFlow(context: CanvasRenderingContext2D, beam: BeamEffect, options: Options) {
  const moon = options.ability === "moongeistBeam";
  const now = options.now ?? Date.now();
  const time = moon ? now / 100 : now / 150 % 1000;
  const distance = Math.hypot(beam.extendedTo.x - beam.from.x, beam.extendedTo.y - beam.from.y);
  const angle = Math.atan2(beam.extendedTo.y - beam.from.y, beam.extendedTo.x - beam.from.x);
  const point = (length: number, offset: number): Point => ({ x: beam.from.x + Math.cos(angle) * length - Math.sin(angle) * offset, y: beam.from.y + Math.sin(angle) * length + Math.cos(angle) * offset });
  context.lineCap = "round";
  context.lineJoin = "round";
  context.beginPath();
  context.strokeStyle = moon ? rgba(100, 30, 180, 0.35 * beam.alpha) : rgba(30, 120, 255, 0.5 * beam.alpha);
  context.lineWidth = beam.width * (moon ? 2.2 : 1.5) * beam.alpha;
  strokeLine(context, beam.from, beam.extendedTo);
  if (moon) {
    const waves = [(length: number) => Math.sin(length * 0.09 - time * 1.5) * 5 + Math.sin(length * 0.035 + time * 0.8) * 4, (length: number) => Math.sin(length * 0.12 - time * 2) * 2.5];
    for (let index = 0; index < 2; index++) {
      if (index === 1) context.globalCompositeOperation = "lighter";
      context.beginPath();
      context.strokeStyle = index === 0 ? rgba(150, 60, 220, 0.75 * beam.alpha) : rgba(235, 190, 255, 0.8 * beam.alpha);
      context.lineWidth = index === 0 ? beam.width * 0.9 * beam.alpha : Math.max(2, beam.width * 0.3) * beam.alpha;
      for (let length = 0; length <= distance; length += 12) {
        const vertex = point(length, waves[index](length));
        if (length === 0) context.moveTo(vertex.x, vertex.y);
        else context.lineTo(vertex.x, vertex.y);
      }
      context.stroke();
    }
    if (options.particles !== false) for (let length = 0; length < distance; length += 48) {
      const vertex = point((length + time * 18) % (distance + 25), Math.sin(time * 0.05 + length) * 12);
      context.beginPath();
      context.fillStyle = rgba(245, 220, 255, 0.7 * beam.alpha);
      context.arc(vertex.x, vertex.y, (1.5 + Math.sin(time * 0.1 + length) * 1) * beam.alpha, 0, turn);
      context.fill();
    }
  } else {
    for (let index = 0; index < 2; index++) {
      if (index === 1) context.globalCompositeOperation = "lighter";
      context.beginPath();
      context.strokeStyle = index === 0 ? rgba(100, 200, 255, 0.7 * (beam.alpha + 0.3)) : rgba(255, 255, 255, 0.6 * (beam.alpha + 0.3));
      context.lineWidth = (index === 0 ? 6 : 3) * beam.alpha;
      for (let length = 0; length <= distance; length += 15) {
        const wave = Math.sin(length * 0.05 - time * 1.5 + (index === 0 ? 0 : Math.PI)) * (12 + length * 0.02);
        const vertex = point(length, wave);
        if (length === 0) context.moveTo(vertex.x, vertex.y);
        else context.lineTo(vertex.x, vertex.y);
      }
      context.stroke();
    }
    context.beginPath();
    context.setLineDash([10, 40]);
    context.lineDashOffset = -time * 30;
    context.strokeStyle = rgba(255, 255, 255, 0.4 * (beam.alpha + 0.3));
    context.lineWidth = 2;
    strokeLine(context, beam.from, beam.extendedTo);
    context.setLineDash([]);
  }
  context.globalCompositeOperation = "source-over";
}

export function paintBeamEffect(context: CanvasRenderingContext2D, beam: BeamEffect, options: Options): boolean {
  const standard = ["solarBeam", "zapCannon", "hyperBeam", "meteorBeam", "iceBeam", "steelBeam", "machBolt", "steamEruption", "soulHeartReactor"].includes(options.ability) || options.item === "revysBook";
  if (!standard && !["psybeam", "hydroCannon", "moongeistBeam", "anchorShot", "signalBeam", "cryogenicCannon"].includes(options.ability)) return false;
  context.save();
  context.globalAlpha *= options.opacity ?? 1;
  context.globalCompositeOperation = "lighter";
  context.lineCap = "round";
  if (options.ability === "anchorShot" && !standard) paintAnchor(context, beam);
  else if (["hydroCannon", "moongeistBeam"].includes(options.ability) && !standard) paintFlow(context, beam, options);
  else {
    const hex = (options.color || "#ffcc00").replace("#", "");
    const red = Number.parseInt(hex.slice(0, 2), 16) || 255, green = Number.parseInt(hex.slice(2, 4), 16) || 204, blue = Number.parseInt(hex.slice(4, 6), 16) || 0;
    const hue = ((options.now ?? Date.now()) / 300 * 40) % 360;
    const currentWidth = beam.width * (standard ? 0.5 + beam.alpha * 0.5 : 0.6 + beam.alpha * 0.4);
    const colors = standard ? [rgba(red, green, blue, 0.9 * beam.alpha), rgba(red, green, blue, 0.35 * beam.alpha), rgba(red, green, blue, 0.95 * beam.alpha), rgba(255, 255, 255, 0.9 * beam.alpha)] : options.ability === "psybeam" ? [rgba(230, 100, 255, 0.8 * beam.alpha), rgba(200, 80, 255, 0.35 * beam.alpha), rgba(230, 120, 255, 0.9 * beam.alpha), rgba(255, 220, 255, 0.9 * beam.alpha)] : options.ability === "signalBeam" ? [`hsla(${hue},100%,65%,${0.8 * beam.alpha})`, `hsla(${hue},100%,60%,${0.3 * beam.alpha})`, `hsla(${hue},100%,55%,${0.9 * beam.alpha})`, rgba(255, 255, 255, 0.85 * beam.alpha)] : [rgba(80, 190, 255, 0.9 * beam.alpha), rgba(70, 170, 255, 0.3 * beam.alpha), rgba(40, 140, 255, 0.9 * beam.alpha), rgba(220, 250, 255, 0.9 * beam.alpha)];
    context.shadowColor = colors[0];
    for (const [index, multiplier] of [standard ? 2.5 : 2.2, 1, standard ? 0.4 : 0.35].entries()) {
      if (index === 1) context.shadowBlur = 0;
      context.strokeStyle = colors[index + 1];
      context.lineWidth = currentWidth * multiplier;
      strokeLine(context, beam.from, options.ability === "signalBeam" || options.ability === "cryogenicCannon" ? beam.to : beam.extendedTo);
    }
  }
  context.restore();
  return true;
}

export function paintLinkEffect(context: CanvasRenderingContext2D, beam: Pick<BeamEffect, "from" | "to" | "width">, options: { opacity?: number; now?: number } = {}): boolean {
  const now = options.now ?? Date.now();
  const opacity = options.opacity ?? 1;
  const shift = (Math.sin(now * 0.003) + 1) / 2;
  context.save();
  context.lineCap = "round";
  context.lineJoin = "round";
  const gradient = context.createLinearGradient(beam.from.x, beam.from.y, beam.to.x, beam.to.y);
  gradient.addColorStop(Math.max(0, shift - 0.4), "rgba(255,107,77,0.6)");
  gradient.addColorStop(Math.min(1, shift + 0.4), "rgba(77,178,255,0.6)");
  context.shadowColor = shift > 0.5 ? "rgba(77,178,255,0.4)" : "rgba(255,107,77,0.4)";
  const angle = Math.atan2(beam.to.y - beam.from.y, beam.to.x - beam.from.x) + Math.PI / 2;
  for (let layer = 0; layer < 3; layer++) {
    if (layer === 1) context.shadowBlur = 0;
    context.strokeStyle = layer === 2 ? "rgba(255,255,255,0.7)" : gradient;
    context.lineWidth = layer === 0 ? beam.width * 1.5 : layer === 1 ? beam.width * 0.8 : Math.max(1, beam.width * 0.25);
    context.globalAlpha = (layer === 0 ? 0.3 : 0.8) * opacity;
    context.beginPath();
    context.moveTo(beam.from.x, beam.from.y);
    for (let index = 1; index < 8; index++) {
      const portion = index / 8;
      const offset = Math.sin((now + layer * 50) * 0.008 + index) * (3 - layer);
      context.lineTo(beam.from.x + (beam.to.x - beam.from.x) * portion + Math.cos(angle) * offset, beam.from.y + (beam.to.y - beam.from.y) * portion + Math.sin(angle) * offset);
    }
    context.lineTo(beam.to.x, beam.to.y);
    context.stroke();
  }
  context.restore();
  return true;
}
