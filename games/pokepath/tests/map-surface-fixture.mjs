import { AssetStore } from "../web/assets";
import { paintBattle } from "../web/battle-renderer";

const { catalog, input, parameters } = window.oracleData;
const store = new AssetStore(new URL("/pokepath/v1/", location.href), () => {});
await store.initialize();
const canvas = document.createElement("canvas");
Object.assign(canvas.style, { position: "absolute", left: "259px", top: "25px", transform: "none", outline: "none" });
document.querySelector("#root").append(canvas);
const ctx = canvas.getContext("2d", { willReadFrequently: true });
const present = { clock: 0, animations: new Map() };
let physicsClock = 0, frame = 0, current;
async function paint(id, mode, flags = {}, delta = 0) {
  const route = { ...catalog.routes[id], waves: {}, paths: [], placements: [] };
  const view = { epoch: id + 1, route: id, wave: 1, paused: delta === 0, clock: physicsClock += delta, physicsClock, enemies: [], team: [], towers: [], tiles: [], box: [], projectiles: [], events: [], progression: { settings: { mapEffectsMode: mode }, secrets: flags.secrets ?? {}, secretItems: flags.secretItems ?? {} } };
  if (current !== id || flags !== window.lastMapFlags) await store.prepare(view, catalog, route);
  current = id;
  window.lastMapFlags = flags;
  if (canvas.width !== route.width) canvas.width = route.width;
  if (canvas.height !== route.height) canvas.height = route.height;
  present.clock = frame++ * 1000 / 60;
  paintBattle(ctx, view, { route, catalog, images: store.images, urls: store.urls, ready: true }, undefined, false, present);
}
const measure = async () => {
  const { width, height } = canvas;
  const bytes = await crypto.subtle.digest("SHA-256", ctx.getImageData(0, 0, width, height).data);
  return { width, height, sha256: Array.from(new Uint8Array(bytes)).map(byte => byte.toString(16).padStart(2, "0")).join("") };
};
const renderSeries = async step => {
  const flags = step.flags ?? {};
  for (let index = 0; index < step.frames; index++) await paint(step.route, step.mode, flags, step.delta);
  return measure();
};
window.renderMapStep = async index => { window.mapSurface = await renderSeries(input.timeline[index]); };
if (parameters.scene === "catalog") {
  const hashes = [];
  const flags = {};
  for (const id of input.routes) { await paint(id, 0, flags); hashes.push({ id, ...await measure() }); }
  await paint(0, 0, flags);
  window.mapSurface = { routes: hashes };
} else if (parameters.scene === "secrets") {
  const results = [];
  for (const [id, flags] of input.secrets) {
    await paint(id, 0);
    const from = await measure();
    await paint(id, 0, flags);
    const to = await measure();
    results.push({ id, from, to });
  }
  window.mapSurface = { secrets: results };
} else await window.renderMapStep(0);
