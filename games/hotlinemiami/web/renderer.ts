import type { LoadedAssets } from "./assets";
import { isMeleeWeapon, playerColors, type GameEvent, type Player, type View } from "./types.ts";
import type { FeedbackPulse } from "./feedback.ts";

export type DrawPlayer = Pick<Player, "x" | "y" | "aim" | "life" | "alive">;
export type Camera = { x: number; y: number; width: number; height: number };

export function cameraForPlayer(focus: { x: number; y: number } | undefined, mapWidth: number, mapHeight: number): Camera {
  const width = Math.min(384, mapWidth), height = width * 5 / 8;
  const x = focus ? focus.x / 256 : mapWidth / 2;
  const y = focus ? focus.y / 256 : mapHeight / 2;
  return {
    x: Math.max(0, Math.min(mapWidth - width, x - width / 2)),
    y: Math.max(0, Math.min(mapHeight - height, y - height / 2)),
    width,
    height,
  };
}

export function pointerWorldPoint(x: number, y: number, camera: Camera) {
  return { x: camera.x + x * camera.width, y: camera.y + y * camera.height };
}

type Mark = { x: number; y: number; frame: number; kind: string; seat: number; at: number };

export class BattleRenderer {
  private canvas: HTMLCanvasElement;
  private context: CanvasRenderingContext2D;
  private assets: LoadedAssets;
  private marks: Mark[] = [];
  private flashes = new Map<number, number>();
  private labelSize = 8;
  private camera: Camera = { x: 0, y: 0, width: 384, height: 240 };
  private reduced = matchMedia("(prefers-reduced-motion: reduce)").matches;

  constructor(canvas: HTMLCanvasElement, assets: LoadedAssets) {
    const context = canvas.getContext("2d", { alpha: false });
    if (!context) throw Error("浏览器未能建立游戏画布");
    this.canvas = canvas;
    this.context = context;
    this.assets = assets;
  }

  get viewCamera() {
    return this.camera;
  }

  pointFromPointer(x: number, y: number, focus: { x: number; y: number }) {
    return pointerWorldPoint(x, y, cameraForPlayer(focus, this.assets.rules.width / 256, this.assets.rules.height / 256));
  }

  ingest(events: GameEvent[]) {
    for (const event of events) {
      if (event.type === "fire") this.flashes.set(event.seat, performance.now() + 70);
      if ((event.type === "kill" || event.type === "down" || event.type === "hit") && event.x !== undefined && event.y !== undefined) {
        this.marks.push({ x: event.x / 256, y: event.y / 256, frame: event.frame, kind: event.type, seat: event.seat, at: performance.now() });
      }
    }
    if (this.marks.length > 96) this.marks.splice(0, this.marks.length - 96);
  }

