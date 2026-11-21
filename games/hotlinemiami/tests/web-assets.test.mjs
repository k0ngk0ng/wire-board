import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { loadBattleAssets, validateViewport } from "../web/assets.ts";

test("verified assets initialize under the platform CSP without blob image or script URLs", async () => {
  const bytes = new Uint8Array([1, 2]);
  const sha256 = createHash("sha256").update(bytes).digest("hex");
  const rules = { id: "test", name: "test", width: 256, height: 256, walls: [], spawns: [], weaponSpawns: [], doors: [] };
  const manifest = {
    version: 1,
    maps: [{ id: "test", image: "map", preview: "map", width: 1, height: 1, rules }],
    images: { map: { file: "test-map.png", bytes: 2, width: 1, height: 1 } },
    sprites: { player: { image: "map", frames: [{ x: 0, y: 0, w: 1, h: 1 }] } },
    characters: [{ id: "jacket", sprite: "player" }],
    weapons: [],
    audio: { pistol: { file: "test-shot.wav", bytes: 2, sha256 }, uzi: { file: "test-other-shot.wav", bytes: 2, sha256 } },
    music: { file: "test-music.ogg", bytes: 2 },
    runtime: { script: { file: "test-go.js", bytes: 2 }, wasm: { file: "test-motion.wasm", bytes: 2 } },
  };
  const original = { fetch: globalThis.fetch, window: globalThis.window, document: globalThis.document, Image: globalThis.Image, createImageBitmap: globalThis.createImageBitmap, instantiate: WebAssembly.instantiate };
  let scriptURL = "", bitmapCount = 0;
  globalThis.fetch = async url => url.endsWith("manifest.json") ? new Response(JSON.stringify(manifest)) : new Response(bytes);
  globalThis.Image = class { set src(url) { if (url.startsWith("blob:")) throw Error("platform CSP blocks blob images"); } async decode() {} };
  globalThis.createImageBitmap = async () => { bitmapCount++; return { width: 1, height: 1 }; };
  globalThis.window = {};
  globalThis.document = {
    createElement: () => ({ src: "", remove() {} }),
    head: { append(element) {
      scriptURL = element.src;
      if (scriptURL.startsWith("blob:")) { element.onerror(); return; }
      window.Go = class {
        importObject = {};
        async run() { window.hotlineMotion = { init: () => true, move: () => [0, 0, 0], weapons: () => "[]", unarmed: () => '{"id":"","cooldown":22,"melee":true}', viewport: () => ({ width: 576, height: 360 }) }; }
      };
      element.onload();
    } },
  };
  WebAssembly.instantiate = async () => ({ instance: {} });
  try {
    const reports = [];
    const assets = await loadBattleAssets("/csp-test", "test", ["jacket"], new AbortController().signal, (...report) => reports.push(report));
    assert.equal(bitmapCount, 1);
    assert.equal(scriptURL, "/csp-test/hotlinemiami/test-go.js");
    assert.equal(assets.images.size, 1);
    assert.equal(assets.weapons.length, 0);
    assert.equal(assets.unarmed.id, "");
    assert.equal(assets.unarmed.melee, true);
    assert.deepEqual(assets.viewport, { width: 576, height: 360 });
    const progress = reports.findLast(report => report[2]);
    assert.equal(assets.sounds.size, 2);
    assert.equal(assets.sounds.get("pistol"), assets.sounds.get("uzi"));
    assert.equal(assets.music, "/csp-test/hotlinemiami/test-music.ogg");
    assert.equal(progress[2].total, new TextEncoder().encode(JSON.stringify(manifest)).length + 10);
    assert.equal(progress[2].loaded, progress[2].total);
  } finally {
    for (const key of ["fetch", "window", "document", "Image", "createImageBitmap"]) {
      if (original[key] === undefined) delete globalThis[key];
      else globalThis[key] = original[key];
    }
    WebAssembly.instantiate = original.instantiate;
  }
});

test("viewport dimensions come from positive integer 8:5 runtime metadata without a frontend default", () => {
  assert.deepEqual(validateViewport({ width: 576, height: 360 }), { width: 576, height: 360 });
  assert.deepEqual(validateViewport({ width: 640, height: 400 }), { width: 640, height: 400 });
  for (const value of [undefined, null, "576x360", { width: 0, height: 360 }, { width: -576, height: 360 }, { width: 577, height: 360 }, { width: 576, height: 361 }, { width: 576.5, height: 360 }]) assert.throws(() => validateViewport(value));
});
