import { createHash } from "node:crypto";
import { readFileSync, writeFileSync, mkdirSync, readdirSync, existsSync, realpathSync, statSync, lstatSync, copyFileSync, mkdtempSync, renameSync, rmSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { spawnSync } from "node:child_process";

const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const { transformSync } = require("esbuild");
const builderFile = fileURLToPath(import.meta.url);
const flags = ["-trimpath", "-buildvcs=false", "-ldflags=-s -w"];
const digest = bytes => createHash("sha256").update(bytes).digest("hex");

function within(root, target) {
  const relative = path.relative(root, target);
  return relative !== ".." && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative);
}

function ownedDirectory(root, relative) {
  const owned = path.resolve(root, ".local/hotlinemiami");
  const target = path.resolve(root, relative);
  if (!within(owned, target)) throw Error("Hotline output path must remain inside .local/hotlinemiami");
  let ancestor = target;
  while (!existsSync(ancestor) && path.dirname(ancestor) !== ancestor) ancestor = path.dirname(ancestor);
  if (!within(path.resolve(root), realpathSync(ancestor))) throw Error("Hotline output path escapes the workspace through a symlink");
  if (within(owned, ancestor) && !within(owned, realpathSync(ancestor))) throw Error("Hotline output path redirects into another directory through a symlink");
  mkdirSync(target, { recursive: true });
  return target;
}

function environment(root) {
  const cache = ownedDirectory(root, ".local/hotlinemiami/cache/go");
  const modules = ownedDirectory(root, ".local/hotlinemiami/cache/go-mod");
  const temporary = ownedDirectory(root, ".local/hotlinemiami/tmp");
  return { ...process.env, GOCACHE: cache, GOMODCACHE: modules, GOTMPDIR: temporary, TMPDIR: temporary, GOTOOLCHAIN: "local", GOTELEMETRY: "off", GOENV: "off", GOOS: "js", GOARCH: "wasm" };
}

function go(root, env, args) {
  const result = spawnSync("go", args, { cwd: root, env, encoding: "utf8" });
  if (result.status !== 0) throw Error("Hotline Go build failed: " + (result.stderr || result.stdout || result.error));
  return result.stdout.trim();
}

function sourceFiles(root) {
  const files = [];
  function visit(directory) {
    if (!existsSync(directory)) return;
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const name = path.join(directory, entry.name);
      if (entry.isSymbolicLink()) throw Error("Go source cannot be a symlink: " + name);
      if (entry.isDirectory()) visit(name);
      else if (entry.name.endsWith(".go")) files.push(name);
    }
  }
  for (const directory of ["engine", "cmd", "data"]) visit(path.join(root, "games/hotlinemiami", directory));
  for (const name of ["go.mod", "go.sum"]) if (existsSync(path.join(root, name))) files.push(path.join(root, name));
  if (!files.some(file => file.endsWith(path.join("cmd", "motion", "main_js.go")))) throw Error("Missing Hotline WASM command source");
  return files.sort().map(file => {
    const bytes = readFileSync(file);
    return { path: path.relative(root, file).split(path.sep).join("/"), bytes: bytes.length, sha256: digest(bytes) };
  });
}

function identity(root, env) {
  const toolchain = JSON.parse(go(root, env, ["env", "-json", "GOROOT", "GOVERSION"]));
  const upstream = readFileSync(path.join(toolchain.GOROOT, "lib/wasm/wasm_exec.js"), "utf8");
  const script = transformSync(upstream, { loader: "js", target: "es2022", legalComments: "none", minifyWhitespace: true }).code;
  const licenseFile = [path.join(toolchain.GOROOT, "LICENSE"), path.join(path.dirname(toolchain.GOROOT), "LICENSE")].find(file => existsSync(file) && readFileSync(file, "utf8").includes("Redistribution and use in source and binary forms"));
  if (!licenseFile) throw Error("Selected Go toolchain has no BSD license");
  const copyright = upstream.split("\n").slice(0, 4).filter(line => line.startsWith("//")).map(line => line.replace(/^\/\/\s?/, "")).join("\n");
  const license = copyright + "\n\n" + readFileSync(licenseFile, "utf8");
  const files = sourceFiles(root);
  const sourceSHA256 = digest(JSON.stringify(files));
  const binding = { version: 1, sourceSHA256, goVersion: toolchain.GOVERSION, builderSHA256: digest(readFileSync(builderFile)), upstreamScriptSHA256: digest(upstream), scriptSHA256: digest(script), licenseSHA256: digest(license), flags };
  return { ...binding, buildSHA256: digest(JSON.stringify(binding)), sourceFiles: files, script, license };
}

