import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, renameSync, rmSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const workspace = realpathSync(fileURLToPath(new URL("../../../", import.meta.url)));
const packagePath = "./games/pokepath/cmd/browser";
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const { transformSync } = require("esbuild");
const sha = bytes => createHash("sha256").update(bytes).digest("hex");
const flags = ["-trimpath", "-buildvcs=false", "-ldflags=-s -w"];
const abi = "wire-board-pokepath-go-v1";

function environment() {
  const result = { ...process.env, GOOS: "js", GOARCH: "wasm", CGO_ENABLED: "0", GOWORK: "off", GOTOOLCHAIN: "local", GOENV: "off", GOFLAGS: "", GOEXPERIMENT: "" };
  for (const [name, directory] of [["GOCACHE", ".local/go-build"], ["GOMODCACHE", ".local/go-mod"], ["GOTMPDIR", ".local/tmp"], ["TMPDIR", ".local/tmp"], ["TEST_TELEMETRY_DIR", ".local/go-telemetry"]]) {
    result[name] = join(workspace, directory);
    mkdirSync(result[name], { recursive: true });
  }
  return result;
}

function execute(args, env) {
  return execFileSync("go", args, { cwd: workspace, env, encoding: "utf8", maxBuffer: 32 << 20, stdio: ["ignore", "pipe", "pipe"] });
}

const unicodeDependency = {
  module: "golang.org/x/text",
  version: "v0.34.0",
  sum: "h1:oL/Qq0Kdaqxa1KbNeMKwQq0reLCCaFtqu2eNuSeNHbk=",
  goModSum: "h1:homfLqTYRFyVYemLBFl5GgL/DWEiH5wcsQ5gSh1yziA=",
  license: "BSD-3-Clause",
  licenseSHA256: "911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad",
  sourcePrefix: "go-module/golang.org/x/text@v0.34.0/"
};
const unicodePackages = new Set(["golang.org/x/text/transform", "golang.org/x/text/unicode/norm"]);

function workspacePath(path) {
  const real = realpathSync(path);
  const local = relative(workspace, real);
  if (local === ".." || local.startsWith(`..${sep}`) || local.startsWith(sep)) throw Error("Browser source dependency escapes the workspace");
  return real;
}

export function verifyUnicodeDependency(module, cacheRoot) {
  const expectedRoot = join(workspace, ".local/go-mod");
  if (realpathSync(cacheRoot) !== realpathSync(expectedRoot)) throw Error("Unicode dependency must use the repository module cache");
  if (!module || module.Path !== unicodeDependency.module || module.Version !== unicodeDependency.version || module.Sum !== unicodeDependency.sum || module.GoModSum !== unicodeDependency.goModSum || module.Replace) throw Error("Unapproved browser Unicode module identity or checksum");
  const directory = workspacePath(join(expectedRoot, `${unicodeDependency.module}@${unicodeDependency.version}`));
  if (workspacePath(module.Dir) !== directory) throw Error("Unicode dependency uses an unexpected module directory");
  const goMod = workspacePath(join(expectedRoot, "cache/download/golang.org/x/text/@v/v0.34.0.mod"));
  if (workspacePath(module.GoMod) !== goMod) throw Error("Unicode dependency uses an unexpected module manifest");
  const sums = new Set(readFileSync(join(workspace, "go.sum"), "utf8").split(/\r?\n/));
  if (!sums.has(`${unicodeDependency.module} ${unicodeDependency.version} ${unicodeDependency.sum}`) || !sums.has(`${unicodeDependency.module} ${unicodeDependency.version}/go.mod ${unicodeDependency.goModSum}`)) throw Error("The pinned Unicode dependency is not bound by go.sum");
  if (sha(readFileSync(join(directory, "LICENSE"))) !== unicodeDependency.licenseSHA256) throw Error("Unicode dependency BSD license checksum mismatch");
  return { ...unicodeDependency };
}

