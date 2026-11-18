import test from "node:test";
import assert from "node:assert/strict";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
const require = createRequire(new URL("../../../web/package.json", import.meta.url));

async function loadStore() {
  const directory = new URL("../../../.local/pokepath/native-ui-test/", import.meta.url);
  await mkdir(directory, { recursive: true });
  const path = new URL("assets.mjs", directory);
  await require("esbuild").build({ entryPoints: [new URL("../web/assets.ts", import.meta.url).pathname], outfile: path.pathname, bundle: true, platform: "node", format: "esm", target: "es2022" });
  return import(`${path.href}?test=${Date.now()}`);
}

test("verified fonts retain the original CSS size adjustment", async () => {
  const { createHash } = await import("node:crypto");
  const { AssetStore } = await loadStore();
  const source = await readFile(new URL("../../../.local/pokepath/research/styles.css", import.meta.url), "utf8");
  const expected = new Map();
  require("postcss").parse(source).walkAtRules("font-face", rule => {
    const declarations = Object.fromEntries(rule.nodes.filter(node => node.type === "decl").map(node => [node.prop, node.value]));
    expected.set(declarations["font-family"].replaceAll("'", ""), declarations["size-adjust"] ?? "100%");
  });
  const originals = { fetch: globalThis.fetch, FontFace: globalThis.FontFace, document: globalThis.document };
  const bytes = new Uint8Array([0, 1, 2, 3]);
  const sha256 = createHash("sha256").update(bytes).digest("hex");
  const names = ["PressStart2P", "NGC", "Lexend", "Unown", "PixelMPlus"];
  const faces = [];
  globalThis.FontFace = class { constructor(name, data, descriptors) { this.family = name; this.sizeAdjust = descriptors?.sizeAdjust ?? "100%"; } async load() { return this; } };
  globalThis.document = { fonts: { add(face) { faces.push(face); } } };
  globalThis.fetch = async url => String(url).endsWith("resources.json") ? new Response(JSON.stringify({ files: names.map(name => ({ path: `src/assets/fonts/${name}.ttf`, bytes: bytes.length, sha256, mime: "font/ttf" })) })) : new Response(bytes);
  try {
    const store = new AssetStore(new URL("http://localhost/font-test/"), () => {});
    await store.initialize();
    for (const name of names) await store.font(name);
    assert.deepEqual(faces.map(face => [face.family, face.sizeAdjust]), names.map(name => [name, expected.get(name)]));
    store.dispose();
  } finally { Object.assign(globalThis, originals); }
});

test("logical WAV events resolve to the actual lossless asset and reject alias cycles", async () => {
  const { AssetStore } = await loadStore();
  const originalFetch = globalThis.fetch;
  const originalWindow = globalThis.window;
  const alias = { "src/assets/sounds/ui/click1.wav": "src/assets/sounds/ui/click1.flac", "bad-a.wav": "bad-b.wav", "bad-b.wav": "bad-a.wav" };
  globalThis.window = { location: { origin: "http://localhost" } };
  globalThis.fetch = async url => String(url).endsWith("cdn-assets-manifest.json") ? new Response("", { status: 404 }) : new Response(JSON.stringify({ version: 1, aliases: alias, files: [{ path: "src/assets/sounds/ui/click1.flac", bytes: 44, sha256: "a".repeat(64), mime: "audio/flac" }] }));
  try {
    const store = new AssetStore(new URL("http://localhost/_media/pokepath/v1/"), () => {});
    await store.initialize();
    assert.equal(store.path("./src/assets/sounds/ui/click1.wav"), "src/assets/sounds/ui/click1.flac");
    assert.equal(store.url("src/assets/sounds/ui/click1.wav"), "http://localhost/_media/pokepath/v1/src/assets/sounds/ui/click1.flac");
    assert.throws(() => store.path("bad-a.wav"), /循环|别名/);
    store.dispose();
  } finally {
    globalThis.fetch = originalFetch;
    globalThis.window = originalWindow;
  }
});

test("media is verified and aliased before becoming a CSP-compatible data URL", async () => {
  const { createHash } = await import("node:crypto");
  const { AssetStore } = await loadStore();
  const originalFetch = globalThis.fetch;
  const originalReader = globalThis.FileReader;
  const bytes = new Uint8Array([102, 76, 97, 67, 0, 1, 2, 3]);
  const sha256 = createHash("sha256").update(bytes).digest("hex");
  let downloads = 0;
  let encodings = 0;
  globalThis.FileReader = class { readAsDataURL(blob) { encodings++; void blob.arrayBuffer().then(bytes => { this.result = `data:${blob.type};base64,${Buffer.from(bytes).toString("base64")}`; this.onload?.(); }); } };
  globalThis.fetch = async url => {
    if (String(url).endsWith("resources.json")) return new Response(JSON.stringify({ aliases: { "click.wav": "click.flac" }, files: [{ path: "click.flac", bytes: bytes.length, sha256, mime: "audio/flac" }, { path: "bad.flac", bytes: bytes.length, sha256: "0".repeat(64), mime: "audio/flac" }] }));
    downloads++;
    return new Response(bytes);
  };
  try {
    const store = new AssetStore(new URL("http://localhost/_media/pokepath/v1/"), () => {});
    await store.initialize();
    assert.equal(await store.media("click.wav"), `data:audio/flac;base64,${Buffer.from(bytes).toString("base64")}`);
    assert.equal(await store.media("click.wav"), await store.media("click.flac"));
    assert.equal(downloads, 1);
    await assert.rejects(store.media("bad.flac"), /校验/);
    assert.equal(encodings, 1);
    store.dispose();
  } finally { globalThis.fetch = originalFetch; globalThis.FileReader = originalReader; }
});

test("current effective species prepare every reachable Trick-or-Treat projectile before first fire", async () => {
  const target = new URL("../../../.local/pokepath/native-ui-test/resource-species.mjs", import.meta.url);
  await require("esbuild").build({ entryPoints: [new URL("../web/resource-species.ts", import.meta.url).pathname], outfile: target.pathname, platform: "node", format: "esm" });
  const { pokemonSprites } = await import(target.href);
  const projectiles = Object.fromEntries(["normal", "candy", "candy1", "spinarak", "apple"].map(key => [key, { sprite: { image: `${key}.png` } }]));
  const species = { ditto: { key: "ditto", abilityId: "transform", projectileId: "normal", sprite: { image: "ditto.png" } }, pumpkaboo: { key: "pumpkaboo", abilityId: "trickOrTreat", projectileId: "normal", sprite: { image: "pumpkaboo.png" } } };
  const values = pokemonSprites({ id: 917, species: "ditto", sprite: { image: "pumpkaboo.png" } }, { species, projectiles }, { 917: "pumpkaboo" });
  assert.deepEqual([...new Set(values.map(value => value.image))].sort(), ["apple.png", "candy.png", "candy1.png", "normal.png", "pumpkaboo.png", "spinarak.png"]);
});