  draw(view: View, local: DrawPlayer | undefined, previous: View | undefined, blend: number, seat: number, names: string[], localFlash: number, at: number, aimPoint?: { x: number; y: number }, pulses: FeedbackPulse[] = []) {
    const canvas = this.canvas, ctx = this.context;
    const rect = canvas.getBoundingClientRect();
    const ratio = Math.min(2, devicePixelRatio || 1);
    const width = Math.max(1, Math.round(rect.width * ratio));
    const height = Math.max(1, Math.round(rect.height * ratio));
    if (canvas.width !== width || canvas.height !== height) {
      canvas.width = width;
      canvas.height = height;
    }
    const mapWidth = this.assets.rules.width / 256;
    const mapHeight = this.assets.rules.height / 256;
    const focus = local ?? view.players.find(player => player.alive) ?? view.players[0];
    this.camera = cameraForPlayer(focus, mapWidth, mapHeight);
    const worldWidth = this.camera.width, worldHeight = this.camera.height;
    this.labelSize = Math.max(8, 10 * worldWidth / Math.max(1, rect.width));
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.fillStyle = "#15131f";
    ctx.fillRect(0, 0, width, height);
    ctx.setTransform(width / worldWidth, 0, 0, height / worldHeight, 0, 0);
    ctx.translate(-this.camera.x, -this.camera.y);
    ctx.imageSmoothingEnabled = false;
    const background = this.assets.images.get(this.assets.map.image);
    if (background) ctx.drawImage(background, 0, 0, mapWidth, mapHeight);
    const broken = new Set(view.brokenGlass ?? []);
    for (const [index, wall] of this.assets.rules.walls.entries()) {
      if (wall.kind !== "glass" || broken.has(index)) continue;
      ctx.fillStyle = "#9de4e452";
      ctx.strokeStyle = "#b4e3df";
      ctx.lineWidth = 0.8;
      ctx.fillRect(wall.x / 256, wall.y / 256, wall.w / 256, wall.h / 256);
      ctx.strokeRect(wall.x / 256 + 1, wall.y / 256 + 1, Math.max(0, wall.w / 256 - 2), Math.max(0, wall.h / 256 - 2));
    }
    this.marks = this.marks.filter(mark => at - mark.at < 30000);
    for (const mark of this.marks) {
      ctx.fillStyle = mark.kind === "kill" ? "#7d203cd9" : "#a43b59a8";
      ctx.beginPath();
      ctx.ellipse(mark.x, mark.y, 10, 6, (mark.x % 5) / 2, 0, Math.PI * 2);
      ctx.fill();
    }
    for (const corpse of view.corpses ?? []) {
      const character = this.assets.manifest.characters.find(candidate => candidate.id === corpse.character);
      if (!character) continue;
      ctx.save();
      ctx.globalAlpha = 0.55;
      this.sprite(character.dead ?? character.sprite, corpse.x / 256, corpse.y / 256, corpse.aim + 256, 0, 1);
      if (character.overlay) this.sprite(character.overlay, corpse.x / 256, corpse.y / 256, corpse.aim + 256, character.overlayFrame ?? 0, 1);
      ctx.restore();
    }
    for (const drop of view.drops) {
      const x = drop.x / 256, y = drop.y / 256;
      ctx.fillStyle = "#f7e4a744";
      ctx.beginPath();
      ctx.ellipse(x, y, 10, 5, 0, 0, Math.PI * 2);
      ctx.fill();
      const weapon = this.assets.manifest.weapons.find(candidate => candidate.id === drop.weapon);
      if (weapon) this.sprite(weapon.sprite, x, y, 0, 0, 0.3);
    }
    for (const door of view.doors) {
      const shape = this.assets.rules.doors.find(candidate => candidate.id === door.id);
      if (!shape) continue;
      ctx.save();
      ctx.translate(shape.x / 256, shape.y / 256);
      if (door.open) ctx.rotate(Math.PI / 2);
      ctx.fillStyle = "#bca07e";
      ctx.strokeStyle = "#413337";
      ctx.lineWidth = 2;
      ctx.fillRect(0, 0, shape.w / 256, shape.h / 256);
      ctx.strokeRect(0, 0, shape.w / 256, shape.h / 256);
      ctx.restore();
    }
    for (const player of view.players) {
      if (!player.alive && view.corpses?.some(corpse => corpse.x === player.x && corpse.y === player.y)) continue;
      const old = previous?.players.find(candidate => candidate.seat === player.seat && candidate.life === player.life);
      const displayed = player.seat === seat && local ? local : {
        ...player,
        x: old && old.alive === player.alive ? old.x + (player.x - old.x) * blend : player.x,
        y: old && old.alive === player.alive ? old.y + (player.y - old.y) * blend : player.y,
      };
      const remoteFlash = player.seat === seat ? 0 : this.flashes.get(player.seat) ?? 0;
      const moving = old ? old.x !== player.x || old.y !== player.y : player.seat === seat && local ? local.x !== player.x || local.y !== player.y : false;
      this.player(player, displayed, view.frame, names[player.seat] ?? "", at, player.seat === seat ? localFlash : remoteFlash, moving);
    }
    ctx.strokeStyle = "#fff3c7";
    ctx.lineWidth = 1.4;
    ctx.lineCap = "round";
    for (const bullet of view.bullets) {
      const angle = bullet.dx !== undefined && bullet.dy !== undefined ? Math.atan2(bullet.dy, bullet.dx) : (bullet.aim ?? 0) * Math.PI * 2 / 1024;
      const x = bullet.x / 256, y = bullet.y / 256;
      if (bullet.thrown) {
        const weapon = this.assets.manifest.weapons.find(candidate => candidate.id === bullet.weapon);
        if (weapon) this.sprite(weapon.sprite, x, y, Math.trunc(at) % 1024, 0, 0.3);
        continue;
      }
      ctx.beginPath();
      ctx.moveTo(x - Math.cos(angle) * 5, y - Math.sin(angle) * 5);
      ctx.lineTo(x + Math.cos(angle) * 2, y + Math.sin(angle) * 2);
      ctx.stroke();
    }
    if (local?.alive) this.feedback(pulses, local, view, at);
    for (const mark of this.marks) {
      if (at - mark.at > 100) continue;
      ctx.strokeStyle = "#fff0c7";
      ctx.lineWidth = 1.8;
      ctx.beginPath();
      ctx.moveTo(mark.x - 7, mark.y - 5);
      ctx.lineTo(mark.x + 7, mark.y + 5);
      ctx.moveTo(mark.x - 6, mark.y + 6);
      ctx.lineTo(mark.x + 6, mark.y - 6);
      ctx.stroke();
    }
    if (aimPoint && local?.alive) {
      const target = pointerWorldPoint(aimPoint.x, aimPoint.y, this.camera);
      ctx.strokeStyle = "#f8f1df";
      ctx.lineWidth = 0.7;
      ctx.beginPath();
      ctx.arc(target.x, target.y, 4, 0, Math.PI * 2);
      ctx.moveTo(target.x - 7, target.y);
      ctx.lineTo(target.x - 3, target.y);
      ctx.moveTo(target.x + 3, target.y);
      ctx.lineTo(target.x + 7, target.y);
      ctx.moveTo(target.x, target.y - 7);
      ctx.lineTo(target.x, target.y - 3);
      ctx.moveTo(target.x, target.y + 3);
      ctx.lineTo(target.x, target.y + 7);
      ctx.stroke();
    }
    return {
      screenX: local ? (local.x / 256 - this.camera.x) * rect.width / worldWidth : 0,
      screenY: local ? (local.y / 256 - this.camera.y) * rect.height / worldHeight : 0,
    };
  }

