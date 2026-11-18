import { AttackReference } from "./reference.mjs";
import { paintAttackModeEffect } from "./native.mjs";

const results = [];
for (const kind of ["arc", "dune", "wave"]) for (let index = 0; index < 6; index++) {
  const angle = [0, 0.7, -1.4, 2.4, 0.2, -2][index];
  const travelled = [0, 60, 130, 30, 85, 160][index];
  const color = ["#9933cc", "#00ff00", "#ac92e6"][index % 3];
  const particles = index % 2 === 0;
  const opacity = index % 3 === 1 ? 0.35 : 1;
  const origin = { x: 256, y: 256 };
  const effect = { kind, origin, position: { x: origin.x + Math.cos(angle) * travelled, y: origin.y + Math.sin(angle) * travelled }, angle, travelled, radius: kind === "wave" ? [0, 20, 70, 100, 170, 190][index] : 45, maxRadius: 200, halfAngle: Math.PI * 0.32, width: kind === "wave" ? 6 : 160, ageMS: 400, delayMS: 0, durationMS: 3000 };
  const reference = document.querySelector("#reference").getContext("2d");
  const native = document.querySelector("#native").getContext("2d");
  reference.clearRect(0, 0, 512, 512);
  native.clearRect(0, 0, 512, 512);
  const original = new AttackReference();
  original.ctx = reference;
  original.main = { particles };
  original.pokemon = { specie: { color } };
  original.getEffectOpacity = () => opacity;
  const wave = { originX: origin.x, originY: origin.y, dirX: Math.cos(angle), dirY: Math.sin(angle), traveled: travelled, dirAngle: angle, arcHalfAngle: Math.PI * 0.32, arcRadius: 45, bandHalfWidth: 80, radius: effect.radius, alpha: 0.9 * (1 - effect.radius / effect.maxRadius), width: effect.width };
  original.psychoCutWaves = [wave];
  original.sandsearWaves = [wave];
  const realNow = Date.now;
  const realPerformanceNow = performance.now;
  Date.now = () => 1730000012345;
  performance.now = () => 12345;
  try { original[kind](wave); }
  finally { Date.now = realNow; performance.now = realPerformanceNow; }
  const before = JSON.stringify(effect);
  const handled = paintAttackModeEffect(native, effect, { color, particles, opacity, now: 1730000012345, motionNow: 12345 });
  const left = reference.getImageData(0, 0, 512, 512).data;
  const right = native.getImageData(0, 0, 512, 512).data;
  let differences = 0;
  let nonzero = 0;
  let maxChannel = 0;
  for (let channel = 0; channel < left.length; channel++) {
    if (left[channel] !== right[channel]) differences++;
    if (left[channel]) nonzero++;
    maxChannel = Math.max(maxChannel, Math.abs(left[channel] - right[channel]));
  }
  results.push({ kind, index, angle, travelled, color, opacity, particles, handled, unchanged: before === JSON.stringify(effect), expectedVisible: kind !== "wave" || effect.radius > 0, differences, nonzero, maxChannel, reference: reference.canvas.toDataURL("image/png").split(",")[1], native: native.canvas.toDataURL("image/png").split(",")[1] });
}
window.results = results;
document.documentElement.dataset.ready = "true";
