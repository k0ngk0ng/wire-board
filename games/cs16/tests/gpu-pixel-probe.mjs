export function createPixelProbe(gl, canvas) {
  let plannedWidth = -1;
  let plannedHeight = -1;
  let plan;
  const read = () => {
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
    gl.readPixels(plan.x, plan.y, plan.width, plan.height, gl.RGBA, gl.UNSIGNED_BYTE, plan.pixels);
    return plan;
  };
  const hash = () => {
    const sampled = read();
    let result = 2166136261;
    for (let index = 0; index < sampled.offsets.length; index++) {
      const offset = sampled.offsets[index];
      result = Math.imul(result ^ sampled.pixels[offset] ^ sampled.pixels[offset + 1] ^ sampled.pixels[offset + 2], 16777619);
    }
    return result >>> 0;
  };
  return { read, hash };
}
