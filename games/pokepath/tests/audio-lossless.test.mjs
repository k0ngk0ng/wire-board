import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { mkdir, mkdtemp, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import path from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { prepareAudio } from "../build/compress-audio.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const evidence = path.join(root, ".local/pokepath/verification/audio-lossless");
const digest = (bytes) => createHash("sha256").update(bytes).digest("hex");
const environment = { ...process.env, TMPDIR: path.join(root, ".local/tmp") };

function probe(filename) {
  return JSON.parse(execFileSync("ffprobe", ["-v", "error", "-select_streams", "a:0", "-show_entries", "stream=codec_name,sample_rate,channels,bits_per_sample,bits_per_raw_sample,duration_ts,time_base", "-of", "json", filename], { env: environment, encoding: "utf8" })).streams[0];
}

function pcm(filename, encoding) {
  return execFileSync("ffmpeg", ["-nostdin", "-v", "error", "-i", filename, "-map", "0:a:0", "-c:a", `pcm_${encoding}`, "-f", encoding, "pipe:1"], { env: environment, maxBuffer: 64 << 20 });
}

async function fixture(name, t) {
  await mkdir(evidence, { recursive: true });
  const base = await mkdtemp(path.join(evidence, `${name}-`));
  t.after(() => rm(base, { recursive: true, force: true }));
  const publicRoot = path.join(base, "public");
  const archiveRoot = path.join(base, "originals");
  const temporaryRoot = path.join(base, "temporary");
  const manifestPath = path.join(base, "manifest.json");
  await mkdir(path.join(publicRoot, "src/assets/sounds/effect"), { recursive: true });
  await mkdir(path.join(publicRoot, "src/assets/sounds/music"), { recursive: true });
  await mkdir(temporaryRoot, { recursive: true });
  const files = [];
  for (const [key, codec, rate, channels] of [["eight", "pcm_u8", 22050, 1], ["sixteen", "pcm_s16le", 44100, 2], ["twentyfour", "pcm_s24le", 48000, 2], ["float", "pcm_f32le", 44100, 1]]) {
    const local = `src/assets/sounds/effect/${key}.wav`;
    const filename = path.join(publicRoot, local);
    execFileSync("ffmpeg", ["-nostdin", "-v", "error", "-y", "-f", "lavfi", "-i", "sine=frequency=701:duration=0.3", "-ar", String(rate), "-ac", String(channels), "-c:a", codec, filename], { env: environment });
    const bytes = await readFile(filename);
    files.push({ path: local, bytes: bytes.length, sha256: digest(bytes), mime: "audio/wav", url: `https://reference.invalid/${local}`, upstreamBytes: bytes.length, upstreamSha256: digest(bytes) });
  }
  const tiny = Buffer.alloc(46);
  tiny.write("RIFF", 0); tiny.writeUInt32LE(38, 4); tiny.write("WAVEfmt ", 8); tiny.writeUInt32LE(16, 16); tiny.writeUInt16LE(1, 20); tiny.writeUInt16LE(1, 22); tiny.writeUInt32LE(44100, 24); tiny.writeUInt32LE(88200, 28); tiny.writeUInt16LE(2, 32); tiny.writeUInt16LE(16, 34); tiny.write("data", 36); tiny.writeUInt32LE(2, 40); tiny.writeInt16LE(32767, 44);
  const tinyPath = "src/assets/sounds/effect/tiny.wav";
  await writeFile(path.join(publicRoot, tinyPath), tiny);
  files.push({ path: tinyPath, bytes: tiny.length, sha256: digest(tiny), mime: "audio/wav", url: `https://reference.invalid/${tinyPath}`, upstreamBytes: tiny.length, upstreamSha256: digest(tiny) });
  const music = Buffer.from("preserved-existing-mp3-byte-carrier");
  const musicPath = "src/assets/sounds/music/route1.mp3";
  await writeFile(path.join(publicRoot, musicPath), music);
  files.push({ path: musicPath, bytes: music.length, sha256: digest(music), mime: "audio/mpeg", url: `https://reference.invalid/${musicPath}` });
  await writeFile(manifestPath, `${JSON.stringify({ version: "1.7.3", files, filesCount: files.length, totalBytes: files.reduce((sum, file) => sum + file.bytes, 0) }, null, 2)}\n`);
  await writeFile(path.join(publicRoot, "resources.json"), `${JSON.stringify({ version: "1.7.3", files })}\n`);
  return { publicRoot, archiveRoot, temporaryRoot, manifestPath, evidencePath: path.join(base, "result.json") };
}

test("真实编码与解码保持整数精度并保留不可压缩和浮点例外", async t => {
  const options = await fixture("fixtures", t);
  const original = JSON.parse(await readFile(options.manifestPath, "utf8"));
  const result = await prepareAudio(options);
  const manifestBytes = await readFile(options.manifestPath);
  const manifest = JSON.parse(manifestBytes);
  const resourcesBytes = await readFile(path.join(options.publicRoot, "resources.json"));
  const resources = JSON.parse(resourcesBytes);
  assert.equal(result.processed, 5);
  assert.equal(result.converted, 3);
  assert.equal(result.retained, 2);
  for (const file of manifest.files.filter((entry) => entry.audio)) {
    const source = path.join(options.archiveRoot, file.audio.logicalPath);
    const actual = path.join(options.publicRoot, file.path);
    const a = probe(source), b = probe(actual);
    assert.equal(a.sample_rate, b.sample_rate, file.path);
    assert.equal(a.channels, b.channels, file.path);
    assert.equal(Number(a.bits_per_raw_sample || a.bits_per_sample), Number(b.bits_per_raw_sample || b.bits_per_sample), file.path);
    assert.deepEqual(pcm(source, file.audio.pcm.encoding), pcm(actual, file.audio.pcm.encoding), file.path);
    assert.equal(digest(pcm(source, file.audio.pcm.encoding)), file.audio.pcm.sha256);
    const bytes = await readFile(actual);
    assert.equal(bytes.length, file.bytes);
    assert.equal(digest(bytes), file.sha256);
    if (file.path.endsWith(".flac")) {
      assert.ok(file.bytes < file.audio.source.bytes);
      assert.equal(file.mime, "audio/flac");
      assert.equal(resources.aliases[file.audio.logicalPath], file.path);
      await assert.rejects(readFile(path.join(options.publicRoot, file.audio.logicalPath)), /ENOENT/);
    }
  }
  const tiny = manifest.files.find((file) => file.path.endsWith("tiny.wav"));
  assert.equal(tiny.audio.transform.reason, "candidate-not-smaller");
  assert.ok(tiny.audio.transform.candidateBytes >= tiny.bytes);
  const float = manifest.files.find((file) => file.path.endsWith("float.wav"));
  assert.equal(float.audio.transform.reason, "unsupported-float-lossless");
  const music = manifest.files.find((file) => file.path.endsWith(".mp3"));
  assert.deepEqual(music, original.files.find((file) => file.path === music.path));
  const beforeStats = await Promise.all(manifest.files.map(async (file) => [file.path, digest(await readFile(path.join(options.publicRoot, file.path)))]));
  const repeated = await prepareAudio(options);
  assert.equal(repeated.converted, 0);
  assert.equal(repeated.reused, 5);
  assert.deepEqual(await readFile(options.manifestPath), manifestBytes);
  assert.deepEqual(await readFile(path.join(options.publicRoot, "resources.json")), resourcesBytes);
  assert.deepEqual(await Promise.all(manifest.files.map(async (file) => [file.path, digest(await readFile(path.join(options.publicRoot, file.path)))])), beforeStats);
});

test("损坏来源或非法路径不改写发布清单", async t => {
  for (const name of ["corrupt-source", "invalid-path"]) {
    const options = await fixture(name, t);
    const bytes = await readFile(options.manifestPath);
    if (name === "corrupt-source") await writeFile(path.join(options.publicRoot, "src/assets/sounds/effect/eight.wav"), "damaged");
    else {
      const invalid = JSON.parse(bytes);
      invalid.files[0].path = "../outside.wav";
      await writeFile(options.manifestPath, `${JSON.stringify(invalid)}\n`);
    }
    const expected = await readFile(options.manifestPath);
    const before = await readdir(path.join(options.publicRoot, "src/assets/sounds/effect"));
    await assert.rejects(prepareAudio(options), /source|digest|path|canonical|outside|mismatch/i);
    assert.deepEqual(await readFile(options.manifestPath), expected);
    assert.deepEqual(await readdir(path.join(options.publicRoot, "src/assets/sounds/effect")), before);
  }
});

test("固定来源可恢复缺失研究缓存而不恢复发布WAV", async t => {
  const options = await fixture("source-recovery", t);
  const logical = "src/assets/sounds/effect/eight.wav";
  const original = await readFile(path.join(options.publicRoot, logical));
  let requests = 0;
  const server = createServer((request, response) => {
    requests++;
    assert.equal(request.url, "/fixed-source.wav");
    response.writeHead(200, { "Content-Type": "audio/wav", "Content-Length": original.length });
    response.end(original);
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  try {
    const manifest = JSON.parse(await readFile(options.manifestPath, "utf8"));
    manifest.files.find((file) => file.path === logical).url = `http://127.0.0.1:${server.address().port}/fixed-source.wav`;
    await writeFile(options.manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);
    await prepareAudio(options);
    await rm(path.join(options.archiveRoot, logical));
    const recovered = await prepareAudio(options);
    assert.equal(recovered.reused, 5);
    assert.equal(requests, 1);
    assert.deepEqual(await readFile(path.join(options.archiveRoot, logical)), original);
    await assert.rejects(readFile(path.join(options.publicRoot, logical)), /ENOENT/);
  } finally {
    await new Promise((resolve) => server.close(resolve));
  }
});

test("全部72个真实WAV来源均具有可复核的无损发布结果", async () => {
  const assetRoot = path.join(root, "games/pokepath/assets");
  const manifest = JSON.parse(await readFile(path.join(assetRoot, "manifest.json"), "utf8"));
  const resources = JSON.parse(await readFile(path.join(assetRoot, "public/pokepath/v1/resources.json"), "utf8"));
  const audio = manifest.files.filter((file) => file.audio);
  assert.equal(audio.length, 72);
  const rows = [];
  for (const file of audio) {
    const source = path.join(root, ".local/pokepath/research/audio-originals", file.audio.logicalPath);
    const actual = path.join(assetRoot, "public/pokepath/v1", file.path);
    const sourceBytes = await readFile(source), actualBytes = await readFile(actual);
    assert.equal(sourceBytes.length, file.audio.source.bytes);
    assert.equal(digest(sourceBytes), file.audio.source.sha256);
    assert.equal(actualBytes.length, file.bytes);
    assert.equal(digest(actualBytes), file.sha256);
    const sourceProbe = probe(source), actualProbe = probe(actual);
    assert.equal(sourceProbe.sample_rate, actualProbe.sample_rate, file.path);
    assert.equal(sourceProbe.channels, actualProbe.channels, file.path);
    assert.equal(Number(sourceProbe.bits_per_raw_sample || sourceProbe.bits_per_sample), Number(actualProbe.bits_per_raw_sample || actualProbe.bits_per_sample), file.path);
    const decoded = pcm(actual, file.audio.pcm.encoding);
    assert.equal(decoded.length, file.audio.pcm.bytes);
    assert.equal(digest(decoded), file.audio.pcm.sha256, file.path);
    assert.deepEqual(decoded, pcm(source, file.audio.pcm.encoding), file.path);
    assert.equal(file.audio.pcm.frames, decoded.length / (file.audio.pcm.channels * file.audio.pcm.bitsPerSample / 8));
    assert.ok(resources.files.some((entry) => entry.path === file.path && entry.bytes === file.bytes && entry.sha256 === file.sha256 && entry.mime === file.mime));
    if (file.path.endsWith(".flac")) {
      assert.ok(file.bytes < sourceBytes.length, file.path);
      assert.equal(resources.aliases[file.audio.logicalPath], file.path);
      await assert.rejects(readFile(path.join(assetRoot, "public/pokepath/v1", file.audio.logicalPath)), /ENOENT/);
    } else assert.ok(["candidate-not-smaller", "unsupported-float-lossless"].includes(file.audio.transform.reason));
    rows.push({ logical: file.audio.logicalPath, published: file.path, sourceBytes: sourceBytes.length, publishedBytes: actualBytes.length, pcm: file.audio.pcm, verified: true });
  }
  await mkdir(evidence, { recursive: true });
  await writeFile(path.join(evidence, "verified.json"), `${JSON.stringify({ command: "node --test games/pokepath/tests/audio-lossless.test.mjs", passed: true, files: rows, sourceBytes: rows.reduce((sum, row) => sum + row.sourceBytes, 0), publishedBytes: rows.reduce((sum, row) => sum + row.publishedBytes, 0) }, null, 2)}\n`);
});
