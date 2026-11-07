// Failure cases identified before changing the client: a disabled/spectator
// controller sends heartbeat pads; disabling repeatedly sends release storms;
// recovery installs an older checkpoint and rewinds the presentation; final
// snapshots continue sending controls. Isolate transport orchestration only.
import test from "node:test";
import assert from "node:assert/strict";
import { build } from "../../../web/node_modules/esbuild/lib/main.js";
const stubs = {
  "./runtime-input": `export class RuntimeInput {enabled=true;constructor(p,r,release){this.release=release}setEnabled(v){if(this.enabled&&!v)this.release();this.enabled=v}sample(){return 1}cancel(){this.release()}dispose(){}configure(){}}`,
  "./runtime-renderer": `export class RuntimeRenderer {async load(){}draw(view){globalThis.__contraDraws.push(view)}}`,
  "./runtime-prediction": `export class RuntimePrediction {confirm(v){this.value=v}cancel(){}dispose(){}sample(now,actor){return actor===0?{...this.value,digest:'predicted',finalized:true,players:[{x:5,y:0,lives:0}],presentation:{tag:'predicted'},audio:{tag:'unconfirmed'}}:undefined}}`,
  "./audio": `export class ContraAudioAdapter {constructor(port){this.port=port}update(view){this.port.seen.push(view)}stop(){}suspend(){}dispose(){}}`,
};
const bundle = await build({
  entryPoints: ["games/contra/web/runtime-client.ts"],
  bundle: true,
  write: false,
  format: "esm",
  platform: "browser",
  plugins: [
    {
      name: "ports",
      setup(b) {
        b.onResolve(
          {
            filter:
              /^\.\/(runtime-input|runtime-renderer|runtime-prediction|audio)$/,
          },
          (a) => ({ path: a.path, namespace: "ports" }),
        );
        b.onLoad({ filter: /.*/, namespace: "ports" }, (a) => ({
          contents: stubs[a.path],
        }));
      },
    },
  ],
});
const { RuntimeClient } = await import(
  "data:text/javascript;base64," +
    Buffer.from(bundle.outputFiles[0].text).toString("base64")
);
function fixture() {
  let tick,
    enabled = true,
    checkpointRevision = "10",
    revision = "10",
    restores = 0;
  globalThis.document = {
    hidden: false,
    addEventListener() {},
    removeEventListener() {},
  };
  globalThis.requestAnimationFrame = (f) => {
    tick = f;
    return 1;
  };
  globalThis.cancelAnimationFrame = () => {};
  const calls = [],
    errors = [],
    seen = [],
    published = [];
  globalThis.__contraDraws = [];
  const snapshot = () => ({
    epoch: "e",
    session: 1,
    revision,
    frame: revision,
    rulesBuildID: "r",
    presentation: {},
    audio: {},
    players: [{ x: 0, y: 0, state: 1, lives: 2, gameOver: 0 }],
    phase: "playing",
    level: 0,
  });
  const transport = {
    inputEnabled: () => enabled,
    sequenceBase: () => 40,
    checkpoint: async () => ({
      epoch: "e",
      rulesBuildID: "r",
      state: { revision: checkpointRevision },
    }),
    ready: async () => {},
    input: async (mask, seq) => {
      calls.push(["input", mask, seq]);
    },
    release: async (seq) => {
      calls.push(["release", seq]);
    },
    poll: async () => ({ snapshot: snapshot(), frames: [] }),
  };
  const client = new RuntimeClient({
    root: { focus() {} },
    canvas: {},
    profile: {},
    audio: { seen, stop() {} },
    audioResources: {},
    manifest: { rulesBuildID: "r" },
    bridge: {
      call(request) {
        if (request.op === "restore") {
          restores++;
          revision = request.checkpoint.state.revision;
        }
        return snapshot();
      },
    },
    initial: snapshot(),
    transport,
    onProgress() {},
    onSnapshot: (v) => published.push(v),
    onError: (e) => errors.push(e.message),
  });
  return {
    client,
    calls,
    errors,
    seen,
    published,
    transport,
    draws: () => globalThis.__contraDraws,
    setEnabled: (v) => (enabled = v),
    setCheckpoint: (v) => (checkpointRevision = v),
    restores: () => restores,
    step: async (n) => {
      tick(n);
      await new Promise((r) => setImmediate(r));
    },
  };
}
test("disabled controls release once and never send heartbeat input; resumed controls use sequence baseline", async () => {
  const f = fixture();
  await f.client.initialize();
  f.setEnabled(false);
  await f.step(1000);
  await f.step(2000);
  assert.deepEqual(
    f.calls.map((c) => c[0]),
    ["release"],
  );
  f.setEnabled(true);
  await f.step(3000);
  assert.deepEqual(f.calls.at(-1), ["input", 1, 42]);
  f.client.dispose();
  assert.deepEqual(f.errors, []);
});
// A checkpoint or stage image can finish after a real hide event. It must not
// consume the adapter's suspended-loop recovery while global output is hidden.
test("an in-flight restore completed while hidden does not consume audio recovery", async () => {
  const f = fixture();
  await f.client.initialize();
  let finish;
  const checkpoint = {
    epoch: "e",
    rulesBuildID: "r",
    state: { revision: "11" },
  };
  f.transport.checkpoint = () => new Promise((r) => (finish = r));
  f.transport.poll = async () => ({
    snapshot: { epoch: "e" },
    frames: [],
    checkpoint: {},
  });
  await f.step(1000);
  globalThis.document.hidden = true;
  f.client.visibility();
  finish(checkpoint);
  await new Promise((r) => setImmediate(r));
  assert.equal(f.seen.length, 1);
  globalThis.document.hidden = false;
  f.client.visibility();
  f.transport.checkpoint = async () => checkpoint;
  await f.step(2000);
  assert.equal(f.seen.length, 2);
  assert.deepEqual(f.errors, []);
  f.client.dispose();
});
test("an old checkpoint from concurrent input recovery cannot rewind a running client", async () => {
  const f = fixture();
  await f.client.initialize();
  f.setCheckpoint("9");
  f.transport.poll = async () => ({
    snapshot: { epoch: "e", revision: "10" },
    frames: [],
    checkpoint: {},
  });
  await f.step(1000);
  assert.equal(f.restores(), 1);
  assert.equal(
    f.seen.length,
    2,
    "resume audio from the newest verified state even when a checkpoint is stale",
  );
  assert.deepEqual(f.errors, []);
  f.client.dispose();
});

