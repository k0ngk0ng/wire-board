// Written before the native bridge and builder. This executes original 98
// scripts, not a hand-authored scene. All unknown host calls fail explicitly.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createHash } from "node:crypto";
import { inflateSync } from "node:zlib";
import {protectedExports} from './native-stack.mjs';

const root = fileURLToPath(new URL("../../../", import.meta.url));
const original = path.join(root, ".local/pal/original-98");
const bytes = await fs.readFile(path.join(root, ".local/pal/native/rules.wasm"));
const compiled = await WebAssembly.compile(bytes);
const authorityRoot=path.join(root,".local/pal/rules");
const authorityManifest=JSON.parse(await fs.readFile(path.join(authorityRoot,"manifest.json"),"utf8"));
const digest = (bytes) => createHash("sha256").update(bytes).digest("hex");
const manifest = JSON.parse(await fs.readFile(path.join(root, "games/pal/assets/source-manifest.json"), "utf8"));
const originals = {};
for (const name of ["sss.mkf", "data.mkf", "word.dat", "m.msg"]) {
  const data = await fs.readFile(path.join(original, name));
  const identity = manifest.files.find((file) => file.path === name);
  assert.equal(digest(data), identity.sha256, `${name} resource mismatch`);
  originals[name] = data;
}
function chunks(data) {
  const count = data.readUInt32LE(0) / 4 - 1;
  return Array.from({ length: count }, (_, i) => data.subarray(data.readUInt32LE(i * 4), data.readUInt32LE(i * 4 + 4)));
}
const sss = chunks(originals["sss.mkf"]);
const offsets = sss[3];
const decoder = new TextDecoder("gbk");
const messages = Array.from({ length: offsets.length / 4 - 1 }, (_, i) => decoder.decode(originals["m.msg"].subarray(offsets.readUInt32LE(i * 4), offsets.readUInt32LE(i * 4 + 4))));
const words = Array.from({ length: originals["word.dat"].length / 10 }, (_, i) => decoder.decode(originals["word.dat"].subarray(i * 10, i * 10 + 10)).replace(/[\0 ]+$/, ""));

