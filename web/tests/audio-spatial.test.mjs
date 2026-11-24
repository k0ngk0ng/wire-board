import assert from "node:assert/strict";
import test from "node:test";
import { GameAudio } from "../src/audio.ts";

const listener = { x: 0, y: 0, z: 0, yaw: 0 };
const options = (playbackID, source = { x: 0, y: -50, z: 0 }) => ({ playbackID, source, listener, maxDistance: 100 });
const flush = async () => { for (let i = 0; i < 24; i++) await Promise.resolve(); };

function setup(t, config = {}) {
  const names = ["AudioContext", "webkitAudioContext", "document", "navigator", "fetch", "performance"];
  const previous = new Map(names.map(name => [name, Object.getOwnPropertyDescriptor(globalThis, name)]));
  const contexts = [], elements = [], connections = [], fetches = [], deferred = [], pendingPlay = [];
  const clock = { now: 0 };
  const reads = { arrayBuffers: 0, chunks: 0, cancelled: false };
  class Param {
    value = 1;
    setValueAtTime(value) { this.value = value; }
    setTargetAtTime(value) { this.value = value; }
    cancelScheduledValues() {}
    cancelAndHoldAtTime() {}
  }
  class Context {
    currentTime = 1;
    state = "running";
    deviceOutput = { physicalDevice: true };
    destination = this.deviceOutput;
    sources = [];
    gains = [];
    panners = [];
    decodeCount = 0;
    tracks = [];
    constructor() { contexts.push(this); }
    createMediaStreamDestination() {
      const track = { stopped: false, stop() { this.stopped = true; } };
      this.tracks.push(track);
      return { stream: { getTracks: () => [track] } };
    }
    createBufferSource() {
      const node = { buffer: null, loop: false, starts: 0, stops: 0, onended: null, connect(target) { connections.push({ node, target }); }, disconnect() {}, start() { this.starts++; }, stop() { this.stops++; } };
      this.sources.push(node);
      return node;
    }
    createGain() {
      const node = { gain: new Param(), connect(target) { connections.push({ node, target }); }, disconnect() {} };
      this.gains.push(node);
      return node;
    }
    createStereoPanner() {
      const node = { pan: new Param(), connect(target) { connections.push({ node, target }); }, disconnect() {} };
      this.panners.push(node);
      return node;
    }
    decodeAudioData() {
      this.decodeCount++;
      if (config.decodeError) return Promise.reject(Error("decode failed"));
      return Promise.resolve({ duration: config.duration ?? 0.3, numberOfChannels: 1, length: config.frames ?? 14400, sampleRate: 48000 });
    }
    resume() { this.state = "running"; return Promise.resolve(); }
    close() { this.state = "closed"; return Promise.resolve(); }
  }
  class Media {
    dataset = {};
    paused = true;
    muted = false;
    volume = 1;
    src = "";
    srcObject = null;
    calls = 0;
    removed = false;
    play() {
      this.calls++;
      if (config.denyPlay && this.srcObject) return Promise.reject(Error("NotAllowedError"));
      if (config.deferPlay && this.srcObject) return new Promise(resolve => pendingPlay.push(() => { this.paused = false; resolve(); }));
      this.paused = false;
      return Promise.resolve();
    }
    pause() { this.paused = true; }
    load() {}
    removeAttribute(name) { if (name === "src") this.src = ""; }
    remove() { this.removed = true; }
  }
  const document = new EventTarget();
  document.hidden = false;
  document.body = { append() {} };
  const fetch = (url, request) => {
    fetches.push({ url, signal: request?.signal });
    if (url === "bad.flac") return Promise.resolve({ ok: false });
    const response = {
      ok: true,
      headers: { get: () => config.contentLength ?? null },
      arrayBuffer: async () => { reads.arrayBuffers++; return new Uint8Array(config.oversized ? 2 * 1024 * 1024 + 1 : 8).buffer; },
      body: config.stream ? { getReader: () => ({
        read: async () => { reads.chunks++; return { done: false, value: new Uint8Array(reads.chunks === 1 ? 1024 * 1024 : 1024 * 1024 + 1) }; },
        cancel: async () => { reads.cancelled = true; },
        releaseLock() {},
      }) } : undefined,
    };
    return config.deferFetch ? new Promise(resolve => deferred.push(() => resolve(response))) : Promise.resolve(response);
  };
  const globals = { AudioContext: config.unsupported ? undefined : Context, webkitAudioContext: undefined, document, navigator: { audioSession: { type: "auto" } }, fetch, performance: { now: () => clock.now } };
  for (const [name, value] of Object.entries(globals)) Object.defineProperty(globalThis, name, { value, writable: true, configurable: true });
  const audio = new GameAudio(() => { const media = new Media(); elements.push(media); return media; });
  const port = audio.channel("room-a");
  t.after(async () => {
    audio.dispose();
    await flush();
    for (const [name, value] of previous) {
      if (value) Object.defineProperty(globalThis, name, value);
      else delete globalThis[name];
    }
  });
  return { audio, port, contexts, elements, connections, fetches, deferred, pendingPlay, clock, document, reads };
}

