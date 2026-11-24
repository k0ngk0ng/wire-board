import test from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { mkdir, rm } from "node:fs/promises";
import { fileURLToPath, pathToFileURL } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const temporary = path.join(root, ".local/yuyuhakusho/effect-mixer/unit-temporary");
await mkdir(temporary, { recursive: true });
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
await require("esbuild").build({ entryPoints: [path.join(root, "games/yuyuhakusho/web/audio.ts")], outfile: path.join(temporary, "adapter.mjs"), bundle: true, format: "esm", legalComments: "none" });
const { MatchAudio } = await import(pathToFileURL(path.join(temporary, "adapter.mjs")));
await rm(temporary, { recursive: true, force: true });

function setup(t) {
  const previous = new Map(["document", "window"].map(name => [name, Object.getOwnPropertyDescriptor(globalThis, name)]));
  const document = new EventTarget();
  document.hidden = false;
  Object.defineProperty(globalThis, "document", { value: document, configurable: true });
  Object.defineProperty(globalThis, "window", { value: new EventTarget(), configurable: true });
  const calls = [];
  const port = {
    transient: (lane, sources, options) => { calls.push({ lane, sources, options }); return Promise.resolve(true); },
    music: () => Promise.resolve(true),
    stop() {},
  };
  const audio = new MatchAudio(port, "/fixture", "epoch", 0);
  audio.sync({ phase: "playing", options: { arena: 0 } }, undefined, true, true, false);
  t.after(() => {
    audio.dispose();
    for (const [name, descriptor] of previous) {
      if (descriptor) Object.defineProperty(globalThis, name, descriptor);
      else delete globalThis[name];
    }
  });
  return { audio, calls };
}

test("combat requests bounded transient lanes with unique event identities", t => {
  const { audio, calls } = setup(t);
  for (let frame = 1; frame <= 80; frame++) audio.consume({ frame: String(frame), paused: false }, [0x77, 0x7b]);
  assert.equal(calls.length, 160);
  assert.equal(new Set(calls.map(call => call.lane)).size, 8);
  assert.equal(new Set(calls.map(call => call.options.playbackID)).size, 160);
  assert.match(calls[0].sources[0], /\/77\.flac$/);
});

test("corrections, mute and recovery do not replay old combat sounds", t => {
  const { audio, calls } = setup(t);
  audio.consume({ frame: "10", paused: false }, [0x77]);
  audio.consume({ frame: "10", paused: false }, [0x77]);
  audio.consume({ frame: "9", paused: false }, [0x77]);
  audio.suspend();
  audio.consume({ frame: "11", paused: false }, [0x77]);
  audio.discardThrough("20");
  audio.sync({ phase: "playing", options: { arena: 0 } }, undefined, true, true, false);
  audio.consume({ frame: "20", paused: false }, [0x77]);
  audio.consume({ frame: "21", paused: false }, [0x77]);
  assert.equal(calls.length, 2);
  assert.deepEqual(calls.map(call => call.options.playbackID), ["epoch:10:0:119", "epoch:21:0:119"]);
});
