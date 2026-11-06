import { drawSprite, type Art } from "./art";
import type { View } from "./types";

export const STAGE = { width: 800, height: 600, left: 40, top: 80, cellWidth: 80, cellHeight: 100, rows: 5, cols: 9 } as const;
const OWNER = ["#f4d263", "#e7a0a2", "#b9def4", "#c9b4ec"];
export type Hover = { row: number; col: number } | null;
export type PaintState = { selected: string | null; shovel: boolean; hover: Hover; actor: number; canAct: boolean; now: number; reducedMotion: boolean };

function rounded(context: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, radius: number) {
  context.beginPath(); context.roundRect(x, y, w, h, radius); context.fill();
}
function lastEvent(view: View, type: string, target: number) {
  for (let i = view.events.length - 1; i >= 0; i--) if (view.events[i].type === type && view.events[i].target === target) return view.events[i];
  return undefined;
}

export function paint(context: CanvasRenderingContext2D, view: View, art: Art, state: PaintState) {
  const { left, top, cellWidth: cw, cellHeight: ch, width, height } = STAGE;
  context.clearRect(0, 0, width, height);
  context.fillStyle = "#6e9751"; context.fillRect(0, 0, width, height);
  const background = art.images.get(art.manifest.background);
  if (background) {
    const crop = art.manifest.backgroundCrop;
    if (crop) context.drawImage(background, crop.x, crop.y, crop.width, crop.height, 0, 0, width, height);
    else context.drawImage(background, 0, 0, width, height);
  }
  // The shared board has the same cell geometry for pointer, touch and rendering.
  for (let row = 0; row < 5; row++) for (let col = 0; col < 9; col++) {
    context.strokeStyle = "rgba(47,87,37,.06)"; context.lineWidth = 1;
    context.strokeRect(left + col * cw, top + row * ch, cw, ch);
  }
  const player = view.players[state.actor];
  if (state.canAct && player?.side === "zombies") {
    for (let row = 0; row < 5; row++) {
      const active = state.hover?.row === row;
      context.fillStyle = active ? "rgba(255,224,124,.72)" : "rgba(46,43,30,.5)";
      rounded(context, 763, top + row * ch + 22, 34, ch - 44, 9);
      context.fillStyle = active ? "#5b4326" : "#f5e5b5";
      context.beginPath(); context.moveTo(770, top + row * ch + 50); context.lineTo(786, top + row * ch + 38); context.lineTo(786, top + row * ch + 62); context.closePath(); context.fill();
    }
  }
  if (state.hover && state.canAct && (state.selected || state.shovel)) {
    const { row, col } = state.hover;
    context.fillStyle = state.shovel ? "rgba(237,144,109,.38)" : "rgba(249,230,133,.35)";
    context.fillRect(left + col * cw, top + row * ch, cw, ch);
    context.strokeStyle = "#fff0aa"; context.lineWidth = 3;
    context.strokeRect(left + col * cw + 2, top + row * ch + 2, cw - 4, ch - 4);
    if (player?.side === "plants" && state.selected && !state.shovel && !view.plants.some(p => p.row === row && p.col === col)) {
      context.globalAlpha = .5;
      drawSprite(context, art, state.selected, 0, left + (col + .5) * cw, top + (row + .9) * ch);
      context.globalAlpha = 1;
    }
  }
  const time = state.reducedMotion ? 0 : view.tick * 50;
  for (const mower of view.mowers) {
    if (mower.state === "spent") continue;
    const x = mower.state === "moving" ? left + mower.x / 1000 * cw : left - 21;
    if (!drawSprite(context, art, "mower", time, x, top + (mower.row + .83) * ch)) {
      const image = art.images.get(art.manifest.images?.mower ?? "");
      if (image) context.drawImage(image, x - 35, top + mower.row * ch + 35, 70, 45);
    }
  }
  for (let row = 0; row < 5; row++) {
    for (const plant of view.plants) {
      if (plant.row !== row) continue;
      const x = left + (plant.col + .5) * cw, y = top + (row + .9) * ch;
      const shadow = art.images.get(art.manifest.images?.shadow ?? "");
      if (shadow) context.drawImage(shadow, x - shadow.naturalWidth / 2, y - shadow.naturalHeight / 2);
      const own = plant.owner === state.actor;
      if (own && state.shovel) {
        context.strokeStyle = "#ffe192"; context.lineWidth = 3;
        context.beginPath(); context.ellipse(x, y - 5, 34, 12, 0, 0, Math.PI * 2); context.stroke();
      }
      const shot = lastEvent(view, "shoot", plant.id);
      const plantAnimation = plant.kind === "potatomine" && plant.readyAt > view.tick ? "unarmed" : shot && view.tick - shot.tick < 30 ? "attack" : "idle";
      drawSprite(context, art, plant.kind, state.reducedMotion ? 0 : (view.tick - (plant.bornAt ?? 0)) * 50, x, y, 1, plantAnimation);
      if (plant.hp < plant.maxHP) health(context, x - 23, y + 1, plant.hp / plant.maxHP, OWNER[plant.owner % OWNER.length]);
    }
    for (const zombie of view.zombies) {
      if (zombie.row !== row) continue;
      const x = left + zombie.x / 1000 * cw, y = top + (row + .94) * ch;
      const shadow = art.images.get(art.manifest.images?.shadow ?? "");
      if (shadow) context.drawImage(shadow, x - shadow.naturalWidth / 2, y - shadow.naturalHeight / 2);
      const animation = zombie.state === "eating" || zombie.state === "eat" ? "attack" : zombie.state === "vaulting" || zombie.state === "jumping" || zombie.state === "jump" ? "jump" : zombie.state === "angry" || zombie.state === "enraged" || zombie.state === "rage" ? "rage" : zombie.kind === "polevaulter" && zombie.vaulted ? "spent" : "walk";
      if ((zombie.slowUntil ?? 0) > view.tick) {
        context.fillStyle = "rgba(144,213,243,.35)"; context.beginPath(); context.ellipse(x, y - 3, 27, 9, 0, 0, Math.PI * 2); context.fill();
      }
      const kind = (zombie.kind === "conehead" || zombie.kind === "buckethead") && zombie.armor === 0 ? "zombie" : zombie.kind;
      const jump = lastEvent(view, "vault", zombie.id);
      const clip = art.manifest.sprites[kind]?.animations?.[animation];
      const frames = Array.isArray(clip?.frames) ? clip.frames.length : clip?.frames ?? 1;
      const animationTime = animation === "jump" && jump ? (view.tick - jump.tick) / 12 * frames / (clip?.fps ?? 12) * 1000 : (view.tick - (zombie.bornAt ?? 0)) * 50;
      drawSprite(context, art, kind, state.reducedMotion ? 0 : animationTime, x, y, 1, animation);
      if (zombie.hp < zombie.maxHP) health(context, x - 23, y + 1, zombie.hp / zombie.maxHP, "#b75a45");
    }
    for (const projectile of view.projectiles) {
      if (projectile.row !== row) continue;
      const x = left + projectile.x / 1000 * cw, y = top + row * ch + 46;
      const ice = projectile.kind === "snowpea" || projectile.kind === "ice" || projectile.kind === "icepea";
      const image = art.images.get(art.manifest.images?.[ice ? "snowProjectile" : "projectile"] ?? "");
      if (image) context.drawImage(image, x - image.naturalWidth / 2, y - image.naturalHeight / 2);
      else {
        context.fillStyle = projectile.kind === "snowpea" || projectile.kind === "ice" ? "#b7e7ed" : "#bbdf68";
        context.strokeStyle = "#527636"; context.lineWidth = 2;
        context.beginPath(); context.arc(x, y, 8, 0, Math.PI * 2); context.fill(); context.stroke();
      }
    }
  }
  for (const event of view.events) {
    const age = view.tick - event.tick;
    if (event.type === "zombie-death" && event.kind && age >= 0 && age < 64) {
      drawSprite(context, art, event.kind, state.reducedMotion ? 0 : age * 50, left + (event.x ?? 0) / 1000 * cw, top + ((event.row ?? 0) + .94) * ch, 1, "death");
      continue;
    }
    if (event.type === "sun" && age >= 0 && age < 22) {
      context.globalAlpha = Math.min(1, (22 - age) / 6);
      drawSprite(context, art, "sun", time, left + (event.x ?? 0) / 1000 * cw, top + ((event.row ?? 0) + .35) * ch - age * 2, .4);
      context.globalAlpha = 1;
      continue;
    }
    if (age < 0 || age > 9 || !["explosion", "mine", "hit"].includes(event.type)) continue;
    const x = left + ((event.x ?? ((event.col ?? 0) * 1000 + 500)) / 1000) * cw;
    const y = top + ((event.row ?? 0) + .5) * ch;
    context.globalAlpha = Math.max(0, 1 - age / 10);
    if (event.type === "hit") {
      context.strokeStyle = "#f9f2b1"; context.lineWidth = 3; context.beginPath(); context.arc(x, y, 9 + age * 1.5, 0, Math.PI * 2); context.stroke();
    } else {
      const size = 50 + age * 9;
      context.fillStyle = "#f8e298"; context.beginPath(); context.arc(x, y, size, 0, Math.PI * 2); context.fill();
      context.strokeStyle = "#d58a42"; context.lineWidth = 6; context.stroke();
    }
    context.globalAlpha = 1;
  }
}

function health(context: CanvasRenderingContext2D, x: number, y: number, ratio: number, color: string) {
  context.fillStyle = "#323727"; rounded(context, x, y, 46, 5, 2);
  context.fillStyle = color; rounded(context, x + 1, y + 1, Math.max(0, Math.min(1, ratio)) * 44, 3, 1);
}

export function hitStage(clientX: number, clientY: number, rect: DOMRect): Hover {
  const x = (clientX - rect.left) / rect.width * STAGE.width;
  const y = (clientY - rect.top) / rect.height * STAGE.height;
  const row = Math.floor((y - STAGE.top) / STAGE.cellHeight);
  const col = Math.floor((x - STAGE.left) / STAGE.cellWidth);
  if (row < 0 || row >= 5 || x < STAGE.left || x >= STAGE.width) return null;
  return { row, col: Math.max(0, Math.min(8, col)) };
}
