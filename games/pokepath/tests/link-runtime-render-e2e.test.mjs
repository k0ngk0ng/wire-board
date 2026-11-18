import test from "node:test";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { referenceDOMClass } from "./dom-oracle.mjs";
import { verifyCanvasOracle } from "./canvas-oracle.mjs";

test("the actual battle renderer preserves runtime link width, repeated paint and tower occlusion", { timeout: 120000 }, async () => {
  const source = await referenceDOMClass("game/component/Tower.js", "LinkBeam", ["draw", "drawZappedLine"]);
  await verifyCanvasOracle({ name: "link-runtime-render", command: "node --test games/pokepath/tests/link-runtime-render-e2e.test.mjs", reference: `${source.code}\nexport { LinkBeam };\n`, sources: [source.identity], nativeEntry: fileURLToPath(new URL("../web/battle-renderer.ts", import.meta.url)), fixture: await readFile(new URL("./link-runtime-render-fixture.mjs", import.meta.url)), exports: ["paintBattle"], expectedCases: 9 });
});
