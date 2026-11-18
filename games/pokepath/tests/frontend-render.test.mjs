import test from "node:test";
import assert from "node:assert/strict";
import { readFile, mkdir, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
const require = createRequire(new URL("../../../web/package.json", import.meta.url));

test("visual interpolation keeps health, removal and impact on one phase", async () => {
  const folder = new URL("../../../.local/pokepath/native-ui-test/", import.meta.url);
  await mkdir(folder, { recursive: true });
  await require("esbuild").build({ entryPoints: [new URL("../web/battle-renderer.ts", import.meta.url).pathname], outfile: new URL("renderer.mjs", folder).pathname, bundle: true, platform: "node", format: "esm" });
  const { interpolateBattle } = await import(new URL("renderer.mjs", folder));
  const previous = { clock: 0, gold: 50, health: 14, towers: [], enemies: [{ id: 7, x: 0, y: 20, health: 10 }], projectiles: [{ id: 9, x: 0, y: 20, targetX: 20, targetY: 20 }], events: [] };
  const current = { ...previous, clock: 20, gold: 54, enemies: [{ id: 7, x: 20, y: 20, health: 0 }], projectiles: [] };
  const middle = interpolateBattle(current, previous, 0.5);
  assert.equal(middle.enemies[0].x, 10);
  assert.equal(middle.enemies[0].health, 10);
  assert.equal(middle.projectiles[0].x, 10);
  assert.equal(middle.gold, 50);
  assert.deepEqual(interpolateBattle(current, previous, 1), current);
  assert.equal(previous.enemies[0].x, 0);
});

test("moving zones share the enemy timeline and retain the previous damage phase", async () => {
  const folder = new URL("../../../.local/pokepath/native-ui-test/", import.meta.url);
  await mkdir(folder, { recursive: true });
  await require("esbuild").build({ entryPoints: [new URL("../web/battle-renderer.ts", import.meta.url).pathname], outfile: new URL("renderer-world.mjs", folder).pathname, bundle: true, platform: "node", format: "esm" });
  const { interpolateBattle } = await import(new URL("renderer-world.mjs", folder));
  const previous = { epoch: 1, route: 0, clock: 0, gold: 50, health: 14, towers: [], enemies: [], projectiles: [], events: [], world: { zones: [{ id: 9, kind: "singularity", x: 10, y: 20, phase: "travel", radius: 90, ageMS: 0 }] } };
  const current = { ...previous, clock: 20, world: { zones: [{ id: 9, kind: "singularity", x: 30, y: 40, phase: "pulling", radius: 90, ageMS: 20 }] } };
  const middle = interpolateBattle(current, previous, 0.5);
  assert.equal(middle.world.zones[0].x, 20);
  assert.equal(middle.world.zones[0].y, 30);
  assert.equal(middle.world.zones[0].phase, "travel");
  assert.equal(middle.world.zones[0].ageMS, 0);
  assert.equal(previous.world.zones[0].x, 10);
  assert.equal(interpolateBattle(current, previous, 1).world.zones[0].phase, "pulling");
});

test("entity animation starts at zero and follows the display clock across rule speed changes", async () => {
  const folder = new URL("../../../.local/pokepath/native-ui-test/", import.meta.url);
  await mkdir(folder, { recursive: true });
  const target = new URL("renderer-animation.mjs", folder);
  await require("esbuild").build({ entryPoints: [new URL("../web/battle-renderer.ts", import.meta.url).pathname], outfile: target.pathname, bundle: true, platform: "node", format: "esm" });
  const { paintBattle } = await import(target.href);
  const draws = [];
  const context = new Proxy({ globalAlpha: 1, drawImage(...args) { draws.push(args); }, createLinearGradient() { return { addColorStop() {} }; } }, { get(target, key) { return target[key] ?? (() => {}); } });
  const image = { naturalWidth: 128, naturalHeight: 256 };
  const spec = { image: "enemy.png", frames: 4, rows: 8, hold: 15, frameWidth: 32, frameHeight: 32 };
  const assets = { route: { width: 720, height: 624, paths: [] }, images: new Map([["enemy.png", image]]), catalog: { enemies: { rattata: { sprite: spec } }, species: {}, projectiles: {} } };
  const view = { epoch: 1, route: 0, clock: 0, physicsClock: 0, team: [], towers: [], tiles: [], enemies: [{ id: 1, key: "rattata", entered: true, x: 100, y: 100, health: 70, maxHealth: 70, armor: 0, maxArmor: 0 }], projectiles: [], events: [] };
  const presentation = { clock: 1000, animations: new Map() };
  paintBattle(context, view, assets, undefined, false, presentation);
  assert.equal(draws.at(-1)[1], 0);
  presentation.clock = 1016;
  paintBattle(context, { ...view, physicsClock: 3250 }, assets, undefined, false, presentation);
  assert.equal(draws.at(-1)[1], 0);
  presentation.clock = 1250;
  paintBattle(context, { ...view, physicsClock: 3250 }, assets, undefined, false, presentation);
  assert.equal(draws.at(-1)[1], 32);
  presentation.clock = 2000;
  paintBattle(context, { ...view, physicsClock: 3250, enemies: [{ ...view.enemies[0], id: 99 }] }, assets, undefined, false, presentation);
  assert.equal(draws.at(-1)[1], 0);
});

test("returning projectile sprites retain their motion orientation and glow", async () => {
  const target = new URL("../../../.local/pokepath/native-ui-test/renderer-projectile.mjs", import.meta.url);
  await require("esbuild").build({ entryPoints: [new URL("../web/battle-renderer.ts", import.meta.url).pathname], outfile: target.pathname, bundle: true, platform: "node", format: "esm" });
  const { paintBattle } = await import(target.href);
  const rotations = [];
  const draws = [];
  const context = new Proxy({ globalAlpha: 1, shadowBlur: 0, rotate(value) { rotations.push(value); }, drawImage() { draws.push({ blur: this.shadowBlur, color: this.shadowColor }); } }, { get(target, key) { return target[key] ?? (() => {}); } });
  const assets = { route: { width: 720, height: 624, paths: [] }, images: new Map([["projectile.png", { naturalWidth: 32, naturalHeight: 32 }]]), catalog: { enemies: {}, species: {}, projectiles: {} } };
  const view = { epoch: 1, route: 0, clock: 0, team: [], towers: [], tiles: [], enemies: [], projectiles: [{ id: 42, kind: "single", profile: { returning: true }, towerID: 1, x: 10, y: 20, targetX: 30, targetY: 40, sprite: { image: "projectile.png", frames: 1, rows: 1 } }], events: [] };
  paintBattle(context, view, assets, undefined, false, { clock: 1000, animations: new Map() });
  assert.ok(rotations.some(value => Math.abs(value - Math.PI / 4) < 0.00001));
  assert.equal(draws.at(-1).blur, 5);
  assert.equal(draws.at(-1).color, "rgba(255,255,255,0.8)");
});

test("hover ranges appear above units and below the final map effect", async () => {
  const target = new URL("../../../.local/pokepath/native-ui-test/renderer-layers.mjs", import.meta.url);
  await require("esbuild").build({ entryPoints: [new URL("../web/battle-renderer.ts", import.meta.url).pathname], outfile: target.pathname, bundle: true, platform: "node", format: "esm" });
  const { paintBattle } = await import(target.href);
  const layers = [];
  const context = new Proxy({ globalAlpha: 1, drawImage(image) { layers.push(image.kind); }, stroke() { if (this.strokeStyle === "#0000ff66") layers.push("range"); }, createLinearGradient() { return { addColorStop() {} }; } }, { get(target, key) { return target[key] ?? (() => {}); } });
  const spec = path => ({ image: path, frames: 1, rows: 1, hold: 15, frameWidth: 32, frameHeight: 32 });
  const assets = { route: { width: 720, height: 624, paths: [], background: "map.png", effect: "effect.png" }, images: new Map([...["map", "effect", "enemy", "tower"].map(kind => [`${kind}.png`, { kind, naturalWidth: 32, naturalHeight: 32 }])]), catalog: { enemies: { rattata: { sprite: spec("enemy.png") } }, species: { charmander: { sprite: spec("tower.png"), rangeType: "circle", terrains: [1] } }, projectiles: {} } };
  const view = { epoch: 1, route: 0, clock: 0, team: [{ id: 7, species: "charmander", stats: { range: 110 }, sprite: spec("tower.png") }], towers: [{ id: 11, pokemonID: 7, x: 100, y: 100 }], tiles: [], enemies: [{ id: 1, key: "rattata", entered: true, x: 100, y: 100, health: 70, maxHealth: 70, armor: 0, maxArmor: 0 }], projectiles: [], events: [] };
  paintBattle(context, view, assets, 7, false, { clock: 1000, animations: new Map() });
  assert.ok(layers.indexOf("range") > layers.indexOf("enemy"));
  assert.ok(layers.indexOf("range") > layers.indexOf("tower"));
  assert.ok(layers.indexOf("range") < layers.indexOf("effect"));
});

test("directional particles persist by effect identity and clear after the effect or epoch ends", async () => {
  const target = new URL("../../../.local/pokepath/native-ui-test/renderer-directional.mjs", import.meta.url);
  await require("esbuild").build({ entryPoints: [new URL("../web/battle-renderer.ts", import.meta.url).pathname], outfile: target.pathname, bundle: true, platform: "node", format: "esm" });
  const { paintBattle } = await import(target.href);
  const context = new Proxy({ globalAlpha: 1, createRadialGradient() { return { addColorStop() {} }; } }, { get(target, key) { return target[key] ?? (() => {}); } });
  const assets = { route: { width: 720, height: 624, paths: [] }, images: new Map(), catalog: { species: {}, enemies: {}, projectiles: {} } };
  const effect = { id: 11, kind: "directional", context: { AbilityID: "icyWind" }, origin: { x: 100, y: 100 }, position: { x: 100, y: 100 }, angle: 0, radius: 100, halfAngle: 0.3, ageMS: 30, delayMS: 0, durationMS: 600 };
  const view = { epoch: 1, route: 0, clock: 0, team: [], towers: [], enemies: [], projectiles: [], events: [], tiles: [], attackModes: { effects: [effect] } };
  const presentation = { clock: 1000, animations: new Map() };
  const original = Math.random;
  let calls = 0;
  Math.random = () => { calls++; return 0.5; };
  try {
    paintBattle(context, view, assets, undefined, false, presentation);
    const once = calls;
    assert.ok(once > 0);
    paintBattle(context, view, assets, undefined, false, presentation);
    assert.equal(calls, once);
    paintBattle(context, { ...view, epoch: 2 }, assets, undefined, false, presentation);
    assert.equal(calls, once * 2);
    paintBattle(context, { ...view, epoch: 2, attackModes: { effects: [] } }, assets, undefined, false, presentation);
    assert.equal(presentation.directionalParticles.size, 0);
  } finally { Math.random = original; }
});

test("authoritative chain dictionaries render active enemy links without iterating timers as IDs", async () => {
  const target = new URL("../../../.local/pokepath/native-ui-test/renderer-chain.mjs", import.meta.url);
  await require("esbuild").build({ entryPoints: [new URL("../web/battle-renderer.ts", import.meta.url).pathname], outfile: target.pathname, bundle: true, platform: "node", format: "esm" });
  const { paintBattle } = await import(target.href);
  const translations = [];
  const context = new Proxy({ globalAlpha: 1, translate(x, y) { translations.push([x, y]); }, createRadialGradient() { return { addColorStop() {} }; }, createLinearGradient() { return { addColorStop() {} }; } }, { get(target, key) { return target[key] ?? (() => {}); } });
  const assets = { route: { width: 720, height: 624, paths: [] }, images: new Map(), catalog: { species: {}, enemies: {}, projectiles: {} } };
  const view = { epoch: 1, route: 0, clock: 0, team: [], towers: [{ id: 11, pokemonID: 7, x: 100, y: 100 }], enemies: [{ id: 17, key: "rattata", entered: false, x: 200, y: 210 }], projectiles: [], events: [], tiles: [], world: { towers: { 7: { chains: { 17: 280 } } } } };
  assert.doesNotThrow(() => paintBattle(context, view, assets, undefined, false, { clock: 1000, animations: new Map() }));
  assert.ok(translations.some(([x, y]) => x === 200 && y === 210));
});

test("bombardment uses the whole verified projectile texture at its authoritative radius", async () => {
  const target = new URL("../../../.local/pokepath/native-ui-test/renderer-bombardment.mjs", import.meta.url);
  await require("esbuild").build({ entryPoints: [new URL("../web/battle-renderer.ts", import.meta.url).pathname], outfile: target.pathname, bundle: true, platform: "node", format: "esm" });
  const { paintBattle } = await import(target.href);
  const draws = [];
  const context = new Proxy({ globalAlpha: 1, drawImage(...args) { draws.push(args); } }, { get(target, key) { return target[key] ?? (() => {}); } });
  const texture = { naturalWidth: 256, naturalHeight: 256 };
  const assets = { route: { width: 720, height: 624, paths: [] }, images: new Map([["shell.png", texture]]), catalog: { species: {}, enemies: {}, projectiles: {} } };
  const effect = { id: 11, kind: "bombardment", origin: { x: 100, y: 100 }, destination: { x: 200, y: 200 }, position: { x: 150, y: 150 }, radius: 27, ageMS: 250, delayMS: 0, durationMS: 500, impacted: false, sprite: { image: "shell.png", frames: 8, rows: 8, frameWidth: 32, frameHeight: 32 } };
  const view = { epoch: 1, route: 0, clock: 0, team: [], towers: [], enemies: [], projectiles: [], events: [], tiles: [], attackModes: { effects: [effect] } };
  paintBattle(context, view, assets, undefined, false, { clock: 1000, animations: new Map() });
  assert.deepEqual(draws, [[texture, -27, -27, 54, 54]]);
});

test("beam drawing consumes authoritative full width, duration and endpoint", async () => {
  const target = new URL("../../../.local/pokepath/native-ui-test/renderer-beam.mjs", import.meta.url);
  await require("esbuild").build({ entryPoints: [new URL("../web/battle-renderer.ts", import.meta.url).pathname], outfile: target.pathname, bundle: true, platform: "node", format: "esm" });
  const { paintBattle } = await import(target.href);
  const strokes = [], endpoints = [];
  const context = new Proxy({ globalAlpha: 1, lineTo(x, y) { endpoints.push([x, y]); }, stroke() { strokes.push({ width: this.lineWidth, alpha: this.globalAlpha }); } }, { get(target, key) { return target[key] ?? (() => {}); } });
  const assets = { route: { width: 720, height: 624, paths: [] }, images: new Map(), catalog: { species: {}, enemies: {}, projectiles: {} } };
  const projectile = { id: 11, kind: "beam", towerID: 7, x: 100, y: 100, targetX: 300, targetY: 110, beamWidth: 42, lifeMS: 130, context: { AbilityID: "solarBeam" }, profile: { beamDurationMs: 260 } };
  const view = { epoch: 1, route: 0, clock: 0, team: [], towers: [], enemies: [], projectiles: [projectile], events: [], tiles: [], progression: { settings: { effectOpacity: { beams: 25 } } } };
  paintBattle(context, view, assets, undefined, false, { clock: 1000, animations: new Map() });
  assert.equal(strokes.length, 3);
  assert.ok(Math.abs(strokes[0].width - 78.75) < 0.0001);
  assert.ok(strokes.every(stroke => stroke.alpha === 0.25));
  assert.ok(endpoints.every(([x, y]) => x === 300 && y === 110));
});

test("beam interpolation keeps its origin attached while its endpoint and fade share the physical phase", async () => {
  const target = new URL("../../../.local/pokepath/native-ui-test/interpolate-beam.mjs", import.meta.url);
  await require("esbuild").build({ entryPoints: [new URL("../web/battle-renderer.ts", import.meta.url).pathname], outfile: target.pathname, bundle: true, platform: "node", format: "esm" });
  const { interpolateBattle } = await import(target.href);
  const beam = { id: 11, kind: "beam", x: 10, y: 20, targetX: 110, targetY: 20, lifeMS: 240 };
  const previous = { epoch: 1, route: 0, clock: 0, physicsClock: 240, team: [], towers: [], enemies: [], projectiles: [beam], events: [] };
  const current = { ...previous, clock: 20, physicsClock: 260, projectiles: [{ ...beam, x: 12, targetX: 130, lifeMS: 260 }] };
  const middle = interpolateBattle(current, previous, 0.5).projectiles[0];
  assert.equal(middle.x, 11);
  assert.equal(middle.targetX, 120);
  assert.equal(middle.lifeMS, 250);
  const removed = interpolateBattle({ ...current, projectiles: [] }, previous, 0.5).projectiles[0];
  assert.equal(removed.x, 10);
  assert.equal(removed.lifeMS, 250);
});
