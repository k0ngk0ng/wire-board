import assert from "node:assert/strict";
import test from "node:test";
import { SceneMusic } from "../web/music-state.ts";

const main = { key: "day-main", role: "main", src: "music/main.flac", required: true, loopStartSeconds: 50 };
const drums = { key: "day-drums", role: "drums", src: "music/drums.flac", required: false, initiallyMuted: true, loopStartSeconds: 50 };
const frame = (extra = {}) => ({ enabled: true, visible: true, ready: true, loading: false, paused: false, finished: false, elapsed: 12, pressure: 0, ...extra });
function fixture(theme = { tracks: [main, drums] }, prepare = async () => {}) {
  const calls = [], known = new Set();
  const port = {
    async track(key, sources, options) { known.add(key); calls.push({ type: "track", key, sources, options }); return true; },
    stopTrack(key) { calls.push({ type: "stop", key }); },
    async restoreTrack(key, id, options) { calls.push({ type: "restore", key, id, options }); return known.has(key); },
    seekTrack(key, offset) { calls.push({ type: "seek", key, offset }); return true; },
    muteTrack(key, muted) { calls.push({ type: "mute", key, muted }); },
  };
  return { driver: new SceneMusic(port, "https://example.test/pvz/classic-v2", theme, "room:day", prepare), calls };
}
test("initial mix fetches only the required melody and snapshots never restart it", async () => {
  const prepared = [], { driver, calls } = fixture(undefined, async src => { prepared.push(src); });
  await driver.sync(frame()); await driver.sync(frame({ elapsed: 15, pressure: 9 }));
  assert.deepEqual(prepared, [main.src]);
  assert.equal(calls.filter(call => call.type === "track").length, 1);
  await driver.sync(frame({ elapsed: 18, pressure: 10 }));
  assert.deepEqual(prepared, [main.src, drums.src]);
  const rhythm = calls.find(call => call.type === "track" && call.key.endsWith("drums"));
  assert.equal(rhythm.options.startOffset, 18); assert.equal(rhythm.options.muted, false);
  await driver.sync(frame({ elapsed: 20, pressure: 0 }));
  assert.ok(calls.some(call => call.type === "mute" && call.key.endsWith("drums") && call.muted));
  await driver.sync(frame({ elapsed: 23, pressure: 11 }));
  assert.ok(calls.some(call => call.type === "seek" && call.key.endsWith("drums") && call.offset === 23));
  assert.equal(calls.filter(call => call.type === "track" && call.key.endsWith("main")).length, 1);
});
test("late optional download aligns to the newest authority time", async () => {
  let done;
  const { driver, calls } = fixture(undefined, src => src === drums.src ? new Promise(resolve => { done = resolve; }) : Promise.resolve());
  await driver.sync(frame());
  const pending = driver.sync(frame({ elapsed: 30, pressure: 10 }));
  await Promise.resolve();
  const newest = driver.sync(frame({ elapsed: 37, pressure: 10 }));
  done(); await Promise.all([pending, newest]);
  const rhythm = calls.find(call => call.type === "track" && call.key.endsWith("drums"));
  assert.equal(rhythm.options.startOffset, 37);
});
test("hidden/muted cleanup restores only current continuous music", async () => {
  const { driver, calls } = fixture();
  await driver.sync(frame()); await driver.sync(frame({ enabled: false, elapsed: 14 }));
  assert.ok(calls.some(call => call.type === "stop"));
  await driver.sync(frame({ elapsed: 25 }));
  assert.ok(calls.some(call => call.type === "restore" && call.options.startOffset === 25));
  driver.dispose(); const count = calls.length;
  await driver.sync(frame({ elapsed: 40, pressure: 100 }));
  assert.equal(calls.length, count);
});
test("night fallback never layers a second melody or a silent standby bar", async () => {
  const quiet = { ...main, key: "night-main" };
  const standby = { ...drums, key: "night-drums", initiallyMuted: false, intentionalSilence: true };
  const { driver, calls } = fixture({ tracks: [quiet, standby], variants: { even: { tracks: [{ ...main, key: "night-even" }] } } });
  await driver.sync(frame({ pressure: 100 }));
  assert.deepEqual(calls.filter(call => call.type === "track").map(call => call.key), ["pvz-music-night-main"]);
});
test("a changed authority clock seeks current lanes without restarting the melody", async () => {
  const { driver, calls } = fixture();
  await driver.sync(frame({ clockEpoch: 1 })); await driver.sync(frame({ elapsed: 11, clockEpoch: 2 }));
  assert.ok(calls.some(call => call.type === "seek" && call.key.endsWith("main") && call.offset === 11));
  assert.equal(calls.filter(call => call.type === "track").length, 1);
});
test("failed media does not retry on every game snapshot", async () => {
  let starts = 0;
  const port = { async restoreTrack() { return false; }, async track() { starts++; return false; }, stopTrack() {}, seekTrack() { return false; }, muteTrack() {} };
  const driver = new SceneMusic(port, "https://example.test", { tracks: [main] }, "room");
  await driver.sync(frame()); await driver.sync(frame({ elapsed: 13 }));
  assert.equal(starts, 1);
  driver.retry(); await driver.sync(frame({ elapsed: 14 }));
  assert.equal(starts, 2);
});