function descriptor(file, bytes, mime) {
  return { file, bytes: bytes.length, sha256: digest(bytes), mime };
}

function resourcePath(directory, file) {
  if (typeof file !== "string" || !file || file.includes("\\") || file.includes("\0") || path.posix.isAbsolute(file) || file.split("/").some(part => !part || part === "." || part === "..")) throw Error("Invalid resource path: " + file);
  const selected = path.resolve(directory, file);
  if (!within(directory, selected)) throw Error("Resource path escapes its manifest root");
  if (!existsSync(selected) || !statSync(selected).isFile()) throw Error("Missing resource file: " + file);
  let ancestor = selected;
  while (within(directory, ancestor)) {
    if (lstatSync(ancestor).isSymbolicLink()) throw Error("Resource path contains a symlink");
    if (ancestor === path.resolve(directory)) break;
    ancestor = path.dirname(ancestor);
  }
  if (!within(realpathSync(directory), realpathSync(selected))) throw Error("Resource path escapes through a symlink");
  return selected;
}

function readResource(directory, item) {
  if (!Number.isSafeInteger(item.bytes) || item.bytes <= 0 || typeof item.sha256 !== "string" || !/^[0-9a-f]{64}$/.test(item.sha256)) throw Error("Invalid resource bytes/digest metadata: " + item.file);
  const bytes = readFileSync(resourcePath(directory, item.file));
  if (bytes.length !== item.bytes || digest(bytes) !== item.sha256) throw Error("Resource size/digest mismatch: " + item.file);
  return bytes;
}

function resources(manifest) {
  const selected = new Map();
  function visit(value) {
    if (!value || typeof value !== "object") return;
    if (!Array.isArray(value) && Object.hasOwn(value, "file")) {
      const previous = selected.get(value.file);
      if (previous && (previous.bytes !== value.bytes || previous.sha256 !== value.sha256)) throw Error("Conflicting resource descriptor: " + value.file);
      selected.set(value.file, value);
    }
    for (const child of Object.values(value)) visit(child);
  }
  visit(manifest);
  return [...selected.values()].sort((left, right) => left.file.localeCompare(right.file));
}

function validateNativeFloorAudit(nativeMaps) {
  for (const source of nativeMaps) {
    const audit = source.arenaAudit;
    const counts = [audit?.floorCells, audit?.walkableFloorCells, audit?.reachableFloorCells, audit?.reachableAreaPx2, audit?.baselineReachableFloorCells];
    if (!audit || audit.gridStep !== 16 || audit.radius !== 9 || counts.some(value => !Number.isSafeInteger(value) || value <= 0) || audit.reachableFloorCells > audit.walkableFloorCells || audit.walkableFloorCells > audit.floorCells || audit.reachableAreaPx2 !== audit.reachableFloorCells * 256 || audit.reachablePermille !== Math.floor(audit.reachableFloorCells * 1000 / audit.walkableFloorCells) || !Number.isSafeInteger(audit.minSpawnPathPx) || audit.minSpawnPathPx < 0 || audit.allSpawnAndWeaponsReachable !== true || !/^[0-9a-f]{64}$/.test(audit.sourceFloorSHA256) || !/^[0-9a-f]{64}$/.test(audit.sourceMetadataSHA256)) throw Error("Invalid original floor source audit or reachable metrics: " + source.id);
  }
  const additions = new Set(["highball", "exposed", "hot-heavy"]);
  const baseline = Math.max(...nativeMaps.filter(source => !additions.has(source.id)).map(source => source.arenaAudit.reachableFloorCells));
  for (const source of nativeMaps) if (source.arenaAudit.baselineReachableFloorCells !== baseline || source.arenaAudit.baselineRatioPermille !== Math.floor(source.arenaAudit.reachableFloorCells * 1000 / baseline)) throw Error("Original floor baseline audit differs: " + source.id);
}

