import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createHash } from "node:crypto";
const raw = readFileSync(new URL("../web/art.json", import.meta.url));
const art = JSON.parse(raw);
const hd = JSON.parse(readFileSync(new URL("../web/hd.json", import.meta.url)));
test("HD delivery covers exactly the classic roster and binds its source metadata", () => {
  assert.equal(hd.pixelScale, 2);
  assert.match(hd.basePath, /^assets\/hd\/arena-2x-[a-z0-9.-]+$/);
  assert.equal(
    hd.sourceArtSHA256,
    createHash("sha256").update(raw).digest("hex"),
  );
  assert.deepEqual(
    Object.keys(hd.resources).sort(),
    Object.keys(art.resources).sort(),
  );
  assert.equal(Object.keys(hd.resources).length, 9477);
  for (const [file, r] of Object.entries(hd.resources)) {
    assert.ok(r.bytes > 0, file);
    assert.match(r.sha256, /^[a-f0-9]{64}$/);
    assert.ok(r.width > 0 && r.height > 0, file);
  }
});
test("HD physical pixels double without changing original logical frames", () => {
  for (const actions of Object.values(art.sprites))
    for (const a of actions)
      for (const f of a.frames) {
        const r = hd.resources[f.file];
        assert.equal(r.width, f.width * 2, f.file);
        assert.equal(r.height, f.height * 2, f.file);
      }
  assert.deepEqual(
    [
      hd.resources["battle/battle_00.png"].width,
      hd.resources["battle/battle_00.png"].height,
    ],
    [1280, 960],
  );
});
