import { AssetStore } from "../web/assets";
import { paintBattle, pokemonAt } from "../web/battle-renderer";

const { catalog, input, parameters } = window.oracleData;
const fixture = input.cases[parameters.scene];
const route = input.route;
const store = new AssetStore(new URL("/pokepath/v1/", location.href), () => {});
await store.initialize();
const resources = [route.background, route.effect, "src/assets/images/icons/curse.png", ...fixture.enemies.map(enemy => catalog.enemies[enemy.key].sprite.image), ...fixture.team.map(pokemon => catalog.species[pokemon.species].sprite.image)];
await Promise.all([...new Set(resources)].map(path => store.image(path)));
const canvas = document.createElement("canvas");
canvas.width = route.width;
canvas.height = route.height;
Object.assign(canvas.style, { position: "absolute", left: "259px", top: "25px", transform: "none", outline: "none" });
document.querySelector("#root").append(canvas);
const view = { ...fixture, epoch: 1, sequence: 0, route: route.id, clock: 0, physicsClock: 0, tiles: fixture.tiles ?? route.placements, projectiles: [], events: [], progression: { challenge: { mirror: fixture.mirror ?? false }, settings: { language: Number(parameters.language), mapEffectsMode: fixture.effect ?? 0, displayHealth: fixture.display ?? 0, particles: Boolean(fixture.particles), durationCC: true } } };
const assets = { catalog, route, images: store.images, urls: store.urls, ready: true, version: 0 };
const serialized = JSON.stringify(view);
const savedNow = Date.now, savedRandom = Math.random;
let seed = 20261118;
Date.now = () => fixture.now ?? 0;
Math.random = () => ((seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0) / 4294967296);
try { paintBattle(canvas.getContext("2d"), view, assets, fixture.selected, Boolean(fixture.deployment), { clock: 0, animations: new Map(), allRanges: fixture.all, deployment: fixture.deployment }); }
finally { Date.now = savedNow; Math.random = savedRandom; }
window.battleObservation = { width: canvas.width, height: canvas.height, unchanged: serialized === JSON.stringify(view), hit: fixture.hit ? pokemonAt(view, fixture.hit)?.id : undefined };
