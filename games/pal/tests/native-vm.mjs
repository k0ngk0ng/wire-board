// Written before original text/UI integration. Failure cases: per-message
// pauses replace four-line pages; titles consume a body line; timed text waits
// for readers; markup is drawn literally; C locals/UI backups are lost on save;
// unacknowledged pages advance; original menu selection is fabricated.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { createHash } from "node:crypto";
import { inflateSync } from "node:zlib";
import { fileURLToPath } from "node:url";
import { protectedExports } from "./native-stack.mjs";
const root = fileURLToPath(new URL("../../../", import.meta.url));
const wasm = await fs.readFile(
  root +
    (process.env.PAL_NATIVE_WASM ?? (process.env.PAL_REFERENCE_FONT === "1"
      ? ".local/pal/native-reference/rules.wasm"
      : ".local/pal/native/rules.wasm")),
);
const compiled = await WebAssembly.compile(wasm);
const digest = (x) => createHash("sha256").update(x).digest("hex");
const ruleIndex = JSON.parse(
  await fs.readFile(root + ".local/pal/rules/manifest.json"),
);
const archives = {};
const sourceManifest = JSON.parse(
  await fs.readFile(root + "games/pal/assets/source-manifest.json"),
);
for (const name of Object.keys(ruleIndex.files)) {
  const original = process.env.PAL_ORIGINAL_PIXELS === "1";
  archives[name] = await fs.readFile(
    root + (original ? ".local/pal/original-98/" : ".local/pal/rules/") + name,
  );
  assert.equal(
    digest(archives[name]),
    original
      ? sourceManifest.files.find((item) => item.path === name).sha256
      : ruleIndex.files[name],
  );
}
const sss = await fs.readFile(root + ".local/pal/original-98/sss.mkf");
const msg = await fs.readFile(root + ".local/pal/original-98/m.msg");
const word = await fs.readFile(root + ".local/pal/original-98/word.dat");
const offsets = sss.subarray(sss.readUInt32LE(12), sss.readUInt32LE(16));
const gbk = new TextDecoder("gbk");
const messages = Array.from({ length: offsets.length / 4 - 1 }, (_, i) =>
  gbk.decode(
    msg.subarray(offsets.readUInt32LE(i * 4), offsets.readUInt32LE(i * 4 + 4)),
  ),
);
const words = Array.from({ length: word.length / 10 }, (_, i) =>
  gbk.decode(word.subarray(i * 10, i * 10 + 10)).replace(/[\0 ]+$/, ""),
);
async function create(seed = 12345) {
  let instance,
    event,
    response = 0;
  const imports = {};
  for (const f of WebAssembly.Module.imports(compiled)) {
    imports[f.module] ??= {};
    imports[f.module][f.name] = () => {
      throw Error("Unbound original call " + f.module + "." + f.name);
    };
  }
  imports.pal.yield = (kind, value) => {
    const e = instance.exports;
    if (e.asyncify_get_state() === 2) {
      e.asyncify_stop_rewind();
      return response;
    }
    event = { kind, value };
    e.asyncify_start_unwind(e.wb_continuation());
    return 0;
  };
  imports.pal.fail = (p) => {
    const memory = new Uint8Array(instance.exports.memory.buffer);
    let end = p;
    while (memory[end]) end++;
    throw Error(new TextDecoder().decode(memory.subarray(p, end)));
  };
  imports.pal.inflate = (p, n, out, capacity) => {
    const data = inflateSync(
      new Uint8Array(instance.exports.memory.buffer, p, n),
      { maxOutputLength: capacity },
    );
    new Uint8Array(instance.exports.memory.buffer).set(data, out);
    return data.length;
  };
  instance = await WebAssembly.instantiate(compiled, imports);
  const e = protectedExports(instance.exports);
  e._initialize();
  assert.notEqual(
    new Uint32Array(e.memory.buffer, e.gpScreen.value, 1)[0],
    new Uint32Array(e.memory.buffer, e.gpScreenReal.value, 1)[0],
    "physical presentation must not overwrite the logical scene during original fades",
  );

  for (const name of [
    "wb_ui_ops",
    "wb_dialog_info",
    "wb_probe_dialogue",
    "wb_probe_menu",
  ])
    assert.equal(
      typeof e[name],
      "function",
      "missing original UI bridge " + name,
    );
  const put = (data) => {
    const ptr = e.wb_allocate(data.length);
    new Uint8Array(e.memory.buffer).set(data, ptr);
    return ptr;
  };
  const textTable = (rows) => {
    const pointers = rows.map((row) => {
      const data = Buffer.alloc((Array.from(row).length + 1) * 4);
      Array.from(row).forEach((char, i) =>
        data.writeUInt32LE(char.codePointAt(0), i * 4),
      );
      return put(data);
    });
    const data = Buffer.alloc(pointers.length * 4);
    pointers.forEach((ptr, i) => data.writeUInt32LE(ptr, i * 4));
    return put(data);
  };
  for (const [name, data] of Object.entries(archives))
    assert.equal(
      e.wb_bind_file(put(Buffer.from(name + "\0")), put(data), data.length),
      1,
    );
  e.wb_bind_text(0, textTable(words), words.length);
  e.wb_bind_text(1, textTable(messages), messages.length);
  assert.equal(e.wb_boot(seed), 1);
  assert.equal(
    e.PAL_CharWidth(0xb7),
    16,
    "simplified-Chinese font override must follow ISO font loading",
  );
  const ops = () => {
    const p = e.wb_ui_ops(),
      v = new Int32Array(e.memory.buffer, p, 1),
      count = v[0];
    assert.ok(count >= 0 && count <= 8192);
    const data = new Int32Array(e.memory.buffer, p + 4, count * 8);
    return Array.from({ length: count }, (_, i) =>
      Array.from(data.subarray(i * 8, i * 8 + 8)),
    );
  };
  const info = () => {
    const p = e.wb_dialog_info();
    return Array.from(new Int32Array(e.memory.buffer, p, 38));
  };
  let started = false;
  return {
    e,
    ops,
    info,
    step(name, arg = 0, ack = 0, keys = 0, direction = 4) {
      event = null;
      response = ack;
      e.wb_input(direction, keys);
      e.wb_budget(1000000);
      if (started) e.asyncify_start_rewind(e.wb_continuation());
      e[name](arg);
      started = true;
      assert.equal(e.asyncify_get_state(), 1);
      e.asyncify_stop_unwind();
      assert.ok(event);
      return { ...event, ops: ops(), info: info() };
    },
    snapshot: () => {
      const buffer = Buffer.from(new Uint8Array(e.memory.buffer));
      buffer.stackPointer = e.__stack_pointer.value;
      return buffer;
    },
    restore(data) {
      if (data.length > e.memory.buffer.byteLength)
        e.memory.grow((data.length - e.memory.buffer.byteLength) / 65536);
      new Uint8Array(e.memory.buffer).set(data);
      assert.ok(
        Number.isInteger(data.stackPointer) && data.stackPointer > 0,
        "missing suspended native C stack pointer",
      );
      e.__stack_pointer.value = data.stackPointer;
      started = true;
    },
  };
}

export { root, wasm, compiled, digest, messages, words, create };