export function validateHotlineAssets(manifest, nativeMaps) {
  if (manifest.version !== 1 || !manifest.images || !Object.keys(manifest.images).length) throw Error("Incomplete Hotline assets manifest");
  for (const [key, count] of [["maps", 9], ["characters", 8], ["weapons", 12]]) if (!Array.isArray(manifest[key]) || manifest[key].length !== count) throw Error("Incomplete Hotline " + key + ": expected " + count);
  if (!Array.isArray(nativeMaps) || nativeMaps.length !== 9) throw Error("Incomplete native scene catalog");
  const sources = new Map(nativeMaps.map(map => [map.id, map]));
  const rooms = new Set();
  const identities = new Set();
  if (sources.size !== 9) throw Error("Duplicate native scene identity");
  for (const map of manifest.maps) {
    const source = sources.get(map.id);
    const rules = map.rules;
    if (!source || identities.has(map.id) || !rules || rules.id !== map.id || rules.sourceRoomId !== source.sourceRoomId || rules.sourceRoomName !== source.sourceRoomName || rules.sourceRoomOffset !== source.sourceRoomOffset || rooms.has(rules.sourceRoomId)) throw Error("Invalid or duplicate native scene source: " + map.id);
    const image = manifest.images[map.image];
    const preview = manifest.images[map.preview];
    if (!image || !preview) throw Error("Missing native scene image or preview: " + map.id);
    if (!Number.isSafeInteger(map.width) || !Number.isSafeInteger(map.height) || map.width <= 0 || map.height <= 0 || source.coordinateScale !== 1 || source.width !== map.width || source.height !== map.height || image.width !== map.width || image.height !== map.height || rules.coordinateScale !== 256 || rules.width !== map.width * 256 || rules.height !== map.height * 256) throw Error("Native scene dimensions or coordinate scale differ: " + map.id);
    rooms.add(rules.sourceRoomId);
    identities.add(map.id);
  }
  for (const [id, room] of [["highball", 65], ["exposed", 148], ["hot-heavy", 59]]) if (!manifest.maps.some(map => map.id === id && map.rules.sourceRoomId === room)) throw Error("Missing original large native scene: " + id);
  validateNativeFloorAudit(nativeMaps);
}

export function validateHotlineNativeSources(nativeMaps, proof) {
  if (!proof || !/^[0-9a-f]{64}$/.test(proof.sourceMetadataSHA256) || !Array.isArray(proof.maps) || proof.maps.length !== nativeMaps.length) throw Error("Incomplete original floor source proof");
  const sources = new Map(proof.maps.map(source => [source.id, source]));
  if (sources.size !== nativeMaps.length) throw Error("Duplicate original floor scene proof");
  for (const map of nativeMaps) {
    const source = sources.get(map.id);
    const audit = map.arenaAudit;
    if (!source || source.sourceRoomId !== map.sourceRoomId || !Array.isArray(source.floor) || source.floor.length !== audit?.floorCells || audit.sourceMetadataSHA256 !== proof.sourceMetadataSHA256) throw Error("Original floor scene or metadata source differs: " + map.id);
    if (source.floor.some(point => !Number.isSafeInteger(point?.x) || !Number.isSafeInteger(point?.y))) throw Error("Invalid original floor coordinates: " + map.id);
    const points = source.floor.map(point => ({ x: point.x, y: point.y }));
    if (new Set(points.map(point => point.x + ":" + point.y)).size !== points.length || digest(JSON.stringify(points)) !== audit.sourceFloorSHA256) throw Error("Original floor source bytes differ: " + map.id);
  }
}

