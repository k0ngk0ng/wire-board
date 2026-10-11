import assert from "node:assert/strict";
import test from "node:test";

import {
  threeV3CampName,
  threeV3PromptLabels,
} from "../src/threev3-labels.ts";

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
