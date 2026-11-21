import test from 'node:test';
import assert from 'node:assert/strict';
import { decodeEmbeddedTexture, assertModelTextures } from '../web/embedded-textures.ts';

test('embedded PNG decodes directly from a verified bufferView without object URL or network fetch', async () => {
  const bytes = new Uint8Array([137, 80, 78, 71]);
  const calls = [];
  const parser = { json: { textures: [{ source: 0, sampler: 0 }], images: [{ bufferView: 3, mimeType: 'image/png' }], samplers: [{ wrapS: 10497 }] }, getDependency: async (kind, index) => { calls.push([kind, index]); return bytes.buffer; } };
  const result = await decodeEmbeddedTexture(parser, 0, async blob => { assert.deepEqual(new Uint8Array(await blob.arrayBuffer()), bytes); return { width: 8, height: 4 }; }, (image, sampler) => ({ image, sampler }));
  assert.deepEqual(calls, [['bufferView', 3]]);
  assert.equal(result.image.width, 8);
  assert.equal(result.sampler.wrapS, 10497);
});

test('missing embedded image or decode failure is fatal instead of a silent null material', async () => {
  const parser = { json: { textures: [{ source: 0 }], images: [{ uri: 'blob:blocked' }] }, getDependency: async () => new ArrayBuffer(0) };
  await assert.rejects(() => decodeEmbeddedTexture(parser, 0, async () => ({ width: 1, height: 1 }), image => ({ image })));
  assert.throws(() => assertModelTextures({ traverse: visit => visit({ isMesh: true, material: { name: 'skin', map: null } }) }));
  assert.doesNotThrow(() => assertModelTextures({ traverse: visit => visit({ isMesh: true, material: { map: { image: { width: 64, height: 64 } } } }) }));
});
