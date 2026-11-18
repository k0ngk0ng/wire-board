import { ReferenceEnemyDrawing } from "./reference.mjs";
import { paintEnemyStatus, paintDamageText } from "./native.mjs";
const reference = document.querySelector("#reference");
const native = document.querySelector("#native");
const originalNow = Date.now;
let clock = 0;
Date.now = () => clock;
const condition = (kind, durationMs = 5000, remainingMs = 2000) => ({ kind, durationMs, remainingMs });
const cases = [
  { name: "slow", conditions: [condition("slow")], cc: true, particles: false },
  { name: "stun", conditions: [condition("stun", 3000, 1234)], cc: true, particles: false },
  { name: "both", conditions: [condition("slow"), condition("stun")], cc: true, particles: false },
  { name: "expired", conditions: [condition("slow", 5000, 0)], cc: true, particles: false },
  { name: "cc-off", conditions: [condition("slow")], cc: false, particles: false },
  { name: "particles-off", conditions: [condition("burn")], cc: false, particles: false },
  ...["burn", "scorch", "poison", "nightmare"].flatMap(kind => [0, 333, 1499].map(now => ({ name: `${kind}-${now}`, conditions: [condition(kind)], cc: false, particles: true, now }))),
  { name: "mixed", conditions: [condition("slow"), condition("stun"), condition("burn"), condition("poison"), condition("nightmare")], cc: true, particles: true, now: 1234 },
  ...[["normal", "-15", "#e06666"], ["critical", "-23!", "#ffeb3b"], ["armor", "-18", "#66ccff"], ["burn-text", "-4", "#c96937"], ["miss", "MISS", "#b9c9e0"]].flatMap(([name, text, color]) => [0, 250, 750, 999, 1000].map(ageMS => ({ name: `${name}-${ageMS}`, text, color, ageMS })))
];
window.results = [];
try {
  for (const value of cases) {
    clock = value.now ?? 0;
    const original = reference.getContext("2d", { willReadFrequently: true });
    const actual = native.getContext("2d", { willReadFrequently: true });
    original.clearRect(0, 0, 256, 256);
    actual.clearRect(0, 0, 256, 256);
    const renderer = new ReferenceEnemyDrawing();
    Object.assign(renderer, { ctx: original, position: { x: 112, y: 108 }, width: 32, height: 40, main: { durationCC: value.cc, particles: value.particles }, statusEffects: (value.conditions ?? []).map(condition => ({ type: condition.kind, duration: condition.durationMs / 1000, timer: condition.durationMs - condition.remainingMs })) });
    if (value.text) {
      renderer.floatingTexts = value.ageMS < 1000 ? [{ text: value.text, color: value.color, y: 110 - 60 * value.ageMS / 1000, alpha: Math.max(0.5, 1 - 0.4 * value.ageMS / 1000) }] : [];
      renderer.drawFloatingTexts();
      paintDamageText(actual, { x: 128, y: 110, text: value.text, color: value.color, ageMS: value.ageMS });
    } else {
      renderer.drawStatusEffects();
      paintEnemyStatus(actual, { x: 128, y: 128, width: 32, height: 40, conditions: value.conditions }, clock, { durationCC: value.cc, particles: value.particles });
    }
    const left = original.getImageData(0, 0, 256, 256).data;
    const right = actual.getImageData(0, 0, 256, 256).data;
    let changed = 0;
    let maximum = 0;
    for (let index = 0; index < left.length; index++) if (left[index] !== right[index]) { changed++; maximum = Math.max(maximum, Math.abs(left[index] - right[index])); }
    window.results.push({ name: value.name, changedRGBABytes: changed, maximumChannelDifference: maximum, reference: reference.toDataURL().split(",")[1], native: native.toDataURL().split(",")[1] });
  }
} finally { Date.now = originalNow; }
document.documentElement.dataset.ready = "true";
