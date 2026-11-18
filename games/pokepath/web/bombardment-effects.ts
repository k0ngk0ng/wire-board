type Point = { x: number; y: number };
type Shell = { kind: string; origin: Point; destination: Point; radius: number; ageMS: number; durationMS: number; impacted: boolean; impactMS: number; boltPoints?: Point[] };
type Aim = { origin: Point; aim: Point; radius: number; width?: number; height?: number };

export function paintBombardmentAim(context: CanvasRenderingContext2D, aim: Aim, options: { color: string; opacity?: number }): boolean {
  const opacity = options.opacity ?? 1;
  const rectangle = (aim.width ?? 0) > 0;
  const shape = (fill: boolean) => {
    if (rectangle) {
      context.save();
      context.translate(aim.aim.x, aim.aim.y);
      context.rotate(Math.atan2(aim.aim.y - aim.origin.y, aim.aim.x - aim.origin.x) + Math.PI / 2);
      const width = aim.width ?? 0, height = aim.height ?? 0;
      if (fill) context.fillRect(-width / 2, -height / 2, width, height);
      else context.strokeRect(-width / 2, -height / 2, width, height);
      context.restore();
    } else {
      context.beginPath();
      context.arc(aim.aim.x, aim.aim.y, aim.radius, 0, Math.PI * 2);
      if (fill) context.fill();
      else context.stroke();
    }
  };
  context.save();
  context.save();
  context.globalAlpha = 0.2 * opacity;
  context.fillStyle = options.color;
  shape(true);
  context.globalAlpha = opacity;
  context.strokeStyle = options.color;
  context.lineWidth = 2;
  shape(false);
  context.globalAlpha = 0.9 * opacity;
  context.beginPath();
  context.moveTo(aim.aim.x - 8, aim.aim.y - 8);
  context.lineTo(aim.aim.x + 8, aim.aim.y + 8);
  context.moveTo(aim.aim.x + 8, aim.aim.y - 8);
  context.lineTo(aim.aim.x - 8, aim.aim.y + 8);
  context.stroke();
  context.restore();
  context.restore();
  return true;
}

export function paintBombardmentEffect(context: CanvasRenderingContext2D, shell: Shell, options: { color: string; projectileOpacity?: number; areaOpacity?: number; image?: CanvasImageSource }): boolean {
  if (shell.kind !== "bombardment" && shell.kind !== "lightningArea") return false;
  context.save();
  if (shell.impacted) {
    const progress = Math.min(1, (shell.impactMS || 0) / 400);
    context.globalAlpha = Math.max(0, 0.75 * (1 - progress)) * (options.areaOpacity ?? 1);
    context.fillStyle = options.color;
    context.beginPath();
    context.arc(shell.destination.x, shell.destination.y, shell.radius * 1.4 * progress, 0, Math.PI * 2);
    context.fill();
  } else {
    const progress = shell.ageMS / shell.durationMS;
    context.globalAlpha = 0.2 * (options.projectileOpacity ?? 1);
    context.fillStyle = "#000";
    context.beginPath();
    context.arc(shell.destination.x, shell.destination.y, shell.radius, 0, Math.PI * 2);
    context.fill();
    context.globalAlpha = options.projectileOpacity ?? 1;
    if (shell.kind === "lightningArea" && shell.boltPoints?.length) {
      const points = shell.boltPoints;
      const count = Math.max(1, Math.ceil(points.length * progress));
      context.save();
      context.globalCompositeOperation = "lighter";
      for (const [color, width] of [["rgba(160, 200, 255, 0.5)", 6], ["rgba(255, 255, 255, 0.95)", 2]] as const) {
        context.strokeStyle = color;
        context.lineWidth = width;
        context.beginPath();
        context.moveTo(points[0].x, points[0].y);
        for (const point of points.slice(1, count)) context.lineTo(point.x, point.y);
        context.stroke();
      }
      context.restore();
    } else {
      const x = shell.origin.x + (shell.destination.x - shell.origin.x) * progress;
      const y = shell.origin.y + (shell.destination.y - shell.origin.y) * progress;
      context.translate(x, y);
      context.rotate(Math.atan2(shell.destination.y - shell.origin.y, shell.destination.x - shell.origin.x));
      if (options.image) context.drawImage(options.image, -shell.radius, -shell.radius, shell.radius * 2, shell.radius * 2);
      else {
        context.fillStyle = "#555";
        context.beginPath();
        context.arc(0, 0, shell.radius, 0, Math.PI * 2);
        context.fill();
      }
    }
  }
  context.restore();
  return true;
}