test("rapid transient effects share one native output and decoded source", async t => {
  const x = setup(t);
  for (let index = 0; index < 80; index++) {
    assert.equal(await x.port.transient("attack-" + index % 8, ["attack.flac"], { playbackID: String(index) }), true);
  }
  const ctx = x.contexts[0];
  assert.equal(x.contexts.length, 1);
  assert.equal(x.elements.length, 1);
  assert.equal(x.elements[0].calls, 1);
  assert.equal(ctx.decodeCount, 1);
  assert.equal(ctx.sources.length, 80);
  assert.ok(ctx.panners.every(node => node.pan.value === 0));
  assert.ok(ctx.gains.every(node => node.gain.value === 1));
  assert.equal(await x.port.transient("attack-7", ["attack.flac"], { playbackID: "79" }), false);
});

test("transient and spatial lanes remain independent when listeners move or stop", async t => {
  const x = setup(t);
  await x.port.spatial("same", ["attack.flac"], options("event"));
  await x.port.transient("same", ["attack.flac"], { playbackID: "event" });
  const ctx = x.contexts[0];
  assert.equal(ctx.decodeCount, 1);
  assert.equal(ctx.sources[0].stops, 0);
  x.port.spatialListener({ x: 100, y: 200, z: 300, yaw: 1 });
  assert.equal(ctx.panners[1].pan.value, 0);
  assert.equal(ctx.gains[1].gain.value, 1);
  x.port.stopSpatial();
  assert.equal(ctx.sources[0].stops, 1);
  assert.equal(ctx.sources[1].stops, 0);
  x.port.stop();
  assert.equal(ctx.sources[1].stops, 1);
});

test("paused and muted transient requests never replay on resume", async t => {
  const x = setup(t);
  await x.port.transient("attack", ["attack.flac"], { playbackID: "first" });
  x.port.pause();
  assert.equal(x.contexts[0].sources[0].stops, 1);
  assert.equal(await x.port.transient("attack", ["attack.flac"], { playbackID: "paused" }), false);
  await x.port.resume();
  assert.equal(await x.port.transient("attack", ["attack.flac"], { playbackID: "paused" }), false);
  assert.equal(await x.port.transient("attack", ["attack.flac"], { playbackID: "next" }), true);
  x.audio.setEnabled(false);
  assert.equal(x.contexts[1].sources[0].stops, 1);
  assert.equal(await x.port.transient("attack", ["attack.flac"], { playbackID: "muted" }), false);
  x.audio.setEnabled(true);
  assert.equal(await x.port.transient("attack", ["attack.flac"], { playbackID: "muted" }), false);
  assert.equal(await x.port.transient("attack", ["attack.flac"], { playbackID: "fresh" }), true);
});

test("frequently used transient clips survive cache pressure from other effects", async t => {
  const x = setup(t);
  const play = (source, id) => x.port.transient("attack", [source], { playbackID: String(id) });
  await play("common.flac", 0);
  for (let index = 1; index < 32; index++) await play("other-" + index + ".flac", index);
  await play("common.flac", 32);
  await play("new.flac", 33);
  await play("common.flac", 34);
  assert.equal(x.contexts[0].decodeCount, 33);
});

