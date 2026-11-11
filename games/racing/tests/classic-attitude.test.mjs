import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import * as T from "../../../web/node_modules/three/build/three.module.js";
import { applyClassicRoadAttitude } from "../web/classic-attitude.ts";

// Failure cases, listed before implementation: reversed lateral bank, reversed
// uphill/downhill pitch, a heading-dependent left/right swap, an incorrect
// Euler composition on simultaneous slopes, and an offset on a flat road.
// Backend angles are two projected plane slopes, not consecutive Euler angles.
function pose(normal, angle) {
  const n = new T.Vector3(...normal).normalize();
  const pitch = Math.atan(-(n.x * Math.cos(angle) + n.z * Math.sin(angle)) / n.y);
  const roll = Math.atan(-(n.x * Math.sin(angle) - n.z * Math.cos(angle)) / n.y);
  const root = new T.Group(), road = new T.Group();
  root.rotation.y = Math.PI / 2 - angle;
  root.add(road);
  applyClassicRoadAttitude(road.rotation, pitch, roll);
  root.updateMatrixWorld(true);
  return { n, pitch, roll, root, road };
}

function verify(normal, angle) {
  const { n, road, pitch, roll } = pose(normal, angle);
  const up = new T.Vector3(0, 1, 0).transformDirection(road.matrixWorld);
  assert.ok(up.distanceTo(n) < 1e-12,
    `up=${up.toArray()}, normal=${n.toArray()}, heading=${angle}, pitch=${pitch}, roll=${roll}`);
  const front = new T.Vector3(0, 0, 1).transformDirection(road.matrixWorld);
  assert.ok(Math.abs(front.dot(n)) < 1e-12, "vehicle forward must lie in the road plane");
  assert.ok(Math.abs(Math.atan2(front.z, front.x) - angle) < 1e-12 ||
    Math.abs(Math.atan2(front.z, front.x) - angle + 2 * Math.PI) < 1e-12,
    "GLB +Z must preserve the backend heading");
}

test("flat roads, both lateral banks, and both longitudinal slopes at eight headings", () => {
  for (let i = 0; i < 8; i++) {
    const angle = i * Math.PI / 4;
    for (const normal of [[0,1,0], [.2,1,0], [-.2,1,0], [0,1,.25], [0,1,-.25]])
      verify(normal, angle);
  }
});

test("compound grades align the chassis up vector with the actual plane normal", () => {
  for (const normal of [[.3,1,.4], [-.3,1,.4], [.3,1,-.4], [-.3,1,-.4]])
    for (let i = 0; i < 16; i++) verify(normal, i * Math.PI / 8);
});

test("all four original track normals align at eight vehicle headings", () => {
  for (const track of ["france", "coastal", "uk", "park"]) {
    const route = JSON.parse(readFileSync(new URL(`../assets/public/racing/v3/tracks/${track}.route.json`, import.meta.url)));
    for (const point of route.points)
      for (let i = 0; i < 8; i++) verify([point.nx, point.ny, point.nz], i * Math.PI / 4);
  }
});

test("positive roll raises the model's local +X side and positive pitch raises its +Z nose", () => {
  const rotation = new T.Euler();
  applyClassicRoadAttitude(rotation, 0, .2);
  assert.ok(new T.Vector3(1,0,0).applyEuler(rotation).y > 0);
  applyClassicRoadAttitude(rotation, .2, 0);
  assert.ok(new T.Vector3(0,0,1).applyEuler(rotation).y > 0);
});
