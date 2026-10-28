// Production failure: the host preloads every Contra WAV/Wasm through the
// picture root or trimmed application origin before its runtime loader starts.
import test from "node:test";
import assert from "node:assert/strict";
import path from "node:path";
import { mkdirSync, mkdtempSync, writeFileSync, rmSync } from "node:fs";
import { pathToFileURL } from "node:url";
import { build } from "../../../web/node_modules/esbuild/lib/main.js";

test("actual Contra module delegates runtime media to its staged loader", async (t) => {
  const modulePath = path.resolve("games/contra/web/module.tsx");
  const compiled = await build({
    entryPoints: ["web/src/game-api/load-assets.ts"],
    bundle: true,
    write: false,
    format: "esm",
    jsx: "automatic",
    nodePaths: [path.resolve("web/node_modules")],
    loader: { ".css": "empty", ".svg": "dataurl" },
    plugins: [
      {
        name: "isolated-game-registry",
        setup(b) {
          b.onResolve({ filter: /^\.\/registry$/ }, (args) =>
            args.importer.endsWith("/load-assets.ts")
              ? { path: "registry", namespace: "fixture" }
              : undefined,
          );
          b.onLoad({ filter: /.*/, namespace: "fixture" }, () => ({
            contents: `import game from ${JSON.stringify(modulePath)};export async function loadGame(){return game}`,
            resolveDir: process.cwd(),
          }));
        },
      },
    ],
  });
  const parent = path.resolve(".local/contra-loading-tests");
  mkdirSync(parent, { recursive: true });
  const directory = mkdtempSync(path.join(parent, "case-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const entry = path.join(directory, "loader.mjs");
  writeFileSync(entry, compiled.outputFiles[0].text);
  const { prepareGame } = await import(pathToFileURL(entry).href);
  const code =
    "https://cdn.ichenj.com/bga/web/" + "a".repeat(40) + "/assets/contra.js";
  const manifest = {
    build: "release",
    entry: code,
    games: { contra: [{ url: code, bytes: 1 }] },
    media: {
      "/contra/runtime-code/audio/song.wav": 103234138,
      "/contra/runtime-code/contra.wasm": 4985033,
    },
  };
  const requests = [],
    progress = [];
  const oldFetch = globalThis.fetch,
    oldDocument = globalThis.document;
  globalThis.document = { querySelector: () => ({}) };
  globalThis.fetch = async (url) => {
    requests.push(String(url));
    if (url === "/game-load-manifest.json")
      return new Response(JSON.stringify(manifest));
    if (url === code) return new Response("x");
    return new Response("trimmed or wrong asset root", { status: 404 });
  };
  try {
    const result = await prepareGame(
      { id: "room", kind: "contra", loading: { build: "release" } },
      "https://images.invalid/old-assets",
      new AbortController().signal,
      (p) => progress.push(p),
    );
    assert.equal(result.module.id, "contra");
    assert.deepEqual(requests, ["/game-load-manifest.json", code]);
    assert.ok(progress.every((p) => (p.total ?? 0) <= 1));
    assert.equal(
      progress.at(-1).percent,
      90,
      "the board still owns actual initialization readiness",
    );
  } finally {
    globalThis.fetch = oldFetch;
    globalThis.document = oldDocument;
  }
});
