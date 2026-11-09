import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, existsSync } from "node:fs";
const read = (p) => JSON.parse(readFileSync(new URL(p, import.meta.url)));
const catalog = read("../server/catalog_source.json");
test("every native mount offers all 8 to 12 compatible appearances", () => {
  for (const mount of catalog.rides) {
    assert.ok(
      mount.riders.length >= 8 && mount.riders.length <= 12,
      mount.name,
    );
    assert.equal(
      new Set(mount.riders.map((r) => r.avatar)).size,
      mount.riders.length,
    );
  }
});
test("48 character appearances have visible portraits/previews and complete weapon/mounted animations", () => {
  const extra = read("../web/avatar-art.json"),
    base = read("../web/art.json");
  assert.equal(extra.basePath, "assets");
  assert.equal(extra.avatars.length, 48);
  const sprites = { ...base.sprites, ...extra.sprites },
    resources = { ...base.resources, ...extra.resources };
  for (const avatar of extra.avatars) {
    assert.ok(resources[avatar.portrait], String(avatar.id));
    const standing = sprites[avatar.id].find(
      (a) => a.direction === 7 && a.action === 3,
    );
    assert.equal(avatar.preview, standing.frames[0].file);
    assert.ok(resources[standing.frames[0].file], String(avatar.id));
    for (let weapon = 0; weapon < 5; weapon++)
      for (const direction of [3, 7])
        for (const action of [0, 1, 2, 3, 4, 10])
          assert.ok(
            sprites[avatar.id + weapon]?.some(
              (a) =>
                a.direction === direction &&
                a.action === action &&
                a.frames.length,
            ),
            `${avatar.id}+${weapon}:${direction}:${action}`,
          );
  }
  for (const mount of catalog.rides)
    for (const rider of mount.riders)
      assert.ok(sprites[rider.graphic], String(rider.graphic));
  for (const r of Object.values(extra.resources)) {
    assert.ok(r.bytes > 0);
    assert.match(r.sha256, /^[a-f0-9]{64}$/);
  }
});
