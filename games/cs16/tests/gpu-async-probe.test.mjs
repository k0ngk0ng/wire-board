import test from 'node:test';
import assert from 'node:assert/strict';
import { createPixelProbe } from './gpu-pixel-probe.mjs';

function fixture() {
  const state = { bound: null, read: null, draw: null, texture: null, scissor: false, samples: 0, frame: 1, buffers: 0, stores: 0, deleted: 0, fences: 0, failed: false, readAreas: [], textures: 0, framebuffers: 0, deletedTextures: 0, deletedFramebuffers: 0 };
  const pixel = (x, y, frame) => [(x * 7 + y * 3 + frame) % 256, (x * 13 + y * 5 + frame) % 256, (x + y * 11 + frame) % 256, 255];
  const gl = {
    PIXEL_PACK_BUFFER: 1, PIXEL_PACK_BUFFER_BINDING: 2, STREAM_READ: 3, RGBA: 4, UNSIGNED_BYTE: 5,
    SYNC_GPU_COMMANDS_COMPLETE: 6, TIMEOUT_EXPIRED: 7, ALREADY_SIGNALED: 8, CONDITION_SATISFIED: 9, WAIT_FAILED: 10,
    READ_FRAMEBUFFER: 11, DRAW_FRAMEBUFFER: 12, READ_FRAMEBUFFER_BINDING: 13, DRAW_FRAMEBUFFER_BINDING: 14,
    TEXTURE_2D: 15, TEXTURE_BINDING_2D: 16, RGBA8: 17, COLOR_ATTACHMENT0: 18, FRAMEBUFFER_COMPLETE: 19,
    SCISSOR_TEST: 20, COLOR_BUFFER_BIT: 21, NEAREST: 22, SAMPLES: 23,
    getParameter(name) { if (name === this.SAMPLES) return state.draw?.samples ?? (state.draw ? 0 : state.samples); if (name === this.READ_FRAMEBUFFER_BINDING) return state.read; if (name === this.DRAW_FRAMEBUFFER_BINDING) return state.draw; if (name === this.TEXTURE_BINDING_2D) return state.texture; if (name === this.SCISSOR_TEST) return state.scissor; return state.bound; },
    createBuffer() { state.buffers++; return { data: null }; },
    bindBuffer(target, value) { state.bound = value; },
    bufferData(target, bytes) { state.stores++; state.bound.data = new Uint8Array(bytes); },
    createTexture() { state.textures++; return {}; },
    bindTexture(target, texture) { state.texture = texture; },
    texStorage2D(target, levels, format, width, height) { Object.assign(state.texture, { width, height, data: new Uint8Array(width * height * 4) }); },
    createFramebuffer() { state.framebuffers++; return {}; },
    bindFramebuffer(target, framebuffer) { if (target === this.READ_FRAMEBUFFER) state.read = framebuffer; else state.draw = framebuffer; },
    framebufferTexture2D(target, attachment, kind, texture) { state.draw.texture = texture; },
    checkFramebufferStatus() { return this.FRAMEBUFFER_COMPLETE; },
    disable() { state.scissor = false; },
    enable() { state.scissor = true; },
    blitFramebuffer(x, y, right, top, dx, dy, dright, dtop) {
      if (!state.read && state.samples && (x !== dx || y !== dy || right !== dright || top !== dtop)) throw Error('GPU multisample resolve rectangles differ');
      const texture = state.draw.texture;
      const source = state.read?.texture;
      for (let row = 0; row < top - y; row++) for (let column = 0; column < right - x; column++) {
        const offset = source ? ((y + row) * source.width + x + column) * 4 : 0;
        const value = source ? source.data.subarray(offset, offset + 4) : pixel(x + column, y + row, state.frame);
        texture.data.set(value, ((dy + row) * texture.width + dx + column) * 4);
      }
    },
    readPixels(x, y, width, height, format, type, destination) {
      const bytes = typeof destination === 'number' ? state.bound.data.subarray(destination) : destination;
      state.readAreas.push(width * height);
      for (let row = 0; row < height; row++) for (let column = 0; column < width; column++) {
        const texture = state.read?.texture;
        const value = texture ? texture.data.subarray(((y + row) * texture.width + x + column) * 4, ((y + row) * texture.width + x + column) * 4 + 4) : pixel(x + column, y + row, state.frame);
        bytes.set(value, (row * width + column) * 4);
      }
    },
    fenceSync() { state.fences++; return { polls: 0 }; },
    flush() {},
    clientWaitSync(fence) { return state.failed ? this.WAIT_FAILED : ++fence.polls < 2 ? this.TIMEOUT_EXPIRED : this.CONDITION_SATISFIED; },
    getBufferSubData(target, offset, output) { output.set(state.bound.data); },
    deleteSync() { state.fences--; },
    deleteBuffer() { state.deleted++; },
    deleteTexture() { state.deletedTextures++; },
    deleteFramebuffer() { state.deletedFramebuffers++; },
  };
  const canvas = { width: 296, height: 420 };
  const reference = frame => {
    let result = 2166136261;
    const points = [[canvas.width / 2, canvas.height / 2], [canvas.width / 2, canvas.height / 4], [canvas.width / 4, canvas.height / 3]];
    for (const [cx, cy] of points) for (let y = 0; y < 16; y++) for (let x = 0; x < 16; x++) {
      const value = pixel(Math.max(0, Math.floor(cx) - 8) + x, Math.max(0, Math.floor(cy) - 8) + y, frame);
      result = Math.imul(result ^ value[0] ^ value[1] ^ value[2], 16777619);
    }
    return result >>> 0;
  };
  return { state, gl, canvas, reference, probe: createPixelProbe(gl, canvas) };
}