function sourceFiles(env) {
  const names = new Map([["go.mod", { disk: join(workspace, "go.mod") }], ["go.sum", { disk: join(workspace, "go.sum") }]]);
  const dependencies = new Map();
  const format = "{{if not .Standard}}{{.ImportPath}}|{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}|{{if .Module.Replace}}replaced{{end}}{{else}}|||{{end}}|{{.Dir}}{{range .GoFiles}}|{{.}}{{end}}{{range .SFiles}}|{{.}}{{end}}{{range .EmbedFiles}}|{{.}}{{end}}{{end}}";
  for (const line of execute(["list", "-deps", "-f", format, packagePath], env).split("\n").filter(Boolean)) {
    const [importPath, modulePath, version, moduleRoot, replaced, directory, ...files] = line.split("|");
    let dependency;
    if (modulePath === unicodeDependency.module) {
      if (!unicodePackages.has(importPath) || replaced || version !== unicodeDependency.version) throw Error(`Unapproved browser Unicode package: ${importPath}@${version}`);
      dependency = dependencies.get(modulePath);
      if (!dependency) {
        const metadata = JSON.parse(execute(["list", "-m", "-json", modulePath], env));
        dependency = { ...verifyUnicodeDependency(metadata, env.GOMODCACHE), packages: [], directory: workspacePath(metadata.Dir) };
        dependencies.set(modulePath, dependency);
        names.set(dependency.sourcePrefix + "go.mod", { disk: workspacePath(metadata.GoMod), dependency: modulePath });
        names.set(dependency.sourcePrefix + "LICENSE", { disk: join(dependency.directory, "LICENSE"), dependency: modulePath });
      }
      if (workspacePath(moduleRoot) !== dependency.directory) throw Error("Unicode package metadata has a different module root");
      if (!dependency.packages.includes(importPath)) dependency.packages.push(importPath);
    } else if (modulePath !== "github.com/k0ngk0ng/wire-board" || replaced || workspacePath(moduleRoot) !== workspace) throw Error(`Unapproved browser rules module: ${modulePath}@${version}`);
    for (const name of files) {
      const path = workspacePath(join(directory, name));
      if (!path.endsWith(".go") && !path.endsWith(".s")) throw Error(`Browser rules dependency is not native source: ${path}`);
      if (dependency) {
        const local = relative(dependency.directory, path).split(sep).join("/");
        if (local === ".." || local.startsWith("../") || local.startsWith("/")) throw Error("Unicode package source escapes its pinned module");
        names.set(dependency.sourcePrefix + local, { disk: path, dependency: modulePath });
      } else {
        const local = relative(workspace, path).split(sep).join("/");
        if (!local.startsWith("games/pokepath/engine/") && !local.startsWith("games/pokepath/cmd/browser/")) throw Error(`Browser rules dependency is outside our Go engine: ${local}`);
        names.set(local, { disk: path });
      }
    }
  }
  const files = [...names].sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0).map(([path, entry]) => {
    const bytes = readFileSync(entry.disk);
    return { path, bytes: bytes.length, sha256: sha(bytes), ...(entry.dependency ? { dependency: entry.dependency } : {}) };
  });
  const modules = [...dependencies.values()].map(({ directory, ...dependency }) => ({ ...dependency, packages: dependency.packages.sort() }));
  return { files, dependencies: modules };
}

function valid(directory, sourceSHA256, buildSHA256, dependencies) {
  try {
    const manifest = JSON.parse(readFileSync(join(directory, "manifest.json"), "utf8"));
    if (manifest.format !== 1 || manifest.abi !== abi || manifest.sourceSHA256 !== sourceSHA256 || manifest.buildSHA256 !== buildSHA256 || JSON.stringify(manifest.dependencies) !== JSON.stringify(dependencies)) return undefined;
    for (const file of manifest.files) {
      const bytes = readFileSync(join(directory, file.path));
      if (bytes.length !== file.bytes || sha(bytes) !== file.sha256) return undefined;
      if (file.path.endsWith(".wasm") && !WebAssembly.validate(bytes)) return undefined;
    }
    return manifest;
  } catch {
    return undefined;
  }
}

