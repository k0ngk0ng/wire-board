import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
const read = (p) => JSON.parse(readFileSync(new URL(p, import.meta.url)));
test("elite candidates use complete native art and highest-tier crafted identities", () => {
  const c = read("../server/elite_source.json"),
    a = read("../web/elite-art.json");
  assert.ok(c.pets.length >= 30);
  assert.equal(c.rides.length, 5);
  for (const pet of [...c.pets, ...c.rides]) {
    assert.ok(pet.weights.reduce((x, y) => x + y, 0) >= 100, pet.name);
    for (const d of [3, 7])
      for (const action of [0, 1, 2, 3, 4, 10])
        assert.ok(
          a.sprites[pet.graphic]?.some(
            (x) => x.direction === d && x.action === action && x.frames.length,
          ),
          String(pet.graphic),
        );
  }
  for (const g of c.equipment) {
    assert.equal(g.tier, g.maxTier);
    assert.ok(g.name.startsWith("合成"));
    if (g.slot !== "accessory") assert.equal(g.tier, 20);
    assert.ok(a.resources[g.icon]);
  }
  for (const resource of Object.values(a.resources)) {
    assert.ok(resource.bytes > 0);
    assert.match(resource.sha256, /^[a-f0-9]{64}$/);
  }
});
