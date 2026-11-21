import { createHash } from "node:crypto";
import { readFileSync, writeFileSync, mkdirSync, readdirSync, existsSync, realpathSync, lstatSync, copyFileSync, mkdtempSync, renameSync, rmSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { spawnSync } from "node:child_process";
import { gunzipSync } from "node:zlib";

const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const { transformSync } = require("esbuild");
const builderFile = fileURLToPath(import.meta.url);
const flags = ["-trimpath", "-buildvcs=false", "-ldflags=-s -w"];
const digest = bytes => createHash("sha256").update(bytes).digest("hex");
const maps = ["de_dust2", "de_inferno", "de_aztec", "cs_assault"];
const players = ["terror", "leet", "arctic", "guerilla", "urban", "gsg9", "sas", "gign"];
const weapons = ["glock18", "usp", "p228", "deagle", "elite", "fiveseven", "m3", "xm1014", "mp5navy", "tmp", "p90", "mac10", "ump45", "galil", "famas", "ak47", "m4a1", "sg552", "aug", "scout", "awp", "g3sg1", "sg550", "m249"];

function within(root, selected) {
  const relative = path.relative(path.resolve(root), path.resolve(selected));
  return relative !== ".." && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative);
}

function directory(root, relative) {
  const base = path.resolve(root, ".local/cs16");
  const target = path.resolve(base, relative);
  if (!within(base, target)) throw Error("CS16 output path must stay in .local/cs16");
  let ancestor = target;
  while (!existsSync(ancestor)) ancestor = path.dirname(ancestor);
  if (!within(path.resolve(root), realpathSync(ancestor)) || within(base, ancestor) && !within(base, realpathSync(ancestor))) throw Error("CS16 output symlink escapes its workspace");
  mkdirSync(target, { recursive: true });
  return target;
}

function environment(root) {
  return { ...process.env, GOCACHE: directory(root, "go-cache"), GOMODCACHE: directory(root, "go-mod"), GOTMPDIR: directory(root, "tmp"), TMPDIR: directory(root, "tmp"), GOTOOLCHAIN: "local", GOTELEMETRY: "off", GOENV: "off", GOOS: "js", GOARCH: "wasm" };
}

function go(root, env, args) {
  const result = spawnSync("go", args, { cwd: root, env, encoding: "utf8" });
  if (result.status !== 0) throw Error("CS16 Go build failed: " + (result.stderr || result.stdout || result.error));
  return result.stdout.trim();
}

export function sourceIdentity(root) {
  const files = [];
  function visit(current) {
    if (!existsSync(current)) return;
    for (const entry of readdirSync(current, { withFileTypes: true })) {
      const file = path.join(current, entry.name);
      if (entry.isSymbolicLink()) throw Error("Source symlink is prohibited");
      if (entry.isDirectory()) visit(file);
      else if (entry.name.endsWith(".go") && !entry.name.endsWith("_test.go")) files.push(file);
    }
  }
  for (const name of ["engine", "data", "cmd/motion"]) visit(path.join(root, "games/cs16", name));
  for (const name of ["go.mod", "go.sum"]) if (existsSync(path.join(root, name))) files.push(path.join(root, name));
  const sources = files.sort().map(file => {
    const bytes = readFileSync(file);
    return { path: path.relative(root, file).split(path.sep).join("/"), bytes: bytes.length, sha256: digest(bytes) };
  });
  return { files: sources, sha256: digest(JSON.stringify(sources)) };
}

export function resourceDescriptors(manifest) {
  const items = new Map();
  function visit(value) {
    if (!value || typeof value !== "object") return;
    if (!Array.isArray(value) && Object.hasOwn(value, "file")) {
      const previous = items.get(value.file);
      if (previous && ["bytes", "sha256", "mime", "encoding", "decodedBytes", "decodedSHA256"].some(key => previous[key] !== value[key])) throw Error("Conflicting resource descriptor: " + value.file);
      items.set(value.file, value);
    }
    for (const child of Object.values(value)) visit(child);
  }
  visit(manifest);
  return [...items.values()].sort((a, b) => a.file.localeCompare(b.file));
}