  private feedback(pulses: FeedbackPulse[], local: DrawPlayer, view: View, at: number) {
    const ctx = this.context;
    const x = local.x / 256, y = local.y / 256;
    const angle = local.aim * Math.PI * 2 / 1024;
    for (const pulse of pulses) {
      const progress = Math.max(0, Math.min(1, (at - pulse.at) / 150));
      ctx.save();
      ctx.globalAlpha = 1 - progress * 0.7;
      ctx.lineWidth = 1.5;
      if (pulse.kind === "pickup") {
        let target = { x, y }, distance = 64;
        for (const drop of view.drops) {
          const delta = Math.hypot(drop.x / 256 - x, drop.y / 256 - y);
          if (delta < distance) { target = { x: drop.x / 256, y: drop.y / 256 }; distance = delta; }
        }
        ctx.strokeStyle = "#ffe29b";
        ctx.beginPath();
        ctx.arc(target.x, target.y, 10 + progress * 7, 0, Math.PI * 2);
        ctx.stroke();
        ctx.beginPath();
        ctx.moveTo(x, y);
        ctx.lineTo(target.x, target.y);
        ctx.stroke();
      } else if (pulse.kind === "interact") {
        ctx.strokeStyle = "#9de4df";
        let target = { x: x + Math.cos(angle) * 22, y: y + Math.sin(angle) * 22 }, distance = 56;
        for (const door of this.assets.rules.doors) {
          const point = { x: (door.x + door.w / 2) / 256, y: (door.y + door.h / 2) / 256 };
          const delta = Math.hypot(point.x - x, point.y - y);
          if (delta < distance) { target = point; distance = delta; }
        }
        ctx.beginPath();
        ctx.moveTo(x + Math.cos(angle) * 8, y + Math.sin(angle) * 8);
        ctx.lineTo(target.x, target.y);
        ctx.stroke();
        ctx.strokeRect(target.x - 5, target.y - 5, 10, 10);
      } else {
        ctx.translate(x, y);
        ctx.rotate(angle);
        ctx.strokeStyle = pulse.kind === "throw" ? "#ffd0ab" : "#f7a1c5";
        ctx.beginPath();
        if (pulse.kind === "throw") {
          ctx.arc(0, 0, 15 + progress * 12, -0.5, 0.5);
          ctx.moveTo(10, 0);
          ctx.lineTo(22 + progress * 12, 0);
        } else {
          ctx.arc(16, 0, 10, -Math.PI / 2, Math.PI / 2);
          ctx.moveTo(8, -8 + progress * 16);
          ctx.lineTo(25, 8 - progress * 16);
        }
        ctx.stroke();
      }
      ctx.restore();
    }
  }

