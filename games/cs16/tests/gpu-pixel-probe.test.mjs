import test from 'node:test';
import assert from 'node:assert/strict';
import { createPixelProbe } from './gpu-pixel-probe.mjs';

const positions = canvas => [[canvas.width / 2, canvas.height / 2], [canvas.width / 2, canvas.height / 4], [canvas.width / 4, canvas.height / 3]].map(([x, y]) => [Math.max(0, Math.floor(x) - 8), Math.max(0, Math.floor(y) - 8)]);
const hashBytes = bytes => {
  let result = 2166136261;
  for (let index = 0; index < bytes.length; index += 4) result = Math.imul(result ^ bytes[index] ^ bytes[index + 1] ^ bytes[index + 2], 16777619);
  return result >>> 0;
};
const originalBytes = (gl, canvas) => {
  const bytes = new Uint8Array(3 * 16 * 16 * 4);
  for (const [index, [x, y]] of positions(canvas).entries()) gl.readPixels(x, y, 16, 16, gl.RGBA, gl.UNSIGNED_BYTE, bytes.subarray(index * 1024, (index + 1) * 1024));
  return bytes;
};
const gather = plan => {
  const bytes = new Uint8Array(plan.offsets.length * 4);
  for (let index = 0; index < plan.offsets.length; index++) bytes.set(plan.pixels.subarray(plan.offsets[index], plan.offsets[index] + 4), index * 4);
  return bytes;
};
const context = (canvas, alignment = 4) => {
  const gl = { RGBA: 6408, UNSIGNED_BYTE: 5121, calls: [], changed: new Map(), alignment };
  gl.readPixels = (x, y, width, height, format, type, output) => {
    assert.equal(format, gl.RGBA);
    assert.equal(type, gl.UNSIGNED_BYTE);
    gl.calls.push({ x, y, width, height });
    const stride = Math.ceil(width * 4 / alignment) * alignment;
    assert.ok(output.length >= stride * (height - 1) + width * 4);
    for (let row = 0; row < height; row++) for (let column = 0; column < width; column++) {
      const px = x + column;
      const py = y + row;
      const rgba = gl.changed.get(px + ':' + py) ?? (px >= canvas.width || py >= canvas.height ? [0, 0, 0, 0] : [(px * 13 + py * 7) % 256, (px * 3 + py * 19) % 256, (px * 29 + py * 5) % 256, (px + py) % 256]);
      output.set(rgba, row * stride + column * 4);
    }
  };
  return gl;
};

for (const [width, height] of [[16, 16], [17, 17], [20, 24], [31, 40], [64, 64], [296, 420], [320, 760], [760, 360], [1396, 785], [1024, 768]]) {
  for (const alignment of [1, 2, 4, 8]) test('identical ordered RGBA and hash at ' + width + 'x' + height + ' pack ' + alignment, () => {
    const canvas = { width, height };
    const gl = context(canvas, alignment);
    const expected = originalBytes(gl, canvas);
    assert.equal(gl.calls.length, 3);
    gl.calls = [];
    const probe = createPixelProbe(gl, canvas);
    const actual = gather(probe.read());
    assert.equal(gl.calls.length, 1);
    assert.deepEqual(actual, expected);
    gl.calls = [];
    assert.equal(probe.hash(), hashBytes(expected));
    assert.equal(gl.calls.length, 1);
  });
}

test('overlapping samples keep all three blocks and their duplicate order', () => {
  const canvas = { width: 16, height: 16 };
  const gl = context(canvas);
  const probe = createPixelProbe(gl, canvas);
  const plan = probe.read();
  const bytes = gather(plan);
  assert.equal(plan.offsets.length, 768);
  assert.deepEqual(bytes.subarray(0, 1024), bytes.subarray(1024, 2048));
  assert.deepEqual(bytes.subarray(0, 1024), bytes.subarray(2048));
  assert.equal(probe.hash(), hashBytes(bytes));
});

test('plan and large buffer are reused until canvas dimensions change', () => {
  const canvas = { width: 296, height: 420 };
  const gl = context(canvas);
  const probe = createPixelProbe(gl, canvas);
  const first = probe.read();
  assert.equal(probe.read(), first);
  probe.hash();
  assert.equal(probe.read().pixels, first.pixels);
  canvas.width = 760;
  canvas.height = 360;
  const resized = probe.read();
  assert.notEqual(resized, first);
  assert.notEqual(resized.pixels, first.pixels);
  assert.equal(probe.hash(), hashBytes(originalBytes(gl, canvas)));
});

test('unused bounding pixels do not change the hash and sampled pixels do', () => {
  const canvas = { width: 64, height: 64 };
  const gl = context(canvas);
  const probe = createPixelProbe(gl, canvas);
  const before = probe.hash();
  gl.changed.set('8:8', [255, 255, 255, 255]);
  assert.equal(probe.hash(), before);
  gl.changed.set('24:24', [255, 255, 255, 255]);
  assert.notEqual(probe.hash(), before);
  assert.equal(probe.hash(), hashBytes(originalBytes(gl, canvas)));
});
