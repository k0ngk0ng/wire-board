// Reference scene raster from unchanged SDLPAL drawing routines and the original
// pixel archives. The production authority continues to use stripped metadata.
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createHash } from "node:crypto";
import {protectedExports} from './native-stack.mjs';
const root = fileURLToPath(new URL("../../../", import.meta.url));
const withDialogue = process.env.PAL_REFERENCE_FONT === "1";
const wasm = await fs.readFile(
  path.join(
    root,
    withDialogue
      ? ".local/pal/native-reference/rules.wasm"
      : ".local/pal/native/rules.wasm",
  ),
);
const program = await WebAssembly.compile(wasm);
let instance,
  event,
  response = 0;
const imports = {};
for (const fn of WebAssembly.Module.imports(program)) {
  if (fn.kind !== "function") throw Error("unexpected host state");
  imports[fn.module] ??= {};
  imports[fn.module][fn.name] = () => {
    throw Error("unbound scene reference " + fn.module + "." + fn.name);
  };
}
imports.pal.yield = (kind, value) => {
  if (instance.exports.asyncify_get_state() === 2) {
    instance.exports.asyncify_stop_rewind();
    return response;
  }
  event = { kind, value };
  instance.exports.asyncify_start_unwind(instance.exports.wb_continuation());
  return 0;
};
instance = await WebAssembly.instantiate(program, imports);
const e = protectedExports(instance.exports);
e._initialize();
function put(data) {
  const p = e.wb_allocate(data.length);
  new Uint8Array(e.memory.buffer).set(data, p);
  return p;
}
const original = path.join(root, ".local/pal/original-98");
for (const name of [
  "sss",
  "data",
  "map",
  "gop",
  "mgo",
  "abc",
  "f",
  "fire",
  "fbp",
  "ball",
  "rgm",
  "pat",
]) {
  const bytes = await fs.readFile(path.join(original, name + ".mkf"));
  if (
    e.wb_bind_file(
      put(Buffer.from(name + ".mkf\0")),
      put(bytes),
      bytes.length,
    ) !== 1
  )
    throw Error("failed original file binding");
}
const sss = await fs.readFile(path.join(original, "sss.mkf"));
const offset = sss.readUInt32LE(12),
  end = sss.readUInt32LE(16),
  message = await fs.readFile(path.join(original, "m.msg"));
const decoder = new TextDecoder("gbk");
const messages = Array.from({ length: (end - offset) / 4 - 1 }, (_, i) =>
  decoder.decode(
    message.subarray(
      sss.readUInt32LE(offset + i * 4),
      sss.readUInt32LE(offset + i * 4 + 4),
    ),
  ),
);
const wordBytes = await fs.readFile(path.join(original, "word.dat"));
const words = Array.from({ length: wordBytes.length / 10 }, (_, i) =>
  decoder
    .decode(wordBytes.subarray(i * 10, i * 10 + 10))
    .replace(/[\0 ]+$/, ""),
);
function textTable(strings) {
  const pointers = strings.map((text) => {
    const chars = Array.from(text, (c) => c.codePointAt(0));
    const data = Buffer.alloc((chars.length + 1) * 4);
    chars.forEach((c, i) => data.writeUInt32LE(c, i * 4));
    return put(data);
  });
  const data = Buffer.alloc(pointers.length * 4);
  pointers.forEach((p, i) => data.writeUInt32LE(p, i * 4));
  return put(data);
}
e.wb_bind_text(0, textTable(words), words.length);
e.wb_bind_text(1, textTable(messages), messages.length);
if (e.wb_boot(12345) !== 1) throw Error("original init failed");
function state() {
  const p = e.wb_view(),
    v = Array.from(new Uint32Array(e.memory.buffer, p, 32));
  const op = e.wb_scene_ops(),
    count = new Uint32Array(e.memory.buffer, op, 1)[0],
    records = new Int32Array(e.memory.buffer, op + 4, count * 8);
  const draws = Array.from({ length: count }, (_, i) => {
    const p = i * 8;
    return {
      kind: records[p],
      a: records[p + 1],
      b: records[p + 2],
      c: records[p + 3],
      d: records[p + 4],
      x: records[p + 5],
      baseY: records[p + 6],
      sortY: records[p + 7],
    };
  }).sort((a, b) => a.sortY - b.sortY);
  const ui = e.wb_ui_ops(),
    uiCount = new Uint32Array(e.memory.buffer, ui, 1)[0],
    uiOps = new Int32Array(e.memory.buffer, ui + 4, uiCount * 8);
  const overlay = Array.from({ length: uiCount }, (_, i) => {
    const p = i * 8;
    return {
      kind: uiOps[p],
      source: uiOps[p + 1],
      id: uiOps[p + 2],
      frame: uiOps[p + 3],
      x: uiOps[p + 4],
      y: uiOps[p + 5],
      color: uiOps[p + 6],
      small: uiOps[p + 7],
    };
  });
  return {
    scene: v[1],
    x: v[2],
    y: v[3],
    sprite: v[8],
    messages: v[15],
    mode: v[17],
    cameraX: v[23] | 0,
    cameraY: v[24] | 0,
    map: v[25],
    palette: v[26],
    night: !!v[27],
    draws,
    overlay,
  };
}

