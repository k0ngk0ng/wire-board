/* Test-only read-only recorder. Install as a page init script before navigation,
 * with window.__sfProbeConfig.atlases = { v1: atlasJSON, "world-v2": atlasJSON }.
 * It forwards every Canvas/WebSocket/fetch call without changing game data or
 * the drawing result. Use __sfCombatProbe.export() for serializable evidence.
 * Four unique tower kinds make consecutive projectile draw runs unambiguous.
 */
(() => {
  if (window.__sfCombatProbe) return true;
  const config = window.__sfProbeConfig || {};
  const lookup = new Map();
  for (const [version, atlas] of Object.entries(config.atlases || {})) {
    for (const [name, frame] of Object.entries(atlas.frames || atlas)) {
      lookup.set(`${version}:${frame.x}:${frame.y}:${frame.w}:${frame.h}`, name);
    }
  }
  const original = {
    clearRect: CanvasRenderingContext2D.prototype.clearRect,
    fillRect: CanvasRenderingContext2D.prototype.fillRect,
    drawImage: CanvasRenderingContext2D.prototype.drawImage,
    WebSocket: window.WebSocket,
    fetch: window.fetch,
  };
  const frames = [], snapshots = [], connections = [];
  const states = new WeakMap(), outlines = new WeakMap(), opaquePixels = new WeakMap();
  let serial = 0;
  const maxFrames = config.maxFrames || 20000;
  const maxSnapshots = config.maxSnapshots || 2000;
  const active = (ctx) => ctx.canvas instanceof HTMLCanvasElement &&
    ctx.canvas.hasAttribute("data-starfort-canvas");
  const compact = (value) => JSON.parse(JSON.stringify(value));
  const rect = (r) => ({ x: r.x, y: r.y, width: r.width, height: r.height });
  const identify = (image, args) => {
    if (args.length === 9 && typeof image?.src === "string") {
      const version = image.src.includes("/world-v2/") ? "world-v2" : "v1";
      return lookup.get(`${version}:${args[1]}:${args[2]}:${args[3]}:${args[4]}`);
    }
    return outlines.get(image);
  };
  function opaquePoint(image, args, name) {
    let cached = opaquePixels.get(image);
    if (!cached) { cached = new Map(); opaquePixels.set(image, cached); }
    if (cached.has(name)) return cached.get(name);
    let result;
    try {
      const isCrop = args.length === 9;
      const width = isCrop ? args[3] : image.width;
      const height = isCrop ? args[4] : image.height;
      const scratch = document.createElement("canvas");
      scratch.width = width; scratch.height = height;
      const ctx = scratch.getContext("2d", { willReadFrequently: true });
      original.drawImage.call(ctx, image, isCrop ? args[1] : 0, isCrop ? args[2] : 0,
        width, height, 0, 0, width, height);
      const pixels = ctx.getImageData(0, 0, width, height).data;
      const centerX = Math.floor(width / 2), centerY = Math.floor(height * .35);
      for (let distance = 0; distance < Math.min(width, height) * .25 && !result; distance++) {
        for (let dy = -distance; dy <= distance && !result; dy++) {
          for (let dx = -distance; dx <= distance && !result; dx++) {
            const x = centerX + dx, y = centerY + dy;
            if (x < 0 || x >= width || y < 0 || y >= height) continue;
            const alpha = pixels[(y * width + x) * 4 + 3];
            if (alpha >= 200) result = { x, y, width, height, alpha };
          }
        }
      }
      result ||= { error: "No opaque tower body pixel in upper-middle search region" };
    } catch (error) {
      result = { error: String(error) };
    }
    cached.set(name, result);
    return result;
  }
  function observe(packet, source = "manual") {
    const room = packet?.state?.room || packet?.room;
    const game = room?.game || packet?.game;
    const data = game?.data || (Array.isArray(packet?.events) ? packet : null);
    if (!data || !Array.isArray(data.events) || !Array.isArray(data.enemies)) return;
    snapshots.push(compact({ at: performance.now(), source,
      roomId: room?.id || packet?.roomId || null,
      version: room?.version || packet?.version || null,
      epoch: data.epoch, tick: data.tick, elapsedMS: data.elapsedMS,
      serverAt: data.serverAt, serverNow: packet?.serverNow,
      phase: data.phase, events: data.events, enemies: data.enemies,
      towers: data.towers }));
    if (snapshots.length > maxSnapshots) snapshots.splice(0, snapshots.length - maxSnapshots);
  }
  CanvasRenderingContext2D.prototype.clearRect = function (...args) {
    if (active(this)) {
      const c = this.canvas;
      const frame = { id: ++serial, at: performance.now(), canvas: rect(c.getBoundingClientRect()),
        viewport: { width: innerWidth, height: innerHeight },
        camera: { x: +c.dataset.cameraX, y: +c.dataset.cameraY, zoom: +c.dataset.zoom },
        size: { width: c.width, height: c.height }, draws: [], bars: [] };
      frames.push(frame);
      if (frames.length > maxFrames) frames.splice(0, frames.length - maxFrames);
      states.set(this, { frame, track: null, enemy: null });
    }
    return original.clearRect.apply(this, args);
  };
  CanvasRenderingContext2D.prototype.drawImage = function (...args) {
    const name = identify(args[0], args);
    if (name && !active(this) && /^(tower|enemy|base)-/.test(name)) outlines.set(this.canvas, name);
    const state = active(this) ? states.get(this) : null;
    if (state && name && /^(tower|enemy|projectile|effect)-/.test(name)) {
      const a = args.length === 9 ? args.slice(5) : args.slice(1);
      if (a.length >= 4) {
        const [x, y, width, height] = a, m = this.getTransform();
        const center = m.transformPoint({ x: x + width / 2, y: y + height / 2 });
        const f = state.frame, scale = f.canvas.width / f.size.width;
        const draw = { name, at: performance.now(), x, y, width, height,
          screenX: f.canvas.x + center.x * scale,
          screenY: f.canvas.y + center.y * scale,
          worldX: (center.x * scale - f.canvas.width / 2) / f.camera.zoom + f.camera.x,
          worldY: (center.y * scale - f.canvas.height / 2) / f.camera.zoom + f.camera.y,
          matrix: [m.a, m.b, m.c, m.d, m.e, m.f], alpha: this.globalAlpha };
        if (name.startsWith("tower-")) {
          const pixel = opaquePoint(args[0], args, name);
          if (pixel.error) draw.opaquePixelError = pixel.error;
          else {
            const point = m.transformPoint({ x: x + width * (pixel.x + .5) / pixel.width,
              y: y + height * (pixel.y + .5) / pixel.height });
            draw.screenPoint = { x: f.canvas.x + point.x * scale,
              y: f.canvas.y + point.y * scale, alpha: pixel.alpha,
              sourceX: pixel.x, sourceY: pixel.y };
          }
        }
        f.draws.push(draw);
        if (name.startsWith("enemy-")) state.enemy = draw;
      }
    }
    return original.drawImage.apply(this, args);
  };
  CanvasRenderingContext2D.prototype.fillRect = function (...args) {
    const state = active(this) ? states.get(this) : null;
    if (state) {
      const color = String(this.fillStyle).toLowerCase();
      if (["#203b2b", "#283b34"].includes(color)) state.track = args.slice();
      else if (["#bfe768", "#ffb482", "#add584"].includes(color) && state.track) {
        const track = state.track, border = (track[3] - args[3]) / 2;
        const width = track[2] - border * 2;
        state.frame.bars.push({ at: performance.now(), x: args[0], y: args[1],
          width: args[2], height: args[3], fullWidth: width,
          ratio: width > 0 ? args[2] / width : null, color,
          enemy: state.enemy ? { name: state.enemy.name,
            worldX: state.enemy.worldX, worldY: state.enemy.worldY } : null });
        state.track = null;
      }
    }
    return original.fillRect.apply(this, args);
  };
  window.WebSocket = class extends original.WebSocket {
    constructor(...args) {
      super(...args);
      const connection = { at: performance.now(), url: String(args[0]), opened: null, closed: null };
      connections.push(connection);
      this.addEventListener("open", () => { connection.opened = performance.now(); });
      this.addEventListener("close", () => { connection.closed = performance.now(); });
      this.addEventListener("message", (event) => {
        if (typeof event.data !== "string") return;
        try { observe(JSON.parse(event.data), "websocket"); } catch { /* non-JSON packet */ }
      });
    }
  };
  window.fetch = function (...args) {
    const response = original.fetch.apply(this, args);
    const url = typeof args[0] === "string" ? args[0] : args[0]?.url || String(args[0]);
    if (/\/api\/(state|rooms)(?:[/?]|$)/.test(url)) {
      response.then((value) => value.clone().json()).then((value) => observe(value, "fetch"), () => {});
    }
    return response;
  };
  window.__sfCombatProbe = {
    observe,
    reset() { frames.length = 0; snapshots.length = 0; connections.length = 0; },
    export() { return compact({ installedAt: started, capturedAt: performance.now(), frames, snapshots, connections }); },
    restore() {
      CanvasRenderingContext2D.prototype.clearRect = original.clearRect;
      CanvasRenderingContext2D.prototype.fillRect = original.fillRect;
      CanvasRenderingContext2D.prototype.drawImage = original.drawImage;
      window.WebSocket = original.WebSocket;
      window.fetch = original.fetch;
    },
  };
  const started = performance.now();
  return true;
})();
