import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { build } from "../../../web/node_modules/esbuild/lib/main.js";

const hd = JSON.parse(readFileSync(new URL("../web/hd.json", import.meta.url)));
const audio = JSON.parse(readFileSync(new URL("../web/canonical-audio.json", import.meta.url)));

test("StoneAge image and audio requests use the game namespace and content identities", async () => {
  const built = await build({ entryPoints: ["games/stoneage/web/assets.ts"], bundle: true, write: false, format: "esm", define: { "import.meta.env.VITE_CDN_ORIGIN": '"https://cdn.example"' } });
  const assets = await import("data:text/javascript;base64," + Buffer.from(built.outputFiles[0].text).toString("base64"));
  const [file, record] = Object.entries(hd.resources)[0];
  assert.equal(assets.assetURL(file), "https://cdn.example/bga/stoneage/" + record.sha256 + ".png");
  for (const [path, record] of Object.entries(audio)) assert.equal(assets.audioURL(path), "https://cdn.example/" + record.key);
});