export function validateResource(root, item) {
  const file = item?.file;
  if (typeof file !== "string" || !file || file.includes("\\") || file.includes("\0") || path.posix.isAbsolute(file) || file.split("/").some(part => !part || part === "." || part === "..")) throw Error("Invalid resource path");
  if (!Number.isSafeInteger(item.bytes) || item.bytes <= 0 || !/^[0-9a-f]{64}$/.test(item.sha256) || typeof item.mime !== "string" || !/^[a-z0-9.+-]+\/[a-z0-9.+-]+$/i.test(item.mime)) throw Error("Invalid resource metadata");
  const target = path.resolve(root, file);
  if (!within(root, target)) throw Error("Resource path escapes its directory");
  let current = target;
  while (within(root, current)) {
    if (lstatSync(current).isSymbolicLink()) throw Error("Resource path contains a symlink");
    if (current === path.resolve(root)) break;
    current = path.dirname(current);
  }
  if (!lstatSync(target).isFile()) throw Error("Resource is not a regular file");
  const bytes = readFileSync(target);
  if (bytes.length !== item.bytes || digest(bytes) !== item.sha256) throw Error("Resource size/digest mismatch: " + file);
  if (item.encoding !== undefined) {
    if (item.encoding !== "gzip") throw Error("Unsupported resource encoding");
    if (!Number.isSafeInteger(item.decodedBytes) || item.decodedBytes <= 0 || item.decodedBytes > 128 * 1024 * 1024 || !/^[0-9a-f]{64}$/.test(item.decodedSHA256)) throw Error("Invalid decoded resource metadata");
    const decoded = gunzipSync(bytes, { maxOutputLength: item.decodedBytes });
    if (decoded.length !== item.decodedBytes || digest(decoded) !== item.decodedSHA256) throw Error("Decoded resource identity mismatch: " + file);
  }
  return bytes;
}

export function validateAssets(manifest) {
  if (manifest.schema !== 1) throw Error("Unsupported CS16 asset schema");
  const resource = value => typeof value?.file === "string" && Number.isSafeInteger(value.bytes) && value.bytes > 0 && /^[0-9a-f]{64}$/.test(value.sha256) && typeof value.mime === "string";
  for (const id of maps) for (const key of ["render", "rules", "preview"]) if (!resource(manifest.maps?.[id]?.[key])) throw Error("Missing classic map resource: " + id + " " + key);
  for (const id of players) if (!resource(manifest.players?.[id])) throw Error("Missing player model: " + id);
  for (const id of weapons) for (const key of ["view", "player", "world"]) if (!resource(manifest.weapons?.[id]?.[key])) throw Error("Missing firearm model: " + id + " " + key);
  if (!resourceDescriptors(manifest).length) throw Error("Empty CS16 resource manifest");
}

function descriptor(file, bytes, mime) {
  return { file, bytes: bytes.length, sha256: digest(bytes), mime };
}

function runtimeIdentity(root, env) {
  const tools = JSON.parse(go(root, env, ["env", "-json", "GOROOT", "GOVERSION"]));
  const source = sourceIdentity(root);
  const upstream = readFileSync(path.join(tools.GOROOT, "lib/wasm/wasm_exec.js"), "utf8");
  const script = transformSync(upstream, { loader: "js", target: "es2022", legalComments: "none", minifyWhitespace: true }).code;
  const licenseFile = [path.join(tools.GOROOT, "LICENSE"), path.join(path.dirname(tools.GOROOT), "LICENSE")].find(file => existsSync(file));
  if (!licenseFile) throw Error("Go toolchain license is missing");
  const copyright = upstream.split("\n").slice(0, 4).filter(line => line.startsWith("//")).map(line => line.replace(/^\/\/\s?/, "")).join("\n");
  const license = copyright + "\n\n" + readFileSync(licenseFile, "utf8");
  const binding = { version: 1, sourceSHA256: source.sha256, goVersion: tools.GOVERSION, builderSHA256: digest(readFileSync(builderFile)), scriptSHA256: digest(script), licenseSHA256: digest(license), flags };
  return { ...binding, sourceFiles: source.files, buildSHA256: digest(JSON.stringify(binding)), script, license };
}

function verifyRuntime(root, runtime, identity) {
  if (runtime?.version !== 1 || runtime.sourceSHA256 !== identity.sourceSHA256 || runtime.buildSHA256 !== identity.buildSHA256 || runtime.goVersion !== identity.goVersion) throw Error("CS16 runtime source binding mismatch");
  const wasm = validateResource(root, runtime.wasm);
  validateResource(root, runtime.script);
  validateResource(root, runtime.license);
  if (!WebAssembly.validate(wasm) || runtime.script.sha256 !== identity.scriptSHA256 || runtime.license.sha256 !== identity.licenseSHA256) throw Error("CS16 runtime support mismatch");
}

