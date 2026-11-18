import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { verifyCanvasOracle, canvasSHA } from "./canvas-oracle.mjs";

const require = createRequire(new URL("../../../web/package.json", import.meta.url));
test("independent sword, claw and whip renderers match fixed original Canvas poses", { timeout: 120000 }, async () => {
  const source = await readFile(new URL("../../../.local/pokepath/research/source/src/js/game/component/Tower.js", import.meta.url), "utf8");
  const ts = require("typescript");
  const parsed = ts.createSourceFile("Tower.js", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
  const tower = parsed.statements.find(node => ts.isClassDeclaration(node) && node.name?.text === "Tower");
  const draw = tower.members.find(node => ts.isMethodDeclaration(node) && node.name?.getText(parsed) === "draw");
  const bodies = {};
  for (const [name, property] of [["blade", "secretSwordEffects"], ["claw", "shadowClawEffects"]]) {
    const branch = draw.body.statements.find(node => ts.isIfStatement(node) && node.expression.getText(parsed) === `this.${property}.length > 0`);
    assert.ok(branch, property);
    bodies[name] = branch.thenStatement.getText(parsed);
  }
  const update = tower.members.find(node => ts.isMethodDeclaration(node) && node.name?.getText(parsed) === "update");
  const visit = node => {
    if (ts.isIfStatement(node) && node.expression.getText(parsed) === "this.fireLashEffect") bodies.whip = node.thenStatement.getText(parsed);
    ts.forEachChild(node, visit);
  };
  visit(update);
  assert.ok(bodies.whip);
  const reference = ts.transpileModule(`export class MeleeReference{blade()${bodies.blade}claw()${bodies.claw}whip()${bodies.whip}}`, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022, removeComments: true } }).outputText;
  await verifyCanvasOracle({ name: "melee-effects", command: "node --test games/pokepath/tests/melee-effects-e2e.test.mjs", reference, sources: [{ path: "src/js/game/component/Tower.js", sha256: canvasSHA(source), version: "1.7.3" }], nativeEntry: fileURLToPath(new URL("../web/attack-effects.ts", import.meta.url)), fixture: await readFile(new URL("./melee-effects-fixture.mjs", import.meta.url)), exports: ["paintAttackModeEffect"], expectedCases: 30 });
});
