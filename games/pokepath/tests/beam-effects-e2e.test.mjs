import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { verifyCanvasOracle, canvasSHA } from "./canvas-oracle.mjs";
const require = createRequire(new URL("../../../web/package.json", import.meta.url));

test("independent beam and link renderers match fixed original Canvas poses", { timeout: 120000 }, async () => {
  const source = await readFile(new URL("../../../.local/pokepath/research/source/src/js/game/component/Tower.js", import.meta.url), "utf8");
  const ts = require("typescript");
  const parsed = ts.createSourceFile("Tower.js", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
  const classes = [];
  for (const [name, methods] of [["Beam", ["draw", "drawPath"]], ["LinkBeam", ["draw", "drawZappedLine"]]]) {
    const declaration = parsed.statements.find(node => ts.isClassDeclaration(node) && node.name?.text === name);
    assert.ok(declaration);
    const bodies = methods.map(method => declaration.members.find(node => ts.isMethodDeclaration(node) && node.name?.getText(parsed) === method).getText(parsed));
    classes.push(`export class ${name}Reference{${bodies.join("\n")}}`);
  }
  const reference = ts.transpileModule(classes.join("\n"), { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022, removeComments: true } }).outputText;
  await verifyCanvasOracle({ name: "beam-effects", command: "node --test games/pokepath/tests/beam-effects-e2e.test.mjs", reference, sources: [{ path: "src/js/game/component/Tower.js", sha256: canvasSHA(source), version: "1.7.3" }], nativeEntry: fileURLToPath(new URL("../web/beam-effects.ts", import.meta.url)), fixture: await readFile(new URL("./beam-effects-fixture.mjs", import.meta.url)), exports: ["paintBeamEffect", "paintLinkEffect"], expectedCases: 48 });
});