function sourceMapCatalog(root) {
  const maps = JSON.parse(readFileSync(path.join(root, "games/hotlinemiami/data/maps.json"), "utf8"));
  const proof = JSON.parse(readFileSync(path.join(root, "games/hotlinemiami/engine/testdata/native-floor-points.json"), "utf8"));
  validateHotlineNativeSources(maps, proof);
  return maps;
}

function verifyRuntime(directory, runtime, expected) {
  if (!runtime || runtime.version !== 1 || runtime.sourceSHA256 !== expected.sourceSHA256 || runtime.buildSHA256 !== expected.buildSHA256 || runtime.goVersion !== expected.goVersion) throw Error("Runtime does not match the current Go source binding");
  for (const key of ["wasm", "script", "license"]) readResource(directory, runtime[key]);
  if (!WebAssembly.validate(readResource(directory, runtime.wasm))) throw Error("Invalid Hotline WASM binary");
  if (runtime.script.sha256 !== expected.scriptSHA256 || runtime.license.sha256 !== expected.licenseSHA256) throw Error("Runtime support files do not match selected Go toolchain");
}

function validCached(directory, expected) {
  try {
    const runtime = JSON.parse(readFileSync(path.join(directory, "build.json"), "utf8"));
    verifyRuntime(directory, runtime, expected);
    return runtime;
  } catch {
    return undefined;
  }
}

export function buildHotlineRuntime(workspace) {
  const root = path.resolve(workspace);
  const env = environment(root);
  const cache = ownedDirectory(root, ".local/hotlinemiami/cache/runtime");
  for (let attempt = 0; attempt < 3; attempt++) {
    const current = identity(root, env);
    const target = path.join(cache, current.buildSHA256);
    const cached = validCached(target, current);
    if (cached) return { runtimeRoot: target, runtime: cached, cached: true };
    const temporary = mkdtempSync(path.join(env.GOTMPDIR, "runtime-"));
    try {
      const wasmFile = path.join(temporary, "hotline-motion.wasm");
      go(root, env, ["build", ...flags, "-o", wasmFile, "./games/hotlinemiami/cmd/motion"]);
      if (digest(JSON.stringify(sourceFiles(root))) !== current.sourceSHA256) continue;
      const wasm = readFileSync(wasmFile);
      if (!WebAssembly.validate(wasm)) throw Error("Go compiler produced invalid Hotline WASM");
      writeFileSync(path.join(temporary, "wasm-exec.js"), current.script);
      writeFileSync(path.join(temporary, "go-runtime-LICENSE.txt"), current.license);
      const runtime = {
        version: 1,
        sourceSHA256: current.sourceSHA256,
        buildSHA256: current.buildSHA256,
        goVersion: current.goVersion,
        builderSHA256: current.builderSHA256,
        upstreamScriptSHA256: current.upstreamScriptSHA256,
        sourceFiles: current.sourceFiles,
        flags,
        wasm: descriptor("hotline-motion.wasm", wasm, "application/wasm"),
        script: descriptor("wasm-exec.js", Buffer.from(current.script), "text/javascript"),
        license: descriptor("go-runtime-LICENSE.txt", Buffer.from(current.license), "text/plain"),
      };
      writeFileSync(path.join(temporary, "build.json"), JSON.stringify(runtime, null, 2) + "\n");
      const concurrent = validCached(target, current);
      if (concurrent) return { runtimeRoot: target, runtime: concurrent, cached: true };
      if (existsSync(target)) rmSync(target, { recursive: true, force: true });
      try {
        renameSync(temporary, target);
      } catch (error) {
        const published = validCached(target, current);
        if (published) return { runtimeRoot: target, runtime: published, cached: true };
        throw error;
      }
      verifyRuntime(target, runtime, current);
      return { runtimeRoot: target, runtime, cached: false };
    } finally {
      if (existsSync(temporary)) rmSync(temporary, { recursive: true, force: true });
    }
  }
  throw Error("Hotline Go source changed during compilation; retry once source writes finish");
}

