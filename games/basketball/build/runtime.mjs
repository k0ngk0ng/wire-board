import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  realpathSync,
  renameSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

const packagePath = "./games/basketball/cmd/browser-wasm";
const flags = ["-trimpath", "-buildvcs=false", "-ldflags=-s -w"];
const digest = (value) => createHash("sha256").update(value).digest("hex");

/** Compile only this game's independent Go runtime. The returned directory
 * contains explicitly public executable files, never ROMs or research data.
 * Source/toolchain-addressed output and checksum verification prevent a failed
 * compile or a stale local cache from entering a new application artifact.
 */
export function prepareBasketballRuntime(directory) {
  const root = realpathSync(directory);
  const env = {
    ...process.env,
    GOOS: "js",
    GOARCH: "wasm",
    CGO_ENABLED: "0",
    GOWORK: "off",
    GOTOOLCHAIN: "local",
    GOENV: "off",
    GOFLAGS: "",
    GOEXPERIMENT: "",
  };
  env.GOCACHE ||= path.join(root, ".local/go-build");
  env.GOMODCACHE ||= path.join(root, ".local/go-mod");
  env.GOTMPDIR ||= path.join(root, ".local/tmp");
  env.TMPDIR ||= env.GOTMPDIR;
  for (const name of ["GOCACHE", "GOMODCACHE", "GOTMPDIR", "TMPDIR"])
    if (env[name] !== "off") mkdirSync(env[name], { recursive: true });
  const go = (args) =>
    execFileSync("go", args, {
      cwd: root,
      env,
      encoding: "utf8",
      maxBuffer: 16 << 20,
      stdio: ["ignore", "pipe", "pipe"],
    });
  const toolchain = JSON.parse(go(["env", "-json", "GOROOT", "GOVERSION"]));
  const loader = readFileSync(
    path.join(toolchain.GOROOT, "lib/wasm/wasm_exec.js"),
  );
  const files = new Set(["go.mod", "go.sum"]);
  const format =
    "{{if not .Standard}}{{.Dir}}{{range .GoFiles}}|{{.}}{{end}}{{range .SFiles}}|{{.}}{{end}}{{range .EmbedFiles}}|{{.}}{{end}}{{end}}";
  for (const line of go(["list", "-deps", "-f", format, packagePath])
    .split("\n")
    .filter(Boolean)) {
    const [dir, ...names] = line.split("|");
    for (const name of names) {
      const source = realpathSync(path.join(dir, name)),
        relative = path.relative(root, source);
      if (
        relative.startsWith(".." + path.sep) ||
        relative === ".." ||
        path.isAbsolute(relative)
      )
        throw Error(
          "Basketball Wasm dependencies must be inside the source checkout",
        );
      files.add(relative.split(path.sep).join("/"));
    }
  }
  const inputs = [...files]
    .sort()
    .map((file) => ({
      path: file,
      sha256: digest(readFileSync(path.join(root, file))),
    }));
  const sourceSHA256 = digest(JSON.stringify(inputs));
  const source = {
    format: 1,
    package: packagePath,
    flags,
    goVersion: toolchain.GOVERSION,
    sourceSHA256,
    loaderSHA256: digest(loader),
    builderSHA256: digest(readFileSync(fileURLToPath(import.meta.url))),
  };
  const key = digest(JSON.stringify(source));
  const parent = path.join(root, ".local/basketball/runtime-build");
  mkdirSync(parent, { recursive: true });
  const cache = path.join(parent, key);
  const publicRoot = path.join(cache, "public");
  const runtimeRoot = path.join(publicRoot, "basketball/v1/runtime");
  function valid() {
    try {
      const manifest = JSON.parse(
        readFileSync(path.join(runtimeRoot, "build.json"), "utf8"),
      );
      const {
        inputs: recordedInputs,
        files: recordedFiles,
        ...recordedSource
      } = manifest;
      if (
        JSON.stringify(recordedSource) !== JSON.stringify(source) ||
        digest(JSON.stringify(recordedInputs)) !== sourceSHA256
      )
        return false;
      for (const file of ["basketball.wasm", "wasm_exec.js"]) {
        const data = readFileSync(path.join(runtimeRoot, file));
        if (
          manifest.files[file]?.sha256 !== digest(data) ||
          manifest.files[file]?.bytes !== data.length
        )
          return false;
        if (file === "wasm_exec.js" && digest(data) !== source.loaderSHA256)
          return false;
        if (file === "basketball.wasm" && !WebAssembly.validate(data))
          return false;
      }
      return true;
    } catch {
      return false;
    }
  }
  if (valid()) return publicRoot;
  const temporary = mkdtempSync(path.join(parent, "compile-"));
  try {
    const target = path.join(temporary, "public/basketball/v1/runtime");
    mkdirSync(target, { recursive: true });
    go([
      "build",
      ...flags,
      "-o",
      path.join(target, "basketball.wasm"),
      packagePath,
    ]);
    const wasm = readFileSync(path.join(target, "basketball.wasm"));
    if (!WebAssembly.validate(wasm))
      throw Error("Go build did not produce a valid WebAssembly module");
    writeFileSync(path.join(target, "wasm_exec.js"), loader);
    const manifest = {
      ...source,
      inputs,
      files: {
        "basketball.wasm": { sha256: digest(wasm), bytes: wasm.length },
        "wasm_exec.js": { sha256: digest(loader), bytes: loader.length },
      },
    };
    writeFileSync(
      path.join(target, "build.json"),
      JSON.stringify(manifest, null, 2) + "\n",
    );
    if (valid()) return publicRoot; // A concurrent identical build finished first.
    if (existsSync(cache)) rmSync(cache, { recursive: true, force: true });
    try {
      renameSync(temporary, cache);
    } catch (error) {
      if (!valid()) throw error;
    }
    if (!valid())
      throw Error(
        "Compiled basketball runtime failed its artifact verification",
      );
    return publicRoot;
  } finally {
    rmSync(temporary, { recursive: true, force: true });
  }
}
