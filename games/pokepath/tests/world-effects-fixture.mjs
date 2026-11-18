import { SpikeZone, ShadowRenderer, SingularityRenderer } from "./reference.mjs";
import { paintPersistentZone, paintSingularityOverlay } from "./native.mjs";

const cases = [];
for (const kind of ["spikes", "web", "magmaSpikes", "smoke", "shadowTag"]) {
  for (const [index, ageMS, opacity, particles] of [[0, 0, 1, true], [1, 400, 1, true], [2, 1300, 0.6, true], [3, 2475, 1, false], [4, 2600, 1, true]]) cases.push({ kind, index, ageMS, opacity, particles, expectedVisible: ageMS > 0 && ageMS < (kind === "shadowTag" ? 2000 : 2600) });
}
for (const phase of ["waiting", "pulling"]) {
  const ages = phase === "waiting" ? [0, 500, 999, 1000] : [0, 20, 225, 449, 450];
  ages.forEach((ageMS, index) => cases.push({ kind: `singularity-${phase}`, phase, ageMS, index, opacity: index === 1 ? 0.6 : 1, particles: true, expectedVisible: phase === "waiting" || [0, 20, 225].includes(ageMS) }));
}

const results = [];
for (const pose of cases) {
  const { kind, ageMS, opacity, particles } = pose;
  const singularity = kind.startsWith("singularity-");
  const parameters = { kind: singularity ? "singularity" : kind, phase: pose.phase, x: 128, y: 128, radius: singularity ? 90 : 60, innerRadius: kind === "magmaSpikes" ? 24 : 0, ageMS, lifeMS: 2600 };
  const reference = document.querySelector("#reference").getContext("2d");
  const native = document.querySelector("#native").getContext("2d");
  reference.clearRect(0, 0, 256, 256);
  native.clearRect(0, 0, 256, 256);
  const now = 1730000012345;
  const realNow = Date.now;
  const realPerformanceNow = performance.now;
  Date.now = () => now;
  performance.now = () => 12345;
  try {
    if (singularity) {
      const original = new SingularityRenderer();
      original.ctx = reference;
      original.position = {};
      original.width = 0;
      original.height = 0;
      original.draw = () => {};
      original.tower = { main: { mute: [true, true, true, true, true], area: { enemies: [] } } };
      original[pose.phase]({ phase: pose.phase, targetX: 128, targetY: 128, radius: 90, waitTimer: ageMS, waitDuration: 1000, pullTimer: ageMS, pullDuration: 450 });
    } else if (kind === "shadowTag") {
      const original = new ShadowRenderer();
      original.ctx = reference;
      original.shadowTagZones = ageMS < 2000 ? [{ x: 128, y: 128, radius: Math.min(60, ageMS * 60 / 2000), maxRadius: 60, elapsed: ageMS }] : [];
      original.drawShadowTagZones();
    } else {
      const tower = { ctx: reference, main: { particles }, getEffectOpacity: () => opacity };
      const original = new SpikeZone({ x: 128, y: 128 }, tower, { radius: 60, innerRadius: parameters.innerRadius, duration: 2600, slow: kind === "web" || kind === "smoke", smoke: kind === "smoke", magma: kind === "magmaSpikes" });
      original.remaining = 2600 - ageMS;
      original.active = ageMS < 2600;
      original.draw();
    }
  } finally {
    Date.now = realNow;
    performance.now = realPerformanceNow;
  }
  const before = JSON.stringify(parameters);
  const handled = singularity ? paintSingularityOverlay(native, parameters) : paintPersistentZone(native, parameters, now, { opacity, particles, motionNow: 12345 });
  const left = reference.getImageData(0, 0, 256, 256).data;
  const right = native.getImageData(0, 0, 256, 256).data;
  let differences = 0;
  let nonzero = 0;
  let maxChannel = 0;
  for (let channel = 0; channel < left.length; channel++) {
    if (left[channel] !== right[channel]) differences++;
    if (left[channel]) nonzero++;
    maxChannel = Math.max(maxChannel, Math.abs(left[channel] - right[channel]));
  }
  results.push({ ...pose, handled, unchanged: before === JSON.stringify(parameters), differences, nonzero, maxChannel, reference: reference.canvas.toDataURL("image/png").split(",")[1], native: native.canvas.toDataURL("image/png").split(",")[1] });
}
window.results = results;
document.documentElement.dataset.ready = "true";