export function verifyHotlineDelivery(workspace, publicRoot) {
  const root = path.resolve(workspace);
  const directory = path.join(publicRoot, "hotlinemiami");
  const manifest = JSON.parse(readFileSync(path.join(directory, "manifest.json"), "utf8"));
  validateHotlineAssets(manifest, sourceMapCatalog(root));
  const selected = resources(manifest);
  for (const item of selected) readResource(directory, item);
  const expected = identity(root, environment(root));
  verifyRuntime(directory, manifest.runtime, expected);
  return { passed: true, sourceSHA256: expected.sourceSHA256, buildSHA256: expected.buildSHA256, resources: selected.length, wasmSHA256: manifest.runtime.wasm.sha256 };
}

export function prepareHotlineDelivery(workspace) {
  const root = path.resolve(workspace);
  const output = ownedDirectory(root, ".local/hotlinemiami/delivery/public");
  const localAssets = path.join(root, ".local/hotlinemiami/assets");
  const hydrated = path.join(root, "games/hotlinemiami/assets/public/hotlinemiami");
  const source = [localAssets, hydrated].find(directory => existsSync(path.join(directory, "manifest.json")));
  if (!source) throw Error("Missing Hotline assets manifest: generate .local/hotlinemiami/assets or hydrate formal resources");
  const raw = readFileSync(path.join(source, "manifest.json"));
  const manifest = JSON.parse(raw);
  validateHotlineAssets(manifest, sourceMapCatalog(root));
  const allResources = resources(manifest);
  for (const item of allResources) readResource(source, item);
  delete manifest.runtime;
  const assetResources = resources(manifest);
  const compiled = buildHotlineRuntime(root);
  const delivered = { ...manifest, runtime: compiled.runtime };
  const expectedText = JSON.stringify(delivered) + "\n";
  const target = path.join(output, "hotlinemiami");
  try {
    if (readFileSync(path.join(target, "manifest.json"), "utf8") === expectedText && verifyHotlineDelivery(root, output).passed) return output;
  } catch {
  }
  const temporaryRoot = ownedDirectory(root, ".local/hotlinemiami/tmp");
  const temporary = mkdtempSync(path.join(temporaryRoot, "delivery-"));
  const staged = path.join(temporary, "hotlinemiami");
  const previous = path.join(temporary, "previous");
  mkdirSync(staged, { recursive: true });
  try {
    for (const item of assetResources) {
      const destination = path.join(staged, item.file);
      mkdirSync(path.dirname(destination), { recursive: true });
      copyFileSync(resourcePath(source, item.file), destination);
    }
    for (const key of ["wasm", "script", "license"]) {
      const item = compiled.runtime[key];
      copyFileSync(resourcePath(compiled.runtimeRoot, item.file), path.join(staged, item.file));
    }
    writeFileSync(path.join(staged, "manifest.json"), expectedText);
    if (digest(readFileSync(path.join(source, "manifest.json"))) !== digest(raw)) throw Error("Hotline assets manifest changed during delivery");
    verifyHotlineDelivery(root, temporary);
    if (existsSync(target)) renameSync(target, previous);
    try {
      renameSync(staged, target);
    } catch (error) {
      if (existsSync(previous) && !existsSync(target)) renameSync(previous, target);
      throw error;
    }
    return output;
  } finally {
    if (existsSync(temporary)) rmSync(temporary, { recursive: true, force: true });
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === builderFile) {
  const root = fileURLToPath(new URL("../../../", import.meta.url));
  const publicRoot = prepareHotlineDelivery(root);
  process.stdout.write(JSON.stringify({ publicRoot, ...verifyHotlineDelivery(root, publicRoot) }) + "\n");
}
