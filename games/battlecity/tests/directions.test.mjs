// Failure inventory: docs/research/battlecity-directions/PLAN.md.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { buildSync } from "../../../web/node_modules/esbuild/lib/main.js";
const bundled = buildSync({
  stdin: {
    contents: `export {drawBattlefield} from './canvas';export {atlasSources,soundSources} from './assets';export {ATLAS_FRAMES} from './atlas';export {MAPS} from './types';`,
    resolveDir: new URL("../web/", import.meta.url).pathname,
  },
  bundle: true,
  write: false,
  format: "esm",
  platform: "node",
  nodePaths: [new URL("../../../web/node_modules/", import.meta.url).pathname],
});
const { drawBattlefield, atlasSources, soundSources, ATLAS_FRAMES, MAPS } =
  await import(
    "data:text/javascript;base64," +
      Buffer.from(bundled.outputFiles[0].text).toString("base64")
  );
const classes = {
  balanced: 0x20,
  basic: 0x80,
  fast: 0xa0,
  power: 0xc0,
  armor: 0xe0,
};
const dirs = ["up", "right", "down", "left"];
function draw(cls, team, dir, frame, local) {
  const calls = [],
    rotations = [];
  const ctx = new Proxy(
    {
      drawImage: (...args) => calls.push(args),
      rotate: (a) => rotations.push(a),
    },
    { get: (o, k) => (k in o ? o[k] : () => {}) },
  );
  const tank = {
    seat: 0,
    team,
    class: cls,
    x: 80,
    y: 80,
    dir,
    alive: true,
    hp: 1,
    maxHp: 1,
    shieldUntil: 0,
    freezeUntil: 0,
  };
  const view = {
    mapId: MAPS[0].id,
    broken: "",
    shovelUntil: [0, 0],
    tanks: [tank],
    bullets: [],
    powerUps: [],
    bases: [{ alive: true }, { alive: true }],
    events: [],
  };
  const image = {};
  drawBattlefield(
    ctx,
    view,
    null,
    1,
    { image, frames: ATLAS_FRAMES },
    0,
    frame * 130,
    local,
  );
  return calls.filter(
    (a) =>
      a[5] === (local?.x ?? 80) &&
      a[6] === (local?.y ?? 80) &&
      a[7] === 16 &&
      a[8] === 16,
  );
}
test("native four-direction metadata keeps every class and tread frame in its own CHR group", () => {
  const atlas = JSON.parse(
    readFileSync(new URL("../assets/atlas.json", import.meta.url)),
  );
  for (const [cls, base] of Object.entries(classes))
    for (const team of ["top", "bottom"])
      for (const d of dirs)
        for (const f of [0, 1]) {
          const nativeDir = { up: 0, left: 1, down: 2, right: 3 }[d],
            b = base + nativeDir * 8 + f * 4;
          const entry = atlas.frames[`tank-${cls}-${team}-${d}-${f}`];
          assert.deepEqual(entry.tiles, [b, b + 1, b + 2, b + 3]);
          assert.equal(entry.flip, "none");
          assert.equal(entry.chr_bank, 0);
        }
});
test("Canvas selects all 80 team/class/direction/tread rectangles without rotating native art", () => {
  for (const cls of Object.keys(classes))
    for (const team of [0, 1])
      for (const dir of [0, 1, 2, 3])
        for (const frame of [0, 1]) {
          const f =
            ATLAS_FRAMES[
              `tank-${cls}-${team === 0 ? "top" : "bottom"}-${dirs[dir]}-${frame}`
            ];
          assert.ok(
            draw(cls, team, dir, frame).some(
              (c) =>
                JSON.stringify(c.slice(1, 5)) ===
                JSON.stringify([f.x, f.y, f.w, f.h]),
            ),
          );
        }
});
test("predicted turn uses the predicted facing immediately while authoritative state is unchanged", () => {
  for (const dir of [0, 1, 2, 3]) {
    const f = ATLAS_FRAMES[`tank-balanced-top-${dirs[dir]}-0`];
    assert.ok(
      draw("balanced", 0, (dir + 1) % 4, 0, { x: 81, y: 82, dir }).some(
        (c) => c[1] === f.x && c[2] === f.y,
      ),
    );
  }
});
test("new atlas uses immutable v2 CDN and local fallback; original sound URLs stay at v1", () => {
  assert.deepEqual(atlasSources("https://cdn.example/"), [
    "https://cdn.example/battlecity/v2/atlas.webp",
    "/battlecity/v2/atlas.webp",
  ]);
  assert.deepEqual(soundSources("https://cdn.example/", "sfx-fire"), [
    "https://cdn.example/battlecity/v1/audio/sfx-fire.mp3",
    "/battlecity/v1/audio/sfx-fire.mp3",
  ]);
});
