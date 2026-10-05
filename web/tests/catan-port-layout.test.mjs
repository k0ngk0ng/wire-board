import test from "node:test";
import assert from "node:assert/strict";
import { catanPortLayout } from "../src/catan-port-layout.ts";
import { fishGroundGeometry } from "../src/catan-fishing-state.ts";

// The problematic coast has two fishing corners flanking a third, port edge.
// Geometry is normalized from the Wonders board; all six rotations matter.
function fixture(angle = 0) {
  const rotate = ([x, y]) => ({
    x: x * Math.cos(angle) - y * Math.sin(angle),
    y: x * Math.sin(angle) + y * Math.cos(angle),
  });
  const coordinates = [
    [0, -80],
    [-Math.sqrt(3) * 20, -60],
    [-Math.sqrt(3) * 20, -20],
    [0, 0],
    [Math.sqrt(3) * 20, -20],
    [Math.sqrt(3) * 20, -60],
    [-500, -500],
    [500, 500],
  ];
  const land = rotate([-Math.sqrt(3) * 20, 20]);
  return {
    hexSize: 40,
    vertices: coordinates.map((v, id) => ({ id, ...rotate(v) })),
    edges: [{ id: 0, a: 2, b: 3, tiles: [0] }],
    ports: [{ edge: 0, resource: -1 }],
    tiles: [{ id: 0, ...land, vertices: [2, 3], resource: 2, number: 3 }],
    seafarers: { scenario: "wonders", pirate: -1 },
    fishing: {
      map: {
        grounds: [
          { number: 8, vertices: [0, 1, 2] },
          { number: 10, vertices: [3, 4, 5] },
        ],
      },
    },
  };
}
function collides(p, g) {
  return g.fishing.map.grounds.some((ground) => {
    const f = fishGroundGeometry(g, ground.vertices);
    return (
      Math.abs(p.px - f.labelX) < p.size / 2 + g.hexSize * 0.2 &&
      Math.abs(p.py - f.labelY) < p.size / 2 + g.hexSize * 0.2
    );
  });
}

test("port callout clears flanking fishing numbers without moving the gameplay edge", () => {
  for (let turn = 0; turn < 6; turn++) {
    const g = fixture((turn * Math.PI) / 3),
      saved = structuredClone(g);
    const original = catanPortLayout({ ...g, fishing: undefined })[0];
    assert.equal(collides(original, g), true);
    const moved = catanPortLayout(g)[0];
    assert.equal(collides(moved, g), false);
    assert.deepEqual(g, saved);
    assert.equal(moved.port.edge, 0);
    assert.deepEqual(moved.a, g.vertices[2]);
    assert.deepEqual(moved.b, g.vertices[3]);
    assert.equal(moved.size, original.size);
    assert.deepEqual(catanPortLayout(g), [moved]);
  }
});

test("ordinary ports and clear fishing coasts retain their original geometry", () => {
  const g = fixture(),
    original = catanPortLayout({ ...g, fishing: undefined });
  g.fishing.map.grounds = [];
  assert.deepEqual(catanPortLayout(g), original);
  const base = catanPortLayout({ ...g, seafarers: undefined })[0];
  assert.equal(base.size, 52);
  assert.ok(
    Math.abs(Math.hypot(base.px - base.x, base.py - base.y) - 34) < 1e-8,
  );
});
