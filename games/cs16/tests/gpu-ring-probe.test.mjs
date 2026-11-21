import test from 'node:test';
import assert from 'node:assert/strict';
import { createPixelRing } from './gpu-pixel-probe.mjs';

function fixture() {
  const probes = [];
  const ring = createPixelRing({}, {}, () => {
    const probe = { disposed: 0, calls: [], hashAsync() { return new Promise((resolve, reject) => this.calls.push({ resolve, reject })); }, dispose() { this.disposed++; for (const call of this.calls) call.reject(Error('disposed')); } };
    probes.push(probe);
    return probe;
  });
  return { probes, ring };
}

test('sixteen independent in-flight frames are bounded and a released slot is reused', async () => {
  const view = fixture();
  const reads = Array.from({ length: 16 }, () => view.ring.hashAsync());
  assert.equal(view.probes.length, 16);
  assert.equal(view.ring.active, 16);
  assert.equal(view.ring.peak, 16);
  await assert.rejects(view.ring.hashAsync(), /full/);
  view.probes[1].calls[0].resolve(12);
  assert.equal(await reads[1], 12);
  const reused = view.ring.hashAsync();
  assert.equal(view.probes.length, 16);
  assert.equal(view.probes[1].calls.length, 2);
  view.probes[1].calls[1].resolve(99);
  for (let index = 0; index < 16; index++) if (index !== 1) view.probes[index].calls[0].resolve(index + 11);
  assert.equal(await reused, 99);
  assert.deepEqual(await Promise.all(reads), Array.from({ length: 16 }, (value, index) => index + 11));
  assert.equal(view.ring.active, 0);
  view.ring.dispose();
  view.ring.dispose();
  assert.deepEqual(view.probes.map(probe => probe.disposed), Array(16).fill(1));
});

test('ring disposal rejects active work and future captures', async () => {
  const view = fixture();
  const pending = view.ring.hashAsync();
  pending.catch(() => {});
  view.ring.dispose();
  await assert.rejects(pending, /disposed/);
  await assert.rejects(view.ring.hashAsync(), /disposed/);
  assert.equal(view.ring.active, 0);
});

test('synchronous GPU submission failures release their slot for an explicit retry', async () => {
  const view = fixture();
  view.probes[0].hashAsync = () => { throw Error('GPU failed'); };
  await assert.rejects(view.ring.hashAsync(), /GPU failed/);
  assert.equal(view.ring.active, 0);
  view.probes[0].hashAsync = () => Promise.resolve(123);
  assert.equal(await view.ring.hashAsync(), 123);
  view.ring.dispose();
});