function raster() {
  const pixels = Buffer.from(
      new Uint8Array(e.memory.buffer, e.wb_pixels(), 64000),
    ),
    palette = Buffer.from(new Uint8Array(e.memory.buffer, e.wb_colors(), 1024)),
    rgba = Buffer.alloc(64000 * 4);
  for (let i = 0; i < pixels.length; i++) {
    rgba[i * 4] = palette[pixels[i] * 4];
    rgba[i * 4 + 1] = palette[pixels[i] * 4 + 1];
    rgba[i * 4 + 2] = palette[pixels[i] * 4 + 2];
    rgba[i * 4 + 3] = 255;
  }
  return rgba;
}
let view;
const pages = [],
  buffers = [];
for (let i = 0; i < 4096; i++) {
  response = event?.kind === 2 ? 1 : 0;
  event = null;
  e.wb_budget(1000000);
  e.wb_input(4, 0);
  if (i) e.asyncify_start_rewind(e.wb_continuation());
  e.wb_run_campaign();
  if (e.asyncify_get_state() === 1) e.asyncify_stop_unwind();
  view = state();
  if (withDialogue && event?.kind === 2 && view.scene === 2) {
    const rgba = raster();
    pages.push({
      view,
      rgbaSHA256: createHash("sha256").update(rgba).digest("hex"),
    });
    buffers.push(rgba);
  }
  if (
    event?.kind === 4 &&
    view.scene === 2 &&
    view.mode === 0 &&
    view.messages > 5
  )
    break;
  if (i === 4095) throw Error("original inn did not reach its reference phase");
}
if (withDialogue && pages.length !== 23)
  throw Error("original dialogue page count changed");
const rgba = withDialogue ? buffers[0] : raster(),
  stem = withDialogue ? "original-inn-dialogue" : "original-inn";
const result = {
  scope: withDialogue
    ? "original PAL scene, text, font and portrait raster from pinned C and original pixel archives"
    : "original PAL scene layer from original pixel archives; dialogue/menu UI not included",
  runtimeSHA256: createHash("sha256").update(wasm).digest("hex"),
  rgbaSHA256: createHash("sha256").update(rgba).digest("hex"),
  view: withDialogue ? pages[0].view : view,
};
if (withDialogue) {
  result.pages = pages;
  await fs.writeFile(
    path.join(root, ".local/pal/verification/" + stem + "-pages.rgba"),
    Buffer.concat(buffers),
  );
}
await fs.writeFile(
  path.join(root, ".local/pal/verification/" + stem + "-reference.json"),
  JSON.stringify(result, null, 2) + "\n",
);
await fs.writeFile(
  path.join(root, ".local/pal/verification/" + stem + ".rgba"),
  rgba,
);
console.log(
  "recorded original inn raster",
  result.rgbaSHA256,
  "pages",
  pages.length,
);
