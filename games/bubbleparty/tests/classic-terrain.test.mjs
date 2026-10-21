// Failure inventory before implementation: unknown/legacy map gets classic art;
// Village00 border/offset drifts; all 366 atlases preload instead of used ones;
// exclusive native rectangles/anchors stretch or mirror; invalid frames draw;
// removed/pushed objects resurrect from static metadata; passable objects hide
// the player or alter following alpha; missing media leaves invisible collision;
// CDN failure has no local fallback; cancelled map load mutates the new map.
import test from "node:test";
import assert from "node:assert/strict";
import catalog from "../data/classic-2002.json" with { type: "json" };
import {
  classicTerrain,
  classicSpritePaths,
  classicSpriteSources,
  classicFrame,
  classicObjectEntries,
  classicObjectOpacity,
  drawClassicGround,
  drawClassicObject,
  loadClassicSprites,
} from "../web/classic-terrain.ts";

function context() {
  const calls = [];
  const ctx = {
    calls,
    globalAlpha: 1,
    fillStyle: "",
    strokeStyle: "",
    lineWidth: 1,
    drawImage(...a) {
      calls.push(["image", ...a, this.globalAlpha]);
    },
    fillRect(...a) {
      calls.push(["fill", ...a, this.fillStyle]);
    },
    strokeRect(...a) {
      calls.push(["stroke", ...a]);
    },
    save() {
      calls.push(["save"]);
      this.savedAlpha = this.globalAlpha;
    },
    restore() {
      calls.push(["restore"]);
      this.globalAlpha = this.savedAlpha;
    },
    beginPath() {},
    rect() {},
    clip() {},
    moveTo() {},
    lineTo() {},
    stroke() {},
    arc() {},
    fill() {},
  };
  return ctx;
}
const id = "classic2002-village00";
test("all 112 maps retain native dimensions and only small Village00 is centered", () => {
  assert.equal(classicTerrain("community10"), null);
  for (const map of catalog.maps) {
    const actual = classicTerrain(map.id);
    assert.equal(actual.width, map.width);
    assert.equal(actual.height, map.height);
    assert.equal(actual.offsetX, Math.floor((15 - map.width) / 2));
    assert.equal(actual.offsetY, Math.floor((13 - map.height) / 2));
  }
  assert.equal(classicTerrain(id).offsetX, 1);
  assert.equal(classicTerrain(id).offsetY, 1);
});
test("load only used Default resources and preserve immutable CDN/local paths", () => {
  assert.deepEqual(classicSpritePaths("community10"), []);
  for (const map of catalog.maps) {
    const ids = new Set(
      [...map.ground.flat(), ...map.objects.flat()].filter((x) => x >= 0),
    );
    const expected = [
      ...new Set(
        [...ids].map((id) => {
          const tile = catalog.tiles.find((t) => t.id === id);
          const resource = tile.sequences.find(
            (s) => s.name === "Default",
          ).resource;
          return (
            "/bubbleparty/v4/classic/sprites/" +
            catalog.sprites.find((s) => s.name === resource).file
          );
        }),
      ),
    ].sort();
    assert.deepEqual(classicSpritePaths(map.id), expected);
  }
  const path = classicSpritePaths(id)[0];
  assert.deepEqual(classicSpriteSources("https://cdn.example/base/", path), [
    "https://cdn.example/base" + path,
    path,
  ]);
  assert.deepEqual(classicSpriteSources("", path), [path]);
});
test("native rectangles, anchors and every default frame stay inside their sheet", () => {
  for (const tile of catalog.tiles) {
    const seq = tile.sequences.find((s) => s.name === "Default");
    const sprite = catalog.sprites.find((s) => s.name === seq.resource);
    const raw = sprite.framesRaw[seq.framePairsRaw[0][0]];
    const f = classicFrame(tile.id, 0);
    assert.equal(f.sx, raw[0]);
    assert.equal(f.sy, raw[1]);
    assert.equal(f.sw, raw[2] - raw[0]);
    assert.equal(f.sh, raw[3] - raw[1]);
    assert.equal(f.ax, raw[4]);
    assert.equal(f.ay, raw[5]);
    assert.ok(f.sx + f.sw <= sprite.width && f.sy + f.sh <= sprite.height);
  }
  assert.equal(classicFrame(-1, 0), null);
  assert.equal(classicFrame(99999, 0), null);
  const animated = catalog.tiles.find(
    (t) =>
      t.sequences.find((s) => s.name === "Default").framePairsRaw.length > 1,
  );
  const pairs = animated.sequences.find(
    (s) => s.name === "Default",
  ).framePairsRaw;
  assert.equal(classicFrame(animated.id, pairs[0][1] * 30).index, pairs[1][0]);
  assert.equal(
    classicFrame(animated.id, 99999, { reduced: true }).index,
    pairs[0][0],
  );
  assert.equal(classicFrame(animated.id, -100).index, pairs[0][0]);
});
test("authoritative removal and pushed positions never restore the original objects", () => {
  const initial = classicObjectEntries({}, id);
  assert.ok(initial.length > 0);
  assert.ok(
    initial.every((o) => o.x >= 1 && o.x <= 13 && o.y >= 1 && o.y <= 11),
  );
  const nativeObjects = Array(195).fill(-1);
  nativeObjects[5 * 15 + 7] = 66;
  assert.deepEqual(
    classicObjectEntries({ nativeObjects }, id).map((o) => [o.id, o.x, o.y]),
    [[66, 7, 5]],
  );
  nativeObjects[5 * 15 + 7] = -1;
  assert.deepEqual(classicObjectEntries({ nativeObjects }, id), []);
  assert.deepEqual(classicObjectEntries({ nativeObjects: [] }, id), []);
});
test("original frame is drawn at native ground anchor with exact cell scale and restored alpha", () => {
  const f = classicFrame(66, 0),
    image = { complete: true, naturalWidth: 132 };
  const ctx = context();
  ctx.globalAlpha = 0.8;
  assert.equal(
    drawClassicObject(ctx, 66, 3, 4, { [f.path]: image }, 0, { opacity: 0.5 }),
    true,
  );
  const draw = ctx.calls.find((c) => c[0] === "image");
  assert.deepEqual(draw.slice(1, 10), [
    image,
    f.sx,
    f.sy,
    f.sw,
    f.sh,
    (3 * 40 - f.ax) * 0.6,
    (4 * 40 + 39 - f.ay) * 0.6,
    f.sw * 0.6,
    f.sh * 0.6,
  ]);
  assert.equal(draw[10], 0.4);
  assert.equal(ctx.globalAlpha, 0.8);
});
test("passable cover fades only for an overlapping character, solid objects stay opaque", () => {
  assert.equal(classicObjectOpacity(85, 3, 4, [{ x: 3, y: 4 }]), 0.5);
  assert.equal(classicObjectOpacity(85, 3, 4, [{ x: 8, y: 4 }]), 1);
  assert.equal(classicObjectOpacity(66, 3, 4, [{ x: 3, y: 4 }]), 1);
});
test("missing art leaves visible differentiated obstacles and centered ground border", () => {
  const ctx = context();
  assert.equal(drawClassicObject(ctx, 66, 3, 4, {}, 0), false);
  assert.ok(ctx.calls.some((c) => c[0] === "fill"));
  assert.ok(ctx.calls.some((c) => c[0] === "stroke"));
  const ground = context();
  assert.equal(drawClassicGround(ground, {}, id, {}, 0), true);
  assert.ok(
    ground.calls.some(
      (c) => c[0] === "fill" && c[3] === 15 * 24 && c[4] === 13 * 24,
    ),
  );
  assert.equal(drawClassicGround(context(), {}, "community10", {}, 0), false);
});
test("failed CDN uses local images; cancellation resolves and cannot fill a later map dictionary", async () => {
  const previous = globalThis.Image,
    requests = [];
  globalThis.Image = class {
    complete = false;
    naturalWidth = 0;
    set src(url) {
      requests.push(url);
      queueMicrotask(() => {
        if (url.startsWith("https:")) this.onerror?.();
        else {
          this.complete = true;
          this.naturalWidth = 100;
          this.onload?.();
        }
      });
    }
  };
  try {
    const loaded = await loadClassicSprites(id, "https://cdn.example");
    assert.equal(loaded.loaded, classicSpritePaths(id).length);
    assert.deepEqual(loaded.failed, []);
    assert.equal(Object.keys(loaded.images).length, loaded.loaded);
    assert.ok(requests.some((p) => p.startsWith("/bubbleparty/v4/")));
    const controller = new AbortController();
    controller.abort();
    const stopped = await loadClassicSprites(id, "", controller.signal);
    assert.equal(stopped.loaded, 0);
    assert.deepEqual(stopped.images, {});
    globalThis.Image = class {
      set src(url) {
        requests.push(url);
      }
    };
    const pending = new AbortController();
    const promise = loadClassicSprites(id, "", pending.signal);
    pending.abort();
    const cancelled = await promise;
    assert.equal(cancelled.loaded, 0);
    assert.deepEqual(cancelled.images, {});
  } finally {
    globalThis.Image = previous;
  }
});
