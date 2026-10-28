// Failure inventory: a selected game omits colocated runtime tests; discovery
// includes other games; unknown selections silently produce an empty success.
import test from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { discoverGameTests } from "../scripts/game-test-discovery.mjs";

const root = fileURLToPath(new URL("../../", import.meta.url));
test("selected game includes its runtime and asset checks only", () => {
  const files = discoverGameTests(root, "contra");
  for (const expected of ["tests/background.test.mjs", "web/runtime-room.test.mjs", "web/controller.test.mjs", "web/audio.test.mjs"])
    assert.ok(files.some((file) => file.endsWith(`/games/contra/${expected}`)));
  assert.ok(files.every((file) => file.startsWith(`${root}games/contra/`)));
  assert.equal(new Set(files).size, files.length);
  assert.deepEqual(files, [...files].sort());
});
test("unknown selections fail and full selection includes public checks", () => {
  assert.throws(() => discoverGameTests(root, "missing-game"), /Unknown game/);
  const files = discoverGameTests(root);
  assert.ok(files.some((file) => file.endsWith("/web/tests/autoplay-action.test.mjs")));
  assert.ok(files.some((file) => file.endsWith("/games/contra/web/runtime-room.test.mjs")));
});
