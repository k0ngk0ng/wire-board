import test from "node:test";
import assert from "node:assert/strict";
import { sampleAttackMotion, impactOffset } from "../web/motion.ts";

const motion = {
  actor: 0,
  from: [517, 384],
  to: [157, 181.5],
  approach: 800,
  attackStart: 800,
  attackDuration: 1000,
  contactOffset: 500,
  contactHold: 133,
  postHold: 333,
  returnDuration: 800,
};
test("approach, complete attack, contact freeze and return have separate native phases", () => {
  assert.equal(sampleAttackMotion(motion, 0).phase, "approach");
  assert.deepEqual(sampleAttackMotion(motion, 0).offset, [0, 0]);
  const start = sampleAttackMotion(motion, 800);
  assert.equal(start.phase, "attack");
  assert.equal(start.progress, 0);
  assert.deepEqual(start.offset, [-360, -202.5]);
  assert.equal(sampleAttackMotion(motion, 1300).progress, 0.5);
  assert.equal(sampleAttackMotion(motion, 1400).progress, 0.5);
  assert.equal(sampleAttackMotion(motion, 1933).progress, 1);
  assert.equal(sampleAttackMotion(motion, 2266).phase, "return");
  assert.deepEqual(sampleAttackMotion(motion, 2666).offset, [-180, -101.25]);
  assert.deepEqual(sampleAttackMotion(motion, 3066).offset, [0, 0]);
});
test("combined actors may wait at contact without compressing their own attack", () => {
  const delayed = { ...motion, attackStart: 1200 };
  assert.equal(sampleAttackMotion(delayed, 1000).phase, "ready");
  assert.equal(sampleAttackMotion(delayed, 1000).action, 3);
  assert.equal(sampleAttackMotion(delayed, 1700).progress, 0.5);
});
test("legacy timelines retain midpoint impact; new timelines use the native contact", () => {
  assert.equal(impactOffset({ duration: 480 }), 240);
  assert.equal(impactOffset({ duration: 3066, impactAt: 1300 }), 1300);
  assert.equal(impactOffset({ duration: 3066, impactAt: 0 }), 0);
});
