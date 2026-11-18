class QuietTrack {
  pause() {}
  play() { return Promise.resolve(); }
  addEventListener() {}
  removeEventListener() {}
}
window.Audio = QuietTrack;
const [{ Main }, { Game }, { Enemy }, { loadData }] = await Promise.all([import("/src/js/game/Main.js"), import("/src/js/game/Game.js"), import("/src/js/game/component/Enemy.js"), import("/src/js/file/data.js")]);
Game.prototype.load = function () { this.lastTime = 1; };
const input = JSON.parse(new URL(location.href).searchParams.get("input"));
localStorage.setItem("data", "");
const data = loadData();
Object.assign(data.config, { font: "PressStart2P", particles: 0, mapEffects: 0, displayHealth: 0, showDamage: false });
data.save.new = false;
if (input.species.includes("megaMawile")) data.save.player.items = [{ id: "mawilite" }];
data.save.team = input.species.map(specieKey => ({ specieKey, lvl: 1, targetMode: "first", ...(specieKey === "megaMawile" ? { item: { id: "mawilite" }, isMega: true } : {}) }));
localStorage.setItem("data", JSON.stringify(data));
const main = new Main(data.save);
if (main.team.pokemon.some((pokemon, index) => pokemon.specie.key !== input.species[index])) throw Error("reference did not retain the controlled form and required item");
main.game.speedFactor = input.speed;
for (const [index, slot] of input.order.entries()) {
  main.game.deployingUnit = main.team.pokemon[slot];
  main.game.moveUnitToTile(main.area.placementTiles.find(tile => tile.id === [1, 2, 14][index]), true);
}
if (main.area.towers.length !== input.species.length) throw Error("reference deployment rejected a controlled owner");
main.area.newWave();
const paths = [...new Set([...main.area.towers.map(tower => tower.projectile.sprite.image), ...main.area.towers.map(tower => tower.sprite.src), ...main.area.enemies.map(enemy => enemy.sprite.src)])];
await Promise.all(paths.map(path => { const image = new Image(); image.src = path; return image.decode(); }));
await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
const owners = new Map(main.team.pokemon.map((pokemon, index) => [pokemon, index]));
const towers = new Map(main.area.towers.map(tower => [tower, owners.get(tower.pokemon)]));
const enemies = main.area.enemies.slice(0, 3);
main.area.enemies = enemies;
for (const [index, enemy] of enemies.entries()) {
  enemy.position.x = 252 + index * 20 + (input.offsetX ?? 0) - enemy.width / 2;
  enemy.position.y = 132 - enemy.height / 2;
  enemy.center = { x: 252 + index * 20 + (input.offsetX ?? 0), y: 132 };
  enemy.hp = input.health;
  enemy.hpMax = input.impact?.maxHealth ?? input.health;
  enemy.armor = enemy.armorMax = 0;
  enemy.speed = enemy.baseSpeed = 0;
  enemy.hasEnteredCanvas = true;
  enemy.distanceTraveled = 100 - index * 10;
}
const ordinals = new Map(enemies.map((enemy, index) => [enemy, index]));
let frame = 0;
const events = [];
const launches = [];
for (const tower of main.area.towers) {
  const slot = towers.get(tower);
  tower.position.x = 240;
  tower.position.y = input.y[slot] - 12;
  tower.center = { x: 252, y: input.y[slot] };
  tower.power = tower.basePower = input.impact?.power ?? 6;
  tower.projectile.power = input.impact?.power ?? 6;
  tower.range = 600;
  tower.speed = 10000;
  tower.attackCooldown = 0;
  if (tower.pokemon.specie.key === "megaMawile" && !input.impact?.ordinary || input.impact && slot > 0) tower.attackCooldown = 10000;
  if (input.impact?.ordinary) for (const enemy of enemies) tower.destructiveMawTriggered.add(enemy);
  if (input.name === "link-minus-first-lethal") tower.attackCooldown = 10000;
  if (tower.pokemon.specie.key === "skarmory") {
    tower._movementAbility = tower.pokemon.ability.id;
    tower.aerialAceOrbitCenter = { x: 252, y: input.y[slot] };
    tower.aerialAceOrbitAngle = Math.PI / 2;
    tower.aerialAceOrbitRadius = 70;
    tower.aerialAceOrbitSpeed = 1;
  }
  tower.critical = input.impact?.critical ?? 0;
  tower.statsRecalcTimer = 100;
  tower.projectiles.push = function (...projectiles) {
    for (const projectile of projectiles) if (!projectile.isRicochet) {
      events.push({ frame, kind: "fire", tower: slot, enemy: ordinals.get(projectile.enemy), amount: 0, resolved: 0 });
      launches.push({ frame, tower: slot, position: { ...projectile.position }, width: projectile.width, height: projectile.height, speed: projectile.speed, enemy: { center: { ...projectile.enemy.center }, width: projectile.enemy.width }, extra: projectile.extra });
    }
    return Array.prototype.push.apply(this, projectiles);
  };
  tower.beams.push = function (...beams) {
    for (const beam of beams) events.push({ frame, kind: "fire", tower: slot, enemy: ordinals.get(beam.attachedEnemy ?? tower.target), amount: 0, resolved: 0 });
    return Array.prototype.push.apply(this, beams);
  };
}
const damaged = Enemy.prototype.getDamaged;
if (input.species[0] === "plusle") {
  main.area.towers.find(tower => tower.ability.id === "plus").update([], 0, false);
  if (main.area.linkBeams.length !== 1) throw Error("reference did not retain the real plus/minus connection");
  if (main.area.linkBeams[0].hitCooldown !== 1000 || main.area.linkBeams[0].width !== 12) throw Error("reference did not use the normal runtime connection parameters");
  main.area.linkBeams[0].simulatedTime = input.linkAge ?? 0;
}
Enemy.prototype.getDamaged = function (...args) {
  const before = Math.max(0, this.hp);
  const totalBefore = args[5]?.damageDealt ?? 0;
  const result = damaged.apply(this, args);
  const amount = before - Math.max(0, this.hp);
  if (amount > 0) events.push({ frame, kind: "hit", tower: towers.get(args[6]) ?? owners.get(args[5]), enemy: ordinals.get(this), amount, resolved: (args[5]?.damageDealt ?? 0) - totalBefore });
  return result;
};
const random = Math.random;
Math.random = () => .5;
const now = performance.now.bind(performance);
Object.defineProperty(performance, "now", { configurable: true, value: () => 1 + frame * 1000 / 60 });
for (frame = 1; frame <= (input.frames || 3); frame++) {
  main.game.lastTime = 1;
  main.game.animate(1 + 1000 / 60);
}
Object.defineProperty(performance, "now", { configurable: true, value: now });
Math.random = random;
Enemy.prototype.getDamaged = damaged;
window.towerReference = { events, launches, health: enemies.map(enemy => Math.max(0, enemy.hp)), updatePositions: main.area.towers.map(tower => ({ tower: towers.get(tower), x: tower.position.x, y: tower.position.y })) };