test('async GPU readback retains exact sampled bytes, frame and original hash without holding a binding', async () => {
  const view = fixture();
  const original = { name: 'original' };
  const later = { name: 'another-renderer-binding' };
  view.gl.bindBuffer(view.gl.PIXEL_PACK_BUFFER, original);
  const pending = view.probe.hashAsync();
  assert.equal(view.state.bound, original);
  view.state.frame = 2;
  view.gl.bindBuffer(view.gl.PIXEL_PACK_BUFFER, later);
  assert.equal(await pending, view.reference(1));
  assert.equal(view.state.bound, later);
  assert.equal(view.state.fences, 0);
  assert.equal(await view.probe.hashAsync(), view.reference(2));
  view.probe.dispose();
  assert.equal(view.state.deleted, 1);
});

test('async readback reuses bounded storage and retains fixed tile storage after a canvas size change', async () => {
  const view = fixture();
  for (let frame = 0; frame < 8; frame++) {
    view.state.frame = frame;
    assert.equal(await view.probe.hashAsync(), view.reference(frame));
  }
  assert.equal(view.state.buffers, 1);
  assert.equal(view.state.stores, 1);
  view.canvas.width = 760;
  view.canvas.height = 360;
  assert.equal(await view.probe.hashAsync(), view.reference(7));
  assert.equal(view.state.buffers, 1);
  assert.equal(view.state.stores, 1);
  view.probe.dispose();
});

test('overlap and disposal reject reads and leave no live fence or GPU buffer', async () => {
  const view = fixture();
  const pending = view.probe.hashAsync();
  await assert.rejects(view.probe.hashAsync(), /pending/);
  view.probe.dispose();
  await assert.rejects(pending, /disposed/);
  assert.equal(view.state.fences, 0);
  assert.equal(view.state.deleted, 1);
  await assert.rejects(view.probe.hashAsync(), /disposed/);
});

test('GPU wait failures preserve bindings, release the fence and can be retried explicitly', async () => {
  const view = fixture();
  const original = { name: 'original' };
  view.state.bound = original;
  view.state.failed = true;
  await assert.rejects(view.probe.hashAsync(), /GPU/);
  assert.equal(view.state.bound, original);
  assert.equal(view.state.fences, 0);
  view.state.failed = false;
  assert.equal(await view.probe.hashAsync(), view.reference(1));
  view.probe.dispose();
});

test('GPU allocation and fence creation failures are explicit and preserve bindings', async () => {
  for (const failure of ['buffer', 'fence']) {
    const view = fixture();
    const original = { name: 'original' };
    view.state.bound = original;
    if (failure === 'buffer') view.gl.createBuffer = () => null;
    else view.gl.fenceSync = () => null;
    await assert.rejects(view.probe.hashAsync(), /GPU/);
    assert.equal(view.state.bound, original);
    assert.equal(view.state.fences, 0);
    view.probe.dispose();
  }
});

test('a GPU that never signals fails within a bounded wait', async () => {
  const view = fixture();
  view.gl.clientWaitSync = () => view.gl.TIMEOUT_EXPIRED;
  await assert.rejects(view.probe.hashAsync(), /GPU.*timeout/);
  assert.equal(view.state.fences, 0);
  view.probe.dispose();
});

