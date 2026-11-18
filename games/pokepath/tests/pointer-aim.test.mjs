import test from "node:test";
import assert from "node:assert/strict";
import { mkdir } from "node:fs/promises";
import { createRequire } from "node:module";
const require = createRequire(new URL("../../../web/package.json", import.meta.url));

test("pointer destinations throttle absolute values, preserve taps and cancel pending drags", async () => {
  const folder = new URL("../../../.local/pokepath/native-ui-test/", import.meta.url);
  await mkdir(folder, { recursive: true });
  const target = new URL("pointer-aim.mjs", folder);
  await require("esbuild").build({ entryPoints: [new URL("../web/pointer-aim.ts", import.meta.url).pathname], outfile: target.pathname, bundle: true, format: "esm", platform: "node" });
  const { createPointerAim } = await import(target.href);
  let now = 0;
  let serial = 0;
  const timers = new Map();
  const sent = [];
  const controller = createPointerAim({ now: () => now, send: point => sent.push({ ...point, at: now }), setTimer: (callback, delay) => { timers.set(++serial, { callback, at: now + delay }); return serial; }, clearTimer: id => timers.delete(id) });
  const move = x => controller.move({ pokemonID: 917, epoch: 3, x, y: 100 });
  move(0);
  now = 10; move(10);
  now = 20; move(20);
  assert.deepEqual(sent.map(value => value.x), [0]);
  now = 50; for (const [id, timer] of [...timers]) if (timer.at <= now) { timers.delete(id); timer.callback(); }
  assert.deepEqual(sent.map(value => value.x), [0, 20]);
  now = 55; move(30);
  now = 60; controller.edge({ pokemonID: 917, epoch: 3, x: 40, y: 100 });
  assert.equal(sent.at(-1).x, 40);
  assert.equal(sent.at(-1).at, 60);
  assert.equal(timers.size, 0);
  now = 65; move(50);
  controller.cancel();
  assert.equal(timers.size, 0);
  now = 200; move(60);
  assert.equal(sent.at(-1).x, 60);
  controller.dispose();
  now = 300; move(70);
  assert.equal(sent.at(-1).x, 60);
});

test("pointer control requires selected deployed followPointer ownership and a live controllable battle", async () => {
  const target = new URL("../../../.local/pokepath/native-ui-test/pointer-target.mjs", import.meta.url);
  await require("esbuild").build({ entryPoints: [new URL("../web/pointer-aim.ts", import.meta.url).pathname], outfile: target.pathname, bundle: true, format: "esm", platform: "node" });
  const { pointerTarget } = await import(target.href);
  const pokemon = { id: 917, species: "drifblim", item: "blimpKeys", combatMoveMode: "followPointer" };
  const view = { epoch: 5, paused: false, team: [pokemon], towers: [{ pokemonID: 917 }] };
  const catalog = { species: { drifblim: { abilityId: "unburden", sprite: {} } } };
  assert.deepEqual(pointerTarget(view, catalog, 917, true), { pokemonID: 917, epoch: 5 });
  for (const [next, selected, allowed] of [[{ ...view, paused: true }, 917, true], [view, 0, true], [view, 917, false], [{ ...view, towers: [] }, 917, true], [{ ...view, team: [{ ...pokemon, combatMoveMode: "static" }] }, 917, true]]) assert.equal(pointerTarget(next, catalog, selected, allowed), undefined);
  assert.deepEqual(pointerTarget({ ...view, team: [{ ...pokemon, combatMoveMode: undefined }] }, catalog, 917, true), { pokemonID: 917, epoch: 5 });
  assert.equal(pointerTarget({ ...view, team: [{ ...pokemon, item: "", combatMoveMode: undefined }] }, catalog, 917, true), undefined);
});
