import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import { classicDefaultVehicleForSeat, classicVehicleByID } from "../web/classic-catalog.ts";
import {
  formatGapDelta,
  formatLeaderGap,
  computeOpponentGaps,
} from "../web/classic-time-gap.ts";
import {
  nfs4CollisionCutoffMeters,
  nfs4CollisionAttenuation,
  selectNfs4CollisionPatch,
  resolveCollisionSpatialSound,
} from "../web/classic-collision-audio.ts";
import {
  minimapBounds,
  minimapTransform,
  orderedMinimapCars,
  minimapMarkerColors,
} from "../web/classic-minimap.ts";

test("waiting seat default vehicle matches server configuration order", () => {
  assert.deepEqual(
    [0, 1, 2, 3, 4, 5].map((seat) => classicDefaultVehicleForSeat(seat)),
    ["p911", "fm50", "ldia", "mcf1", "ccor", "p911"],
  );
  assert.equal(classicVehicleByID(classicDefaultVehicleForSeat(0)).name, "保时捷 911 Turbo");
});

test("time gap formatting follows motorsport and NFS4 rules for ahead, behind, leader and lapped cars", () => {
  assert.equal(formatLeaderGap(0, 0), "领先");
  assert.equal(formatLeaderGap(1240, 0), "+1.24s");
  assert.equal(formatLeaderGap(65040, 0), "+1:05.04");
  assert.equal(formatLeaderGap(12000, 1), "+1 圈");
  assert.equal(formatGapDelta(0, 0), "—");
  assert.equal(formatGapDelta(-850, 0), "-0.85s");
  assert.equal(formatGapDelta(1420, 0), "+1.42s");
  assert.equal(formatGapDelta(-61230, 0), "-1:01.23");
  assert.equal(formatGapDelta(0, 2), "+2 圈");
  assert.equal(formatGapDelta(0, -1), "-1 圈");
  const rows = computeOpponentGaps(
    [
      { seat: 1, rank: 1, gapMs: 0, gapLaps: 0, finishAt: 0 },
      { seat: 0, rank: 2, gapMs: 1240, gapLaps: 0, finishAt: 0 },
      { seat: 2, rank: 3, gapMs: 3100, gapLaps: 0, finishAt: 0 },
      { seat: 3, rank: 4, gapMs: 0, gapLaps: 1, finishAt: 0 },
    ],
    0,
  );
  assert.deepEqual(
    rows.map((r) => [r.seat, r.relativeText, r.leaderText]),
    [
      [1, "-1.24s", "领先"],
      [0, "—", "+1.24s"],
      [2, "+1.86s", "+3.10s"],
      [3, "+1 圈", "+1 圈"],
    ],
  );
});

test("finished cars compute exact finishAt gap even if gapMs was zeroed", () => {
  const rows = computeOpponentGaps(
    [
      { seat: 0, rank: 1, gapMs: 0, gapLaps: 0, finishAt: 105000 },
      { seat: 1, rank: 2, gapMs: 0, gapLaps: 0, finishAt: 106750 },
    ],
    0,
  );
  assert.equal(rows[1].relativeText, "+1.75s");
  assert.equal(rows[1].leaderText, "+1.75s");
});

test("NFS4 collision audio attenuates quadratically and cuts off completely beyond 53.03m", () => {
  assert.ok(Math.abs(nfs4CollisionCutoffMeters - Math.sqrt(5625 / 2)) < 0.01);
  assert.equal(nfs4CollisionAttenuation(0), 1);
  assert.ok(nfs4CollisionAttenuation(25) > 0.75 && nfs4CollisionAttenuation(25) < 0.80);
  assert.equal(nfs4CollisionAttenuation(53.04), 0);
  assert.equal(nfs4CollisionAttenuation(120), 0);
  const distant = resolveCollisionSpatialSound(
    { id: 9, type: "collision", seat: 2, x: 1600, y: 0, intensity: 0.9, at: 1000 },
    { seat: 0, x: 1000, y: 0, angle: 0, height: 0 },
    0,
  );
  assert.equal(distant, null);
  const near = resolveCollisionSpatialSound(
    { id: 10, type: "collision", seat: 2, x: 1200, y: 50, intensity: 0.8, at: 1000 },
    { seat: 0, x: 1000, y: 0, angle: 0, height: 0 },
    0,
  );
  assert.ok(near !== null);
  assert.ok(near.volume > 0 && near.volume < 1);
  const own = resolveCollisionSpatialSound(
    { id: 11, type: "collision", seat: 0, x: 1000, y: 0, intensity: 0.9, at: 1000 },
    { seat: 0, x: 1000, y: 0, angle: 0, height: 0 },
    0,
  );
  assert.ok(own !== null && own.volume > near.volume);
});

