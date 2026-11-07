import test from "node:test";
import assert from "node:assert/strict";
import { readdirSync, readFileSync, existsSync } from "node:fs";
const root = new URL("../../games/", import.meta.url);
for (const entry of readdirSync(root, { withFileTypes: true })) {
  const module = new URL(`${entry.name}/web/module.tsx`, root);
  if (!entry.isDirectory() || !existsSync(module)) continue;
  test(`${entry.name} declares its supported computer-seat capability`, () => {
    const source = readFileSync(
      new URL(`${entry.name}/web/catalog.tsx`, root),
      "utf8",
    );
    if (entry.name === "contra") {
      // Explicit delivery scope: human solo/co-op remains available while
      // computer strategy acceptance is deferred by the user.
      assert.match(source, /supportsBots:\s*false/);
      assert.match(source, /playerCounts:\s*\[\s*1,\s*2\s*\]/);
    } else {
      assert.match(source, /supportsBots:\s*true/);
    }
  });
}
