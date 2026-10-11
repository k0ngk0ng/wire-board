import assert from "node:assert/strict";
import test from "node:test";

import {
  threeV3CampName,
  threeV3PromptLabels,
} from "../src/threev3-labels.ts";

test("3v3 prompt labels cover the draft, first side and action right", () => {
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
