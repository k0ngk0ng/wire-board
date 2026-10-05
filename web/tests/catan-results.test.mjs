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

test("combined results retain saved scenario score instead of independent thirteen points", () => {
  for (const target of [14, 15, 16]) {
    const text = catanResultDescription({
      ...base(),
      citiesKnights: {},
      seafarers: { victoryPoints: target },
    });
    assert.ok(text.includes(`${target}分`));
    assert.match(text, /航海家＋城市与骑士/);
    assert.match(text, /本剧本的额外得分/);
    assert.doesNotMatch(text, /13分|最大骑士军队|隐藏胜利点/);
  }
});

test("combined wonders retain the alternative four-level victory and require twelve points to lead", () => {
  const text = catanResultDescription({
    ...base(),
    citiesKnights: {},
    seafarers: { scenario: "wonders", wonders: {}, victoryPoints: 12 },
  });
  assert.match(text, /城市与骑士＋卡坦奇迹/);
  assert.match(text, /4级奇迹.*12分且奇迹等级独自领先/);
  assert.doesNotMatch(text, /10分|13分/);
});

test("cloth combination retains depletion ending and uses sixteen points without route awards", () => {
  const text = catanResultDescription({
    ...base(),
    citiesKnights: {},
    seafarers: { scenario: "cloth", victoryPoints: 16 },
  });
  assert.match(text, /城市与骑士＋卡坦布匹/);
  assert.match(text, /16分/);
  assert.match(text, /五座村落.*同分比较布匹/);
  assert.doesNotMatch(text, /14分|13分|最长路线/);
});

test("harbors results use actual targets and preserve special endings", () => {
 for (const [scenario,ck,target,condition] of [["",false,11,/最大骑士军队/],["",true,14,/大都会/],["wonders",true,13,/4级奇迹.*独自领先/],["cloth",true,17,/五座村落.*同分比较布匹/],["pirate_islands",false,11,/夺回自己的要塞/]]) {
  const g={...base(),harbors:{owner:0,points:[3,0,0]},victoryTarget:target,...(ck?{citiesKnights:{}}:{}),...(scenario?{seafarers:{scenario,victoryPoints:target-1}}:{})};
  const text=catanResultDescription(g);
  assert.ok(text.includes(`${target}分`));
  assert.match(text,/港口霸主/);
  assert.match(text,condition);
  delete g.victoryTarget;
  assert.ok(catanResultDescription(g).includes(`${target}分`));
 }
});
