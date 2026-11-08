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
test("a repeated hit stays at contact until the sequence ends", () => {
  const first = { ...motion, stay: true, returnDuration: 0 };
  assert.equal(sampleAttackMotion(first, 2500).phase, "ready");
  assert.deepEqual(sampleAttackMotion(first, 2500).offset, [-360, -202.5]);
  const second = { ...motion, start: motion.to, approach: 0, attackStart: 0 };
  assert.deepEqual(sampleAttackMotion(second, 0).offset, [-360, -202.5]);
  assert.equal(sampleAttackMotion(second, 0).phase, "attack");
  assert.deepEqual(sampleAttackMotion(second, 2266).offset, [0, 0]);
});
test("redirected continuation travels from contact; a counter can park the original attacker", () => {
  const next = { ...motion, start: [277, 249], stay: true };
  assert.deepEqual(sampleAttackMotion(next, 0).offset, [-240, -135]);
  assert.deepEqual(sampleAttackMotion(next, 400).offset, [-300, -168.75]);
  const waiting = {
    ...motion,
    start: motion.to,
    approach: 0,
    idle: true,
    returnStart: 500,
    returnDuration: 800,
  };
  assert.equal(sampleAttackMotion(waiting, 300).action, 3);
  assert.deepEqual(sampleAttackMotion(waiting, 300).offset, [-360, -202.5]);
  assert.deepEqual(sampleAttackMotion(waiting, 900).offset, [-180, -101.25]);
});
