import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { verifyCanvasOracle, canvasSHA } from "./canvas-oracle.mjs";
const require = createRequire(new URL("../../../web/package.json", import.meta.url));

test("independent directional cones preserve original pixels and particle distributions", { timeout: 120000 }, async () => {
  const source = await readFile(new URL("../../../.local/pokepath/research/source/src/js/game/component/Tower.js", import.meta.url), "utf8");
  const ts = require("typescript");
  const parsed = ts.createSourceFile("Tower.js", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
  const tower = parsed.statements.find(node => ts.isClassDeclaration(node) && node.name?.text === "Tower");
  const method = name => tower.members.find(node => ts.isMethodDeclaration(node) && node.name?.getText(parsed) === name);
  const branch = method("draw").body.statements.find(node => ts.isIfStatement(node) && node.expression.getText(parsed) === "this.directionalAreaEffects.length > 0");
  assert.ok(branch);
  const reference = ts.transpileModule(`export class DirectionalReference{draw()${branch.thenStatement.getText(parsed)}${method("getDirectionalAreaConfig").getText(parsed)}${method("directionalAreaAttack").getText(parsed)}}`, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022, removeComments: true } }).outputText;
  await verifyCanvasOracle({ name: "directional-effects", command: "node --test games/pokepath/tests/directional-effects-e2e.test.mjs", reference, sources: [{ path: "src/js/game/component/Tower.js", sha256: canvasSHA(source), version: "1.7.3" }], nativeEntry: fileURLToPath(new URL("../web/directional-effects.ts", import.meta.url)), fixture: await readFile(new URL("./directional-effects-fixture.mjs", import.meta.url)), exports: ["paintDirectionalAttack", "createDirectionalParticles"], expectedCases: 12 });
});
