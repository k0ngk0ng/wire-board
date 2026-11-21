export function createPixelProbe(gl, canvas) {
  let plannedWidth = -1;
  let plannedHeight = -1;
  let plan;
  let transfer;
  let gather;
  let staging;
  let resolved;
  let transferBytes = -1;
  let pending;
  let disposed = false;
  const destroyTarget = target => {
    if (!target) return;
    gl.deleteFramebuffer(target.framebuffer);
    gl.deleteTexture(target.texture);
  };
  const createTarget = (width, height) => {
    let texture;
    let framebuffer;
    try {
      texture = gl.createTexture();
      if (!texture) throw Error('GPU texture allocation failed');
      gl.bindTexture(gl.TEXTURE_2D, texture);
      gl.texStorage2D(gl.TEXTURE_2D, 1, gl.RGBA8, width, height);
      framebuffer = gl.createFramebuffer();
      if (!framebuffer) throw Error('GPU framebuffer allocation failed');
      gl.bindFramebuffer(gl.DRAW_FRAMEBUFFER, framebuffer);
      gl.framebufferTexture2D(gl.DRAW_FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, texture, 0);
      if (gl.checkFramebufferStatus(gl.DRAW_FRAMEBUFFER) !== gl.FRAMEBUFFER_COMPLETE) throw Error('GPU framebuffer incomplete');
      return { texture, framebuffer, width, height };
    } catch (error) {
      if (framebuffer) gl.deleteFramebuffer(framebuffer);
      if (texture) gl.deleteTexture(texture);
      throw error;
    }
  };
  const prepare = () => {
    if (disposed) throw Error('GPU probe disposed');
    if (canvas.width !== plannedWidth || canvas.height !== plannedHeight) {
      const positions = [[canvas.width / 2, canvas.height / 2], [canvas.width / 2, canvas.height / 4], [canvas.width / 4, canvas.height / 3]].map(([x, y]) => [Math.max(0, Math.floor(x) - 8), Math.max(0, Math.floor(y) - 8)]);
      const x = Math.min(...positions.map(point => point[0]));
      const y = Math.min(...positions.map(point => point[1]));
      const span = Math.max(...positions.map(point => point[0])) - x + 16;
      const width = span + span % 2;
      const height = Math.max(...positions.map(point => point[1])) - y + 16;
      const offsets = new Uint32Array(3 * 16 * 16);
      let index = 0;
      for (const [sampleX, sampleY] of positions) for (let row = 0; row < 16; row++) for (let column = 0; column < 16; column++) offsets[index++] = ((sampleY - y + row) * width + sampleX - x + column) * 4;
      plan = { x, y, width, height, positions, offsets, pixels: new Uint8Array(width * height * 4) };
      plannedWidth = canvas.width;
      plannedHeight = canvas.height;
    }
    return plan;
  };
  const hashPlan = sampled => {
    let result = 2166136261;
    for (let index = 0; index < sampled.offsets.length; index++) {
      const offset = sampled.offsets[index];
      result = Math.imul(result ^ sampled.pixels[offset] ^ sampled.pixels[offset + 1] ^ sampled.pixels[offset + 2], 16777619);
    }
    return result >>> 0;
  };
  const read = () => {
    if (pending) throw Error('GPU read pending');
    const sampled = prepare();
    gl.readPixels(sampled.x, sampled.y, sampled.width, sampled.height, gl.RGBA, gl.UNSIGNED_BYTE, sampled.pixels);
    return sampled;
  };
  const readAsync = () => new Promise((resolve, reject) => {
    if (pending) { reject(Error('GPU read pending')); return; }
    let sampled;
    try {
      const source = prepare();
      if (!gather) gather = { pixels: new Uint8Array(3072), offsets: Uint32Array.from({ length: 768 }, (value, index) => index * 4) };
      Object.assign(gather, { x: source.x, y: source.y, width: source.width, height: source.height, positions: source.positions });
      sampled = gather;
    } catch (error) { reject(error); return; }
    let sync;
    let timer;
    let animation;
    const clearWake = () => { clearTimeout(timer); timer = undefined; if (animation !== undefined) cancelAnimationFrame(animation); animation = undefined; };
    let finished = false;
    const started = Math.round(performance.now() * 1000);
    const finish = error => {
      if (finished) return;
      finished = true;
      clearWake();
      if (sync) gl.deleteSync(sync);
      pending = undefined;
      error ? reject(error) : resolve(sampled);
    };
    pending = { cancel: () => finish(Error('GPU probe disposed')) };
    const previous = gl.getParameter(gl.PIXEL_PACK_BUFFER_BINDING);
    const previousRead = gl.getParameter(gl.READ_FRAMEBUFFER_BINDING);
    const previousDraw = gl.getParameter(gl.DRAW_FRAMEBUFFER_BINDING);
    const previousTexture = gl.getParameter(gl.TEXTURE_BINDING_2D);
    const scissor = gl.getParameter(gl.SCISSOR_TEST);
    try {
      if (!transfer) transfer = gl.createBuffer();
      if (!transfer) throw Error('GPU buffer allocation failed');
      if (!staging) staging = createTarget(16, 48);
      if (scissor) gl.disable(gl.SCISSOR_TEST);
      gl.bindFramebuffer(gl.READ_FRAMEBUFFER, previousRead);
      gl.bindFramebuffer(gl.DRAW_FRAMEBUFFER, previousRead);
      if (gl.getParameter(gl.SAMPLES) > 0) {
        if (!resolved || resolved.width !== canvas.width || resolved.height !== canvas.height) {
          const old = resolved;
          resolved = undefined;
          destroyTarget(old);
          resolved = createTarget(canvas.width, canvas.height);
        }
        gl.bindFramebuffer(gl.DRAW_FRAMEBUFFER, resolved.framebuffer);
        gl.blitFramebuffer(0, 0, canvas.width, canvas.height, 0, 0, canvas.width, canvas.height, gl.COLOR_BUFFER_BIT, gl.NEAREST);
        gl.bindFramebuffer(gl.READ_FRAMEBUFFER, resolved.framebuffer);
      }
      gl.bindFramebuffer(gl.DRAW_FRAMEBUFFER, staging.framebuffer);
      for (const [index, [x, y]] of sampled.positions.entries()) gl.blitFramebuffer(x, y, x + 16, y + 16, 0, index * 16, 16, (index + 1) * 16, gl.COLOR_BUFFER_BIT, gl.NEAREST);
      gl.bindFramebuffer(gl.READ_FRAMEBUFFER, staging.framebuffer);
      gl.bindBuffer(gl.PIXEL_PACK_BUFFER, transfer);
      if (transferBytes !== sampled.pixels.byteLength) {
        gl.bufferData(gl.PIXEL_PACK_BUFFER, sampled.pixels.byteLength, gl.STREAM_READ);
        transferBytes = sampled.pixels.byteLength;
      }
      gl.readPixels(0, 0, 16, 48, gl.RGBA, gl.UNSIGNED_BYTE, 0);
      sync = gl.fenceSync(gl.SYNC_GPU_COMMANDS_COMPLETE, 0);
      if (!sync) throw Error('GPU fence creation failed');
      gl.flush();
    } catch (error) { finish(error); }
    finally {
      gl.bindFramebuffer(gl.READ_FRAMEBUFFER, previousRead);
      gl.bindFramebuffer(gl.DRAW_FRAMEBUFFER, previousDraw);
      gl.bindTexture(gl.TEXTURE_2D, previousTexture);
      if (scissor) gl.enable(gl.SCISSOR_TEST);
      gl.bindBuffer(gl.PIXEL_PACK_BUFFER, previous);
    }
    if (finished) return;
    const poll = () => {
      if (finished) return;
      clearWake();
      try {
        const status = gl.clientWaitSync(sync, 0, 0);
        if (status === gl.TIMEOUT_EXPIRED) {
          if (Math.round(performance.now() * 1000) - started >= 1000000) throw Error('GPU read timeout');
          timer = setTimeout(poll, 0);
          if (typeof requestAnimationFrame === 'function') animation = requestAnimationFrame(poll);
          return;
        }
        if (status !== gl.ALREADY_SIGNALED && status !== gl.CONDITION_SATISFIED) throw Error('GPU read wait failed');
        const binding = gl.getParameter(gl.PIXEL_PACK_BUFFER_BINDING);
        try {
          gl.bindBuffer(gl.PIXEL_PACK_BUFFER, transfer);
          gl.getBufferSubData(gl.PIXEL_PACK_BUFFER, 0, sampled.pixels);
        } finally { gl.bindBuffer(gl.PIXEL_PACK_BUFFER, binding); }
        finish();
      } catch (error) { finish(error); }
    };
    poll();
  });
  const dispose = () => {
    if (disposed) return;
    disposed = true;
    pending?.cancel();
    if (transfer) gl.deleteBuffer(transfer);
    destroyTarget(staging);
    destroyTarget(resolved);
    transfer = undefined;
    staging = undefined;
    resolved = undefined;
  };
  return { read, hash: () => hashPlan(read()), readAsync, hashAsync: () => readAsync().then(hashPlan), dispose };
}

export function createPixelRing(gl, canvas, factory = createPixelProbe) {
  const slots = Array.from({ length: 16 }, () => ({ probe: factory(gl, canvas), busy: false }));
  let disposed = false;
  let active = 0;
  let peak = 0;
  const hashAsync = () => {
    if (disposed) return Promise.reject(Error('GPU ring disposed'));
    const slot = slots.find(slot => !slot.busy);
    if (!slot) return Promise.reject(Error('GPU read ring full'));
    slot.busy = true;
    active++;
    peak = Math.max(peak, active);
    const release = () => { slot.busy = false; active--; };
    let result;
    try { result = slot.probe.hashAsync(); }
    catch (error) { release(); return Promise.reject(error); }
    return Promise.resolve(result).finally(release);
  };
  const dispose = () => {
    if (disposed) return;
    disposed = true;
    for (const slot of slots) slot.probe.dispose();
  };
  return { hashAsync, dispose, get active() { return active; }, get peak() { return peak; } };
}
