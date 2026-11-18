import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { canvasSHA, verifyCanvasOracle } from "./canvas-oracle.mjs";
import { referenceDOMClass } from "./dom-oracle.mjs";

test("ordinary projectile frames match the reference at their actual center", { timeout: 120000 }, async () => {
  const catalog = JSON.parse(await readFile(new URL("../data/catalog.json", import.meta.url)));
  const source = await referenceDOMClass("game/component/Projectile.js", "Projectile", ["draw"]);
  const media = new URL("../assets/public/pokepath/v1/", import.meta.url);
  const textures = Object.fromEntries(await Promise.all(["red1", "green1", "blue7"].map(async key => {
    const sprite = catalog.projectiles[key].sprite;
    const bytes = await readFile(new URL(sprite.image, media));
    assert.ok(bytes.length > 0);
    return [key, { sprite, bytes: bytes.toString("base64"), sha256: canvasSHA(bytes) }];
  })));
  const report = await verifyCanvasOracle({
    name: "projectile-flight-render",
    command: "node --test games/pokepath/tests/projectile-flight-render.test.mjs",
    reference: `${source.code}\nexport { Projectile };\nexport const textures = ${JSON.stringify(textures)};`,
    sources: [source.identity, ...Object.values(textures).map(({ sprite, sha256 }) => ({ path: sprite.image, sha256 }))],
    nativeEntry: fileURLToPath(new URL("../web/battle-renderer.ts", import.meta.url)),
    fixture: await readFile(new URL("./projectile-flight-render-fixture.mjs", import.meta.url)),
    exports: ["paintBattle"],
    expectedCases: 9,
    size: 192,
  });
  assert.equal(report.passed, true);
});