async function create() {
  let instance, input = 0, event = null;
  const imports = {};
  for (const entry of WebAssembly.Module.imports(compiled)) {
    imports[entry.module] ??= {};
    assert.equal(entry.kind, "function", `unbound native global ${entry.module}.${entry.name}`);
    imports[entry.module][entry.name] = (...args) => { throw Error(`Unsupported native host call ${entry.module}.${entry.name}(${args.join(",")})`); };
  }
  imports.pal ??= {};
  imports.pal.yield = (kind, value) => {
    if (instance.exports.asyncify_get_state() === 2) {
      instance.exports.asyncify_stop_rewind();
      return input;
    }
    event = { kind, value };
    instance.exports.asyncify_start_unwind(instance.exports.wb_continuation());
    return 0;
  };
  imports.pal.inflate=(source,size,target,capacity)=>{
    const input=Buffer.from(instance.exports.memory.buffer,source,size);
    const output=inflateSync(input,{maxOutputLength:capacity});
    new Uint8Array(instance.exports.memory.buffer).set(output,target);return output.length;
  };
  instance = await WebAssembly.instantiate(compiled, imports);
  instance.exports._initialize?.();
  const e = protectedExports(instance.exports);
  const put = (data) => {
    const address = e.wb_allocate(data.length);
    assert.ok(address, "native allocation failed");
    new Uint8Array(e.memory.buffer).set(data, address);
    return address;
  };
  const textTable = (strings) => {
    const addresses = strings.map((text) => {
      const chars = Array.from(text, (char) => char.codePointAt(0));
      const data = Buffer.alloc((chars.length + 1) * 4);
      chars.forEach((char, i) => data.writeUInt32LE(char, i * 4));
      return put(data);
    });
    const data = Buffer.alloc(addresses.length * 4);
    addresses.forEach((address, i) => data.writeUInt32LE(address, i * 4));
    return put(data);
  };
  return {
    instance,
    async boot() {
      for (const name of Object.keys(authorityManifest.files).sort()) {
        const body=await fs.readFile(path.join(authorityRoot,name));
        assert.equal(digest(body),authorityManifest.files[name]);
        const namePtr = put(Buffer.from(name + "\0"));
        const address = put(body);
        assert.equal(e.wb_bind_file(namePtr,address,body.length),1);
      }
      e.wb_bind_text(0, textTable(words), words.length);
      e.wb_bind_text(1, textTable(messages), messages.length);
      assert.equal(e.wb_boot(12345), 1, "original rule initialization failed");
    },
    state() {
      const address = e.wb_view();
      const data = Array.from(new Uint32Array(e.memory.buffer, address, 20));
      return { schema: data[0], scene: data[1], x: data[2], y: data[3], party: data.slice(5, 5 + data[4]), sprite: data[8], hp: data[9], mp: data[10], cash: data[11], seed: data[12], clock: data[13], pc: data[14], messages: data[15], currentMessage: data[16] - 1 };
    },
    step(rewind = true, response = 0) {
      event = null;
      input = response;
      e.wb_budget(1000000);
      if (rewind) e.asyncify_start_rewind(e.wb_continuation());
      e.wb_run_opening();
      if (e.asyncify_get_state() === 1) e.asyncify_stop_unwind();
      assert.ok(event, "native script stopped without a declared yield");
      return { ...event, state: this.state() };
    },
    snapshot() { const memory=Buffer.from(new Uint8Array(e.memory.buffer));memory.stackPointer=e.__stack_pointer.value;return memory; },
    restore(snapshot) {
      if (snapshot.length > e.memory.buffer.byteLength) e.memory.grow((snapshot.length - e.memory.buffer.byteLength) / 65536);
      new Uint8Array(e.memory.buffer).set(snapshot);
      assert.ok(Number.isInteger(snapshot.stackPointer));e.__stack_pointer.value=snapshot.stackPointer;
    },
  };
}
const trace = [];
const a = await create(); await a.boot();
let event = a.step(false);
function advance(runtime, event, target) {
  for (let i = 0; i < 4096; i++) {
    trace.push(event);
    if (event.state.messages >= target) return event;
    event = runtime.step();
  }
  throw Error("original script did not reach the expected phase");
}
event = advance(a, event, 3);
assert.equal(event.state.currentMessage, 2);
assert.notEqual(event.kind,2,"original timed dream text gained a manual wait");
assert.equal(messages[0], "$10李～逍～遥，李～逍～遥！~30");
assert.deepEqual({ scene: event.state.scene, x: event.state.x, y: event.state.y, sprite: event.state.sprite, party: event.state.party }, { scene: 1, x: 1024, y: 1024, sprite: 193, party: [0] });
const snapshot = a.snapshot();
const snapshotID = digest(snapshot);
const atSave = a.state();
const b = await create(); b.restore(snapshot);
assert.deepEqual(b.state(), atSave, "new instance lost C state or resource cursors");
function finish(runtime) {
  const output = [];
  let e = runtime.step(), count = atSave.messages;
  for (let i = 0; i < 4096; i++) {
    assert.notEqual(e.kind,2,"timed opening gained a manual page");
    if (e.state.messages > count) { count = e.state.messages; output.push({ id: e.state.currentMessage, text: messages[e.state.currentMessage], state: e.state }); }
    if (e.kind === 3) return { messages: output, final: e.state };
    e = runtime.step();
  }
  throw Error("original opening failed to finish");
}
const resultA = finish(a);
assert.equal(b.state().scene, 1, "independent instance was advanced by another room");
const resultB = finish(b);
assert.deepEqual(resultA, resultB, "restored opening changed messages, state, time or random sequence");
assert.deepEqual(resultA.messages.map((event) => event.id), [3, 4]);
assert.equal(resultA.final.scene, 2);
assert.equal(resultA.final.cash, atSave.cash);
assert.equal(digest(snapshot), snapshotID, "resume mutated original saved bytes");
const out = path.join(root, ".local/pal/verification");
await fs.mkdir(out, { recursive: true });
await fs.writeFile(path.join(out, "native-opening.json"), JSON.stringify({ passed: true, scope: "original 98 opening trigger script and independent memory plus C stack-pointer resume; no render/multiplayer/full campaign claim", runtimeSHA256: digest(bytes), resourceCommit: manifest.commit, snapshotBytes: snapshot.length, stackPointer: snapshot.stackPointer, snapshotSHA256: snapshotID, trace, restored: resultB }, null, 2) + "\n");
console.log("PASS original PAL 98 opening, dialogue save and fresh-instance continuation");
