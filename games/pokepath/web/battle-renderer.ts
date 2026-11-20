import { paintEnemyStatus, paintDamageText } from "./enemy-effects";
import { enemyAuraReach, paintEnemyAura } from "./enemy-auras";
import { createDamageLabels, updateDamageLabels, type DamageLabels } from "./damage-labels";
import { effectiveResourceSpecies } from "./resource-species";
import { paintPersistentZone, paintSingularityOverlay, paintShadowExplosion } from "./world-effects";
import { createWorldAnimations, updateWorldAnimations, type WorldAnimations } from "./world-animations";
import { paintBombardmentEffect, paintBombardmentAim } from "./bombardment-effects";
import { paintBeamEffect, paintLinkEffect } from "./beam-effects";
import { paintAttackModeEffect } from "./attack-effects";
import { createDirectionalParticles, paintDirectionalAttack, type DirectionalParticle } from "./directional-effects";
import { formatGameNumber } from "./number-format";
import { siteAllowsDeployment, siteAt, type DeploymentPreview, type DeploymentRange } from "./placement";
import { mapArtwork, mapEffectClock, updateMapEffect, type MapEffectClock } from "./map-presentation";
import type { Assets, Enemy, Pokemon, Point, Projectile, Sprite, View } from "./types";

const beamKinds = new Set(["beam", "trackingBeam", "orthogonalBeam", "linkBeam"]);

export function interpolateBattle(current: View, previous: View, fraction: number): View {
  if (fraction >= 1 || current.epoch !== previous.epoch || current.route !== previous.route || current.clock <= previous.clock) return current;
  const amount = Math.max(0, fraction);
  const between = (first: number, second: number) => first + (second - first) * amount;
  return { ...current, clock: between(previous.clock, current.clock), physicsClock: current.physicsClock === undefined || previous.physicsClock === undefined ? current.physicsClock : between(previous.physicsClock, current.physicsClock), gold: previous.gold, health: previous.health,
    enemies: previous.enemies.map(enemy => {
      const next = current.enemies.find(candidate => candidate.id === enemy.id);
      const elapsed = Math.max(0, (current.physicsClock ?? current.clock) - (previous.physicsClock ?? previous.clock));
      const deathRemainingMS = enemy.dead && enemy.deathRemainingMS !== undefined ? between(enemy.deathRemainingMS, next?.deathRemainingMS ?? Math.max(0, enemy.deathRemainingMS - elapsed)) : enemy.deathRemainingMS;
      const reviveRemainingMS = enemy.reviveRemainingMS && next?.reviveRemainingMS ? between(enemy.reviveRemainingMS, next.reviveRemainingMS) : enemy.reviveRemainingMS;
      return next ? { ...enemy, x: between(enemy.x, next.x), y: between(enemy.y, next.y), deathRemainingMS, reviveRemainingMS, conditions: enemy.conditions?.map(condition => { const after = next.conditions?.find(value => value.kind === condition.kind); return after && after.durationMs === condition.durationMs && after.remainingMs <= condition.remainingMs ? { ...condition, remainingMs: between(condition.remainingMs, after.remainingMs) } : condition; }) } : enemy.dead ? { ...enemy, deathRemainingMS } : enemy;
    }),
    projectiles: previous.projectiles.map(projectile => {
      const next = current.projectiles.find(candidate => candidate.id === projectile.id);
      const beam = beamKinds.has(projectile.kind ?? "");
      const elapsed = Math.max(0, (current.physicsClock ?? current.clock) - (previous.physicsClock ?? previous.clock));
      return { ...projectile, x: between(projectile.x, next?.x ?? (beam ? projectile.x : projectile.targetX ?? projectile.x)), y: between(projectile.y, next?.y ?? (beam ? projectile.y : projectile.targetY ?? projectile.y)), ...(beam ? { targetX: between(projectile.targetX ?? projectile.x, next?.targetX ?? projectile.targetX ?? projectile.x), targetY: between(projectile.targetY ?? projectile.y, next?.targetY ?? projectile.targetY ?? projectile.y), lifeMS: between(projectile.lifeMS ?? 0, next?.lifeMS ?? (projectile.lifeMS ?? 0) + elapsed) } : {}) };
    }),
    towers: current.towers.map(tower => { const before = previous.towers?.find(candidate => candidate.id === tower.id); return before ? { ...tower, x: between(before.x, tower.x), y: between(before.y, tower.y) } : tower; }),
    events: previous.events,
    world: previous.world ? { ...previous.world, zones: previous.world.zones?.map(zone => { const next = current.world?.zones?.find(candidate => candidate.id === zone.id); return next ? { ...zone, x: between(zone.x, next.x), y: between(zone.y, next.y), radius: zone.phase === next.phase ? between(zone.radius, next.radius) : zone.radius, ageMS: zone.phase === next.phase ? between(zone.ageMS, next.ageMS) : zone.ageMS } : zone; }) } : current.world,
    attackModes: previous.attackModes ? { ...previous.attackModes, effects: previous.attackModes.effects?.map(effect => { const next = current.attackModes?.effects?.find(candidate => candidate.id === effect.id); return next ? { ...effect, ageMS: between(effect.ageMS, next.ageMS), radius: between(effect.radius, next.radius), angle: between(effect.angle, next.angle), position: { x: between(effect.position.x, next.position.x), y: between(effect.position.y, next.position.y) } } : effect; }) } : current.attackModes,
  };
}

