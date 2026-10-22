import test from "node:test";
import assert from "node:assert/strict";
import { readdirSync, readFileSync, existsSync } from "node:fs";
const root = new URL("../../games/", import.meta.url);
for (const entry of readdirSync(root, { withFileTypes: true })) {
  const module = new URL(`${entry.name}/web/module.tsx`, root);
  if (!entry.isDirectory() || !existsSync(module)) continue;
  test(`${entry.name} exposes computer seats through the common room UI`, () => {
    assert.match(
      readFileSync(new URL(`${entry.name}/web/catalog.tsx`, root), "utf8"),
      /supportsBots:\s*true/,
    );
  });
}