test("NFS4 collision patch selection matches gen.bnk intensity bands and avoids repeating mid patches", () => {
  const heavy = selectNfs4CollisionPatch("car", 0.95, 0);
  assert.equal(heavy, 33);
  const lightA = selectNfs4CollisionPatch("wall", 0.15, 0);
  const lightB = selectNfs4CollisionPatch("wall", 0.15, lightA);
  assert.notEqual(lightA, lightB);
  const propPatch = selectNfs4CollisionPatch("prop", 0.6, 0);
  assert.ok([24, 25, 31, 35].includes(propPatch));
});

test("minimap fits tightly to polyline bounds and draws followed car on top with vivid markers", () => {
  const points = [
    { x: 300, y: 500 },
    { x: 1300, y: 500 },
    { x: 1300, y: 1000 },
    { x: 300, y: 1000 },
  ];
  const bounds = minimapBounds(points);
  assert.deepEqual(bounds, { minX: 300, maxX: 1300, minY: 500, maxY: 1000 });
  const tf = minimapTransform(bounds, 220, 150, 14);
  assert.ok(tf.scale > 0.18, `expected tight fit scale > 0.18, got ${tf.scale}`);
  const ordered = orderedMinimapCars(
    [{ seat: 0 }, { seat: 1 }, { seat: 2 }],
    0,
  );
  assert.equal(ordered.at(-1)?.seat, 0);
  assert.equal(minimapMarkerColors.length, 8);
});

test("fullscreen gear badge does not stretch dark background with padding-bottom", () => {
  const css = readFileSync(new URL("../web/style.css", import.meta.url), "utf8");
  const rule = css.match(/\[data-room-fullscreen\]\s+\.racing-gauges\[data-mode="analog"\]\s+\.racing-gauges-gear\s*\{[^}]*\}/);
  assert.ok(rule, "fullscreen analog gear rule should exist");
  assert.doesNotMatch(rule[0], /padding-bottom\s*:/);
});

test("Kindiak Park train animations align with travel curves and step across teleport resets", () => {
  const bytes = readFileSync(new URL("../assets/public/racing/v3/tracks/park.glb", import.meta.url));
  const jsonLen = bytes.readUInt32LE(12);
  const doc = JSON.parse(bytes.subarray(20, 20 + jsonLen).toString("utf8"));
  const binStart = 20 + jsonLen + 8;
  const bin = bytes.subarray(binStart);
  const readAccessor = (index) => {
    const acc = doc.accessors[index];
    const bv = doc.bufferViews[acc.bufferView];
    const width = { SCALAR: 1, VEC3: 3, VEC4: 4 }[acc.type];
    const off = (bv.byteOffset ?? 0) + (acc.byteOffset ?? 0);
    const out = [];
    for (let i = 0; i < acc.count; i++) {
      const row = [];
      for (let k = 0; k < width; k++) row.push(bin.readFloatLE(off + (i * width + k) * 4));
      out.push(width === 1 ? row[0] : row);
    }
    return out;
  };
  const trainAnims = doc.animations.filter((a) => /^object_5(5[6-9]|6[0-5])_loop$/.test(a.name));
  assert.equal(trainAnims.length, 10);
  for (const anim of trainAnims) {
    const times = readAccessor(anim.samplers[0].input);
    const locs = readAccessor(anim.samplers[0].output);
    const rots = readAccessor(anim.samplers[1].output);
    const errors = [];
    for (let i = 0; i < locs.length - 1; i++) {
      const dx = locs[i + 1][0] - locs[i][0];
      const dz = locs[i + 1][2] - locs[i][2];
      const dist = Math.hypot(dx, dz);
      const dt = times[i + 1] - times[i];
      if (dist > 20) {
        assert.ok(dt <= 0.005, `${anim.name} frame ${i} jumps ${dist.toFixed(1)}m over ${dt.toFixed(4)}s instead of stepping`);
        continue;
      }
      if (dist < 0.2) continue;
      const [qx, qy, qz, qw] = rots[i];
      const fx = 2 * (qx * qz + qw * qy);
      const fz = 1 - 2 * (qx * qx + qy * qy);
      const fl = Math.hypot(fx, fz) || 1;
      const dot = Math.abs((fx * dx + fz * dz) / (dist * fl));
      errors.push((Math.acos(Math.min(1, dot)) * 180) / Math.PI);
    }
    const avg = errors.reduce((a, b) => a + b, 0) / errors.length;
    assert.ok(avg < 3, `${anim.name} average curve yaw error ${avg.toFixed(2)} deg >= 3 deg`);
  }
});
