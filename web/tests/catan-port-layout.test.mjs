import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
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

test("printed five/six-player Wonders inlet clears fish numbers and nearby markers", () => {
  // Geometry exported from the actual NewCatanFishingSeafarers fixed board,
  // with no player hands/state. Edge 53 overlaps both the 5 and 9 fish discs.
  const g = JSON.parse(
    readFileSync(
      new URL(
        "./fixtures/catan-wonders-fishing-five-six.json",
        import.meta.url,
      ),
    ),
  );
  const saved = structuredClone(g);
  const original = catanPortLayout({ ...g, fishing: undefined });
  assert.equal(
    collides(
      original.find((p) => p.port.edge === 53),
      g,
    ),
    true,
  );
  const moved = catanPortLayout(g);
  const overlap = (p, x, y, half) =>
    Math.abs(p.px - x) < p.size / 2 + half &&
    Math.abs(p.py - y) < p.size / 2 + half;
  moved.forEach((p, i) => {
    assert.equal(collides(p, g), false, `port ${p.port.edge}`);
    moved
      .slice(i + 1)
      .forEach((other) =>
        assert.equal(overlap(p, other.px, other.py, other.size / 2), false),
      );
    for (const t of g.tiles.filter((t) => t.number > 0))
      assert.equal(overlap(p, t.x, t.y, g.hexSize * 0.3), false);
    for (const marker of g.seafarers.wonders.markers) {
      const v = g.vertices[marker.vertex];
      assert.equal(overlap(p, v.x, v.y, g.hexSize * 0.3), false);
    }
    assert.deepEqual(p.port, original[i].port);
    assert.deepEqual(p.a, original[i].a);
    assert.deepEqual(p.b, original[i].b);
    assert.ok(p.size >= original[i].size * 0.7 && p.size <= original[i].size);
    assert.ok(
      Math.hypot(p.px - original[i].px, p.py - original[i].py) < g.hexSize * 2,
    );
  });
  assert.ok(
    moved.find((p) => p.port.edge === 53).size <
      original.find((p) => p.port.edge === 53).size,
  );
  assert.deepEqual(g, saved);
  assert.deepEqual(catanPortLayout(g), moved);
});

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

// Two nonadjacent legal port edges face one small sea inlet on the 63-hex
// New World fixture. Their default artwork overlaps even before any fish is
// placed. The illustration can move, but never its edge or leader endpoints.
test("extended New World inlet separates port artwork before fishing setup", () => {
  for (let turn = 0; turn < 6; turn++) {
    const angle = (turn * Math.PI) / 3;
    const rotate = ([x, y]) => ({
      x: x * Math.cos(angle) - y * Math.sin(angle),
      y: x * Math.sin(angle) + y * Math.cos(angle),
    });
    const h = 33.4863156129983;
    const coordinates = [
      [398, 323.4863156129983],
      [398, 356.97263122599657],
      [456, 356.97263122599657],
      [427, 373.7157890324957],
      [427, 306.74315780649914],
      [456, 323.4863156129983],
    ];
    const g = {
      hexSize: h,
      vertices: [...coordinates, [-1000, -1000], [1000, 1000]].map((v, id) => ({
        id,
        ...rotate(v),
      })),
      edges: [
        { id: 0, a: 0, b: 1, tiles: [0] },
        { id: 1, a: 2, b: 3, tiles: [1] },
      ],
      ports: [
        { edge: 0, resource: 0 },
        { edge: 1, resource: 1 },
      ],
      tiles: [
        {
          id: 0,
          ...rotate([369, 340.2294734194974]),
          vertices: [0, 1],
          resource: 0,
          number: 3,
        },
        {
          id: 1,
          ...rotate([456, 390.4589468389949]),
          vertices: [2, 3],
          resource: 0,
          number: 12,
        },
      ],
      seafarers: { scenario: "new_world", pirate: -1 },
      fishing: { map: { grounds: [] } },
    };
    const saved = structuredClone(g);
    const original = catanPortLayout({ ...g, fishing: undefined });
    const overlap = ([a, b]) =>
      Math.abs(a.px - b.px) < (a.size + b.size) / 2 &&
      Math.abs(a.py - b.py) < (a.size + b.size) / 2;
    assert.equal(overlap(original), true);
    const moved = catanPortLayout(g);
    assert.equal(overlap(moved), false);
    assert.deepEqual(g, saved);
    moved.forEach((p, i) => {
      assert.equal(p.port.edge, i);
      assert.deepEqual(p.a, g.vertices[i * 2]);
      assert.deepEqual(p.b, g.vertices[i * 2 + 1]);
      assert.equal(p.size, original[i].size);
      assert.ok(
        (p.px - p.x) * Math.cos(p.angle) + (p.py - p.y) * Math.sin(p.angle) >=
          h * 0.4,
      );
    });
    assert.deepEqual(catanPortLayout(g), moved);
    // The later 8-point ground occupies this same inlet. Vacated artwork
    // locations must be reusable so the second port can clear its number.
    g.fishing.map.grounds = [{ number: 8, vertices: [4, 5, 2] }];
    const withGround = catanPortLayout(g);
    assert.equal(overlap(withGround), false);
    withGround.forEach((p) => assert.equal(collides(p, g), false));
  }
});
