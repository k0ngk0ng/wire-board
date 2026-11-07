import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createHash } from "node:crypto";
const root = new URL("../assets/", import.meta.url);
test("production art is present, bounded, attributed and matches its manifest", () => {
  const m = JSON.parse(readFileSync(new URL("manifest.json", root)));
  assert.equal(m.model, "gpt-image-2.5");
  assert.ok(m.promptFile);
  assert.equal(m.assets.length, 10);
  for (const entry of m.assets) {
    const bytes = readFileSync(new URL("public" + entry.path, root));
    assert.equal(
      createHash("sha256").update(bytes).digest("hex"),
      entry.sha256,
    );
    assert.equal(bytes.length, entry.bytes);
    assert.ok(bytes.length < 2_500_000);
    const source = readFileSync(new URL("source/" + entry.sourceFile, root));
    assert.equal(
      createHash("sha256").update(source).digest("hex"),
      entry.sourceSHA256,
    );
    assert.equal(bytes.toString("ascii", 0, 4), "RIFF");
    assert.equal(bytes.toString("ascii", 8, 12), "WEBP");
    if (
      entry.path.includes("characters") ||
      entry.path.includes("devices") ||
      entry.path.includes("walk-")
    ) {
      assert.equal(entry.alpha, true);
      assert.equal(entry.frames.length, 16);
      for (const frame of entry.frames) {
        assert.ok(frame.width > 10 && frame.height > 10);
        if (entry.path.includes("walk-"))
          assert.ok(
            frame.contentHeight / frame.contentWidth > 1.3,
            "opaque background must not determine humanoid sprite bounds",
          );
      }
    }
  }
});
test("original audio uses lossless, verified media files", () => {
  const m = JSON.parse(readFileSync(new URL("audio-manifest.json", root)));
  assert.equal(m.origin, "original synthesized composition");
  for (const a of m.assets) {
    const b = readFileSync(new URL("public" + a.path, root));
    assert.equal(b.toString("ascii", 0, 4), "fLaC");
    assert.equal(createHash("sha256").update(b).digest("hex"), a.sha256);
    assert.equal(a.pcmSHA256, a.decodedPCM_SHA256);
    assert.ok(a.frames > 1000);
    assert.equal(a.channels, 1);
  }
});
