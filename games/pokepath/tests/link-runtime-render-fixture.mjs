import { LinkBeam } from "./reference.mjs";
import { paintBattle } from "./native.mjs";

const reference = document.querySelector("#reference").getContext("2d");
const native = document.querySelector("#native").getContext("2d");
const textures = await Promise.all(["#d85c43", "#4678cf", "#57a641"].map(async color => {
  const canvas = new OffscreenCanvas(32, 32), context = canvas.getContext("2d");
  context.fillStyle = color;
  context.fillRect(0, 0, 32, 32);
  context.fillStyle = "#ffffff77";
  context.fillRect(3, 3, 12, 8);
  return createImageBitmap(canvas);
}));
const results = [];
for (const [orientation, angle] of [0, .7, -1.2].entries()) for (const [variation, opacity] of [1, .35, 0].entries()) {
  const from = { x: 256 - Math.cos(angle) * 180, y: 256 - Math.sin(angle) * 180 };
  const to = { x: 256 + Math.cos(angle) * 180, y: 256 + Math.sin(angle) * 180 };
  const ordered = [{ id: 11, pokemonID: 101, ...from }, { id: 13, pokemonID: 103, x: 256, y: 256 }, { id: 12, pokemonID: 102, ...to }];
  const towers = orientation === 1 ? [ordered[2], ordered[0], ordered[1]] : ordered;
  const team = ["plusle", "minun", "charmander"].map((species, index) => ({ id: 101 + index, species, stats: { power: 6, range: 100, attackMS: 10000, criticalPct: 0 }, sprite: { image: `${species}.png`, frames: 1, rows: 1, width: 32, height: 32, frameWidth: 32, frameHeight: 32 } }));
  const assets = { catalog: { species: Object.fromEntries(team.map(pokemon => [pokemon.species, { abilityId: pokemon.species === "plusle" ? "plus" : pokemon.species === "minun" ? "minus" : "burn", sprite: pokemon.sprite }])) }, route: { id: 0, width: 512, height: 512 }, images: new Map(team.map((pokemon, index) => [pokemon.sprite.image, textures[index]])) };
  const view = { epoch: 1, sequence: 0, route: 0, clock: 0, physicsClock: 0, gold: 50, health: 14, team, box: [], towers, enemies: [], projectiles: [], tiles: [], events: [], progression: { settings: { mapEffectsMode: 0, showDamage: false, effectAlpha: 80, effectOpacity: { beams: opacity * 100 } } }, world: { links: [{ first: 11, second: 12, ageMS: 0 }] } };
  const now = 1730000012345 + orientation * 470;
  const link = Object.assign(new LinkBeam(), { ctx: reference, from, to, width: 12, fromTower: { getEffectOpacity: () => opacity * .8 } });
  const before = JSON.stringify(view), savedNow = Date.now;
  Date.now = () => now;
  reference.clearRect(0, 0, 512, 512);
  native.clearRect(0, 0, 512, 512);
  try {
    for (const tower of [...towers].sort((a, b) => a.y - b.y)) {
      reference.imageSmoothingEnabled = false;
      reference.drawImage(textures[tower.pokemonID - 101], tower.x - 16, tower.y - 16);
      if (tower.id === towers[0].id) link.draw();
    }
    link.draw();
    paintBattle(native, view, assets);
  } finally { Date.now = savedNow; }
  const left = reference.getImageData(0, 0, 512, 512).data, right = native.getImageData(0, 0, 512, 512).data;
  let differences = 0, nonzero = 0, maxChannel = 0;
  for (let index = 0; index < left.length; index++) {
    if (left[index] !== right[index]) differences++;
    if (left[index]) nonzero++;
    maxChannel = Math.max(maxChannel, Math.abs(left[index] - right[index]));
  }
  results.push({ kind: "runtimeLink", index: orientation * 3 + variation, opacity, angle, differences, nonzero, maxChannel, handled: true, unchanged: before === JSON.stringify(view), expectedVisible: true, reference: reference.canvas.toDataURL("image/png").split(",")[1], native: native.canvas.toDataURL("image/png").split(",")[1] });
}
for (const texture of textures) texture.close();
window.results = results;
document.documentElement.dataset.ready = "true";