  private player(player: Player, displayed: DrawPlayer, frame: number, name: string, at: number, flash: number, moving: boolean) {
    const ctx = this.context;
    const x = displayed.x / 256, y = displayed.y / 256;
    const color = playerColors[player.seat % playerColors.length];
    const character = this.assets.manifest.characters.find(candidate => candidate.id === player.character);
    if (!character) return;
    ctx.save();
    if (!player.alive) {
      ctx.globalAlpha = 0.55;
      this.sprite(character.dead ?? character.sprite, x, y, displayed.aim + 256, 0, 1);
      ctx.restore();
      return;
    }
    ctx.strokeStyle = color;
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.ellipse(x, y + 3, 15, 9, 0, 0, Math.PI * 2);
    ctx.stroke();
    if (player.protectedUntil > frame) {
      ctx.strokeStyle = `${color}c9`;
      ctx.setLineDash([3, 3]);
      ctx.beginPath();
      ctx.arc(x, y, 17, 0, Math.PI * 2);
      ctx.stroke();
      ctx.setLineDash([]);
    }
    const down = player.downUntil > frame;
    const busy = (player.busyUntil ?? 0) > frame;
    const pose = character.poses?.[player.weapon] ?? character.sprite;
    this.sprite(pose, x, y, down || busy ? displayed.aim + 256 : displayed.aim, this.reduced || !moving ? 0 : Math.trunc(at / 110), 1);
    if (character.overlay) this.sprite(character.overlay, x, y, displayed.aim, character.overlayFrame ?? 0, 1);
    if (!character.poses?.[player.weapon] && player.weapon && !down && !busy) {
      const weapon = this.assets.manifest.weapons.find(candidate => candidate.id === player.weapon);
      const angle = displayed.aim * Math.PI * 2 / 1024;
      if (weapon) this.sprite(weapon.sprite, x + Math.cos(angle) * 14, y + Math.sin(angle) * 14, displayed.aim, 0, 0.25);
    }
    if (flash > at) {
      const angle = displayed.aim * Math.PI * 2 / 1024;
      const length = isMeleeWeapon(player.weapon) ? 24 : 18;
      ctx.strokeStyle = "#ffe9a2";
      ctx.lineWidth = 3;
      ctx.beginPath();
      if (!player.weapon) ctx.arc(x + Math.cos(angle) * 14, y + Math.sin(angle) * 14, 8, angle - Math.PI / 2, angle + Math.PI / 2);
      else {
        ctx.moveTo(x + Math.cos(angle) * 9, y + Math.sin(angle) * 9);
        ctx.lineTo(x + Math.cos(angle) * length, y + Math.sin(angle) * length);
      }
      ctx.stroke();
    }
    ctx.textAlign = "center";
    ctx.font = `bold ${this.labelSize}px system-ui, sans-serif`;
    ctx.lineWidth = 3;
    ctx.strokeStyle = "#17141f";
    ctx.fillStyle = color;
    const shortName = name.length > 12 ? name.slice(0, 11) + "…" : name;
    ctx.strokeText(shortName, x, y - 20);
    ctx.fillText(shortName, x, y - 20);
    ctx.restore();
  }

  private sprite(key: string, x: number, y: number, aim: number, frame: number, scale: number) {
    const sprite = this.assets.manifest.sprites[key];
    const image = sprite && this.assets.images.get(sprite.image);
    if (!sprite?.frames.length || !image) return;
    const clip = sprite.frames[frame % sprite.frames.length];
    const ctx = this.context;
    ctx.save();
    ctx.translate(x, y);
    ctx.rotate(aim * Math.PI * 2 / 1024);
    ctx.scale(scale, scale);
    ctx.drawImage(image, clip.x, clip.y, clip.w, clip.h, -(clip.originX ?? clip.w / 2), -(clip.originY ?? clip.h / 2), clip.w, clip.h);
    ctx.restore();
  }
}
