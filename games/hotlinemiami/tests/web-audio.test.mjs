import test from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { mkdir, readFile, unlink, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { GameAudio } from "../../../web/src/audio.ts";

const webRequire = createRequire(new URL("../../../web/package.json", import.meta.url));
const ts = webRequire("typescript");
const source = await readFile(new URL("../web/Board.tsx", import.meta.url), "utf8");
const parsed = ts.createSourceFile("Board.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
const calls = [];
function visit(node) {
  if (ts.isCallExpression(node) && node.expression.getText(parsed) === "props.audio.effect") calls.push(node.getText(parsed));
  ts.forEachChild(node, visit);
}
visit(parsed);
assert.equal(calls.length, 1);
const compiled = ts.transpileModule(`export function emit(audio,event,epoch,url){const props={audio};const data={epoch};return ${calls[0]};}`, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, removeComments: true } }).outputText;
const directory = new URL("../../../.local/hotlinemiami/verification/web/", import.meta.url);
await mkdir(directory, { recursive: true });
const output = new URL("audio-call.cjs", directory);
await writeFile(output, compiled);
const require = createRequire(import.meta.url);
const { emit } = require(fileURLToPath(output));
await unlink(output);

class NativeMedia {
  dataset = {};
  paused = true;
  muted = false;
  loop = false;
  volume = 1;
  duration = NaN;
  readyState = 1;
  onloadedmetadata = null;
  onended = null;
  onerror = null;
  removed = false;
  plays = 0;
  seeks = 0;
  source = "";
  position = 0;
  get src() { return this.source; }
  set src(value) { this.source = value; this.position = 0; }
  get currentSrc() { return this.source; }
  get currentTime() { return this.position; }
  set currentTime(value) { this.seeks++; this.position = value; }
  play() { this.paused = false; this.plays++; this.onloadedmetadata?.(); return Promise.resolve(); }
  pause() { this.paused = true; }
  load() { this.position = 0; }
  remove() { this.removed = true; }
  removeAttribute(name) { if (name === "src") this.source = ""; }
}

function setup() {
  const media = [];
  const audio = new GameAudio(() => { const voice = new NativeMedia(); media.push(voice); return voice; });
  return { audio, port: audio.channel("hotline-test"), media };
}

test("the actual game effect call plays unknown-duration FLAC without a metadata seek", async () => {
  const { audio, port, media } = setup();
  try {
    assert.equal(await emit(port, { id: 17 }, "epoch", "/hotlinemiami/runtime/pistol.flac"), true);
    assert.equal(media.length, 1);
    assert.equal(media[0].plays, 1);
    assert.equal(media[0].paused, false);
    assert.equal(media[0].removed, false);
    assert.equal(media[0].seeks, 0);
    assert.equal(media[0].onloadedmetadata, null);
  } finally { audio.dispose(); }
});

test("game playback IDs remain idempotent and replacement stays within eight finite lanes", async () => {
  const { audio, port, media } = setup();
  try {
    assert.equal(await emit(port, { id: 1 }, "epoch", "/hotlinemiami/runtime/pistol.flac"), true);
    assert.equal(await emit(port, { id: 1 }, "epoch", "/hotlinemiami/runtime/pistol.flac"), true);
    assert.equal(media.length, 1);
    assert.equal(media[0].plays, 1);
    for (let id = 2; id <= 40; id++) assert.equal(await emit(port, { id }, "epoch", "/hotlinemiami/runtime/pistol.flac"), true);
    assert.equal(media.filter(voice => voice.src && !voice.paused && !voice.removed).length, 8);
    assert.ok(media.every(voice => voice.seeks === 0));
  } finally { audio.dispose(); }
});
