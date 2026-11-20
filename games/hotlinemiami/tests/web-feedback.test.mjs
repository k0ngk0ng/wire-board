import test from "node:test";
import assert from "node:assert/strict";
import { ActionFeedback } from "../web/feedback.ts";

for (const kind of ["throw", "pickup", "finish", "interact"]) {
  test(`${kind} has bounded next-draw local feedback without authority results`, () => {
    const feedback = new ActionFeedback();
    feedback.push({ seq: 1, life: 1, [kind]: true }, 1000);
    const first = feedback.sample(1001, 1, true);
    assert.equal(first.length, 1);
    assert.equal(first[0].kind, kind);
    feedback.push({ seq: 1, life: 1, [kind]: true }, 1010);
    assert.equal(feedback.sample(1149, 1, true)[0].at, 1000);
    assert.deepEqual(feedback.sample(1150, 1, true), []);
  });
}

test("interruption and death remove all local action presentation", () => {
  const feedback = new ActionFeedback();
  feedback.push({ seq: 1, life: 1, throw: true, pickup: true }, 0);
  feedback.clear();
  assert.deepEqual(feedback.sample(1, 1, true), []);
  feedback.push({ seq: 2, life: 1, finish: true }, 2);
  assert.deepEqual(feedback.sample(3, 1, false), []);
  feedback.push({ seq: 3, life: 1, interact: true }, 4);
  assert.deepEqual(feedback.sample(5, 2, true), []);
});
