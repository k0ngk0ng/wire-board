import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { verifyCanvasOracle, canvasSHA } from "./canvas-oracle.mjs";

const require = createRequire(new URL("../../../web/package.json", import.meta.url));

test("independent travelling attack shapes preserve original Canvas pixels", { timeout: 120000 }, async () => {
  const source = await readFile(new URL("../../../.local/pokepath/research/source/src/js/game/component/Tower.js", import.meta.url), "utf8");
  const ts = require("typescript");
  const parsed = ts.createSourceFile("Tower.js", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
  const tower = parsed.statements.find(node => ts.isClassDeclaration(node) && node.name?.text === "Tower");
  const draw = tower?.members.find(node => ts.isMethodDeclaration(node) && node.name?.getText(parsed) === "draw");
  assert.ok(draw);
  const blocks = {};
  for (const [name, property] of [["arc", "psychoCutWaves"], ["dune", "sandsearWaves"]]) {
    const branch = draw.body.statements.find(node => ts.isIfStatement(node) && node.expression.getText(parsed) === `this.${property}.length > 0`);
    assert.ok(branch, property);
    blocks[name] = branch.thenStatement.getText(parsed);
  }
  const supersonic = tower.members.find(node => ts.isMethodDeclaration(node) && node.name?.getText(parsed) === "updateSupersonicWaves");
  let waveBody;
  const visit = node => {
    if (ts.isIfStatement(node) && node.expression.getText(parsed) === "shouldDraw && wave.alpha > 0") waveBody = node.thenStatement.getText(parsed);
    ts.forEachChild(node, visit);
  };
  visit(supersonic);
  assert.ok(waveBody);
  const reference = ts.transpileModule(`export class AttackReference{arc()${blocks.arc}dune()${blocks.dune}wave(wave)${waveBody}}`, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022, removeComments: true } }).outputText;
  await verifyCanvasOracle({ name: "attack-effects", command: "node --test games/pokepath/tests/attack-effects-e2e.test.mjs", reference, sources: [{ path: "src/js/game/component/Tower.js", sha256: canvasSHA(source), version: "1.7.3" }], nativeEntry: fileURLToPath(new URL("../web/attack-effects.ts", import.meta.url)), fixture: await readFile(new URL("./attack-effects-fixture.mjs", import.meta.url)), exports: ["paintAttackModeEffect"], expectedCases: 18 });
});
