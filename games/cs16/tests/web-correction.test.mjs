import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { DisplayCorrection } from '../web/presentation.ts';
import { MotionPrediction } from '../web/prediction.ts';
import { emptyInput } from '../web/input.ts';

const player = changes => ({ seat: 0, x: 0, y: 0, z: 0, vx: 0, vy: 0, vz: 0, yaw: 0, pitch: 0, alive: true, life: 1, crouch: false, receivedSeq: 0, appliedSeq: 0, ...changes });

test('authority, input replay and display correction produce a repeatable bounded trajectory', () => {
  const trace = [];
  const predictor = new MotionPrediction((state, input, frames) => ({ ...state, x: state.x + input.moveY * frames }), () => true);
  predictor.confirm(player({}), 100, 1000, 'room');
  predictor.input({ ...emptyInput(), moveY: 256, type: 'input', epoch: 'epoch', seq: 1, life: 1, viewFrame: 100 }, 1001);
  const before = predictor.present(predictor.sample(1001), 1001);
  predictor.confirm(player({ x: 128, appliedSeq: 1 }), 101, 1010, 'room');
  for (const at of [1010, 1030, 1050, 1070, 1090]) {
    const raw = predictor.sample(at);
    const displayed = predictor.present(raw, at);
    trace.push({ at, raw: raw.x, displayed: displayed.x, offset: displayed.x - raw.x });
    assert.equal(raw.y, displayed.y);
    assert.ok(Math.abs(displayed.x - raw.x) <= 8 * 256);
  }
  assert.ok(trace[0].offset > 0);
  assert.equal(trace.at(-1).offset, 0);
  assert.equal(before.x, 256);
  const output = fileURLToPath(new URL('../../../.local/cs16/verification/web', import.meta.url));
  fs.mkdirSync(output, { recursive: true });
  fs.writeFileSync(`${output}/correction-trajectory.json`, JSON.stringify({ passed: true, scope: 'authoritative projection, shared motion callback, queued input replay and display-only interpolation; real map trace is verified by production WASM tests', trace }, null, 2) + '\n');
});

test('small corrections finish within the first 80 ms window while new raw movement remains immediate', () => {
  const correction = new DisplayCorrection(() => true);
  correction.reconcile(player({ x: 512 }), player({ x: 0 }), 0);
  assert.equal(correction.sample(player({ x: 256 }), 40).x, 512);
  correction.reconcile(correction.sample(player({ x: 256 }), 40), player({ x: 384 }), 40);
  assert.equal(correction.sample(player({ x: 640 }), 80).x, 640);
  assert.equal(correction.sample(player({ x: 1000 }), 100).x, 1000);
});

test('wall or closing door, large error, posture and life transitions snap without changing rules', () => {
  let clear = true;
  const correction = new DisplayCorrection(() => clear);
  correction.reconcile(player({ x: 512 }), player({}), 0);
  clear = false;
  assert.equal(correction.sample(player({}), 10).x, 0);
  clear = true;
  for (const from of [player({ x: 9 * 256 }), player({ x: 256, crouch: true }), player({ x: 256, life: 2 }), player({ x: 256, alive: false })]) {
    correction.reconcile(from, player({}), 20);
    assert.equal(correction.sample(player({}), 20).x, 0);
  }
});