// Before predictive rendering: waiting on an input ACK blocks drawing; drawing
// a speculative snapshot accidentally publishes unconfirmed lives/result/audio.
test("unacknowledged local input can draw without publishing prediction as authority", async () => {
  const f = fixture();
  f.transport.actor = () => 0;
  let finish;
  f.transport.input = () => new Promise((r) => (finish = r));
  await f.client.initialize();
  const sounds = f.seen.length,
    now = performance.now();
  await f.step(now + 17);
  await f.step(now + 34);
  assert.ok(
    f.draws().some((v) => v.tag === "predicted"),
    "local view waited for network",
  );
  assert.ok(f.published.every((s) => s.digest !== "predicted" && !s.finalized));
  assert.equal(f.seen.length, sounds, "predicted audio escaped");
  finish();
  await new Promise((r) => setImmediate(r));
  f.client.dispose();
  assert.deepEqual(f.errors, []);
});
test("dispose still drains the final physical release before closing transport", async () => {
  const f = fixture();
  await f.client.initialize();
  f.client.dispose();
  await new Promise((r) => setImmediate(r));
  assert.ok(f.calls.some((c) => c[0] === "release"));
});
// Recovery may install a different CHR mapping before its graphics arrive.
// Prediction must not render that partially prepared checkpoint.
test("checkpoint graphics preparation suspends predictive drawing and input", async () => {
  const f = fixture();
  f.transport.actor = () => 0;
  await f.client.initialize();
  let finish;
  f.setCheckpoint("11");
  f.client.preparePresentation = () => new Promise((r) => (finish = r));
  f.transport.poll = async () => ({
    snapshot: { epoch: "e" },
    frames: [],
    checkpoint: {},
  });
  const now = performance.now();
  await f.step(now + 17);
  const painted = f.draws().length;
  await f.step(now + 34);
  assert.equal(f.draws().length, painted, "unprepared checkpoint was drawn");
  finish();
  await new Promise((r) => setImmediate(r));
  f.client.dispose();
  assert.deepEqual(f.errors, []);
});