test("spatial short sounds use the platform native stream with real pan and distance gain", async t => {
  const x = setup(t);
  assert.equal(await x.port.spatial("gun", ["shot.flac"], options("shot-1")), true);
  const ctx = x.contexts[0], media = x.elements[0];
  assert.equal(ctx.sources.length, 1);
  assert.equal(ctx.sources[0].loop, false);
  assert.equal(ctx.sources[0].starts, 1);
  assert.equal(ctx.panners[0].pan.value, 1);
  assert.equal(ctx.gains[0].gain.value, 0.25);
  assert.equal(media.dataset.platformSpatial, "");
  assert.ok(media.srcObject);
  assert.ok(x.connections.every(connection => connection.target !== ctx.deviceOutput));
  assert.equal(navigator.audioSession.type, "playback");
  x.port.spatialListener({ ...listener, yaw: Math.PI });
  assert.equal(ctx.panners[0].pan.value, -1);
  x.port.spatialListener({ ...listener, y: -25 });
  assert.equal(ctx.gains[0].gain.value, 0.5625);
  assert.equal(ctx.sources[0].starts, 1);
  x.audio.setVolume(0.4);
  assert.equal(media.volume, 0.4);
  assert.equal(media.calls, 1);
});

test("ended, stopped and muted event identities cannot replay while new events reuse decoded buffers", async t => {
  const x = setup(t);
  await x.port.spatial("gun", ["shot.flac"], options("shot-1"));
  const ctx = x.contexts[0];
  ctx.sources[0].onended();
  assert.equal(await x.port.spatial("gun", ["shot.flac"], options("shot-1")), false);
  assert.equal(await x.port.spatial("gun", ["shot.flac"], options("shot-2")), true);
  assert.equal(ctx.decodeCount, 1);
  x.port.stopSpatial("gun");
  assert.equal(await x.port.spatial("gun", ["shot.flac"], options("shot-2")), false);
  x.audio.setEnabled(false);
  assert.equal(await x.port.spatial("gun", ["shot.flac"], options("shot-3")), false);
  x.audio.setEnabled(true);
  assert.equal(await x.port.spatial("gun", ["shot.flac"], options("shot-2")), false);
  assert.equal(await x.port.spatial("gun", ["shot.flac"], options("shot-3")), false);
  assert.equal(await x.port.spatial("gun", ["shot.flac"], options("shot-4")), true);
});

for (const action of ["mute", "zero", "hidden", "stop", "pause", "dispose", "owner"])
  test("pending spatial fetch cannot revive after " + action, async t => {
    const x = setup(t, { deferFetch: true });
    const pending = x.port.spatial("gun", ["shot.flac"], options("slow"));
    await flush();
    assert.equal(x.deferred.length, 1);
    if (action === "mute") x.audio.setEnabled(false);
    if (action === "zero") x.audio.setVolume(0);
    if (action === "hidden") { x.document.hidden = true; x.document.dispatchEvent(new Event("visibilitychange")); }
    if (action === "stop") x.port.stop();
    if (action === "pause") x.port.pause();
    if (action === "dispose") x.audio.dispose();
    if (action === "owner") void x.audio.channel("room-b").spatial("gun", ["new.flac"], options("next"));
    assert.equal(await pending, false);
    assert.equal(x.fetches[0].signal.aborted, true);
    x.deferred[0]();
    await flush();
    assert.equal(x.contexts[0].sources.length, 0);
    assert.ok(x.contexts[0].tracks.every(track => track.stopped));
  });

