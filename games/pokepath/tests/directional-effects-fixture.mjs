import { DirectionalReference } from "./reference.mjs";
import { paintDirectionalAttack, createDirectionalParticles } from "./native.mjs";
const results = [];
const random = () => {
  let seed = 17411;
  return () => { seed = (seed * 1664525 + 1013904223) >>> 0; return seed / 4294967296; };
};
for (const ability of ["icyWind", "dazzlingGleam"]) for (let index = 0; index < 6; index++) {
  const ageMS = [0, 40, 200, 390, 500, 520][index];
  const angle = [0, 0.7, -1.4, 2.4, 0.2, -2][index];
  const particles = index % 2 === 0;
  const opacity = index % 3 === 1 ? 0.35 : 1;
  const origin = { x: 256, y: 256 };
  const reference = document.querySelector("#reference").getContext("2d");
  const native = document.querySelector("#native").getContext("2d");
  reference.clearRect(0, 0, 512, 512);
  native.clearRect(0, 0, 512, 512);
  const original = new DirectionalReference();
  original.ctx = reference;
  original.center = origin;
  original.range = 170;
  original.main = { particles };
  original.directionalAreaEffects = [];
  original.getEffectOpacity = () => opacity;
  const realNow = Date.now;
  const realRandom = Math.random;
  Date.now = () => 1730000012345 - ageMS;
  Math.random = random();
  original.directionalAreaAttack({ center: { x: origin.x + Math.cos(angle) * 100, y: origin.y + Math.sin(angle) * 100 } }, ability);
  Date.now = () => 1730000012345;
  try { original.draw(); }
  finally { Date.now = realNow; Math.random = realRandom; }
  const generated = createDirectionalParticles(ability, random());
  const distributionMatches = JSON.stringify(generated) === JSON.stringify(original.directionalAreaEffects[0].particles);
  if (!distributionMatches) throw Error(`${ability}: particle generator differs`);
  const effect = { origin, angle: original.directionalAreaEffects[0].baseAngle, radius: 170, halfAngle: Math.PI / 4, ageMS, durationMS: 520 };
  const before = JSON.stringify([effect, generated]);
  const handled = paintDirectionalAttack(native, effect, { ability, opacity, particles, now: 1730000012345, startedAt: 1730000012345 - ageMS, particleData: generated });
  const left = reference.getImageData(0, 0, 512, 512).data;
  const right = native.getImageData(0, 0, 512, 512).data;
  let differences = 0, nonzero = 0, maxChannel = 0;
  for (let channel = 0; channel < left.length; channel++) {
    if (left[channel] !== right[channel]) differences++;
    if (left[channel]) nonzero++;
    maxChannel = Math.max(maxChannel, Math.abs(left[channel] - right[channel]));
  }
  results.push({ kind: ability, index, ageMS, opacity, particles, distributionMatches, handled, unchanged: before === JSON.stringify([effect, generated]), expectedVisible: ageMS > 0 && ageMS < 520, differences, nonzero, maxChannel, reference: reference.canvas.toDataURL("image/png").split(",")[1], native: native.canvas.toDataURL("image/png").split(",")[1] });
}
window.results = results;
document.documentElement.dataset.ready = "true";
