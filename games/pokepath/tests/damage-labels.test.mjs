import test from "node:test";
import assert from "node:assert/strict";
import { mkdir } from "node:fs/promises";
import { createRequire } from "node:module";
const require = createRequire(new URL("../../../web/package.json", import.meta.url));

test("damage labels outlive the event ring, follow enemy centers and reset without replay", async () => {
  const folder = new URL("../../../.local/pokepath/native-ui-test/", import.meta.url);
  await mkdir(folder, { recursive: true });
  const target = new URL("damage-labels.mjs", folder);
  await require("esbuild").build({ entryPoints: [new URL("../web/damage-labels.ts", import.meta.url).pathname], outfile: target.pathname, bundle: true, format: "esm", platform: "node" });
  const { createDamageLabels, updateDamageLabels } = await import(target.href);
  const state = createDamageLabels();
  const catalog = { enemies: { rattata: { sprite: { frameWidth: 32, frameHeight: 40 } } } };
  const view = { epoch: 1, clock: 1000, physicsClock: 1000, enemies: [{ id: 7, key: "rattata", x: 100, y: 100 }], events: [{ id: 1, at: 900, type: "hit", target: 7, x: 100, y: 100, displayHPDamage: 8 }] };
  assert.equal(updateDamageLabels(state, view, catalog).length, 0);
  const hit = { id: 2, at: 1000, type: "hit", target: 7, x: 100, y: 100, displayHPDamage: 15, displayArmorDamage: 5, critical: true };
  const labels = updateDamageLabels(state, { ...view, events: [hit] }, catalog);
  assert.deepEqual(labels.map(value => value.text), ["-15!", "-5!"]);
  assert.deepEqual(labels.map(value => value.y), [82, 70]);
  const afterRing = updateDamageLabels(state, { ...view, clock: 1500, physicsClock: 1500, events: [], enemies: [{ ...view.enemies[0], x: 200 }] }, catalog);
  assert.equal(afterRing.length, 2);
  assert.equal(afterRing[0].x, 200);
  assert.equal(updateDamageLabels(state, { ...view, clock: 2000, physicsClock: 2000, events: [] }, catalog).length, 0);
  assert.equal(updateDamageLabels(state, { ...view, epoch: 2, events: [hit] }, catalog).length, 0);
  updateDamageLabels(state, { ...view, epoch: 2, events: [{ ...hit, id: 3 }] }, catalog);
  assert.equal(state.labels.length, 2);
  assert.equal(updateDamageLabels(state, { ...view, epoch: 2, events: [{ ...hit, id: 3 }] }, catalog, true).length, 0);
});
