// Run after npm --prefix web run build. Build/network contract, not a source snapshot.
import { readFileSync, statSync, readdirSync } from "node:fs";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
const dir = "web/dist/";
const html = readFileSync(dir + "index.html");
const manifest = JSON.parse(readFileSync(dir + "game-load-manifest.json"));
assert.equal(manifest.build, createHash("sha256").update(html).digest("hex"));
assert.equal(
  Object.keys(manifest.games).length,
  readdirSync("games").filter((g) => {
    try {
      return statSync(`games/${g}/web/module.tsx`).isFile();
    } catch {
      return false;
    }
  }).length,
);
const home = manifest.home;
assert(
  home.bytes < 900000,
  `homepage static JS graph too large: ${home.bytes}`,
);
assert(
  !home.modules.some((p) =>
    /games\/.*\/(?:Board|board|catan|dota|sanguosha)\.tsx$/.test(p),
  ),
  "homepage includes a game board",
);
for (const [kind, files] of Object.entries(manifest.games)) {
  assert(files.length, kind);
  for (const f of files)
    assert.equal(statSync(dir + f.url.slice(1)).size, f.bytes, f.url);
}
console.log(
  JSON.stringify(
    { pass: true, home, games: Object.keys(manifest.games) },
    null,
    2,
  ),
);
