import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";
import { mkdir, readFile, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../../", import.meta.url));
const require = createRequire(new URL("../package.json", import.meta.url));

test("page image references reuse the exact published OSS keys", async () => {
  const images = JSON.parse(
    await readFile(path.join(root, "web/src/ui/published-images.json"), "utf8"),
  );
  const external = JSON.parse(
    await readFile(path.join(root, "assets/external-resources.json"), "utf8"),
  ).files;
  const covers = JSON.parse(
    await readFile(path.join(root, "assets/game-covers.json"), "utf8"),
  ).games;
  assert.deepEqual(
    Object.keys(images).sort(),
    [
      ...Object.values(covers).map((cover) => cover.path),
      "/_common/login/hero.webp",
    ].sort(),
  );
  for (const [logical, key] of Object.entries(images))
    assert.equal(key, external[logical].key);
});

test("direct image URLs use the configured CDN and production never falls back to application aliases", async () => {
  const temporary = path.join(
    root,
    ".local/_common/direct-images/verification/url-cache",
  );
  await mkdir(temporary, { recursive: true });
  for (const [index, origin, production] of [
    [0, "https://images.example.test/", true],
    [1, "", false],
    [2, "", true],
    [3, "http://images.example.test", true],
  ]) {
    const server = await require("vite").createServer({
      configFile: false,
      root: path.join(root, "web"),
      publicDir: false,
      optimizeDeps: { noDiscovery: true },
      cacheDir: path.join(temporary, String(index)),
      define: {
        "import.meta.env.VITE_CDN_ORIGIN": JSON.stringify(origin),
        "import.meta.env.PROD": JSON.stringify(production),
      },
      server: { middlewareMode: true, hmr: false, fs: { allow: [root] } },
    });
    try {
      const { publishedImageURL } = await server.ssrLoadModule(
        "/src/ui/published-image.ts",
      );
      if (production && !origin)
        assert.throws(
          () => publishedImageURL("/_common/login/hero.webp"),
          /CDN_ORIGIN/,
        );
      else if (origin.startsWith("http:"))
        assert.throws(
          () => publishedImageURL("/_common/login/hero.webp"),
          /HTTPS/,
        );
      else if (origin)
        assert.match(
          publishedImageURL("/_common/login/hero.webp"),
          /^https:\/\/images\.example\.test\/bga\/_common\/[a-f0-9]{64}\.webp$/,
        );
      else
        assert.equal(
          publishedImageURL("/_common/login/hero.webp"),
          "/_common/login/hero.webp",
        );
      assert.throws(() => publishedImageURL("/missing.png"), /未登记/);
    } finally {
      await server.close();
    }
  }
  await rm(temporary, { recursive: true, force: true });
});
