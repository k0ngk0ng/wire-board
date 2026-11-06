import type { Art } from "./art";
import { deviceFrames } from "./art";
import {
  playerColors,
  roomImages,
  type Device,
  type Player,
  type Point,
  type Room,
  type View,
} from "./types";
export type Frame = {
  view: View;
  room: Room;
  art: Art;
  positions: Map<number, Point>;
  you: number;
  names: string[];
  now: number;
  selected: string;
  hovered: string;
  reduced: boolean;
  camera: number;
  width: number;
  input?: { dx: number; dy: number };
};
const H = 666.667;
function glow(
  c: CanvasRenderingContext2D,
  x: number,
  y: number,
  r: number,
  color: string,
  alpha: number,
) {
  c.save();
  c.globalCompositeOperation = "screen";
  c.globalAlpha = alpha;
  const g = c.createRadialGradient(x, y, 0, x, y, r);
  g.addColorStop(0, color);
  g.addColorStop(1, "transparent");
  c.fillStyle = g;
  c.fillRect(x - r, y - r, r * 2, r * 2);
  c.restore();
}
function sprite(
  c: CanvasRenderingContext2D,
  art: Art,
  index: number,
  x: number,
  y: number,
  size: number,
) {
  const im = art.devices;
  c.drawImage(
    im,
    (index % 4) * 192,
    Math.floor(index / 4) * 192,
    192,
    192,
    x - size / 2,
    y - size,
    size,
    size,
  );
}
function ellipse(
  c: CanvasRenderingContext2D,
  x: number,
  y: number,
  w: number,
  h: number,
  color: string,
) {
  c.fillStyle = color;
  c.beginPath();
  c.ellipse(x, y, w, h, 0, 0, Math.PI * 2);
  c.fill();
}
function device(c: CanvasRenderingContext2D, f: Frame, d: Device) {
  const active = d.done || (!!d.heldUntil && d.heldUntil > f.view.tick);
  const selected = f.selected === d.id || f.hovered === d.id;
  const isPaper = d.kind.startsWith("clue") || d.kind === "briefing";
  const size = isPaper ? 94 : d.kind === "extract" ? 160 : 126;
  ellipse(c, d.x, d.y + 1, size * 0.3, 10, "#071d1aa0");
  if (selected || !d.done) {
    ellipse(
      c,
      d.x,
      d.y + 3,
      size * 0.35,
      9,
      selected ? "#e7bf6460" : "#c4ab5530",
    );
  }
  let index = deviceFrames[d.kind] ?? 6;
  if (d.kind === "power" && f.view.options.scenario === "archive") index = 8;
  sprite(c, f.art, index, d.x, d.y, size);
  if (active && !isPaper) {
    glow(c, d.x, d.y - size * 0.55, 48, "#81e9bc", 0.14);
    c.fillStyle = "#a8e9b9";
    c.beginPath();
    c.arc(d.x + size * 0.23, d.y - size * 0.74, 3, 0, Math.PI * 2);
    c.fill();
  }
  if (selected) {
    c.save();
    c.font = "500 14px system-ui";
    c.textAlign = "center";
    const w = c.measureText(d.label).width + 24;
    c.fillStyle = "#092321ec";
    c.strokeStyle = "#c5a970";
    c.lineWidth = 1;
    c.beginPath();
    c.roundRect(d.x - w / 2, d.y - size - 29, w, 25, 5);
    c.fill();
    c.stroke();
    c.fillStyle = "#f5e9c9";
    c.fillText(d.label, d.x, d.y - size - 12);
    c.restore();
  }
  if (d.heldUntil && d.heldUntil > f.view.tick && !f.view.stats.route) {
    const left = (d.heldUntil - f.view.tick) / 12000;
    c.fillStyle = "#152b27";
    c.fillRect(d.x - 26, d.y + 10, 52, 4);
    c.fillStyle = "#9cd9b1";
    c.fillRect(d.x - 26, d.y + 10, 52 * left, 4);
  }
}
function person(c: CanvasRenderingContext2D, f: Frame, p: Player) {
  const point = f.positions.get(p.seat) ?? p;
  const intent = p.seat === f.you ? f.input : undefined;
  const moving =
    Math.hypot(intent?.dx ?? p.dx, intent?.dy ?? p.dy) > 0 || !!p.goal;
  const walk = f.reduced
    ? 0
    : Math.sin(f.now / 85 + p.seat) * 1.5 * (moving ? 1 : 0.15);
  let facing = p.facing;
  if (intent && (intent.dx || intent.dy))
    facing =
      Math.abs(intent.dx) > Math.abs(intent.dy)
        ? intent.dx > 0
          ? "right"
          : "left"
        : intent.dy > 0
          ? "down"
          : "up";
  const dir = ["down", "up", "left", "right"].indexOf(facing);
  const height = 118,
    width = 78.667;
  ellipse(c, point.x, point.y + 1, 22, 8, "#061c1ac0");
  if (p.seat === f.you) {
    c.strokeStyle = "#e8d7a4";
    c.lineWidth = 1.5;
    c.setLineDash([3, 4]);
    c.beginPath();
    c.ellipse(point.x, point.y + 2, 25, 10, 0, 0, Math.PI * 2);
    c.stroke();
    c.setLineDash([]);
  }
  c.save();
  c.translate(point.x, point.y);
  if (p.hp <= 0) {
    c.rotate(-Math.PI / 2);
    c.globalAlpha = 0.85;
  } else if (moving && !f.reduced) c.rotate(Math.sin(f.now / 115) * 0.018);
  const sheet = moving && !f.reduced ? f.art["walk-" + p.seat] : undefined;
  if (sheet) {
    const phase = Math.floor(f.now / 125) % 4;
    c.drawImage(
      sheet,
      phase * 96,
      Math.max(0, dir) * 144,
      96,
      144,
      -width / 2,
      -height + walk,
      width,
      height,
    );
  } else {
    c.drawImage(
      f.art.characters,
      Math.max(0, dir) * 96,
      p.seat * 144,
      96,
      144,
      -width / 2,
      -height + walk,
      width,
      height,
    );
  }
  c.restore();
  if (p.carry) {
    sprite(c, f.art, 10, point.x + 27, point.y - 5, 35);
  }
  if (p.hp < 100) {
    c.fillStyle = "#0b2424";
    c.fillRect(point.x - 22, point.y + 13, 44, 4);
    c.fillStyle = p.hp > 35 ? "#9fceb1" : "#f3a48c";
    c.fillRect(point.x - 22, point.y + 13, (44 * p.hp) / 100, 4);
  }
  c.save();
  c.textAlign = "center";
  c.font = "500 13px system-ui";
  const name = f.names[p.seat] ?? "";
  if (name) {
    const w = c.measureText(name).width + 16;
    c.fillStyle = "#071f22d9";
    c.beginPath();
    c.roundRect(point.x - w / 2, point.y - height - 20, w, 21, 5);
    c.fill();
    c.fillStyle = playerColors[p.seat];
    c.fillText(name, point.x, point.y - height - 5);
  }
  c.restore();
  if (p.progress) {
    c.strokeStyle = "#f1d38c";
    c.lineWidth = 4;
    c.beginPath();
    c.arc(
      point.x,
      point.y - 55,
      22,
      -Math.PI / 2,
      -Math.PI / 2 + p.progress * Math.PI * 2,
    );
    c.stroke();
  }
}
export function draw(
  c: CanvasRenderingContext2D,
  canvas: HTMLCanvasElement,
  f: Frame,
) {
  const ratio = canvas.width / f.width;
  c.setTransform(ratio, 0, 0, ratio, 0, 0);
  c.clearRect(0, 0, f.width, H);
  c.save();
  c.translate(-f.camera, 0);
  c.drawImage(
    f.art[roomImages(f.view.options.scenario, f.room.kind)],
    0,
    0,
    1000,
    H,
  );
  // Localized light and quiet particles keep the handcrafted room legible.
  const pulse = f.reduced ? 1 : 0.94 + Math.sin(f.now / 1700) * 0.06;
  glow(c, 92, 330, 150, "#ffbb59", 0.12 * pulse);
  glow(c, 910, 330, 145, "#e7b65f", 0.12 * pulse);
  glow(c, 500, 170, 180, "#4f9baf", 0.07);
  for (const door of f.room.doors ?? []) {
    ellipse(c, door.x, door.y + 3, 36, 11, "#061e1ed9");
    glow(c, door.x, door.y - 45, 80, "#dcb56b", 0.13);
    sprite(
      c,
      f.art,
      f.view.options.scenario === "station" ? 12 : 13,
      door.x,
      door.y + 5,
      104,
    );
    c.fillStyle = "#efd28d";
    c.beginPath();
    c.moveTo(door.x - 5, door.y + 17);
    c.lineTo(door.x + 5, door.y + 17);
    c.lineTo(door.x, door.y + 23);
    c.fill();
  }
  const depth: (Device | Player)[] = [
    ...(f.room.objects ?? []),
    ...f.view.players.filter((p) => p.room === f.room.id),
  ];
  depth.sort(
    (a, b) =>
      (("seat" in a ? f.positions.get(a.seat)?.y : undefined) ?? a.y) -
      (("seat" in b ? f.positions.get(b.seat)?.y : undefined) ?? b.y),
  );
  for (const value of depth) {
    if ("seat" in value) person(c, f, value);
    else device(c, f, value);
  }
  const water = Math.max(
    f.view.options.scenario === "archive" ? 8 : 2,
    f.room.flood,
  );
  if (water > 4) {
    c.save();
    c.beginPath();
    c.rect(120, 277, 760, 309);
    c.clip();
    c.fillStyle = `rgba(35,127,132,${Math.min(0.3, water / 220)})`;
    c.fillRect(120, 277, 760, 309);
    c.strokeStyle = "#88c7bc";
    c.lineWidth = 0.65;
    for (let i = 0; i < 11; i++) {
      const x = 150 + ((i * 137) % 720),
        y = 310 + ((i * 59) % 250),
        t = f.reduced ? 0 : f.now / 1700 + i;
      c.globalAlpha = 0.08 + Math.sin(t) * 0.04;
      c.beginPath();
      c.ellipse(
        x + Math.sin(t) * 8,
        y,
        12 + (Math.sin(t) + 1) * 9,
        3,
        0,
        0,
        Math.PI * 2,
      );
      c.stroke();
    }
    c.restore();
  }
  if (!f.reduced) {
    for (let i = 0; i < 32; i++) {
      const x = (i * 97 + (f.now / 160) * ((i % 3) + 1)) % 1000,
        y = (i * 53 + Math.sin(f.now / 5000 + i) * 11) % 620;
      c.fillStyle = `rgba(235,211,155,${0.08 + (i % 4) * 0.025})`;
      c.fillRect(x, y, i % 3 === 0 ? 2 : 1, 1);
    }
  }
  if (f.view.power < 25 || !f.view.circuits?.includes("lights")) {
    const me = f.positions.get(f.you) ?? { x: 500, y: 420 };
    const g = c.createRadialGradient(me.x, me.y - 30, 30, me.x, me.y - 30, 430);
    g.addColorStop(0, "#02141500");
    g.addColorStop(0.45, "#02141538");
    g.addColorStop(1, "#021415b0");
    c.fillStyle = g;
    c.fillRect(0, 0, 1000, H);
  }
  if (f.view.incident?.room === f.room.id) {
    const active = f.view.tick >= f.view.incident.begins;
    c.fillStyle = active ? "#d5824d16" : "#d5965510";
    c.fillRect(0, 0, 1000, H);
    if (active && !f.reduced) {
      for (let i = 0; i < 6; i++) {
        const x = (f.now / 20 + i * 161) % 1000;
        glow(
          c,
          x,
          520 - Math.sin(f.now / 1100 + i) * 30,
          130,
          "#b7c5be",
          0.045,
        );
      }
    }
  }
  const vignette = c.createRadialGradient(500, 365, 160, 500, 365, 630);
  vignette.addColorStop(0, "#001f2100");
  vignette.addColorStop(1, "#00171999");
  c.fillStyle = vignette;
  c.fillRect(0, 0, 1000, H);
  c.restore();
}
