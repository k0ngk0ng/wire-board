


import test from "node:test";
import assert from "node:assert/strict";
import {
  mkdirSync,
  writeFileSync,
  readFileSync,
  mkdtempSync,
  rmSync,
} from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { gameLoadManifest } from "../scripts/game-load-manifest.mjs";
const root = fileURLToPath(new URL("../../", import.meta.url));
const base = path.join(root, ".local/conquest/manifest-tests");
mkdirSync(base, { recursive: true });
test("preload static programs only; lazy maps load on selection and legacy shell stays explicit", () => {
  const fixture = mkdtempSync(path.join(base, "case-"));
  try {
    const out = path.join(fixture, "dist");
    mkdirSync(path.join(out, "assets"), { recursive: true });
    const modern = path.join(fixture, "games/modern/web/module.tsx"),
      legacy = path.join(fixture, "games/legacy/web/module.tsx");
    for (const [module, text] of [
      [modern, "export default {};"],
      [legacy, "export default {legacy: true};"],
    ]) {
      mkdirSync(path.dirname(module), { recursive: true });
      writeFileSync(module.replace("module.tsx", "catalog.tsx"), text);
    }
    const chunk = (
      name,
      modules,
      imports = [],
      dynamicImports = [],
      css = [],
    ) => ({
      type: "chunk",
      fileName: name,
      facadeModuleId: modules[0],
      modules: Object.fromEntries(modules.map((x) => [x, {}])),
      imports,
      dynamicImports,
      viteMetadata: { importedCss: new Set(css) },
    });
    const bundle = {
      "assets/resident-worker-qwertyui.js": { type: "asset", fileName: "assets/resident-worker-qwertyui.js" },
      "assets/home.js": {
        ...chunk(
          "assets/home.js",
          ["/index.html"],
          ["assets/shared.js"],
          ["assets/modern.js", "assets/legacy.js"],
        ),
        isEntry: true,
      },
      "assets/shared.js": chunk("assets/shared.js", ["/shared.ts"]),
      "assets/modern.js": chunk(
        "assets/modern.js",
        [modern],
        ["assets/shared.js"],
        ["assets/world.js", "assets/europe.js"],
        ["assets/modern.css"],
      ),
      "assets/world.js": chunk("assets/world.js", [
        "/games/modern/maps/world.json",
      ]),
      "assets/europe.js": chunk("assets/europe.js", [
        "/games/modern/maps/europe.json",
      ]),
      "assets/legacy.js": chunk(
        "assets/legacy.js",
        [legacy],
        ["assets/shared.js"],
      ),
      "assets/room-ui.js": chunk(
        "assets/room-ui.js",
        ["/web/src/legacy/room-ui.tsx"],
        ["assets/legacy-static.js"],
        ["assets/optional.js"],
      ),
      "assets/legacy-static.js": chunk("assets/legacy-static.js", [
        "/legacy-static.ts",
      ]),
      "assets/optional.js": chunk("assets/optional.js", ["/optional.ts"]),
      "assets/modern.css": { type: "asset", fileName: "assets/modern.css" },
      "assets/modern-cover.png": { type: "asset", fileName: "assets/modern-cover.png", originalFileNames: [path.join(fixture, "games/modern/assets/cover.png")] },
      "assets/controller.woff2": { type: "asset", fileName: "assets/controller.woff2", originalFileNames: ["/web/src/controllers/font.woff2"] },
    };
    bundle["assets/modern.js"].viteMetadata.importedAssets = new Set(["assets/modern-cover.png"]);
    bundle["assets/shared.js"].viteMetadata.importedAssets = new Set(["assets/controller.woff2"]);
    writeFileSync(
      path.join(out, "index.html"),
      '<script src="/assets/home.js"></script>',
    );
    for (const name of Object.keys(bundle))
      writeFileSync(path.join(out, name), "export const ready = true;");
    const plugin = gameLoadManifest();
    plugin.configResolved({
      base: "https://cdn.ichenj.com/bga/",
    });
    plugin.writeBundle({ dir: out }, bundle);
    const manifest = JSON.parse(
      readFileSync(path.join(out, "game-load-manifest.json")),
    );
    assert.equal(manifest.worker, "https://cdn.ichenj.com/bga/assets/resident-worker-qwertyui.js");
    assert.equal(manifest.workerPath, "assets/resident-worker-qwertyui.js");
    assert.ok(manifest.home.files.some(file=>file.url===manifest.worker));
    assert.equal(manifest.owners["assets/modern-cover.png"], "modern");
    assert.equal(manifest.owners["assets/controller.woff2"], "_common");
    const modernURLs = manifest.games.modern.map((f) => path.basename(f.url));
    assert.deepEqual(modernURLs.sort(), [
      "controller.woff2",
      "modern-cover.png",
      "modern.css",
      "modern.js",
      "shared.js",
    ]);
    assert.deepEqual(
      manifest.games.legacy.map((f) => path.basename(f.url)).sort(),
      ["controller.woff2", "legacy-static.js", "legacy.js", "room-ui.js", "shared.js"],
    );
    assert.ok(
      manifest.entry.startsWith(
        "https://cdn.ichenj.com/bga/",
      ),
    );
    assert.ok(
      Object.values(manifest.games)
        .flat()
        .every(
          (f) =>
            f.bytes > 0 &&
            f.url.startsWith(
              "https://cdn.ichenj.com/bga/",
            ),
        ),
    );
    writeFileSync(
      path.join(base, "verification.json"),
      JSON.stringify(
        {
          passed: true,
          scope: "synthetic bundle graph, actual plugin output",
          manifest,
        },
        null,
        2,
      ),
    );
  } finally {
    rmSync(fixture, { recursive: true, force: true });
  }
});
