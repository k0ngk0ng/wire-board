import { BombardmentReference } from "./reference.mjs";
import { paintBombardmentEffect, paintBombardmentAim } from "./native.mjs";
const texture = document.createElement("canvas");
texture.width = 12; texture.height = 8;
const textureContext = texture.getContext("2d");
textureContext.fillStyle = "#ff8844"; textureContext.fillRect(0, 0, 6, 8);
textureContext.fillStyle = "#3488cc"; textureContext.fillRect(6, 0, 6, 8);
const shellImage = texture;
Object.defineProperty(shellImage, "complete", { value: true });
const results = [];
for (const kind of ["image", "fallback", "lightning", "circleAim", "rectangleAim"]) for (let index = 0; index < 6; index++) {
  const progress = [0, 0.2, 0.75, 1, 1, 1][index];
  const impacted = index >= 3;
  const impactMS = [0, 0, 0, 0, 200, 400][index];
  const opacity = index % 3 === 1 ? 0.35 : 1;
  const color = ["#9933cc", "#00ff00", "#ac92e6"][index % 3];
  const origin = { x: 100 + index * 10, y: 70 };
  const destination = { x: 285, y: 300 };
  const boltPoints = Array.from({ length: 7 }, (_, part) => ({ x: destination.x + (part > 0 && part < 6 ? Math.sin(part * 14) * 8 : 0), y: destination.y - 260 + part * 260 / 6 }));
  const radius = 18 + index * 3;
  const effect = { kind: kind === "lightning" ? "lightningArea" : "bombardment", origin, destination, ageMS: progress * 500, durationMS: 500, impacted, impactMS, radius, boltPoints };
  const reference = document.querySelector("#reference").getContext("2d");
  const native = document.querySelector("#native").getContext("2d");
  reference.clearRect(0, 0, 512, 512); native.clearRect(0, 0, 512, 512);
  const original = new BombardmentReference();
  original.ctx = reference;
  original.center = origin;
  original.ability = { id: kind === "rectangleAim" ? "rockThrow" : "none" };
  original.pokemon = { bombardmentSplash: radius * 2, bombardmentArea: radius * 2, specie: { color } };
  original.bombardZone = { ...destination, radius };
  original.bombardShells = [{ impacted, progress, impactTime: impactMS, from: origin, to: destination, isLightning: kind === "lightning", boltPoints }];
  original.bombardShellSprite = kind === "image" ? shellImage : undefined;
  original.getEffectOpacity = category => category === "areas" ? opacity * 0.7 : opacity;
  const aim = { origin, aim: destination, radius, width: kind === "rectangleAim" ? radius * 10 : 0, height: kind === "rectangleAim" ? radius * 2 : 0 };
  const before = JSON.stringify([effect, aim]);
  const isAim = kind.endsWith("Aim");
  original[isAim ? "aim" : "shell"]();
  const handled = isAim ? paintBombardmentAim(native, aim, { color, opacity }) : paintBombardmentEffect(native, effect, { color, projectileOpacity: opacity, areaOpacity: opacity * 0.7, image: kind === "image" ? shellImage : undefined });
  const left = reference.getImageData(0, 0, 512, 512).data, right = native.getImageData(0, 0, 512, 512).data;
  let differences = 0, nonzero = 0, maxChannel = 0;
  for (let channel = 0; channel < left.length; channel++) {
    if (left[channel] !== right[channel]) differences++;
    if (left[channel]) nonzero++;
    maxChannel = Math.max(maxChannel, Math.abs(left[channel] - right[channel]));
  }
  results.push({ kind, index, impactMS, opacity, handled, unchanged: before === JSON.stringify([effect, aim]), expectedVisible: isAim || !impacted || impactMS > 0 && impactMS < 400, differences, nonzero, maxChannel, reference: reference.canvas.toDataURL("image/png").split(",")[1], native: native.canvas.toDataURL("image/png").split(",")[1] });
}
window.results = results; document.documentElement.dataset.ready = "true";
