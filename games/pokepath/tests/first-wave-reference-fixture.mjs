class QuietTrack {
  pause() {}
  play() { return Promise.resolve(); }
  addEventListener() {}
  removeEventListener() {}
}
window.Audio = QuietTrack;
const [{ Main }, { Game }, { Enemy }, { Projectile }, { loadData }] = await Promise.all([import("/src/js/game/Main.js"), import("/src/js/game/Game.js"), import("/src/js/game/component/Enemy.js"), import("/src/js/game/component/Projectile.js"), import("/src/js/file/data.js")]);
Game.prototype.load = function () { this.lastTime = 1; };
const options = new URL(location.href).searchParams;
const starter = options.get("starter");
const deploy = options.get("deploy") === "1";
const ideal = options.get("ideal") === "1";
const target = options.get("target");
const item = options.get("item");
const critical = options.get("critical") === "1";
const frameMS = 1000 / 60;
localStorage.setItem("data", "");
const data = loadData();
Object.assign(data.config, { font: "PressStart2P", particles: 0, mapEffects: 0, displayHealth: 0, showDamage: false });
data.save.new = false;
if (item) data.save.player.items = [{ id: item }];
data.save.team = [{ specieKey: starter, lvl: 1, ...(target ? { targetMode: target } : {}), ...(item ? { item: { id: item } } : {}) }];
localStorage.setItem("data", JSON.stringify(data));
const main = new Main(data.save);
if (["0", "1"].includes(options.get("critical"))) main.team.pokemon[0].critical = critical ? 100 : 0;
if (item && main.team.pokemon[0].item?.id !== item) throw Error("original did not retain the equipped fixture item");
main.game.speedFactor = Number(options.get("speed") ?? 1);
if (deploy) {
  main.game.deployingUnit = main.team.pokemon[0];
  main.game.moveUnitToTile(main.area.placementTiles.find(tile => tile.id === 1), true);
  if (main.area.towers.length !== 1) throw Error("original did not deploy a tower");
}
if (options.get("warm") === "1") {
  const paths = [...new Set(main.area.waves[1].wave.filter(Boolean).map(enemy => enemy.sprite.image))];
  const images = paths.map(path => { const image = new Image(); image.src = path; return image; });
  await Promise.all(images.map(image => image.decode()));
  await new Promise(resolve => requestAnimationFrame(resolve));
}
const savedRandom = Math.random;
const jitter = options.has("jitter") ? JSON.parse(options.get("jitter")) : undefined;
const jitterSites = options.has("jitterSites") ? JSON.parse(options.get("jitterSites")) : undefined;
let jitterCursor = 0;
Math.random = () => {
  if (jitter) {
    const caller = new Error().stack?.split("\n")[2] ?? "";
    const axis = caller.includes(`/Projectile.js:${jitterSites.x}:`) ? 0 : caller.includes(`/Projectile.js:${jitterSites.y}:`) ? 1 : -1;
    if (axis >= 0) {
      if (jitterCursor % 2 !== axis || !jitter[Math.floor(jitterCursor / 2)]) throw Error("Original ricochet jitter consumption differs from the controlled inputs");
      return jitter[Math.floor(jitterCursor++ / 2)][axis];
    }
  }
  return .5;
};
main.area.newWave();
const projectileImages = [...new Set(main.area.towers.map(tower => tower.projectile.sprite.image))].map(path => { const image = new Image(); image.src = path; return image; });
await Promise.all([main.game.canvasBackground, ...projectileImages, ...[...main.area.towers, ...main.area.enemies].map(entity => entity.sprite)].map(image => image.decode()));
await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
if ([...main.area.towers, ...main.area.enemies].some(entity => !entity.loaded)) throw Error("original sprite not loaded before clock started");
const ordinal = new Map(main.area.enemies.map((enemy, index) => [enemy, index]));
let now = 0;
let phase = "loop";
const attacks = [];
const shots = [];
const observedShots = new WeakSet();
const origins = new WeakMap();
const projectileUpdate = Projectile.prototype.update;
Projectile.prototype.update = function (...args) {
  if (!origins.has(this)) origins.set(this, { x: this.position.x + this.width / 2, y: this.position.y + this.height / 2 });
  return projectileUpdate.apply(this, args);
};
const account = [];
const trace = [];
const getDamaged = Enemy.prototype.getDamaged;
Enemy.prototype.getDamaged = function (...args) {
  const before = this.hp;
  const result = getDamaged.apply(this, args);
  const hpDamage = Math.max(0, Math.max(0, before) - Math.max(0, this.hp));
  if (hpDamage > 0) attacks.push({ time: now, enemy: ordinal.get(this), kind: ["burn", "poison"].includes(args[1]) ? args[1] : "direct", critical: Boolean(args[3]), amount: hpDamage, x: this.center.x, y: this.center.y });
  return result;
};
const changeGold = main.player.changeGold.bind(main.player);
main.player.changeGold = amount => { const result = changeGold(amount); if (amount) account.push({ time: now, kind: phase === "complete" ? "bonus" : "killGold", amount }); return result; };
const getHurt = main.player.getDamaged.bind(main.player);
main.player.getDamaged = amount => { const before = main.player.health[0], result = getHurt(amount); if (main.player.health[0] < before) account.push({ time: now, kind: "escape", amount: before - main.player.health[0] }); return result; };
const endWave = main.area.endWave.bind(main.area);
main.area.endWave = () => { phase = "complete"; return endWave(); };
const units = () => main.area.enemies.map(enemy => ({ index: ordinal.get(enemy), x: enemy.center.x, y: enemy.center.y, hp: Math.max(0, enemy.hp), entered: enemy.hasEnteredCanvas, dead: enemy.dying }));
const finish = () => ({ gold: main.player.gold, health: main.player.health[0], stars: main.player.stars, wave: main.area.waveNumber, kills: main.player.stats.defeatedEnemies });
trace.push({ time: 0, units: units() });
const wall = performance.now.bind(performance);
Object.defineProperty(performance, "now", { configurable: true, value: () => 1 + now });
main.area.waveStartTime = 1;
for (let frame = 1; frame <= 9000 && main.area.waveActive; frame++) {
  now = frame * frameMS + (ideal ? 0 : frame * 1e-6);
  if (ideal) main.game.lastTime = 1;
  main.game.animate(1 + (ideal ? frameMS : now));
  for (const tower of main.area.towers) for (const projectile of tower.projectiles) {
    if (observedShots.has(projectile)) continue;
    observedShots.add(projectile);
    if (!projectile.isRicochet) shots.push({ time: now, enemy: ordinal.get(projectile.enemy), x: tower.center.x, y: tower.center.y, origin: origins.get(projectile), projectileX: projectile.center?.x, projectileY: projectile.center?.y, power: tower.power, incenseBuff: tower.incenseBuff, incenseTimer: tower.incenseTimer, statsRecalcTimer: tower.statsRecalcTimer, range: tower.range, width: projectile.width, units: units() });
  }
  if (frame % 12 === 0 || !main.area.waveActive) trace.push({ time: now, units: units() });
}
Object.defineProperty(performance, "now", { configurable: true, value: wall });
Math.random = savedRandom;
Enemy.prototype.getDamaged = getDamaged;
Projectile.prototype.update = projectileUpdate;
if (main.area.waveActive) throw Error("original did not complete the first wave within 150 seconds");
if (jitter && jitterCursor !== jitter.length * 2) throw Error("Original did not consume every controlled ricochet random input");
window.waveReference = { starter, deploy, item: item || undefined, forceCritical: critical, controlledJitterPairs: jitter ? jitterCursor / 2 : undefined, targetMode: main.team.pokemon[0].targetMode, attacks, shots, account, trace, outcome: finish(), completedAt: now };
