import { ExplosionReference } from "./reference.mjs";
import { paintShadowExplosion } from "./native.mjs";
const results = [];
for (let index = 0; index < 6; index++) {
  const ageMS = [0, 20, 120, 225, 400, 450][index];
  const effect = { x: 256, y: 248, radius: 60, ageMS, durationMS: 450 };
  const reference = document.querySelector("#reference").getContext("2d"), native = document.querySelector("#native").getContext("2d");
  reference.clearRect(0, 0, 512, 512); native.clearRect(0, 0, 512, 512);
  const original = new ExplosionReference();
  original.ctx = reference;
  original.shadowTagExplosions = [{ x: effect.x, y: effect.y + 8, radius: effect.radius, elapsed: ageMS, duration: effect.durationMS }];
  original.drawShadowTagExplosions(0);
  const before = JSON.stringify(effect);
  const handled = paintShadowExplosion(native, effect);
  const left = reference.getImageData(0, 0, 512, 512).data, right = native.getImageData(0, 0, 512, 512).data;
  let differences = 0, nonzero = 0, maxChannel = 0;
  for (let channel = 0; channel < left.length; channel++) {
    if (left[channel] !== right[channel]) differences++;
    if (left[channel]) nonzero++;
    maxChannel = Math.max(maxChannel, Math.abs(left[channel] - right[channel]));
  }
  results.push({ kind: "shadowExplosion", index, ageMS, handled, unchanged: before === JSON.stringify(effect), expectedVisible: ageMS < 450, differences, nonzero, maxChannel, reference: reference.canvas.toDataURL("image/png").split(",")[1], native: native.canvas.toDataURL("image/png").split(",")[1] });
}
window.results = results; document.documentElement.dataset.ready = "true";
