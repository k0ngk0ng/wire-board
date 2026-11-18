import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { copyFile, mkdir, mkdtemp, readFile, realpath, rename, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { mediaResourceFacts } from "./media-resources.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const assetRoot = path.join(root, "games/pokepath/assets");
const policy = "flac-best-no-padding-no-seektable-v1";
const digest = (bytes) => createHash("sha256").update(bytes).digest("hex");

async function workspacePath(value) {
  const target = path.resolve(value);
  const inside = (candidate) => {
    const local = path.relative(root, candidate);
    return local !== ".." && !local.startsWith(`..${path.sep}`) && !path.isAbsolute(local);
  };
  if (!inside(target)) throw Error(`Path outside workspace: ${target}`);
  let current = target;
  for (;;) {
    try {
      if (!inside(await realpath(current))) throw Error(`Path outside workspace through symlink: ${target}`);
      break;
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
      const parent = path.dirname(current);
      if (parent === current) throw Error(`Invalid workspace path: ${target}`);
      current = parent;
    }
  }
  return target;
}

function execute(command, args, environment, encoding) {
  return execFileSync(command, args, { env: environment, encoding, maxBuffer: 64 << 20, stdio: ["ignore", "pipe", "pipe"] });
}

function inspectAudio(filename, environment, encoding) {
  const data = JSON.parse(execute("ffprobe", ["-v", "error", "-select_streams", "a:0", "-show_entries", "stream=codec_name,sample_rate,channels,bits_per_sample,bits_per_raw_sample,duration_ts,time_base", "-of", "json", filename], environment, "utf8"));
  const stream = data.streams?.[0];
  if (!stream) throw Error(`Audio source has no stream: ${filename}`);
  const bitsPerSample = Number(stream.bits_per_raw_sample || stream.bits_per_sample);
  const sampleFormat = stream.codec_name.includes("f32") ? "float" : "integer";
  const encodings = { 8: "u8", 16: "s16le", 24: "s24le", 32: sampleFormat === "float" ? "f32le" : "s32le" };
  const selected = encoding ?? encodings[bitsPerSample];
  if (!selected || !Number.isSafeInteger(bitsPerSample) || !Number.isSafeInteger(stream.channels) || stream.channels < 1 || !Number.isSafeInteger(Number(stream.sample_rate))) throw Error(`Unsupported source audio format: ${filename}`);
  const pcm = execute("ffmpeg", ["-nostdin", "-v", "error", "-i", filename, "-map", "0:a:0", "-c:a", `pcm_${selected}`, "-f", selected, "pipe:1"], environment);
  const frames = pcm.length / (stream.channels * bitsPerSample / 8);
  if (!Number.isSafeInteger(frames) || frames <= 0) throw Error(`Invalid decoded source frame count: ${filename}`);
  return { codec: stream.codec_name, pcm: { encoding: selected, sampleFormat, sampleRate: Number(stream.sample_rate), channels: stream.channels, bitsPerSample, frames, bytes: pcm.length, sha256: digest(pcm) } };
}

function samePCM(source, candidate) {
  return ["sampleRate", "channels", "bitsPerSample", "frames", "sampleFormat", "bytes", "sha256"].every((key) => source[key] === candidate[key]);
}

async function optionalRead(filename) {
  try {
    return await readFile(filename);
  } catch (error) {
    if (error.code === "ENOENT") return undefined;
    throw error;
  }
}

export async function prepareAudio(options = {}) {
  const manifestPath = await workspacePath(options.manifestPath ?? path.join(assetRoot, "manifest.json"));
  const publicRoot = await workspacePath(options.publicRoot ?? path.join(assetRoot, "public/pokepath/v1"));
  const archiveRoot = await workspacePath(options.archiveRoot ?? path.join(root, ".local/pokepath/research/audio-originals"));
  const temporaryRoot = await workspacePath(options.temporaryRoot ?? path.join(root, ".local/pokepath/audio-compression"));
  const evidencePath = await workspacePath(options.evidencePath ?? path.join(root, ".local/pokepath/verification/audio-lossless/prepare.json"));
  await mkdir(temporaryRoot, { recursive: true });
  const temporary = await mkdtemp(path.join(temporaryRoot, "prepare-"));
  const environment = { ...process.env, TMPDIR: temporary };
  try {
    const manifest = JSON.parse(await readFile(manifestPath, "utf8"));
    mediaResourceFacts(manifest);
    const encoder = execute("flac", ["--version"], environment, "utf8").trim();
    const tools = { encoder, probe: execute("ffprobe", ["-version"], environment, "utf8").split("\n")[0], decoder: execute("ffmpeg", ["-version"], environment, "utf8").split("\n")[0] };
    const updates = new Map();
    const publications = [];
    const removals = [];
    const rows = [];
    let converted = 0, retained = 0, reused = 0;
    for (const entry of manifest.files) {
      if (!entry.path.endsWith(".wav") && !entry.audio) continue;
      const logical = entry.audio?.logicalPath ?? entry.path;
      if (!/^[A-Za-z0-9][A-Za-z0-9._/-]*\.wav$/.test(logical) || logical.split("/").some((part) => part === "." || part === "..")) throw Error(`Invalid source audio path: ${logical}`);
      const original = entry.audio?.source ?? { path: logical, bytes: entry.bytes, sha256: entry.sha256, mime: entry.mime };
      const sourceFile = await workspacePath(path.join(archiveRoot, logical));
      let sourceBytes = await optionalRead(sourceFile);
      if (!sourceBytes) {
        sourceBytes = await optionalRead(path.join(publicRoot, logical));
        if (!sourceBytes) {
          const sourceURL = new URL(entry.url);
          if (!["http:", "https:"].includes(sourceURL.protocol) || !Number.isSafeInteger(original.bytes) || original.bytes < 1 || original.bytes > (64 << 20)) throw Error(`Invalid original source URL or size: ${logical}`);
          const response = await fetch(sourceURL, { signal: AbortSignal.timeout(15000) });
          if (!response.ok || Number(response.headers.get("Content-Length") ?? 0) > original.bytes) throw Error(`Original source download failed: ${logical}`);
          sourceBytes = Buffer.from(await response.arrayBuffer());
        }
      }
      if (sourceBytes.length !== original.bytes || digest(sourceBytes) !== original.sha256) throw Error(`Original source digest mismatch: ${logical}`);
      await mkdir(path.dirname(sourceFile), { recursive: true });
      if (!await optionalRead(sourceFile)) await writeFile(sourceFile, sourceBytes, { flag: "wx" });
      const source = inspectAudio(sourceFile, environment);
      if (!source.codec.startsWith("pcm_")) throw Error(`Unsupported source audio codec: ${source.codec}`);
      const publishedFile = path.join(publicRoot, entry.path);
      const existing = await optionalRead(publishedFile);
      if (entry.audio?.transform.policy === policy && entry.audio.transform.encoder === encoder && existing?.length === entry.bytes && digest(existing) === entry.sha256) {
        const decoded = inspectAudio(publishedFile, environment, source.pcm.encoding);
        if (!samePCM(source.pcm, decoded.pcm) || !samePCM(source.pcm, entry.audio.pcm)) throw Error(`Existing audio PCM mismatch: ${logical}`);
        reused++;
        if (!entry.path.endsWith(".flac")) retained++;
        rows.push({ logical, published: entry.path, sourceBytes: original.bytes, publishedBytes: entry.bytes, pcm: source.pcm, reason: entry.audio.transform.reason, reused: true });
        continue;
      }
      let reason = "unsupported-float-lossless";
      let actual = logical;
      let candidateBytes;
      let candidateSHA256;
      let outputBytes = sourceBytes;
      if (source.pcm.sampleFormat !== "float") {
        const candidateFile = path.join(temporary, `${rows.length}.flac`);
        execute("flac", ["--silent", "--force", "--best", "--no-padding", "--no-seektable", "--verify", "-o", candidateFile, sourceFile], environment);
        const candidate = inspectAudio(candidateFile, environment, source.pcm.encoding);
        if (candidate.codec !== "flac" || !samePCM(source.pcm, candidate.pcm)) throw Error(`Lossless FLAC PCM mismatch: ${logical}`);
        const bytes = await readFile(candidateFile);
        candidateBytes = bytes.length;
        candidateSHA256 = digest(bytes);
        if (bytes.length < sourceBytes.length) {
          actual = logical.replace(/\.wav$/, ".flac");
          outputBytes = bytes;
          reason = "smaller-lossless";
          converted++;
          publications.push({ staged: candidateFile, actual });
          removals.push(logical);
        } else {
          reason = "candidate-not-smaller";
          retained++;
        }
      } else retained++;
      const sourceIdentity = { ...original, path: logical, codec: source.codec };
      const transformed = { ...entry, path: actual, sourcePath: logical, bytes: outputBytes.length, sha256: digest(outputBytes), mime: actual.endsWith(".flac") ? "audio/flac" : "audio/wav", audio: { logicalPath: logical, source: sourceIdentity, pcm: source.pcm, transform: { policy, method: actual.endsWith(".flac") ? "flac-lossless" : "retain-original", reason, encoder, ...(candidateBytes !== undefined ? { candidateBytes, candidateSHA256 } : {}) }, compression: { sourceBytes: original.bytes, publishedBytes: outputBytes.length, ratio: outputBytes.length / original.bytes, savedBytes: original.bytes - outputBytes.length } } };
      updates.set(entry.path, transformed);
      rows.push({ logical, published: actual, sourceBytes: original.bytes, publishedBytes: outputBytes.length, pcm: source.pcm, reason, reused: false });
    }
    manifest.files = manifest.files.map((entry) => updates.get(entry.path) ?? entry).sort((left, right) => left.path.localeCompare(right.path));
    manifest.filesCount = manifest.files.length;
    manifest.totalBytes = manifest.files.reduce((sum, entry) => sum + entry.bytes, 0);
    const facts = mediaResourceFacts(manifest);
    const resourcesPath = path.join(publicRoot, "resources.json");
    const previousResources = JSON.parse(await readFile(resourcesPath, "utf8"));
    const generated = previousResources.files.filter((entry) => entry.path.endsWith(".json"));
    const resources = { version: manifest.version, files: [...facts.files, ...generated].sort((left, right) => left.path.localeCompare(right.path)), aliases: facts.aliases };
    const manifestBytes = Buffer.from(`${JSON.stringify(manifest, null, 2)}\n`);
    const resourcesBytes = Buffer.from(`${JSON.stringify(resources)}\n`);
    for (const publication of publications) {
      const filename = await workspacePath(path.join(publicRoot, publication.actual));
      await mkdir(path.dirname(filename), { recursive: true });
      await copyFile(publication.staged, filename);
    }
    if (!manifestBytes.equals(await readFile(manifestPath))) {
      const staged = path.join(temporary, "manifest.json");
      await writeFile(staged, manifestBytes);
      await rename(staged, manifestPath);
    }
    if (!resourcesBytes.equals(await readFile(resourcesPath))) {
      const staged = path.join(temporary, "resources.json");
      await writeFile(staged, resourcesBytes);
      await rename(staged, resourcesPath);
    }
    for (const logical of removals) await rm(path.join(publicRoot, logical), { force: true });
    const summary = { command: "node games/pokepath/build/compress-audio.mjs", passed: true, policy, tools, processed: rows.length, converted, retained, reused, sourceBytes: rows.reduce((sum, row) => sum + row.sourceBytes, 0), publishedBytes: rows.reduce((sum, row) => sum + row.publishedBytes, 0), files: rows };
    summary.savedBytes = summary.sourceBytes - summary.publishedBytes;
    await mkdir(path.dirname(evidencePath), { recursive: true });
    await writeFile(evidencePath, `${JSON.stringify(summary, null, 2)}\n`);
    return summary;
  } finally {
    await rm(temporary, { recursive: true, force: true });
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) prepareAudio().then(({ files, ...summary }) => process.stdout.write(`${JSON.stringify(summary)}\n`)).catch((error) => { process.stderr.write(`${error.stack}\n`); process.exitCode = 1; });