export function buildBrowserRuntime(options = {}) {
  const env = environment();
  const toolchain = JSON.parse(execute(["env", "-json", "GOROOT", "GOVERSION"], env));
  const standardRuntime = readFileSync(join(toolchain.GOROOT, "lib/wasm/wasm_exec.js"), "utf8");
  const runtimeCode = transformSync(standardRuntime, { loader: "js", target: "es2022", legalComments: "none", minifyWhitespace: true }).code;
  const copyright = standardRuntime.split("\n").slice(0, 4).filter(line => line.startsWith("//")).map(line => line.replace(/^\/\/\s?/, "")).join("\n");
  const licensePath = [join(toolchain.GOROOT, "LICENSE"), join(dirname(toolchain.GOROOT), "LICENSE")].find(path => existsSync(path) && /Redistribution and use in source and binary forms/.test(readFileSync(path, "utf8")));
  if (!licensePath) throw Error("The selected Go toolchain does not provide its BSD license");
  const license = `${copyright}\n\n${readFileSync(licensePath, "utf8")}`;
  const goSupport = { sourcePath: "lib/wasm/wasm_exec.js", sourceSHA256: sha(standardRuntime), transformation: "esbuild-remove-comments-minify-whitespace", licenseSHA256: sha(readFileSync(licensePath)) };
  const ownedRoot = join(workspace, ".local/pokepath");
  const parent = resolve(options.cacheRoot ?? join(ownedRoot, "browser-runtime"));
  const local = relative(ownedRoot, parent);
  if (local === ".." || local.startsWith(`..${sep}`) || local.startsWith(sep)) throw Error("Browser runtime cache must remain inside this game's workspace cache");
  let existing = parent;
  while (!existsSync(existing) && dirname(existing) !== existing) existing = dirname(existing);
  const real = relative(workspace, realpathSync(existing));
  if (real === ".." || real.startsWith(`..${sep}`) || real.startsWith(sep)) throw Error("Browser runtime cache escapes the workspace through a symlink");
  mkdirSync(parent, { recursive: true });
  for (let attempt = 0; attempt < 3; attempt++) {
    const source = sourceFiles(env);
    const inputs = source.files;
    const dependencies = source.dependencies;
    const sourceSHA256 = sha(JSON.stringify(inputs));
    const licenseText = license + dependencies.map(dependency => `\n\nDependency: ${dependency.module} ${dependency.version}\nLicense: ${dependency.license}\nModule checksum: ${dependency.sum}\n\n${readFileSync(join(env.GOMODCACHE, `${dependency.module}@${dependency.version}`, "LICENSE"), "utf8")}`).join("");
    const catalogBytes = readFileSync(join(workspace, "games/pokepath/assets/public/pokepath/v1/resources.json"));
    const catalog = { manifest: { path: "../resources.json", bytes: catalogBytes.length, sha256: sha(catalogBytes), mime: "application/json" }, core: "catalog.json" };
    const buildSHA256 = sha(JSON.stringify({ sourceSHA256, dependencies, catalog, goSupport, goVersion: toolchain.GOVERSION, flags, runtimeSHA256: sha(runtimeCode), builderSHA256: sha(readFileSync(fileURLToPath(import.meta.url))) }));
    const target = join(parent, buildSHA256);
    const publicRoot = join(target, "public");
    const runtimeRoot = join(publicRoot, "pokepath/v1/native");
    const cached = valid(runtimeRoot, sourceSHA256, buildSHA256, dependencies);
    if (cached) return { publicRoot, runtimeRoot, manifest: cached, cached: true };
    const temporary = mkdtempSync(join(parent, "compile-"));
    try {
      const destination = join(temporary, "public/pokepath/v1/native");
      mkdirSync(destination, { recursive: true });
      execute(["build", ...flags, "-o", join(destination, "engine.wasm"), packagePath], env);
      const after = sourceFiles(env);
      if (JSON.stringify(source) !== JSON.stringify(after) || sha(readFileSync(join(workspace, "games/pokepath/assets/public/pokepath/v1/resources.json"))) !== catalog.manifest.sha256) continue;
      const wasm = readFileSync(join(destination, "engine.wasm"));
      if (!WebAssembly.validate(wasm)) throw Error("Go compiler did not produce valid browser Wasm");
      writeFileSync(join(destination, "wasm_exec.js"), runtimeCode);
      writeFileSync(join(destination, "go-runtime-LICENSE.txt"), licenseText);
      const files = ["engine.wasm", "wasm_exec.js", "go-runtime-LICENSE.txt"].map(path => {
        const bytes = readFileSync(join(destination, path));
        return { path, bytes: bytes.length, sha256: sha(bytes), mime: path.endsWith(".wasm") ? "application/wasm" : path.endsWith(".js") ? "text/javascript" : "text/plain" };
      });
      const manifest = { format: 1, engine: "wire-board-pokepath-go", abi, goVersion: toolchain.GOVERSION, sourceSHA256, buildSHA256, flags, sourceFiles: inputs, dependencies, catalog, goSupport, files };
      writeFileSync(join(destination, "manifest.json"), JSON.stringify(manifest, null, 2) + "\n");
      const concurrent = valid(runtimeRoot, sourceSHA256, buildSHA256, dependencies);
      if (concurrent) return { publicRoot, runtimeRoot, manifest: concurrent, cached: true };
      if (existsSync(target)) throw Error("An existing runtime cache failed validation; its files were preserved");
      try { renameSync(temporary, target); }
      catch (error) {
        const published = valid(runtimeRoot, sourceSHA256, buildSHA256, dependencies);
        if (published) return { publicRoot, runtimeRoot, manifest: published, cached: true };
        throw error;
      }
      return { publicRoot, runtimeRoot, manifest, cached: false };
    } finally {
      if (existsSync(temporary)) rmSync(temporary, { recursive: true, force: true });
    }
  }
  throw Error("Native rules changed during browser compilation; retry after source writes finish");
}

export function preparePokepathNative() {
  return buildBrowserRuntime().publicRoot;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const args = process.argv.slice(2);
  if (args.length && (args.length !== 2 || args[0] !== "--cache-root")) throw Error("Expected optional --cache-root <workspace cache directory>");
  const artifact = buildBrowserRuntime(args.length ? { cacheRoot: args[1] } : {});
  process.stdout.write(JSON.stringify({ publicRoot: artifact.publicRoot, runtimeRoot: artifact.runtimeRoot, sourceSHA256: artifact.manifest.sourceSHA256, buildSHA256: artifact.manifest.buildSHA256, cached: artifact.cached, files: artifact.manifest.files }) + "\n");
}
