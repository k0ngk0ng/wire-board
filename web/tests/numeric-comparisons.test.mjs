import assert from "node:assert/strict";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { inspectSources } from "../scripts/platform-source-lint.mjs";

const root = fileURLToPath(new URL("../../", import.meta.url));
const output = path.join(root, ".local/cs16/verification/lint/numeric-comparisons");
mkdirSync(output, { recursive: true });

export const numericCases = [
  ["pure decimal", "const display = 1.25 * 2;", true],
  ["pure parameters", "function mean(a: number, b: number) { return (a + b) / 2; }", true],
  ["pure math", "const display = Math.sqrt(2);", true],
  ["pure epsilon", "const display = Number.EPSILON;", true],
  ["direct comparison", "const valid = 1.25 < 2;", false],
  ["coerced relational comparison", "const valid = 1.25 < '2';", false],
  ["coerced equality", "const valid = '1.25' == 1.25;", false],
  ["variable comparison", "const value = 1.25; const valid = value >= 1;", false],
  ["alias comparison", "const value = 1.25; const alias = value; const valid = alias === 1;", false],
  ["parameter comparison", "function valid(value: number) { return value < 2; }", false],
  ["return comparison", "function value() { return 1.25; } const valid = value() > 1;", false],
  ["declared return", "declare function value(): number; const valid = value() > 1;", false],
  ["assertion is not quantization", "function valid(value: unknown) { return (value as number) < 2; }", false],
  ["unproven numeric operand", "declare function value(): unknown; const valid = value() < 2;", false],
  ["field comparison", "const state = { value: 1.25 }; const valid = state.value < 2;", false],
  ["array element", "const values = [1.25, 2]; const valid = values[0] < 2;", false],
  ["reassignment", "let value = 1; value = 1.25; const valid = value < 2;", false],
  ["division assignment", "let value = 3; value /= 2; const valid = value < 2;", false],
  ["stale integer guard", "function valid(value: number) { if (!Number.isInteger(value)) return false; value = 1.25; return value < 2; }", false],
  ["array write", "const values = [1, 2]; values[0] = 1.25; const valid = values[0] < 2;", false],
  ["array mutation sorting", "const values = [1, 2]; values.push(1.25); values.sort();", false],
  ["division result", "const value = 3 / 2; const valid = value < 2;", false],
  ["tolerance", "function same(a: number, b: number) { return Math.abs(a - b) < 0.001; }", false],
  ["epsilon comparison", "function same(a: number, b: number) { return Math.abs(a - b) <= Number.EPSILON; }", false],
  ["min", "const display = Math.min(1.25, 2);", false],
  ["max alias", "const choose = Math.max; const display = choose(1.25, 2);", false],
  ["maximum call", "const display = Math.max.call(null, 1.25, 2);", false],
  ["maximum apply", "const display = Math.max.apply(null, [1.25, 2]);", false],
  ["integer maximum apply", "const display = Math.max.apply(null, [1, 2]);", true],
  ["untyped maximum", "function choose(value) { return Math.max(value, 2); }", false],
  ["default sorting", "const values = [1.25, 2]; values.sort();", false],
  ["float comparator", "const values = [1.25, 2]; values.sort((a, b) => a - b);", false],
  ["untyped comparator", "function order(values) { values.sort((a, b) => a - b); }", false],
  ["unproven numeric membership", "function has(values) { return values.includes(1); }", false],
  ["quantized comparator", "const values = [1.25, 2]; values.sort((a, b) => Math.round(a * 1000) - Math.round(b * 1000));", true],
  ["integer comparison", "const value = 1250; const valid = value < 2000;", true],
  ["quantized comparison", "const value = 1.25; const units = Math.round(value * 1000); const valid = units < 2000;", true],
  ["quantized function", "function units(value: number) { return Math.round(value * 1000); } const valid = units(1.25) <= 2000;", true],
  ["nonfinite is not quantized integer", "const units = Math.round(Infinity); const valid = units < 2000;", false],
  ["integer guard", "function valid(value: number) { return Number.isInteger(value) && value < 2; }", true],
  ["safe integer guard", "function valid(value: number) { if (!Number.isSafeInteger(value)) return false; return value < 2; }", true],
  ["finite is not integer", "function valid(value: number) { return Number.isFinite(value) && value < 2; }", false],
  ["integer collection guard", "function order(values: number[]) { if (!values.every(Number.isInteger)) return; values.sort(); }", true],
  ["sentinel", "const value: number | undefined = 1.25; const valid = value !== undefined;", true],
  ["string identity", "const valid = 'left' !== 'right';", true],
  ["object identity", "const a = {}; const b = {}; const valid = a !== b;", true],
  ["old float new comparison", "const value = 1.25;\nconst valid = value < 2;", false, [2]],
  ["old comparison new output", "const value = 1.25; const valid = value < 2;\nconst display = value * 2;", true, [2]],
];

test("generic TS permits pure floating calculation and rejects floating comparison flows", () => {
  const items = numericCases.map(([, source, , changedLines]) => ({ file: "web/src/game-api/default-players.ts", language: "typescript", source, ...(changedLines ? { changedLines } : {}) }));
  const rows = inspectSources(items);
  const cases = rows.map((row, index) => ({ name: numericCases[index][0], expected: numericCases[index][2], actual: row.passed, issues: row.issues }));
  writeFileSync(path.join(output, "public-ts.json"), JSON.stringify({ passed: cases.every(row => row.actual === row.expected), cases }, null, 2) + "\n");
  for (const row of cases) assert.equal(row.actual, row.expected, JSON.stringify(row));
});
