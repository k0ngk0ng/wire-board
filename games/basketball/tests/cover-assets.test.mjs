// Failure inventory before implementation: cold builds omit the cover; CDN
// errors or wrong MIME/hash/dimensions become a publishable fallback; corrupted
// caches survive; hostile pins or cache symlinks escape the checkout.
import { test } from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import {
  mkdtempSync,
  mkdirSync,
  writeFileSync,
  readFileSync,
  rmSync,
  symlinkSync,
} from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  materializeCover,
  verifyCover,
  loadCoverPin,
} from "../build/cover.mjs";
const root = fileURLToPath(new URL("../../../", import.meta.url));
const parent = path.join(root, ".local/basketball-cover-tests");
mkdirSync(parent, { recursive: true });
const png = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a6z8AAAAASUVORK5CYII=",
  "base64",
);
const sha = (b) => createHash("sha256").update(b).digest("hex");
function fixture(t) {
  const dir = mkdtempSync(path.join(parent, "case-"));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  const pin = {
    schema: 1,
    game: "basketball",
    sha256: sha(png),
    bytes: png.length,
    mime: "image/png",
    width: 1,
    height: 1,
    localPath: "/basketball/v1/cover/heroes-" + sha(png) + ".png",
    url:
      "https://cdn.ichenj.com/bga/basketball/cover-" + sha(png) + "/heroes.png",
  };
  const pinFile = path.join(dir, "games/basketball/assets/cover-manifest.json");
  mkdirSync(path.dirname(pinFile), { recursive: true });
  writeFileSync(pinFile, JSON.stringify(pin));
  return { dir, pin, pinFile };
}
test("cold cover download produces verified local fallback and warm cache reuses bytes", async (t) => {
  const f = fixture(t);
  let calls = 0;
  const output = await materializeCover(f.dir, {
    fetchImpl: async (url, opts) => {
      calls++;
      assert.equal(url, f.pin.url);
      assert.equal(opts.redirect, "error");
      return new Response(png, { headers: { "content-type": "image/png" } });
    },
  });
  assert.equal(calls, 1);
  assert.equal(
    sha(readFileSync(path.join(output, f.pin.localPath))),
    f.pin.sha256,
  );
  verifyCover(f.dir, output);
  assert.equal(
    await materializeCover(f.dir, {
      fetchImpl: () => {
        throw Error("warm cache fetched");
      },
    }),
    output,
  );
  writeFileSync(path.join(output, "private.nes"), "private");
  assert.throws(() => verifyCover(f.dir, output));
  await materializeCover(f.dir, {
    fetchImpl: async () =>
      new Response(png, { headers: { "content-type": "image/png" } }),
  });
  writeFileSync(path.join(output, f.pin.localPath), "broken");
  assert.throws(() => verifyCover(f.dir, output));
  await assert.rejects(
    materializeCover(f.dir, {
      fetchImpl: async () => new Response("missing", { status: 404 }),
    }),
  );
  await materializeCover(f.dir, {
    fetchImpl: async () =>
      new Response(png, { headers: { "content-type": "image/png" } }),
  });
  verifyCover(f.dir, output);
});
test("invalid transfer never creates a verified cover", async (t) => {
  for (const kind of ["mime", "hash", "length", "dimensions"]) {
    const f = fixture(t);
    if (kind === "dimensions") {
      f.pin.width = 2;
      writeFileSync(f.pinFile, JSON.stringify(f.pin));
    }
    await assert.rejects(
      materializeCover(f.dir, {
        fetchImpl: async () =>
          new Response(
            kind === "hash"
              ? Buffer.alloc(png.length)
              : kind === "length"
                ? Buffer.concat([png, png])
                : png,
            {
              headers: {
                "content-type": kind === "mime" ? "text/html" : "image/png",
              },
            },
          ),
      }),
    );
    assert.throws(() => verifyCover(f.dir));
  }
});
test("pins and symlinks cannot redirect the artifact outside the fixed public location", async (t) => {
  for (const changes of [
    { url: "https://unrelated.invalid/hero.png" },
    { localPath: "/../outside.png" },
    { sha256: "../escape" },
  ]) {
    const f = fixture(t);
    writeFileSync(f.pinFile, JSON.stringify({ ...f.pin, ...changes }));
    assert.throws(() => loadCoverPin(f.dir));
  }
  const f = fixture(t);
  mkdirSync(path.join(f.dir, ".local"));
  symlinkSync(parent, path.join(f.dir, ".local/basketball-cover-cache"), "dir");
  await assert.rejects(
    materializeCover(f.dir, {
      fetchImpl: () => {
        throw Error("must reject before fetch");
      },
    }),
  );
});
