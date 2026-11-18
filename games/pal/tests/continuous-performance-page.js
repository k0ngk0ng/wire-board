(() => {
  const api = window.palContinuousPerformance = {
    async prepareArrow() {
      const base = "/pal/v1/atlases/data/9";
      const [atlas, png] = await Promise.all([fetch(base+".json").then(response => response.json()), fetch(base+".png").then(response => response.blob())]);
      const bitmap = await createImageBitmap(png, { colorSpaceConversion: "none" });
      const ctx = new OffscreenCanvas(bitmap.width, bitmap.height).getContext("2d");
      ctx.drawImage(bitmap,0,0);
      const frames = [68,69].map(index => {
        const frame = atlas.frames[index];
        if (frame.width !== 9 || frame.height !== 6) throw Error("Original arrow dimensions changed");
        return ctx.getImageData(frame.x,frame.y,9,6).data;
      });
      this.mask = Array.from({length:54}, (_, i) => i*4).filter(i => frames[0][i+3] === 255 && frames[1][i+3] === 255 && frames[0][i] !== frames[1][i]);
      if (this.mask.length < 8) throw Error("Missing original shared arrow pixels");
      return this.mask.length;
    },
    begin(mode, direction) {
      const canvas = document.querySelector("[data-pal-ready=true] canvas");
      if (!canvas || document.hidden || !document.hasFocus() || canvas.width !== 320 || canvas.height !== 200) throw Error("Original canvas not ready");
      this.capture = { mode, direction, started: performance.now(), wall: performance.timeOrigin, samples: [], controls: [], canvas };
      previous = undefined;
    },
    end() {
      const { canvas, ...result } = this.capture;
      this.capture = undefined;
      return { ...result, stopped: performance.now() };
    }
  };
  let previous;
  function control(e) {
    if (!api.capture) return;
    api.capture.controls.push({ type: e.type, code: e.code, pointer: e.pointerType, trusted: e.isTrusted, at: performance.now() });
  }
  for (const event of ["keydown", "keyup", "pointerdown", "pointerup", "pointercancel"])
    window.addEventListener(event, control, true);
  function sample() {
    const run = api.capture;
    if (run) {
      const current = { at: performance.now(), focused: document.hasFocus() && !document.hidden, phase: run.canvas.closest("[data-pal-board]")?.dataset.palPhase };
      const ctx = run.canvas.getContext("2d");
      if (run.mode === "battle") {
        const rgba = ctx.getImageData(172, 106, 9, 6).data;
        let hash = 2166136261;
        for (const i of api.mask) for (let c=0;c<3;c++) hash = Math.imul(hash ^ rgba[i+c], 16777619);
        current.hash = hash >>> 0;
        current.changed = !!previous && current.hash !== previous;
        previous = current.hash;
      } else {
        const pixels = new Uint32Array(ctx.getImageData(0, 0, 320, 200).data.buffer);
        const [dx, dy] = run.direction;
        let evidence = 0, matched = 0;
        if (previous) for (let y = 12; y < 188; y += 2) for (let x = 20; x < 300; x += 2) {
          if (x > 92 && x < 228 && y > 36 && y < 156) continue;
          const i = y * 320 + x, shifted = (y + dy) * 320 + x + dx;
          if (previous[i] === previous[shifted]) continue;
          evidence++;
          matched += +(pixels[i] === previous[shifted]);
        }
        current.stance = evidence;
        current.match = evidence ? matched / evidence : 0;
        current.changed = evidence >= 128 && current.match >= .85;
        if (current.changed || !previous) previous = pixels;
      }
      run.samples.push(current);
    }
    requestAnimationFrame(sample);
  }
  requestAnimationFrame(sample);
})();