function image(assets: Assets, path?: string) {
  return path ? assets.images.get(path.replace(/^\.\//, "")) : undefined;
}

export type BattlePresentation = { clock: number; epoch?: number; route?: number; animations: Map<string, { startedAt: number; lastAt: number }>; directionalParticles?: Map<number, { startedAt: number; particles: DirectionalParticle[] }>; prunedAt?: number; damageLabels?: DamageLabels; worldAnimations?: WorldAnimations; resetEvents?: boolean; allRanges?: boolean; mapLight?: MapEffectClock; surface?: string; deployment?: DeploymentPreview & { point?: Point; footprint?: DeploymentRange } };

function animationClock(presentation: BattlePresentation | undefined, key: string) {
  if (!presentation) return performance.now();
  let value = presentation.animations.get(key);
  if (!value) { value = { startedAt: presentation.clock, lastAt: presentation.clock }; presentation.animations.set(key, value); }
  value.lastAt = presentation.clock;
  return Math.max(0, presentation.clock - value.startedAt);
}

function sprite(context: CanvasRenderingContext2D, assets: Assets, spec: Sprite | undefined, point: Point, clock: number, direction = 0, opacity = 1) {
  if (!spec) return false;
  const texture = image(assets, spec.image ?? spec.base);
  if (!texture) return false;
  const frames = Math.max(1, spec.frames ?? 1);
  const rows = Math.max(1, spec.rows ?? 1);
  const width = spec.frameWidth || texture.naturalWidth / frames;
  const height = spec.frameHeight || texture.naturalHeight / rows;
  const current = Math.floor(clock / (Math.max(1, spec.hold ?? 12) * 1000 / 60)) % frames;
  context.save();
  context.globalAlpha *= opacity;
  context.drawImage(texture, current * width, direction % rows * height, width, height, point.x - width / 2, point.y - height / 2, width, height);
  context.restore();
  return true;
}

function enemyFacing(view: View, assets: Assets, enemy: Enemy) {
  const route = assets.route;
  const alternate = enemy.pathVariant ? route?.facts?.[enemy.pathVariant + (enemy.pathMirrored ? "Mirror" : "")] : undefined;
  const paths: Point[][] | undefined = Array.isArray(alternate) && alternate.length ? alternate : enemy.pathMirrored === undefined ? view.paths ?? route?.paths : enemy.pathMirrored && route?.mirrorPaths?.length ? route.mirrorPaths : route?.paths;
  if (!paths?.length) return 0;
  const path = paths[Math.max(0, enemy.pathIndex ?? 0) % paths.length];
  const waypoint = path[Math.max(0, enemy.pathOriginWaypoint ?? 0) + (enemy.waypoint ?? 0)];
  if (!waypoint) return 0;
  const dx = waypoint.x - enemy.x, dy = waypoint.y - enemy.y;
  if (Math.abs(dy) * Math.sqrt(3) < Math.abs(dx) || dy === 0) return dx < 0 ? 6 : 2;
  return dy < 0 ? 4 : 0;
}

function paintEnemy(context: CanvasRenderingContext2D, view: View, assets: Assets, enemy: Enemy, presentation?: BattlePresentation) {
  if (enemy.escaped || enemy.dead && !(enemy.deathRemainingMS && enemy.deathRemainingMS > 0) || enemy.health <= 0 && !enemy.dead) return;
  const spec = enemy.sprite ?? assets.catalog?.enemies[enemy.key]?.sprite;
  const width = spec?.frameWidth ?? 32, height = spec?.frameHeight ?? 40;
  const deathAlpha = enemy.dead ? Math.max(0, 1 - (560 - (enemy.deathRemainingMS ?? 0)) * .03 / (1000 / 60)) : 1;
  if (!deathAlpha) return;
  const rebirthAge = enemy.reviveRemainingMS ? Math.max(0, 5000 - enemy.reviveRemainingMS) : 0;
  const opacity = deathAlpha * Math.min(1, 1 + .5 * Math.sin(rebirthAge / 120));
  const settings = view.progression?.settings;
  const now = Date.now();
  const baseAlpha = context.globalAlpha;
  context.save();
  context.globalAlpha *= opacity;
  paintEnemyAura(context, enemy, enemy.passiveId, now, settings?.particles !== false);
  context.save();
  if (rebirthAge) {
    const scale = 1 + .05 * Math.sin(rebirthAge / 150);
    context.translate(enemy.x, enemy.y);
    context.scale(scale, scale);
    context.translate(-enemy.x, -enemy.y);
  }
  if (enemy.invisible) context.globalAlpha = .3;
  if (!sprite(context, assets, spec, enemy, animationClock(presentation, `enemy:${enemy.id}:${spec?.image}`), enemyFacing(view, assets, enemy))) {
    context.beginPath();
    context.arc(enemy.x, enemy.y, 9, 0, Math.PI * 2);
    context.fillStyle = "#aa745e";
    context.fill();
  }
  context.restore();
  if (enemy.passiveId === "flameBody") context.globalAlpha = baseAlpha;

  const barX = enemy.x - 20, barY = enemy.y - height / 2 - 14;
  const health = Math.max(0, enemy.health / enemy.maxHealth);
  const gradient = context.createLinearGradient(barX, barY, barX + 40, barY);
  const colors = health > .5 ? ["#00ff73", "#00c04b"] : health > .25 ? ["#ffe600", "#d4b400"] : ["#ff4c4c", "#b30000"];
  gradient.addColorStop(0, colors[0]);
  gradient.addColorStop(1, colors[1]);
  const display = settings?.displayHealth ?? (settings?.showHealth ? 2 : 0);
  const numbers = display === 2 || display === 1 && view.wave === 100;
  const number = (value: number) => formatGameNumber(Math.max(0, Math.floor(value)), settings?.language);
  const hudText = (value: string, x: number, y: number, passive = false) => {
    context.save();
    context.font = passive ? "bold 12px Arial" : "8px Arial";
    context.textAlign = passive ? "right" : "center";
    context.textBaseline = "middle";
    context.lineWidth = 2;
    context.strokeStyle = "black";
    context.strokeText(value, x, y);
    context.fillStyle = passive ? "#c0c0c0" : "white";
    context.fillText(value, x, y);
    context.restore();
  };
  const bar = (y: number, fraction: number, fill: string | CanvasGradient, text: string) => {
    context.fillStyle = "#222";
    context.beginPath();
    context.roundRect(barX, y, 40, 6, 3);
    context.fill();
    context.fillStyle = fill;
    context.beginPath();
    context.roundRect(barX, y, 40 * Math.max(0, fraction), 6, 3);
    context.fill();
    context.strokeStyle = "black";
    context.lineWidth = 1;
    context.beginPath();
    context.roundRect(barX, y, 40, 6, 3);
    context.stroke();
    if (numbers) hudText(text, enemy.x, y + 4);
  };
  bar(barY, health, gradient, enemy.key === "mew" || enemy.key === "calyrex" ? "???" : number(enemy.health));
  if (enemy.maxArmor > 0) bar(barY - 8, enemy.armor / enemy.maxArmor, "#66ccff", number(enemy.armor));
  const hits = enemy.receivedHits ?? 0;
  const guard = enemy.passiveId === "ironDefense" ? Math.min(enemy.passiveCounters?.ironDefenseHits ?? 0, 100) : enemy.passiveId === "dauntlessShield" ? Math.min(400 - hits * 2, 400) : enemy.passiveId === "intrepidSword" ? Math.min(-100 + hits * 2, 100) : undefined;
  if (guard !== undefined) hudText(`${guard}%`, barX + 75, barY, true);
  const conditions = enemy.conditions ?? [];
  if (conditions.some(condition => condition.kind === "curse" && condition.remainingMs > 0)) {
    const curse = image(assets, "src/assets/images/icons/curse.png");
    if (curse) context.drawImage(curse, barX - 14, barY - 2.5, 12, 12);
  }
  paintEnemyStatus(context, { x: enemy.x, y: enemy.y, width, height, conditions }, now, { durationCC: settings?.durationCC, particles: settings?.particles !== false });
  context.restore();
}

const rangeColors: ReadonlyArray<readonly [string, string[]]> = [
  ["255,0,0", ["burn", "burnNerf", "burnSplash", "willOWisp", "drought", "lavaPlume", "fireLash", "flameWheel"]],
  ["0,255,0", ["poison", "poisonDoubleShot"]],
  ["160,60,200", ["nightmare"]],
  ["0,0,0", ["curse", "magicBounce", "curseDoubleShot"]],
  ["255,215,0", ["stun", "stunMono", "stunMonoNerf", "stunArea", "static"]],
  ["255,255,255", ["slow", "slowSplash", "cradily"]],
  ["100,180,255", ["genesisAura", "powerAura", "triage", "criticalAura", "illuminate"]]
];

function towersByDepth(towers: View["towers"], party: ReadonlyMap<number, Pokemon>) {
  const upperLayer = (id: number) => party.get(id)?.species === "spectrier" ? 1 : 0;
  return [...towers].sort((first, second) => upperLayer(first.pokemonID) - upperLayer(second.pokemonID) || first.y - second.y);
}

function effectOpacity(view: View, category: "projectiles" | "areas" | "beams" | "waves" | "auras" | "bombardment" | "special") {
  return (view.progression?.settings.effectAlpha ?? 100) * (view.progression?.settings.effectOpacity?.[category] ?? (category === "auras" ? 50 : 100)) / 10000;
}

function paintWorldLinks(context: CanvasRenderingContext2D, view: View) {
  for (const link of view.world?.links ?? []) {
    const first = view.towers.find(tower => tower.id === link.first);
    const second = view.towers.find(tower => tower.id === link.second);
    if (!first || !second) continue;
    context.save();
    context.globalAlpha = 1;
    paintLinkEffect(context, { from: first, to: second, width: 12 }, { opacity: effectOpacity(view, "beams"), now: Date.now() });
    context.restore();
  }
}

function paintWorld(context: CanvasRenderingContext2D, view: View, assets: Assets, selectedID?: number, presentation?: BattlePresentation, layer: "under" | "over" = "under") {
  const clock = view.physicsClock ?? view.clock;
  const alpha = (view.progression?.settings.effectAlpha ?? 100) / 100;
  context.save();
  context.globalAlpha = alpha;
  if (layer === "over") {
  paintWorldLinks(context, view);
  for (const [id, status] of Object.entries(view.world?.towers ?? {})) {
    const tower = view.towers.find(tower => tower.pokemonID === Number(id));
    if (!tower) continue;
    context.globalAlpha = effectOpacity(view, "beams");
    for (const enemyID of Object.keys(status.chains ?? {}).map(Number)) {
      const enemy = view.enemies.find(enemy => enemy.id === enemyID);
      if (!enemy) continue;
      context.save();
      context.globalAlpha = 1;
      paintBeamEffect(context, { from: tower, to: enemy, extendedTo: enemy, width: 6, alpha: 1 }, { ability: "anchorShot", opacity: effectOpacity(view, "beams"), now: Date.now() });
      context.restore();
    }
    if ((status.boostMS ?? 0) > 0) {
      context.globalAlpha = effectOpacity(view, "auras");
      context.beginPath();
      context.arc(tower.x, tower.y, 20 + Math.sin(clock / 120) * 2, 0, Math.PI * 2);
      context.strokeStyle = "#ffdc7a99";
      context.lineWidth = 2;
      context.stroke();
    }
  }
  }
  for (const zone of view.world?.zones ?? []) {
    if ((zone.kind === "shadowTag") !== (layer === "under")) continue;
    context.save();
    context.globalAlpha = 1;
    const persistent = paintPersistentZone(context, zone, Date.now(), { opacity: effectOpacity(view, "special"), particles: view.progression?.settings.particles !== false });
    context.restore();
    if (persistent) continue;
    const owner = (view.battleTeam ?? view.team).find(pokemon => pokemon.id === zone.pokemonID);
    const definition = owner && assets.catalog ? effectiveResourceSpecies(owner, assets.catalog, view.progression?.dna) : undefined;
    const projectile = assets.catalog?.projectiles[String(definition?.projectileId ?? "")];
    const spec = typeof projectile?.sprite === "object" ? projectile.sprite : undefined;
    if (zone.kind === "singularity") {
      context.save();
      context.globalAlpha = effectOpacity(view, "projectiles");
      sprite(context, assets, spec, zone, animationClock(presentation, `zone:${zone.id}:${spec?.image}`));
      context.restore();
      context.save();
      context.globalAlpha = 1;
      paintSingularityOverlay(context, presentation?.worldAnimations?.flashes.has(zone.id) ? { ...zone, phase: "waiting", ageMS: 1000 } : zone);
      context.restore();
      continue;
    }
    context.save();
    context.globalAlpha = effectOpacity(view, ["web", "spikes"].includes(zone.kind) ? "areas" : zone.kind === "skyArrow" ? "bombardment" : zone.kind === "toyProjectile" ? "projectiles" : "special");
    context.translate(zone.x, zone.y);
    if (zone.kind === "shadowTag" && zone.phase === "travel") {
      context.rotate(Math.atan2((zone.targetY ?? zone.y) - zone.y, (zone.targetX ?? zone.x) - zone.x));
      context.shadowBlur = 5;
      context.shadowColor = "rgba(255,255,255,0.8)";
      sprite(context, assets, spec, { x: 0, y: 0 }, animationClock(presentation, `zone:${zone.id}:${spec?.image}`));
    } else if (zone.kind === "web") {
      context.strokeStyle = "#eee9d5bb";
      context.lineWidth = 1;
      for (let ring = 1; ring <= 3; ring++) { context.beginPath(); context.arc(0, 0, zone.radius * ring / 3, 0, Math.PI * 2); context.stroke(); }
      for (let direction = 0; direction < 8; direction++) { const angle = direction * Math.PI / 4; context.beginPath(); context.moveTo(0, 0); context.lineTo(Math.cos(angle) * zone.radius, Math.sin(angle) * zone.radius); context.stroke(); }
    } else if (zone.kind === "spikes") {
      context.fillStyle = "#d3d8d5";
      context.strokeStyle = "#535e64";
      context.lineWidth = 1;
      for (let index = 0; index < 7; index++) { const angle = index * Math.PI * 2 / 7; const x = Math.cos(angle) * zone.radius * 0.6; const y = Math.sin(angle) * zone.radius * 0.6; context.beginPath(); context.moveTo(x - 4, y + 4); context.lineTo(x, y - 7); context.lineTo(x + 4, y + 4); context.closePath(); context.fill(); context.stroke(); }
    } else if (zone.kind === "skyArrow") {
      context.beginPath();
      context.arc(0, 0, zone.radius, 0, Math.PI * 2);
      context.strokeStyle = zone.ageMS < zone.delayMS ? "#e8cc6988" : "#ecf8ef";
      context.lineWidth = 2;
      context.stroke();
      if (zone.ageMS >= zone.delayMS) { context.beginPath(); context.moveTo(0, -Math.max(0, 220 - (zone.ageMS - zone.delayMS) * 2)); context.lineTo(0, 0); context.strokeStyle = "#b7e7bb"; context.lineWidth = 4; context.stroke(); }
    } else if (zone.kind === "singularity" || zone.kind === "shadowTag") {
      const radius = zone.phase === "travel" ? 9 : Math.max(4, zone.radius);
      context.beginPath(); context.arc(0, 0, radius, 0, Math.PI * 2); context.fillStyle = "#29183d77"; context.fill();
      context.strokeStyle = "#b87ad8bb"; context.lineWidth = 2; context.setLineDash([6, 4]); context.stroke(); context.setLineDash([]);
      if (zone.kind === "singularity") { context.rotate(zone.ageMS / 150); context.beginPath(); context.ellipse(0, 0, radius, radius * 0.45, 0, 0, Math.PI * 2); context.stroke(); }
    } else if (zone.kind === "tornado") {
      context.shadowBlur = 5;
      context.shadowColor = "rgba(255,255,255,0.8)";
      sprite(context, assets, spec, { x: 0, y: 0 }, animationClock(presentation, `zone:${zone.id}:${spec?.image}`));
    } else sprite(context, assets, spec, { x: 0, y: 0 }, animationClock(presentation, `zone:${zone.id}:${spec?.image}`));
    context.restore();
  }
  if (layer === "over") {
  for (const effect of presentation?.worldAnimations?.explosions ?? []) {
    context.save(); context.globalAlpha = 1;
    paintShadowExplosion(context, { x: effect.x, y: effect.y, radius: effect.radius, ageMS: clock - effect.startMS, durationMS: 450 });
    context.restore();
  }
  for (const effect of view.attackModes?.effects ?? []) {
    if (effect.ageMS < effect.delayMS) continue;
    const owner = (view.battleTeam ?? view.team).find(pokemon => pokemon.id === effect.pokemonID);
    const color = owner && assets.catalog ? effectiveResourceSpecies(owner, assets.catalog, view.progression?.dna)?.color ?? "#fff0a5" : "#fff0a5";
    context.save();
    context.globalAlpha = 1;
    const bombardment = paintBombardmentEffect(context, effect, { color, projectileOpacity: effectOpacity(view, "projectiles"), areaOpacity: effectOpacity(view, "areas"), image: image(assets, effect.sprite?.image ?? effect.sprite?.base) });
    context.restore();
    if (bombardment) continue;
    if (effect.kind === "directional") {
      const ability = effect.context?.AbilityID ?? "";
      const now = Date.now();
      const particles = presentation?.directionalParticles;
      let saved = particles?.get(effect.id);
      if (!saved) { saved = { startedAt: now - effect.ageMS, particles: createDirectionalParticles(ability) }; particles?.set(effect.id, saved); }
      context.save();
      context.globalAlpha = 1;
      const painted = paintDirectionalAttack(context, effect, { ability, opacity: effectOpacity(view, "areas"), particles: view.progression?.settings.particles !== false, now, startedAt: saved.startedAt, particleData: saved.particles });
      context.restore();
      if (painted) continue;
    }
    context.save();
    context.globalAlpha = 1;
    const painted = paintAttackModeEffect(context, effect, { color, opacity: effectOpacity(view, effect.kind === "wave" ? "waves" : effect.kind === "arc" ? "projectiles" : "special"), particles: view.progression?.settings.particles !== false, now: Date.now(), motionNow: performance.now() });
    context.restore();
    if (painted) continue;
    context.save();
    context.globalAlpha = effectOpacity(view, effect.kind === "wave" ? "waves" : ["bomb", "lightningArea"].includes(effect.kind) ? "bombardment" : "special");
    context.strokeStyle = color;
    context.fillStyle = `${color}55`;
    context.lineWidth = effect.width || 2;
    if (effect.kind === "wave") {
      context.beginPath(); context.arc(effect.origin.x, effect.origin.y, effect.radius, 0, Math.PI * 2); context.lineWidth = Math.max(1, effect.width); context.stroke();
    } else if (effect.shape === "arc") {
      context.beginPath(); context.arc(effect.position.x, effect.position.y, effect.radius, effect.angle - (effect.halfAngle ?? Math.PI * 0.32), effect.angle + (effect.halfAngle ?? Math.PI * 0.32)); context.lineWidth = Math.max(1, effect.width); context.stroke();
    } else if (effect.shape === "cone") {
      context.beginPath(); context.moveTo(effect.origin.x, effect.origin.y); context.arc(effect.origin.x, effect.origin.y, effect.radius, effect.angle - (effect.halfAngle ?? Math.PI / 4), effect.angle + (effect.halfAngle ?? Math.PI / 4)); context.closePath(); context.fill(); context.lineWidth = 2; context.stroke();
    } else if (effect.shape === "blade") {
      context.translate(effect.origin.x, effect.origin.y); context.rotate(effect.angle); context.fillRect(0, -effect.width, effect.radius, effect.width * 2);
    } else if (effect.shape === "targetCircle") {
      context.beginPath(); context.arc(effect.destination.x, effect.destination.y, effect.radius, 0, Math.PI * 2); context.fill(); context.lineWidth = 2; context.stroke();
    } else if (effect.shape === "band") {
      context.translate(effect.position.x, effect.position.y); context.rotate(effect.angle); context.fillRect(-Math.hypot(effect.position.x - effect.origin.x, effect.position.y - effect.origin.y) - 80, -effect.width / 2, Math.hypot(effect.position.x - effect.origin.x, effect.position.y - effect.origin.y) + 80, effect.width);
      context.beginPath(); context.moveTo(0, -effect.width / 2); context.lineTo(0, effect.width / 2); context.lineWidth = 3; context.stroke();
    } else if (effect.kind === "lightningArea" && effect.boltPoints?.length) {
      context.beginPath(); effect.boltPoints.forEach((point, index) => index === 0 ? context.moveTo(point.x, point.y) : context.lineTo(point.x, point.y)); context.strokeStyle = "#fff5a0"; context.lineWidth = 3; context.stroke();
    } else if (!effect.impacted) sprite(context, assets, effect.sprite, effect.position, animationClock(presentation, `effect:${effect.id}:${effect.sprite?.image}`));
    if (effect.impacted) {
      context.beginPath(); context.arc(effect.position.x, effect.position.y, Math.max(1, effect.radius), 0, Math.PI * 2); context.fillStyle = "#ffdb6f33"; context.fill(); context.strokeStyle = "#ffe9a4aa"; context.lineWidth = 2; context.stroke();
    }
    context.restore();
  }
  }
  context.restore();
}

function paintChess(context: CanvasRenderingContext2D, view: View, assets: Assets, presentation?: BattlePresentation) {
  context.save();
  for (const placement of assets.route?.chessPlacements ?? []) {
    const active = view.world?.chessActive?.includes(placement.key);
    sprite(context, assets, assets.catalog?.species[placement.species]?.sprite, placement, animationClock(presentation, `chess:${placement.key}`), 0, active ? 1 : 0.35);
  }
  context.restore();
}

function paintAim(context: CanvasRenderingContext2D, view: View, assets: Assets, selectedID?: number) {
  const aim = selectedID === undefined ? undefined : view.attackModes?.towers?.[String(selectedID)];
  const tower = view.towers.find(tower => tower.pokemonID === selectedID);
  const owner = (view.battleTeam ?? view.team).find(pokemon => pokemon.id === selectedID);
  const species = owner && assets.catalog ? effectiveResourceSpecies(owner, assets.catalog, view.progression?.dna) : undefined;
  if (!aim?.hasAim || !tower) return;
  context.save();
  context.globalAlpha = 1;
  paintBombardmentAim(context, { origin: tower, aim: aim.aim, radius: aim.areaRadius, width: aim.areaWidth, height: aim.areaHeight }, { color: species?.color ?? "#fff0a5", opacity: effectOpacity(view, "bombardment") });
  context.restore();
}

function paintProjectiles(context: CanvasRenderingContext2D, view: View, assets: Assets, projectiles: Projectile[], presentation?: BattlePresentation) {
  for (const projectile of projectiles) {
    if (beamKinds.has(projectile.kind ?? "") && (projectile.beamWidth ?? 0) > 0) {
      const tower = view.towers.find(tower => tower.id === projectile.towerID);
      const owner = (view.battleTeam ?? view.team).find(pokemon => pokemon.id === (projectile.pokemonID ?? tower?.pokemonID));
      const species = owner && assets.catalog ? effectiveResourceSpecies(owner, assets.catalog, view.progression?.dna) : undefined;
      const duration = projectile.profile?.beamDurationMs || 260;
      const endpoint = { x: projectile.targetX ?? projectile.x, y: projectile.targetY ?? projectile.y };
      context.save();
      context.globalAlpha = 1;
      const painted = paintBeamEffect(context, { from: projectile, to: endpoint, extendedTo: endpoint, width: projectile.beamWidth!, alpha: duration < 0 ? 1 : Math.max(0, 1 - (projectile.lifeMS ?? 0) / duration) }, { ability: projectile.context?.AbilityID ?? species?.abilityId ?? "", item: projectile.context?.ItemID ?? owner?.item, color: species?.color ?? projectile.color, opacity: effectOpacity(view, "beams"), particles: view.progression?.settings.particles !== false, now: Date.now() });
      context.restore();
      if (painted) continue;
    }
    context.save();
    context.globalAlpha = effectOpacity(view, ["beam", "stream"].includes(projectile.kind ?? "") ? "beams" : "projectiles");
    context.save();
    context.translate(projectile.x, projectile.y);
    const ordinary = !projectile.profile?.returning && !projectile.rayLength && (projectile.kind === "single" || projectile.kind === "criticalRicochet" || projectile.kind === "webProjectile");
    if (!ordinary) context.rotate(Math.atan2((projectile.targetY ?? projectile.y) - projectile.y, (projectile.targetX ?? projectile.x) - projectile.x));
    context.shadowBlur = 5;
    context.shadowColor = projectile.color ?? "rgba(255,255,255,0.8)";
    const drawn = sprite(context, assets, projectile.sprite, { x: 0, y: 0 }, animationClock(presentation, `projectile:${projectile.id}:${projectile.sprite?.image}`));
    context.restore();
    if (drawn) { context.restore(); continue; }
    const tower = view.towers.find(tower => tower.id === projectile.towerID);
    const pokemon = (view.battleTeam ?? view.team).find(pokemon => pokemon.id === tower?.pokemonID);
    const color = pokemon ? assets.catalog?.species[pokemon.species]?.color : undefined;
    const dx = (projectile.targetX ?? projectile.x) - projectile.x;
    const dy = (projectile.targetY ?? projectile.y) - projectile.y;
    const length = Math.max(1, Math.hypot(dx, dy));
    context.beginPath();
    context.moveTo(projectile.x - dx / length * 7, projectile.y - dy / length * 7);
    context.lineTo(projectile.x, projectile.y);
    context.strokeStyle = color ?? "#fff2b4";
    context.lineWidth = 4;
    context.lineCap = "round";
    context.stroke();
    context.restore();
  }
}

function paintRange(context: CanvasRenderingContext2D, assets: Assets, selected: Pokemon, center: Point, colored: boolean, footprint?: DeploymentRange) {
  const species = assets.catalog?.species[selected.species];
  const range = footprint?.range ?? selected.combatStats?.range ?? selected.stats.range;
  if (!Number.isFinite(range) || range < 0) return;
  let shape = footprint?.shape ?? selected.combatRangeType ?? species?.rangeType ?? "circle";
  if (selected.item === "starPiece" && (shape === "cross" || shape === "xShape")) shape = "star";
  if (selected.item === "condensedBlizzard" && shape === "xShape") shape = "circle";
  const combatWidth = footprint?.halfWidth ?? selected.combatRangeHalfWidth;
  const halfWidth = combatWidth !== undefined ? Math.max(0, combatWidth - (shape === "xShape" && selected.item === "wideLens" ? 24 : 0)) : selected.item === "wideLens" ? 48 : 24;
  const tint = colored ? rangeColors.find(([, abilities]) => abilities.includes(species?.abilityId ?? ""))?.[0] ?? "0,0,200" : undefined;
  context.save();
  context.fillStyle = tint ? `rgba(${tint},0.2)` : "#0000ff33";
  context.strokeStyle = tint ? `rgba(${tint},0.4)` : "#0000ff66";
  context.lineWidth = 1;
  context.beginPath();
  const circle = (radius: number, reverse = false) => { context.arc(center.x, center.y, radius, 0, Math.PI * 2, reverse); };
  const vertical = (x: number, y: number) => context.rect(x - halfWidth, y - range, 2 * halfWidth, 2 * range);
  const horizontal = (x: number, y: number) => context.rect(x - range, y - halfWidth, 2 * range, 2 * halfWidth);
  if (["donut", "ring"].includes(shape)) {
    const inner = Math.max(0, footprint?.innerRange ?? selected.combatInnerRange ?? species?.innerRange ?? 0);
    circle(range);
    circle(inner, true);
    context.fill("evenodd");
    for (const radius of [range, inner]) { context.beginPath(); circle(radius); context.stroke(); }
    context.restore();
    return;
  }
  if (shape === "circle") circle(range);
  else if (shape === "vShape") {
    for (const [index, point] of [{ x: 0, y: -0.06 }, { x: -0.7, y: -0.65 }, { x: 0, y: 1 }, { x: 0.7, y: -0.65 }].entries()) index === 0 ? context.moveTo(center.x + point.x * range, center.y + point.y * range) : context.lineTo(center.x + point.x * range, center.y + point.y * range);
    context.closePath();
  } else if (shape === "verticalLine") vertical(center.x, center.y);
  else if (shape === "horizontalLine") horizontal(center.x, center.y);
  else {
    if (shape === "cross" || shape === "star") { vertical(center.x, center.y); horizontal(center.x, center.y); }
    if (shape === "xShape" || shape === "star") { context.translate(center.x, center.y); context.rotate(Math.PI / 4); vertical(0, 0); horizontal(0, 0); }
  }
  context.fill();
  context.stroke();
  context.restore();
}

export function paintBattle(context: CanvasRenderingContext2D, view: View, assets: Assets, selectedID: number | undefined, placing = false, presentation?: BattlePresentation) {
  if (presentation) {
    if (presentation.epoch !== view.epoch || presentation.route !== view.route) { presentation.epoch = view.epoch; presentation.route = view.route; presentation.animations.clear(); presentation.directionalParticles?.clear(); }
    presentation.worldAnimations ??= createWorldAnimations();
    updateWorldAnimations(presentation.worldAnimations, view, presentation.resetEvents);
    presentation.directionalParticles ??= new Map();
    const directionalIDs = new Set(view.attackModes?.effects?.filter(effect => effect.kind === "directional").map(effect => effect.id));
    for (const id of presentation.directionalParticles.keys()) if (!directionalIDs.has(id)) presentation.directionalParticles.delete(id);
    if (presentation.animations.size > 4096 && presentation.clock - (presentation.prunedAt ?? 0) > 1000) {
      presentation.prunedAt = presentation.clock;
      for (const [key, value] of presentation.animations) if (presentation.clock - value.lastAt > 10000) presentation.animations.delete(key);
    }
  }
  const width = assets.route?.width ?? 720;
  const height = assets.route?.height ?? 624;
  context.save();
  context.setTransform(1, 0, 0, 1, 0, 0);
  context.imageSmoothingEnabled = true;
  const background = image(assets, mapArtwork(view, assets.route)) ?? image(assets, assets.route?.background);
  const surface = `${assets.route?.id ?? view.route}:${Boolean(view.progression?.custom)}`;
  if (!background || presentation?.surface !== surface) context.clearRect(0, 0, width, height);
  if (presentation) presentation.surface = surface;
  if (background) context.drawImage(background, 0, 0, width, height);
  context.imageSmoothingEnabled = true;
  const selected = (view.battleTeam ?? view.team).find(pokemon => pokemon.id === selectedID);
  const selectedTower = view.towers.find(tower => tower.pokemonID === selectedID);
  paintWorld(context, view, assets, selectedID, presentation, "under");
  for (let index = view.enemies.length - 1; index >= 0; index--) {
    const enemy = view.enemies[index];
    const padding = Math.max(64, (enemy.sprite?.frameHeight ?? assets.catalog?.enemies[enemy.key]?.sprite.frameHeight ?? 0) / 2 + 30, enemyAuraReach(enemy.passiveId));
    if (enemy.x < -padding || enemy.x > width + padding || enemy.y < -padding || enemy.y > height + padding) continue;
    paintEnemy(context, view, assets, enemy, presentation);
  }
  const party = new Map((view.battleTeam ?? view.team).map(pokemon => [pokemon.id, pokemon]));
  for (const tower of towersByDepth(view.towers, party)) {
    const pokemon = party.get(tower.pokemonID);
    if (!pokemon) continue;
    const definition = assets.catalog?.species[pokemon.species];
    const spec = pokemon.sprite ?? definition?.sprite;
    context.save();
    context.imageSmoothingEnabled = false;
    context.imageSmoothingQuality = "low";
    if (pokemon.item === "inverter") {
      context.translate(tower.x, tower.y);
      if (definition?.abilityId === "simple") context.rotate(Math.PI / 2);
      else context.scale(1, -1);
      context.translate(-tower.x, -tower.y);
    }
    sprite(context, assets, spec, tower, animationClock(presentation, `tower:${tower.id}:${spec?.image}`));
    context.restore();
    paintProjectiles(context, view, assets, view.projectiles.filter(projectile => projectile.towerID === tower.id), presentation);
    if (tower.id === view.towers[0]?.id) paintWorldLinks(context, view);
  }
  const knownTowers = new Set(view.towers.map(tower => tower.id));
  paintProjectiles(context, view, assets, view.projectiles.filter(projectile => !knownTowers.has(projectile.towerID)), presentation);
  const preview = presentation?.deployment;
  if (placing && selected && preview) {
    const { tileSize, sites, point } = preview;
    const hovered = point ? siteAt(view.tiles, point, tileSize) : undefined;
    const occupied = new Set(view.towers.map(tower => tower.tileID));
    for (const tile of view.tiles) {
      const kind = sites[tile.id] ?? "blocked";
      const focus = tile.id === hovered?.id && siteAllowsDeployment(kind);
      context.fillStyle = kind === "blocked" ? "rgba(255,50,50,0.5)" : occupied.has(tile.id) ? "rgba(150,150,150,0.5)" : focus ? "white" : "rgba(255,255,255,0.5)";
      context.fillRect(tile.x, tile.y, tileSize, tileSize);
      context.strokeStyle = "#333";
      context.lineWidth = 1;
      context.strokeRect(tile.x, tile.y, tileSize, tileSize);
      if (focus && selected.tileID !== tile.id && preview.footprint) paintRange(context, assets, selected, preview.footprint.center, false, preview.footprint);
    }
  }
  paintWorld(context, view, assets, selectedID, presentation, "over");
  paintChess(context, view, assets, presentation);
  if (presentation) {
    presentation.damageLabels ??= createDamageLabels();
    const labels = updateDamageLabels(presentation.damageLabels, view, assets.catalog, presentation.resetEvents);
    presentation.resetEvents = false;
    if (view.progression?.settings.showDamage !== false) for (const label of labels) paintDamageText(context, { x: label.x, y: label.y, text: label.text, color: label.color, ageMS: (view.physicsClock ?? view.clock) - label.startMS });
  }
  for (const rangeTower of presentation?.allRanges ? view.towers : !placing && selectedTower ? [selectedTower] : []) {
    const pokemon = party.get(rangeTower.pokemonID);
    if (pokemon) paintRange(context, assets, pokemon, rangeTower, Boolean(presentation?.allRanges));
  }
  paintAim(context, view, assets, selectedID);
  const effectMode = view.progression?.settings.mapEffectsMode ?? (view.progression?.settings.mapEffects === false ? 0 : 1);
  const effect = effectMode > 0 ? image(assets, assets.route?.effect) : undefined;
  const physical = view.physicsClock ?? view.clock;
  const opacity = presentation ? updateMapEffect(presentation.mapLight ??= mapEffectClock(presentation.clock, physical), effect ? effectMode : 0, presentation.clock, physical) : effectMode === 2 ? 1 : .65;
  if (effect) {
    context.globalAlpha = opacity;
    context.imageSmoothingEnabled = true;
    context.drawImage(effect, 0, 0, width, height);
  }
  context.restore();
}

export function pokemonAt(view: View, point: Point): Pokemon | undefined {
  const party = new Map((view.battleTeam ?? view.team).map(pokemon => [pokemon.id, pokemon]));
  const tower = towersByDepth(view.towers, party).reverse().find(tower => {
    const pokemon = party.get(tower.pokemonID);
    if (!pokemon) return false;
    const width = Math.max(24, pokemon.sprite?.frameWidth ?? 32);
    const height = Math.max(24, pokemon.sprite?.frameHeight ?? 40);
    return Math.abs(tower.x - point.x) <= width / 2 + 3 && Math.abs(tower.y - point.y) <= height / 2 + 3;
  });
  return tower ? party.get(tower.pokemonID) : undefined;
}