test("old source completion cannot remove a newer shot and active voices stay bounded", async t => {
  const x = setup(t);
  await x.port.spatial("gun", ["shot.flac"], options("first"));
  const ctx = x.contexts[0], ended = ctx.sources[0].onended;
  await x.port.spatial("gun", ["shot.flac"], options("second"));
  assert.equal(ctx.sources[0].stops, 1);
  ended();
  assert.equal(ctx.sources[1].stops, 0);
  for (let i = 0; i < 48; i++) await x.port.spatial("voice-" + i, ["shot.flac"], options("event-" + i));
  assert.ok(ctx.sources.filter(source => source.starts && !source.stops).length <= 32);
  assert.equal(ctx.decodeCount, 1);
  x.port.stopSpatial();
  assert.ok(ctx.sources.every(source => source.stops));
});

test("fallback sources decode exactly one short sound", async t => {
  const x = setup(t);
  assert.equal(await x.port.spatial("gun", ["bad.flac", "shot.flac"], options("fallback")), true);
  assert.deepEqual(x.fetches.map(value => value.url), ["bad.flac", "shot.flac"]);
  assert.equal(x.contexts[0].sources.length, 1);
});

test("short sounds that load too late are discarded", async t => {
  const y = setup(t, { deferFetch: true });
  const pending = y.port.spatial("gun", ["slow.flac"], options("late"));
  await flush();
  y.clock.now = 251;
  y.deferred[0]();
  assert.equal(await pending, false);
  assert.equal(y.contexts[0].sources.length, 0);
});

for (const config of [{ unsupported: true }, { decodeError: true }, { denyPlay: true }, { oversized: true }, { duration: 6 }])
  test("spatial failure is contained for " + Object.keys(config)[0], async t => {
    const x = setup(t, config);
    assert.equal(await x.port.spatial("gun", ["shot.flac"], options("failed")), false);
    x.audio.dispose();
    assert.ok(x.elements.every(media => media.paused && media.srcObject === null));
    assert.ok(x.contexts.every(ctx => ctx.state === "closed" && ctx.tracks.every(track => track.stopped)));
  });

test("invalid spatial inputs never allocate native output", async t => {
  const x = setup(t);
  for (const request of [options(""), options(42), { ...options("bad"), source: { x: NaN, y: 0, z: 0 } }, { ...options("bad"), source: { x: 1e308, y: 0, z: 0 } }, { ...options("bad"), listener: { ...listener, yaw: Infinity } }, { ...options("bad"), maxDistance: 0 }, { ...options("bad"), volume: NaN }])
    assert.equal(await x.port.spatial("gun", ["shot.flac"], request), false);
  assert.equal(x.contexts.length, 0);
});

test("spatial decoded buffers stay within their entry and byte budgets", async t => {
  const x = setup(t, { duration: 5, frames: 240000 });
  for (let i = 0; i < 45; i++) {
    assert.equal(await x.port.spatial("gun", ["shot-" + i + ".flac"], options("shot-" + i)), true);
    x.contexts[0].sources.at(-1).onended();
  }
  const media = x.elements[0];
  assert.ok(Number(media.dataset.spatialBuffers) <= 32);
  assert.ok(Number(media.dataset.spatialDecodedBytes) <= 16 * 1024 * 1024);
  assert.equal(x.contexts[0].decodeCount, 45);
});

for (const config of [{ contentLength: "3000000" }, { stream: true }])
  test("oversized spatial downloads are stopped before buffering the whole response for " + Object.keys(config)[0], async t => {
    const x = setup(t, config);
    assert.equal(await x.port.spatial("gun", ["shot.flac"], options("too-big")), false);
    assert.equal(x.reads.arrayBuffers, 0);
    assert.equal(x.contexts[0].decodeCount, 0);
    assert.equal(x.fetches[0].signal.aborted, true);
    if (config.stream) assert.equal(x.reads.cancelled, true);
  });

test("denied or late native output cannot replay a cancelled short sound", async t => {
  const x = setup(t, { deferPlay: true });
  const pending = x.port.spatial("gun", ["shot.flac"], options("pending-play"));
  await flush();
  assert.equal(x.pendingPlay.length, 1);
  x.port.stop();
  assert.equal(await pending, false);
  x.pendingPlay[0]();
  await flush();
  assert.ok(x.elements.every(media => media.paused && media.srcObject === null));
});
