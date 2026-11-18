import { MeleeReference } from "./reference.mjs";
import { paintAttackModeEffect } from "./native.mjs";

const results = [];
for (const ability of ["secretSword", "ceaselessEdge", "intrepidSword", "shadowClaw", "fireLash"]) for (let index = 0; index < 6; index++) {
  const elapsed = [0, 40, 100, 180, 270, 400][index];
  const kind = ability === "shadowClaw" ? "claw" : ability === "fireLash" ? "whip" : "blade";
  const duration = kind === "claw" ? 340 : kind === "whip" ? 400 : 300;
  const angle = [0, 0.7, -1.4, 2.4, 0.2, -2][index];
  const origin = { x: 256, y: 256 };
  const direction = index % 2 ? -1 : 1;
  const radius = kind === "claw" ? index % 2 ? 78 : 52 : 140;
  const opacity = index % 3 === 1 ? 0.35 : 1;
  const effect = { kind: "sweep", shape: kind === "blade" ? "blade" : kind === "claw" ? "targetCircle" : "cone", origin, position: origin, angle, direction, travelled: 0, radius, halfAngle: Math.PI / 2, width: 50, ageMS: elapsed, delayMS: 0, durationMS: duration, context: { AbilityID: ability } };
  const reference = document.querySelector("#reference").getContext("2d");
  const native = document.querySelector("#native").getContext("2d");
  reference.clearRect(0, 0, 512, 512);
  native.clearRect(0, 0, 512, 512);
  const original = new MeleeReference();
  original.ctx = reference;
  original.center = origin;
  original.range = radius;
  original.getEffectOpacity = () => opacity;
  original.secretSwordEffects = [{ start: 1730000012345 - elapsed, duration, direction, baseAngle: angle, ability }];
  original.shadowClawEffects = [{ start: 1730000012345 - elapsed, duration, x: origin.x, y: origin.y, radius, angle, empowered: radius === 78, scale: radius / 52 }];
  original.fireLashEffect = { elapsed, duration, angle, halfAngle: Math.PI / 2, radius };
  const realNow = Date.now;
  Date.now = () => 1730000012345;
  try { original[kind](); }
  finally { Date.now = realNow; }
  const before = JSON.stringify(effect);
  const handled = paintAttackModeEffect(native, effect, { opacity, now: 1730000012345 });
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
  results.push({ kind: ability, index, elapsed, radius, opacity, direction, handled, unchanged: before === JSON.stringify(effect), expectedVisible: elapsed < duration && (kind !== "claw" || elapsed > 0 || radius === 78), differences, nonzero, maxChannel, reference: reference.canvas.toDataURL("image/png").split(",")[1], native: native.canvas.toDataURL("image/png").split(",")[1] });
}
window.results = results;
document.documentElement.dataset.ready = "true";
