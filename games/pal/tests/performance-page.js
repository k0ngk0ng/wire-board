(() => {
  const directions = { ArrowDown: [-16, 8], ArrowUp: [16, -8] };
  const api = window.palPerformance = { mode: "", trials: [], raf: [] };
  let pending;
  function begin(direction, kind, trusted) {
    const canvas = document.querySelector("[data-pal-phase=field][data-pal-ready=true] canvas");
    if (!canvas || !api.mode) return;
    const [dx, dy] = direction;
    const image = canvas.getContext("2d").getImageData(0, 0, 320, 200);
    const pixels = new Uint32Array(image.data.buffer), expected = [];
    for (let y = 12; y < 188; y += 2) for (let x = 20; x < 300; x += 2) {
      if (x > 92 && x < 228 && y > 36 && y < 156) continue;
      const index = y * 320 + x, shifted = (y + dy) * 320 + x + dx;
      if (pixels[index] !== pixels[shifted]) expected.push([index, pixels[shifted]]);
    }
    if (pending) pending.error = "overlapping native inputs";
    pending = { mode: api.mode, kind, trusted, at: performance.now(), epochAt: performance.timeOrigin + performance.now(), informativePixels: expected.length, expected, canvas };
    api.trials.push(pending);
  }
  window.addEventListener("keydown", event => {
    if (!event.repeat && directions[event.code]) begin(directions[event.code], "keyboard", event.isTrusted);
  }, true);
  window.addEventListener("pointerdown", event => {
    if (event.pointerType !== "touch" || event.target.tagName !== "CANVAS" || !api.touchCentre) return;
    const r = event.target.getBoundingClientRect();
    const dx = (event.clientX-r.x)*320/r.width-api.touchCentre.x;
    const dy = (event.clientY-r.y)*200/r.height-api.touchCentre.y;
    if (Math.abs(dx)<=13 && Math.abs(dy)<=20) return;
    if (dx<0 && dy>0) begin(directions.ArrowDown,"touch",event.isTrusted);
    if (dx>0 && dy<0) begin(directions.ArrowUp,"touch",event.isTrusted);
  }, true);
  function sample() {
    const now = performance.now();
    api.raf.push({ at: now, mode: api.mode });
    if (api.raf.length > 10000) api.raf.shift();
    if (pending) {
      const trial = pending;
      const image = trial.canvas.getContext("2d").getImageData(0, 0, 320, 200);
      const pixels = new Uint32Array(image.data.buffer);
      const matches = trial.expected.reduce((n, [index, value]) => n + (pixels[index] === value ? 1 : 0), 0);
      const score = matches / trial.expected.length;
      if (trial.expected.length >= 128 && score >= .85) {
        trial.pixelAt = now; trial.responseMS = now - trial.at; trial.match = score;
        delete trial.expected; delete trial.canvas; pending = undefined;
      } else if (now - trial.at > 5000) {
        trial.error = "no expected camera translation in actual canvas pixels";
        trial.match = score; delete trial.expected; delete trial.canvas; pending = undefined;
      }
    }
    requestAnimationFrame(sample);
  }
  requestAnimationFrame(sample);
})();
