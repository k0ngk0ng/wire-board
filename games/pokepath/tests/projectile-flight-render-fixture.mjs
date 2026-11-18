import { Projectile, textures } from "./reference.mjs";
import { paintBattle } from "./native.mjs";

const results = [];
const reference = document.querySelector("#reference").getContext("2d");
const native = document.querySelector("#native").getContext("2d");
const settings = { mapEffects: false, effectAlpha: 100, effectOpacity: { projectiles: 100 }, showDamage: false };
for (const [kind, { sprite, bytes }] of Object.entries(textures)) {
  const texture = await createImageBitmap(new Blob([Uint8Array.from(atob(bytes), value => value.charCodeAt(0))], { type: "image/png" }));
  for (const [index, scene] of [
    { x: 64, y: 70, targetX: 119, targetY: 102, frame: 0, opacity: 100 },
    { x: 89.25, y: 84.5, targetX: 50, targetY: 22, frame: 1, opacity: 100 },
    { x: 140.25, y: 69, targetX: 148, targetY: 69, frame: 0, opacity: 35 },
  ].entries()) {
    const { x, y, targetX, targetY, frame, opacity } = scene;
    const projectile = { id: 7, kind: "single", towerID: 1, targetID: 6, x, y, targetX, targetY, sprite };
    const view = { epoch: 1, route: 0, clock: 0, enemies: [], towers: [], team: [], tiles: [], projectiles: [projectile], events: [], progression: { settings: { ...settings, effectOpacity: { projectiles: opacity } } } };
    const assets = { route: { id: 0, width: 192, height: 192 }, images: new Map([[sprite.image, texture]]) };
    const context = { clock: 0, animations: new Map() };
    const before = JSON.stringify(view);
    paintBattle(native, view, assets, undefined, false, context);
    if (frame > 0) {
      context.clock = 100;
      paintBattle(native, view, assets, undefined, false, context);
    }
    const { frameWidth: width, frameHeight: height } = sprite;
    reference.clearRect(0, 0, 192, 192);
    const original = Object.assign(new Projectile(), { ctx: reference, sprite: texture, loaded: true, width, height, position: { x: x - width / 2, y: y - height / 2 }, velocity: { x: 0, y: 0 }, frames: { current: frame, direction: 0, max: sprite.frames, hold: 5, elapsed: frame * 5 }, tower: { getEffectOpacity: () => opacity / 100 } });
    original.draw();
    const left = reference.getImageData(0, 0, 192, 192).data;
    const right = native.getImageData(0, 0, 192, 192).data;
    let differences = 0, nonzero = 0, maxChannel = 0;
    for (let channel = 0; channel < left.length; channel++) {
      if (left[channel] !== right[channel]) differences++;
      if (left[channel]) nonzero++;
      maxChannel = Math.max(maxChannel, Math.abs(left[channel] - right[channel]));
    }
    results.push({ kind, index, scene, handled: true, unchanged: before === JSON.stringify(view), expectedVisible: true, differences, nonzero, maxChannel, reference: reference.canvas.toDataURL("image/png").split(",")[1], native: native.canvas.toDataURL("image/png").split(",")[1] });
  }
}
window.results = results;
document.documentElement.dataset.ready = "true";
