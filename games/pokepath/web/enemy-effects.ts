export type VisualCondition = { kind: string; durationMs: number; remainingMs: number };
export type EnemyStatusPresentation = { x: number; y: number; width: number; height: number; conditions: VisualCondition[] };
type StatusOptions = { durationCC?: boolean; particles?: boolean };
const controls = {
  slow: { frame: "rgba(4, 14, 18, 0.78)", track: "rgba(16, 47, 57, 0.92)", fill: "rgba(86, 226, 255, 0.95)", text: "#d6fbff" },
  stun: { frame: "rgba(18, 16, 4, 0.78)", track: "rgba(57, 48, 16, 0.92)", fill: "rgba(255, 230, 86, 0.95)", text: "#fff9d6" }
};

function paintControl(context: CanvasRenderingContext2D, enemy: EnemyStatusPresentation, kind: "slow" | "stun") {
  const effects = enemy.conditions.filter(value => value.kind === kind && value.durationMs > 0);
  if (!effects.length) return;
  const remaining = Math.max(...effects.map(value => Math.max(0, value.durationMs / 1000 - (value.durationMs - value.remainingMs) / 1000)));
  if (remaining <= 0) return;
  const duration = Math.max(...effects.map(value => value.durationMs / 1000 || remaining));
  const fraction = Math.max(0, Math.min(1, remaining / Math.max(0.1, duration)));
  const palette = controls[kind];
  const x = enemy.x - 14;
  const y = enemy.y - enemy.height / 2 + enemy.height - 3;
  context.save();
  context.fillStyle = palette.frame;
  context.strokeStyle = "rgba(0, 0, 0, 0.65)";
  context.lineWidth = 1;
  context.fillRect(x - 1, y - 1, 30, 10);
  context.strokeRect(x - 1, y - 1, 30, 10);
  context.fillStyle = palette.track;
  context.fillRect(x, y, 28, 8);
  context.fillStyle = palette.fill;
  context.fillRect(x, y, 28 * fraction, 8);
  context.font = "10px monospace";
  context.textAlign = "center";
  context.textBaseline = "middle";
  context.lineWidth = 2;
  context.strokeStyle = "rgba(0, 0, 0, 0.85)";
  const label = `${remaining.toFixed(1)}s`;
  context.strokeText(label, enemy.x, y + 5);
  context.fillStyle = palette.text;
  context.fillText(label, enemy.x, y + 5);
  context.restore();
}

export function paintEnemyStatus(context: CanvasRenderingContext2D, enemy: EnemyStatusPresentation, now: number, options: StatusOptions = {}) {
  if (!enemy.conditions.length) return;
  if (options.durationCC) { paintControl(context, enemy, "slow"); paintControl(context, enemy, "stun"); }
  if (!options.particles) return;
  const palettes: Record<string, number[]> = { burn: [255, 80, 0], scorch: [135, 21, 15], poison: [180, 50, 255], nightmare: [80, 20, 120] };
  for (const [index, condition] of enemy.conditions.entries()) {
    const palette = palettes[condition.kind];
    if (!palette) continue;
    const offset = (index - (enemy.conditions.length - 1) / 2) * (enemy.width * 0.15);
    for (let particle = 0; particle < 6; particle++) {
      const progress = ((now + particle * 400) % 1500) / 1500;
      const y = (enemy.y + enemy.height * 0.2) - (progress * enemy.height * 1.6);
      const x = enemy.x + offset + Math.sin(now / 400 + particle) * (enemy.width * 0.12);
      const color = `rgba(${palette.join(",")},${(1 - progress) * 0.9})`;
      context.save();
      context.beginPath();
      context.fillStyle = color;
      context.shadowBlur = 5;
      context.shadowColor = color;
      context.arc(x, y, (1 - progress) * 3.5, 0, Math.PI * 2);
      context.fill();
      context.restore();
    }
  }
}

export function paintDamageText(context: CanvasRenderingContext2D, text: { x: number; y: number; text: string; ageMS: number; color?: string }) {
  if (text.ageMS < 0 || text.ageMS >= 1000) return;
  const y = text.y - 60 * text.ageMS / 1000;
  context.save();
  context.globalAlpha = Math.max(0.5, 1 - 0.4 * text.ageMS / 1000);
  context.font = "900 14px Arial";
  context.lineWidth = 1.5;
  context.textAlign = "center";
  context.textBaseline = "middle";
  context.strokeStyle = "black";
  context.strokeText(text.text, text.x, y);
  context.fillStyle = text.color || "#c96937";
  context.fillText(text.text, text.x, y);
  context.restore();
}
