import test from "node:test";
import assert from "node:assert/strict";
import { residentResourcePlan } from "../src/game-api/resident-resource-plan.ts";

const media = { "/first/picture.png": 12, "/first/next.json": 21, "/other/picture.png": 30 };
test("board-owned loading still prepares its declared complete published resource set", () => {
  assert.deepEqual(residentResourcePlan("first", media, []), [{ url: "/first/picture.png", bytes: 12 }, { url: "/first/next.json", bytes: 21 }]);
});
test("a selected map and explicit match resources do not pull unrelated maps or games", () => {
  assert.deepEqual(residentResourcePlan("first", media, ["/first/picture.png"], [{ url: "https://assets.example.test/effect.flac", bytes: 32 }]), [{ url: "/first/picture.png", bytes: 12 }, { url: "https://assets.example.test/effect.flac", bytes: 32 }]);
});
test("aliases deduplicate and missing or conflicting requirements fail before readiness", () => {
  assert.equal(residentResourcePlan("first", media, ["/first/picture.png", "/first/picture.png"]).length, 1);
  assert.throws(() => residentResourcePlan("first", media, ["/first/missing.png"]));
  assert.throws(() => residentResourcePlan("first", media, ["/first/picture.png"], [{ url: "/first/picture.png", bytes: 17 }]));
  assert.throws(() => residentResourcePlan("first", media, [], [{ url: "effect.flac", bytes: 0 }]));
});
