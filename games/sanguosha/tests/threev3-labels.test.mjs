import assert from "node:assert/strict";
import test from "node:test";

import {
  threeV3CampName,
  threeV3PoolHint,
  threeV3PoolList,
  threeV3PromptLabels,
} from "../web/threev3-labels.ts";

test("3v3 prompt labels cover the pile stage, draft, first side and action right", () => {
  assert.deepEqual(
    threeV3PromptLabels("sg_3v3_pile").map((x) => x.value),
    ["1", "2", "3", "4"],
  );
  assert.deepEqual(
    threeV3PromptLabels("sg_3v3_pile").map((x) => x.label),
    [
      "第 1 堆（8 名，暗置）",
      "第 2 堆（8 名，暗置）",
      "第 3 堆（8 名，暗置）",
      "第 4 堆（8 名，暗置）",
    ],
  );
  assert.deepEqual(
    threeV3PromptLabels("sg_3v3_first").map((x) => x.value),
    ["us", "them"],
  );
  assert.deepEqual(
    threeV3PromptLabels("sg_3v3_first_side").map((x) => x.label),
    ["冷色方先手", "暖色方先手"],
  );
  assert.deepEqual(
    threeV3PromptLabels("sg_3v3_side").map((x) => x.value),
    ["vanguards", "leader"],
  );
  assert.deepEqual(threeV3PromptLabels("sg_3v3_pick"), []);
  assert.equal(threeV3CampName(0), "冷色方");
  assert.equal(threeV3CampName(1), "暖色方");
});

test("3v3 draft pool stays render-safe before both leaders take a pile", () => {
  // The official room sends no pool until both piles are laid out; the table
  // must never read `.length` of null.
  assert.deepEqual(threeV3PoolList(null), []);
  assert.deepEqual(threeV3PoolList(undefined), []);
  assert.deepEqual(threeV3PoolList(["caocao", "liubei"]), ["caocao", "liubei"]);
  assert.equal(
    threeV3PoolHint("piles", null),
    "取堆阶段：两方主帅各取一堆后，平摊 16 名武将。",
  );
  assert.equal(
    threeV3PoolHint("pick", []),
    "选将结束，等待双方主帅分配出场武将。",
  );
  assert.equal(threeV3PoolHint("pick", ["caocao"]), null);
  assert.equal(
    threeV3PoolHint("firstSide", []),
    "双方布阵完毕，等待冷色方主帅选择先手阵营。",
  );
  assert.equal(threeV3PoolHint(undefined, null), null);
});
