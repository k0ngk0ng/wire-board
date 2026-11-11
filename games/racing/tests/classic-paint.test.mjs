// Before implementation: native alpha stores four paint labels, not ordinary
// opacity. Holes/fixed pixels must stay unchanged; tagged regions become opaque;
// colouring must never mutate the source atlas or resample its pixels.
import test from 'node:test';
import assert from 'node:assert/strict';
import { hsv8ToRGB, recolorCarPixels } from '../web/classic-paint.ts';

test('HSV byte data decodes known primary colours and achromatic interiors', () => {
  assert.deepEqual(hsv8ToRGB([0, 255, 255]), [255, 0, 0]);
  assert.deepEqual(hsv8ToRGB([85, 255, 255]), [0, 255, 0]);
  assert.deepEqual(hsv8ToRGB([170, 255, 255]), [0, 0, 255]);
  assert.deepEqual(hsv8ToRGB([137, 0, 80]), [80, 80, 80]);
});

test('all native paint tags recolour at source pixel resolution and preserve fixed pixels', () => {
  const source = new Uint8ClampedArray([100,100,100,223, 100,100,100,230, 100,100,100,160, 100,100,100,182, 100,100,100,96, 100,100,100,118, 100,100,100,32, 100,100,100,46, 10,20,30,0, 10,20,30,255, 10,20,30,150]);
  const original = source.slice();
  const output = recolorCarPixels(source, [255,0,0], {primary:[0,255,255],secondary:[85,255,255],interior:[0,0,255],driver:[170,255,255]});
  assert.deepEqual(source, original);
  assert.equal(output.length, source.length);
  assert.deepEqual([...output.slice(0,4)], [100,0,0,255]);
  assert.deepEqual([...output.slice(4,8)], [100,0,0,255]);
  assert.deepEqual([...output.slice(8,12)], [100,100,100,255]);
  assert.deepEqual([...output.slice(16,20)], [0,100,0,255]);
  assert.deepEqual([...output.slice(24,28)], [0,0,100,255]);
  assert.deepEqual(output.slice(32), source.slice(32));
});