test('animation frames can finish a read while browser timer tasks are delayed', async () => {
  const priorFrame = globalThis.requestAnimationFrame;
  const priorCancel = globalThis.cancelAnimationFrame;
  const frames = new Map();
  let serial = 0;
  globalThis.requestAnimationFrame = run => { const id = ++serial; frames.set(id, run); return id; };
  globalThis.cancelAnimationFrame = id => frames.delete(id);
  const view = fixture();
  const pending = view.probe.hashAsync();
  pending.catch(() => {});
  try {
    assert.equal(frames.size, 1);
    const [id, run] = frames.entries().next().value;
    frames.delete(id);
    run();
    assert.equal(await pending, view.reference(1));
    assert.equal(frames.size, 0);
    assert.equal(view.state.fences, 0);
  } finally {
    view.probe.dispose();
    globalThis.requestAnimationFrame = priorFrame;
    globalThis.cancelAnimationFrame = priorCancel;
  }
});

test('GPU gather reads exactly the original three tiles and restores framebuffer, texture and scissor state', async () => {
  const view = fixture();
  const before = { read: { name: 'read' }, draw: { name: 'draw' }, texture: { name: 'texture' }, scissor: true };
  Object.assign(view.state, before);
  for (const [width, height] of [[32, 32], [64, 48], [296, 420], [760, 360], [1024, 768]]) {
    Object.assign(view.canvas, { width, height });
    view.state.readAreas = [];
    const plan = await view.probe.readAsync();
    assert.equal(plan.pixels.byteLength, 3072);
    assert.equal(view.state.readAreas.reduce((sum, count) => sum + count, 0), 768);
    let hash = 2166136261;
    for (const offset of plan.offsets) hash = Math.imul(hash ^ plan.pixels[offset] ^ plan.pixels[offset + 1] ^ plan.pixels[offset + 2], 16777619);
    assert.equal(hash >>> 0, view.reference(1));
    for (const key of ['read', 'draw', 'texture', 'scissor']) assert.equal(view.state[key], before[key]);
  }
  view.probe.dispose();
  assert.equal(view.state.textures, 1);
  assert.equal(view.state.deletedTextures, 1);
  assert.equal(view.state.framebuffers, 1);
  assert.equal(view.state.deletedFramebuffers, 1);
});

test('GPU gather allocation and framebuffer completeness failures preserve caller state', async () => {
  for (const failure of ['texture', 'framebuffer', 'incomplete']) {
    const view = fixture();
    const before = { read: { name: 'read' }, draw: { name: 'draw' }, texture: { name: 'texture' }, scissor: true };
    Object.assign(view.state, before);
    if (failure === 'texture') view.gl.createTexture = () => null;
    else if (failure === 'framebuffer') view.gl.createFramebuffer = () => null;
    else view.gl.checkFramebufferStatus = () => 0;
    await assert.rejects(view.probe.hashAsync(), /GPU/);
    for (const key of ['read', 'draw', 'texture', 'scissor']) assert.equal(view.state[key], before[key]);
    view.probe.dispose();
  }
});

test('multisampled canvases resolve in place before the same three tile samples are gathered', async () => {
  const view = fixture();
  view.state.samples = 4;
  assert.equal(await view.probe.hashAsync(), view.reference(1));
  assert.deepEqual(view.state.readAreas, [768]);
  assert.equal(view.state.textures, 2);
  assert.equal(view.state.framebuffers, 2);
  assert.equal(view.state.read, null);
  assert.equal(view.state.draw, null);
  view.probe.dispose();
  assert.equal(view.state.deletedTextures, view.state.textures);
  assert.equal(view.state.deletedFramebuffers, view.state.framebuffers);
});

test('resized multisample storage is released safely when its replacement fails', async () => {
  const view = fixture();
  view.state.samples = 4;
  assert.equal(await view.probe.hashAsync(), view.reference(1));
  const create = view.gl.createTexture.bind(view.gl);
  view.canvas.width = 64;
  view.canvas.height = 48;
  view.gl.createTexture = () => null;
  await assert.rejects(view.probe.hashAsync(), /GPU/);
  assert.equal(view.state.read, null);
  assert.equal(view.state.draw, null);
  view.gl.createTexture = create;
  assert.equal(await view.probe.hashAsync(), view.reference(1));
  view.probe.dispose();
  assert.equal(view.state.deletedTextures, view.state.textures);
  assert.equal(view.state.deletedFramebuffers, view.state.framebuffers);
});