export function buildCS16Runtime(workspace) {
  const root = path.resolve(workspace);
  const env = environment(root);
  const identity = runtimeIdentity(root, env);
  const target = path.join(directory(root, "runtime"), identity.buildSHA256);
  if (existsSync(path.join(target, "build.json"))) {
    const runtime = JSON.parse(readFileSync(path.join(target, "build.json"), "utf8"));
    verifyRuntime(target, runtime, identity);
    return { root: target, runtime, cached: true };
  }
  const temporary = mkdtempSync(path.join(directory(root, "tmp"), "runtime-"));
  try {
    go(root, env, ["build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -X main.sourceSHA256=" + identity.sourceSHA256, "-o", path.join(temporary, "motion.wasm"), "./games/cs16/cmd/motion"]);
    if (sourceIdentity(root).sha256 !== identity.sourceSHA256) throw Error("CS16 source changed during compilation; retry after writes finish");
    const wasm = readFileSync(path.join(temporary, "motion.wasm"));
    if (!WebAssembly.validate(wasm)) throw Error("Invalid CS16 WASM output");
    writeFileSync(path.join(temporary, "wasm-exec.js"), identity.script);
    writeFileSync(path.join(temporary, "go-runtime-LICENSE.txt"), identity.license);
    const { script, license, ...binding } = identity;
    const runtime = { ...binding, wasm: descriptor("motion.wasm", wasm, "application/wasm"), script: descriptor("wasm-exec.js", Buffer.from(script), "text/javascript"), license: descriptor("go-runtime-LICENSE.txt", Buffer.from(license), "text/plain") };
    writeFileSync(path.join(temporary, "build.json"), JSON.stringify(runtime, null, 2) + "\n");
    verifyRuntime(temporary, runtime, identity);
    renameSync(temporary, target);
    return { root: target, runtime, cached: false };
  } finally {
    if (existsSync(temporary)) rmSync(temporary, { recursive: true, force: true });
  }
}

export function verifyCS16Delivery(workspace, publicRoot) {
  const root = path.resolve(workspace);
  const selected = path.join(publicRoot, "cs16");
  const manifest = JSON.parse(readFileSync(path.join(selected, "manifest.json"), "utf8"));
  validateAssets(manifest);
  for (const item of resourceDescriptors(manifest)) validateResource(selected, item);
  verifyRuntime(selected, manifest.runtime, runtimeIdentity(root, environment(root)));
  return { passed: true, resources: resourceDescriptors(manifest).length, sourceSHA256: manifest.runtime.sourceSHA256, wasmSHA256: manifest.runtime.wasm.sha256 };
}

export function prepareCS16Delivery(workspace) {
  const root = path.resolve(workspace);
  const source = [path.join(root, ".local/cs16/assets"), path.join(root, "games/cs16/assets/public/cs16")].find(candidate => existsSync(path.join(candidate, "manifest.json")));
  if (!source) throw Error("Missing CS16 assets; restore or generate the classic resources first");
  const manifestBytes = readFileSync(path.join(source, "manifest.json"));
  const manifest = JSON.parse(manifestBytes);
  delete manifest.runtime;
  validateAssets(manifest);
  for (const item of resourceDescriptors(manifest)) validateResource(source, item);
  const compiled = buildCS16Runtime(root);
  const publicRoot = directory(root, "delivery/public");
  const target = path.join(publicRoot, "cs16");
  const temporary = mkdtempSync(path.join(directory(root, "tmp"), "delivery-"));
  const staged = path.join(temporary, "cs16");
  mkdirSync(staged, { recursive: true });
  try {
    for (const item of resourceDescriptors(manifest)) {
      const file = path.join(staged, item.file);
      mkdirSync(path.dirname(file), { recursive: true });
      copyFileSync(path.join(source, item.file), file);
    }
    for (const item of [compiled.runtime.wasm, compiled.runtime.script, compiled.runtime.license]) copyFileSync(path.join(compiled.root, item.file), path.join(staged, item.file));
    writeFileSync(path.join(staged, "manifest.json"), JSON.stringify({ ...manifest, runtime: compiled.runtime }) + "\n");
    if (digest(readFileSync(path.join(source, "manifest.json"))) !== digest(manifestBytes)) throw Error("CS16 assets changed during delivery");
    verifyCS16Delivery(root, temporary);
    const previous = path.join(temporary, "previous");
    if (existsSync(target)) renameSync(target, previous);
    try {
      renameSync(staged, target);
    } catch (error) {
      if (existsSync(previous)) renameSync(previous, target);
      throw error;
    }
    return publicRoot;
  } finally {
    rmSync(temporary, { recursive: true, force: true });
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === builderFile) {
  const root = fileURLToPath(new URL("../../../", import.meta.url));
  const publicRoot = prepareCS16Delivery(root);
  process.stdout.write(JSON.stringify({ publicRoot, ...verifyCS16Delivery(root, publicRoot) }) + "\n");
}
