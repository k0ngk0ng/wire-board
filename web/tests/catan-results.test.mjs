import { test } from "node:test";
import assert from "node:assert/strict";
import { catanResultDescription } from "../src/catan-results.ts";

const base = () => ({ players: [{}, {}, {}] });
test("cities and knights results explain the actual thirteen-point game and its scoring sources", () => {
  const text = catanResultDescription({ ...base(), citiesKnights: {} });
  assert.match(text, /13分/);
  for (const word of ["大都会", "防御者", "公开进步牌", "商人"])
    assert.ok(text.includes(word));
  assert.doesNotMatch(text, /最大骑士军队|10分/);
  assert.match(catanResultDescription(base()), /10分.*最大骑士军队/);
});
test("seafarer results retain scenario thresholds and alternate victory conditions", () => {
  assert.match(
    catanResultDescription({
      ...base(),
      seafarers: { scenario: "shores", victoryPoints: 14 },
    }),
    /14分/,
  );
  assert.match(
    catanResultDescription({
      ...base(),
      seafarers: { scenario: "wonders", victoryPoints: 10 },
    }),
    /4级.*独自领先/,
  );
  assert.match(
    catanResultDescription({
      ...base(),
      seafarers: { scenario: "pirate_islands", victoryPoints: 10 },
    }),
    /夺回自己的要塞/,
  );
  assert.match(
    catanResultDescription({
      ...base(),
      seafarers: { scenario: "cloth", victoryPoints: 14 },
    }),
    /五座村落/,
  );
  assert.match(
    catanResultDescription({
      players: [{}, { eliminated: true }],
      citiesKnights: {},
    }),
    /最后留在/,
  );
});
