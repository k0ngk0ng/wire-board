(() => {
  const canvas = () => document.querySelector("[data-pal-ready=true] canvas");
  const pixels = (area) => {
    const image = canvas().getContext("2d").getImageData(...area);
    return Array.from(new Uint32Array(image.data.buffer));
  };
  const api = window.palSelectionPerformance = {
    references: {}, trials: [],
    capture(key, areas) {
      this.references[key] = areas.map(pixels);
    },
    arm(spec) {
      this.armed = spec;
      this.done = undefined;
    },
    menuColors(areas, colors) {
      return areas.map(area => {
        const rgba = pixels(area);
        return {
          normal: rgba.filter(color => color === colors.normal).length,
          highlight: rgba.filter(color => colors.highlights.includes(color)).length,
          common: Object.entries(rgba.reduce((result, color) => {
            result[color] = (result[color] ?? 0) + 1;
            return result;
          }, {})).sort((a, b) => b[1] - a[1]).slice(0, 8),
        };
      });
    },
  };
  let trial;
  const record = (event, kind) => {
    if (!api.armed || api.armed.kind !== kind || !event.isTrusted) return;
    if (kind === "key" && (event.repeat || event.code !== api.armed.code)) return;
    if (kind === "touch" && (event.pointerType !== "touch" || event.target !== canvas())) return;
    const spec = api.armed;
    api.armed = undefined;
    const at = performance.now();
    trial = { spec, at, epochAt: performance.timeOrigin + at, old: spec.areas.map(pixels), frames: [] };
  };
  window.addEventListener("keydown", event => record(event, "key"), true);
  window.addEventListener("pointerup", event => record(event, "touch"), true);
  function sample(at) {
    if (trial) {
      const value = trial, { spec, old } = value;
      const current = spec.areas.map(pixels), reference = api.references[spec.target];
      const diff = current.map((area, i) => area.filter((color, j) => color !== old[i][j]).length);
      const oldRow = spec.oldIndex;
      const selected = 1 - oldRow;
      const restored = spec.mode === "menu"
        ? current[oldRow].filter((color, j) => color === spec.normal && spec.highlights.includes(old[oldRow][j])).length
        : current[oldRow].filter((color, j) => color === reference[oldRow][j] && color !== old[oldRow][j]).length;
      const targetPixels = spec.mode === "menu"
        ? current[selected].filter((color, j) => spec.highlights.includes(color) && old[selected][j] === spec.normal).length
        : current[selected].filter((color, j) => color === reference[selected][j] && color !== old[selected][j]).length;
      const visible = restored >= 12 && targetPixels >= 12;
      value.frames.push({ at, diff, restored, targetPixels });
      if (visible || performance.now() - value.at > 3000) {
        const now = performance.now();
        api.done = { trusted: true, kind: spec.kind, epochAt: value.epochAt, responseMS: visible ? now - value.at : null, pixelEpochAt: visible ? performance.timeOrigin + now : null, target: spec.target, frames: value.frames };
        api.trials.push(api.done);
        trial = undefined;
      }
    }
    requestAnimationFrame(sample);
  }
  requestAnimationFrame(sample);
})();
