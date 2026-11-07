import { animationDuration, drawSprite, hasAnimation, type Art } from "./art";
import { mapFor, plantKind, slopeAt, stageFor, visibleCell } from "./scene";
import type { Event, View } from "./types";

export const STAGE = { width: 800, height: 600, left: 40, top: 80, cellWidth: 80, cellHeight: 100, rows: 5, cols: 9 } as const;
const OWNER = ["#f4d263", "#e7a0a2", "#b9def4", "#c9b4ec"];
export type Hover = { row: number; col: number } | null;
export type PaintState = { selected: string | null; shovel: boolean; hover: Hover; actor: number; canAct: boolean; now: number; reducedMotion: boolean };
const backgrounds = new WeakMap<Art, Map<string, HTMLCanvasElement>>();
const deathTrails = new WeakMap<Art, { epoch: string; events: Map<number, Event> }>();
function rounded(context: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, radius: number) {
  context.beginPath(); context.roundRect(x, y, w, h, radius); context.fill();
}
function lastEvent(view: View, type: string, target: number) {
  for (let i = view.events.length - 1; i >= 0; i--) if (view.events[i].type === type && view.events[i].target === target) return view.events[i];
  return undefined;
}
function drawImage(context: CanvasRenderingContext2D, art: Art, key: string, x: number, y: number, scale = 1) {
  const image = art.images.get(art.manifest.images?.[key] ?? "");
  if (!image) return false;
  context.drawImage(image, x - image.naturalWidth * scale / 2, y - image.naturalHeight * scale, image.naturalWidth * scale, image.naturalHeight * scale);
  return true;
}
function background(art: Art, view: View) {
  let cache = backgrounds.get(art); if (!cache) { cache = new Map(); backgrounds.set(art, cache); }
  const map = mapFor(view), saved = cache.get(map.id); if (saved) return saved;
  const layer = document.createElement("canvas"); layer.width = 800; layer.height = 600;
  const context = layer.getContext("2d")!;
  context.fillStyle = map.night ? "#26322f" : "#6e9751"; context.fillRect(0, 0, 800, 600);
  const definition = art.manifest.maps?.[map.id];
  const image = art.images.get(definition?.background ?? art.manifest.background);
  const crop = definition?.backgroundCrop ?? art.manifest.backgroundCrop;
  if (image) {
    if (crop) context.drawImage(image, crop.x, crop.y, crop.width, crop.height, 0, 0, 800, 600);
    else context.drawImage(image, 0, 0, 800, 600);
  }
  cache.set(map.id, layer); return layer;
}

