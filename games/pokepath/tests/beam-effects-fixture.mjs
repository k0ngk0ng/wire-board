import { BeamReference, LinkBeamReference } from "./reference.mjs";
import { paintBeamEffect, paintLinkEffect } from "./native.mjs";
const results = [];
for (const ability of ["solarBeam", "psybeam", "hydroCannon", "moongeistBeam", "anchorShot", "signalBeam", "cryogenicCannon", "link"]) for (let index = 0; index < 6; index++) {
  const angle = [0, 0.7, -1.4, 2.4, 0.2, -2][index];
  const alpha = [1, 0.8, 0.5, 0.2, 0.01, 0][index];
  const color = ["#9933cc", "#00ff00", "#ac92e6"][index % 3];
  const opacity = index % 3 === 1 ? 0.35 : 1;
  const particles = index % 2 === 0;
  const from = { x: 256 - Math.cos(angle) * 130, y: 256 - Math.sin(angle) * 130 };
  const to = { x: from.x + Math.cos(angle) * 260, y: from.y + Math.sin(angle) * 260 };
  const beam = { from, to, extendedTo: to, width: 8 + index * 3, alpha };
  const reference = document.querySelector("#reference").getContext("2d");
  const native = document.querySelector("#native").getContext("2d");
  reference.clearRect(0, 0, 512, 512);
  native.clearRect(0, 0, 512, 512);
  const original = ability === "link" ? new LinkBeamReference() : new BeamReference();
  Object.assign(original, beam, { ctx: reference, tower: { pokemon: { specie: { color } }, ability: { id: ability }, main: { particles }, getEffectOpacity: () => opacity }, fromTower: { getEffectOpacity: () => opacity } });
  const realNow = Date.now;
  Date.now = () => 1730000012345;
  try { original.draw(); }
  finally { Date.now = realNow; }
  const before = JSON.stringify(beam);
  const handled = ability === "link" ? paintLinkEffect(native, beam, { opacity, now: 1730000012345 }) : paintBeamEffect(native, beam, { ability, color, opacity, particles, now: 1730000012345 });
  const left = reference.getImageData(0, 0, 512, 512).data;
  const right = native.getImageData(0, 0, 512, 512).data;
  let differences = 0, nonzero = 0, maxChannel = 0;
  for (let channel = 0; channel < left.length; channel++) {
    if (left[channel] !== right[channel]) differences++;
    if (left[channel]) nonzero++;
    maxChannel = Math.max(maxChannel, Math.abs(left[channel] - right[channel]));
  }
  results.push({ kind: ability, index, alpha, color, opacity, particles, handled, unchanged: before === JSON.stringify(beam), expectedVisible: ability === "link" || alpha >= 0.2, differences, nonzero, maxChannel, reference: reference.canvas.toDataURL("image/png").split(",")[1], native: native.canvas.toDataURL("image/png").split(",")[1] });
}
window.results = results;
document.documentElement.dataset.ready = "true";