export function paint(context: CanvasRenderingContext2D, view: View, art: Art, state: PaintState) {
  art.drawnPoses = [];
  const map = mapFor(view), stage = stageFor(view, art), { left, top, cellWidth: cw, cellHeight: ch, width, height } = stage;
  const foot = (row: number, x: number, fraction = .9) => top + (row + fraction) * ch + slopeAt(x, stage.roofSlope);
  const cellY = (row: number, col: number) => top + row * ch + (stage.roofSlope[col] ?? 0);
  const time = state.reducedMotion ? 0 : view.tick * 50;
  let deaths = deathTrails.get(art);
  if (!deaths || deaths.epoch !== view.epoch) { deaths = { epoch: view.epoch, events: new Map() }; deathTrails.set(art, deaths); }
  for (const event of view.events) if (event.type === "zombie-death" && !deaths.events.has(event.id)) deaths.events.set(event.id, event);
  context.clearRect(0, 0, width, height); context.drawImage(background(art, view), 0, 0);
  const player = view.players[state.actor], card = view.catalog[player?.side ?? "plants"]?.find(spec => spec.kind === state.selected);
  if (state.canAct && player?.side === "zombies" && card?.target !== "cell" && card?.target !== "plant") {
    for (let row = 0; row < map.rows; row++) {
      const active = state.hover?.row === row;
      context.fillStyle = active ? "rgba(255,224,124,.72)" : "rgba(46,43,30,.5)";
      rounded(context, 763, top + row * ch + 16, 34, ch - 32, 9);
      context.fillStyle = active ? "#5b4326" : "#f5e5b5";
      context.beginPath(); context.moveTo(770, top + (row + .5) * ch); context.lineTo(786, top + (row + .5) * ch - 12); context.lineTo(786, top + (row + .5) * ch + 12); context.closePath(); context.fill();
    }
  }
  if (state.hover && state.canAct && (state.selected || state.shovel)) {
    const { row, col } = state.hover;
    context.fillStyle = state.shovel ? "rgba(237,144,109,.38)" : "rgba(249,230,133,.35)";
    context.fillRect(left + col * cw, cellY(row, col), cw, ch);
    context.strokeStyle = "#fff0aa"; context.lineWidth = 3;
    context.strokeRect(left + col * cw + 2, cellY(row, col) + 2, cw - 4, ch - 4);
    if (player?.side === "plants" && state.selected && !state.shovel) {
      context.globalAlpha = .5;
      drawSprite(context, art, state.selected === "imitater" ? player.imitaterKind : state.selected, 0, left + (col + .5) * cw, foot(row, col * 1000 + 500));
      context.globalAlpha = 1;
    }
  }
  for (const grave of view.graves ?? []) {
    const x = left + (grave.col + .5) * cw, y = foot(grave.row, grave.col * 1000 + 500);
    if (!drawSprite(context, art, "grave", time, x, y)) drawImage(context, art, "grave", x, y);
  }
  for (const crater of view.craters ?? []) if (crater.until > view.tick) drawImage(context, art, "crater", left + (crater.col + .5) * cw, foot(crater.row, crater.col * 1000 + 500));
  for (const terrain of view.terrain ?? []) {
    const image = art.images.get(art.manifest.images?.[terrain.kind] ?? "");
    if (!image || terrain.until <= view.tick) continue;
    const x1 = left + terrain.fromX / 1000 * cw, x2 = left + terrain.toX / 1000 * cw;
    context.save(); context.beginPath(); context.rect(x1, top + terrain.row * ch, x2 - x1, ch); context.clip();
    for (let x = x1; x < x2; x += image.naturalWidth) context.drawImage(image, x, foot(terrain.row, (x - left) / cw * 1000) - image.naturalHeight);
    context.restore();
  }
  for (const mower of view.mowers) {
    if (mower.state === "spent") continue;
    const x = mower.state === "moving" ? left + mower.x / 1000 * cw : left - 21;
    const key = map.waterRows.includes(mower.row) && art.manifest.sprites.poolcleaner ? "poolcleaner" : map.roof && art.manifest.sprites.roofcleaner ? "roofcleaner" : "mower";
    drawSprite(context, art, key, time, x, foot(mower.row, 0, .83));
  }
  const layers = { support: 0, body: 1, shell: 2, instant: 3 };
  const plants = [...view.plants].sort((a, b) => (layers[a.layer ?? "body"] ?? 1) - (layers[b.layer ?? "body"] ?? 1));
  for (let row = 0; row < map.rows; row++) {
    for (const plant of plants) {
      if (plant.row !== row) continue;
      const x = left + (plant.col + .5) * cw, y = foot(row, plant.col * 1000 + 500), kind = plantKind(plant);
      const shadow = art.images.get(art.manifest.images?.shadow ?? "");
      if (shadow && plant.layer !== "support") context.drawImage(shadow, x - shadow.naturalWidth / 2, y - shadow.naturalHeight / 2);
      if (plant.owner === state.actor && state.shovel) {
        context.strokeStyle = "#ffe192"; context.lineWidth = 3; context.beginPath(); context.ellipse(x, y - 5, 34, 12, 0, 0, Math.PI * 2); context.stroke();
      }
      const shot = lastEvent(view, "shoot", plant.id);
      const attackAt = plant.flags?.attackAt ?? (plant.kind === "cobcannon" ? shot?.tick : undefined);
      const attackClip = plant.state === "high" && hasAnimation(art, kind, "shootinghigh") ? "shootinghigh" : "attack";
      const attackTime = attackAt === undefined ? Infinity : Math.max(0, view.tick - attackAt) * 50;
      const attacking = attackTime < animationDuration(art, kind, attackClip) && !["scared", "biting", "digesting", "recovering", "grabbing", "jumping"].includes(plant.state ?? "");
      const animation = plant.asleep ? "sleep" : plant.kind === "potatomine" && plant.readyAt > view.tick ? "unarmed" : attacking ? attackClip : plant.state && hasAnimation(art, kind, plant.state) ? plant.state : "idle";
      const phaseTime = (view.tick - (plant.flags?.phaseAt ?? plant.bornAt ?? 0)) * 50;
      const tinted = plant.imitated && plant.kind !== "imitater";
      if (tinted) { context.save(); context.filter = "grayscale(1) brightness(1.15)"; }
      drawSprite(context, art, kind, state.reducedMotion ? 0 : attacking ? attackTime : phaseTime, x, y, 1, animation);
      if (tinted) context.restore();
      if (plant.asleep) { context.fillStyle = "#d8e5ee"; context.font = "bold 11px sans-serif"; context.fillText("z", x + 21, y - 58); context.font = "bold 8px sans-serif"; context.fillText("z", x + 31, y - 69); }
      if (plant.hp < plant.maxHP) health(context, x - 23, y + 1, plant.hp / plant.maxHP, OWNER[plant.owner % OWNER.length]);
    }
    for (const zombie of view.zombies) {
      if (zombie.row !== row) continue;
      if (zombie.kind === "bobsled" && zombie.flags?.sled === 1 && (zombie.flags.leader ?? 0) > 0) continue;
      const x = left + zombie.x / 1000 * cw, altitude = (zombie.altitude ?? 0) / 1000 * ch, y = foot(row, zombie.x, .94) - altitude;
      const shadow = art.images.get(art.manifest.images?.shadow ?? "");
      if (shadow) context.drawImage(shadow, x - shadow.naturalWidth / 2, foot(row, zombie.x, .94) - shadow.naturalHeight / 2);
      let animation = zombie.state && hasAnimation(art, zombie.kind, zombie.state) ? zombie.state : zombie.diving ? "submerged" : zombie.digging ? "digging" : zombie.altitude ? "fly" : zombie.state === "eating" ? "attack" : zombie.state === "vaulting" ? "jump" : zombie.state === "angry" ? "rage" : zombie.kind === "polevaulter" && zombie.vaulted ? "spent" : "walk";
      if (zombie.kind === "zomboss") {
        const targetRow = zombie.flags?.targetRow ?? zombie.row;
        const exact = zombie.state === "boss-summon" ? `spawn_${Math.max(1, Math.min(5, targetRow + 1))}` : zombie.state === "boss-stomp" ? `stomp_${Math.max(1, Math.min(4, zombie.row + 1))}` : zombie.state === "boss-head-spit" ? `head_attack_${Math.max(1, Math.min(5, zombie.row + 1))}` : "";
        if (exact && hasAnimation(art, zombie.kind, exact)) animation = exact;
      }
      if ((zombie.slowUntil ?? 0) > view.tick) { context.fillStyle = "rgba(144,213,243,.35)"; context.beginPath(); context.ellipse(x, y - 3, 27, 9, 0, 0, Math.PI * 2); context.fill(); }
      const kind = (zombie.kind === "conehead" || zombie.kind === "buckethead") && zombie.armor === 0 ? "zombie" : zombie.kind;
      const animationTime = (view.tick - (zombie.flags?.phaseAt ?? zombie.bornAt ?? 0)) * 50;
      const scenePlacement = art.manifest.sprites[kind]?.placement === "scene";
      if (!scenePlacement && zombie.flags?.direction === 1) { context.save(); context.translate(x * 2, 0); context.scale(-1, 1); }
      drawSprite(context, art, kind, state.reducedMotion ? 0 : animationTime, scenePlacement ? 0 : x, scenePlacement ? 0 : y, 1, animation);
      if (!scenePlacement && zombie.flags?.direction === 1) context.restore();
      if (zombie.kind === "bungee" && zombie.flags?.stolenPlant) {
        const held = view.catalog.plants.find(spec => spec.originalId === zombie.flags!.stolenPlant - 1);
        const attachment = art.manifest.sprites.bungee?.attachments;
        if (held && attachment) {
          const offset = ["spikeweed", "spikerock"].includes(held.kind) ? attachment.spikeFoot : attachment.plantFoot;
          const roofAdjustment = map.roof && Math.floor(zombie.x / 1000) <= 4 ? attachment.roofWestAdjustmentY : 0;
          drawSprite(context, art, held.kind, time, x + offset.x, y + offset.y + roofAdjustment);
        }
      }
      if (zombie.hp < zombie.maxHP) health(context, x - 23, y + 1, zombie.hp / zombie.maxHP, "#b75a45");
    }
    for (const projectile of view.projectiles) {
      if (projectile.row !== row) continue;
      const x = left + projectile.x / 1000 * cw;
      const y = (projectile.y === undefined ? top + row * ch + 46 : top + projectile.y / 1000 * ch) + slopeAt(projectile.x, stage.roofSlope) - (projectile.height ?? 0) / 1000 * ch;
      const ice = ["snowpea", "ice", "icepea"].includes(projectile.kind);
      const key = art.manifest.images?.[projectile.kind] ? projectile.kind : ice ? "snowProjectile" : "projectile";
      const projectileKey = projectile.kind === "icepea" ? "snowpea" : projectile.kind;
      if (!drawSprite(context, art, projectile.kind, time, x, y, 1, undefined, art.manifest.projectiles?.[projectileKey])) drawImage(context, art, key, x, y + 12);
    }
  }
  for (const ladder of view.ladders ?? []) drawImage(context, art, "ladder", left + (ladder.col + .5) * cw, foot(ladder.row, ladder.col * 1000 + 500));
  for (const pickup of view.pickups ?? []) drawSprite(context, art, pickup.kind, time, left + pickup.x / 1000 * cw, pickup.y === undefined ? foot(pickup.row, pickup.x, .4) : top + pickup.y / 1000 * ch, .4);
  for (const [id, event] of deaths.events) {
    const age = view.tick - event.tick;
    if (!event.kind || age < 0 || age * 50 >= animationDuration(art, event.kind, "death")) { deaths.events.delete(id); continue; }
    const placed = art.manifest.sprites[event.kind]?.placement === "scene";
    drawSprite(context, art, event.kind, state.reducedMotion ? 0 : age * 50, placed ? 0 : left + (event.x ?? 0) / 1000 * cw, placed ? 0 : foot(event.row ?? 0, event.x ?? 0), 1, "death");
  }
  for (const event of view.events) {
    const age = view.tick - event.tick, x = left + (event.x ?? 0) / 1000 * cw, y = foot(event.row ?? 0, event.x ?? 0);
    if (event.type === "sun" && age >= 0 && age < 22) { context.globalAlpha = Math.min(1, (22 - age) / 6); drawSprite(context, art, "sun", time, x, y - 55 - age * 2, .4); context.globalAlpha = 1; continue; }
    if (age < 0 || age > 9 || !["explosion", "mine", "hit"].includes(event.type)) continue;
    context.globalAlpha = Math.max(0, 1 - age / 10);
    if (event.type === "hit") { context.strokeStyle = "#f9f2b1"; context.lineWidth = 3; context.beginPath(); context.arc(x, y - 45, 9 + age * 1.5, 0, Math.PI * 2); context.stroke(); }
    else { context.fillStyle = "#f8e298"; context.beginPath(); context.arc(x, y - 45, 50 + age * 9, 0, Math.PI * 2); context.fill(); context.strokeStyle = "#d58a42"; context.lineWidth = 6; context.stroke(); }
    context.globalAlpha = 1;
  }
  if (map.fog) {
    for (let row = 0; row < map.rows; row++) for (let col = 0; col < map.cols; col++) if (!visibleCell(view, row, col)) {
      const x = left + col * cw, y = top + row * ch;
      context.fillStyle = "rgba(193,205,199,.96)"; context.fillRect(x, y, cw + 1, ch + 1);
      const image = art.images.get(art.manifest.images?.fog ?? "");
      if (image) context.drawImage(image, 0, 0, image.naturalWidth, image.naturalHeight, x - 12, y - 12, cw + 24, ch + 24);
    }
  }
}
function health(context: CanvasRenderingContext2D, x: number, y: number, ratio: number, color: string) {
  context.fillStyle = "#323727"; rounded(context, x, y, 46, 5, 2);
  context.fillStyle = color; rounded(context, x + 1, y + 1, Math.max(0, Math.min(1, ratio)) * 44, 3, 1);
}
export function hitStage(clientX: number, clientY: number, rect: DOMRect, view?: View, art?: Art | null): Hover {
  const stage = view ? stageFor(view, art) : { ...STAGE, roofSlope: [] as number[] };
  const x = (clientX - rect.left) / rect.width * stage.width, y = (clientY - rect.top) / rect.height * stage.height;
  const col = Math.max(0, Math.min(stage.cols - 1, Math.floor((x - stage.left) / stage.cellWidth)));
  const row = Math.floor((y - stage.top - (stage.roofSlope[col] ?? 0)) / stage.cellHeight);
  if (row < 0 || row >= stage.rows || x < stage.left || x >= stage.width) return null;
  return { row, col };
}
