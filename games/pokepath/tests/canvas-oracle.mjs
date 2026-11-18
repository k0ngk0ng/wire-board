import assert from "node:assert/strict";
import { createServer } from "node:http";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { createHash, randomUUID } from "node:crypto";
import { createRequire } from "node:module";
import { fileURLToPath, pathToFileURL } from "node:url";
import { join } from "node:path";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const execute = promisify(execFile);
export const canvasSHA = bytes => createHash("sha256").update(bytes).digest("hex");

export async function verifyCanvasOracle({ name, command, reference, sources, nativeEntry, fixture, exports: requiredExports, expectedCases, size = 512 }) {
  assert.match(name, /^[a-z][a-z0-9-]+$/);
  const output = join(root, ".local/pokepath/verification", name);
  const identity = randomUUID().slice(0, 8);
  const temporary = join(output, `fixture-${identity}`);
  const profile = join(output, `profile-${identity}`);
  const environment = { ...process.env, TMPDIR: join(root, ".local/tmp"), AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), AGENT_BROWSER_PROFILE: profile, AGENT_BROWSER_NO_VERSION_CHECK: "1", AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter" };
  const report = { command, passed: false, scope: "Fixed research render-only reference versus independently written product renderer in an owned muted Canvas browser; not a full-game or authenticated performance claim", sources, cases: [] };
  await mkdir(temporary, { recursive: true });
  let session;
  let server;
  const browser = async (...args) => {
    const response = await execute("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env: environment, timeout: 45000, maxBuffer: 32 << 20 });
    const value = JSON.parse(response.stdout);
    assert.equal(value.success, true, JSON.stringify(value));
    return value.data;
  };
  try {
    await writeFile(join(temporary, "reference.mjs"), reference);
    await require("esbuild").build({ entryPoints: [nativeEntry], outfile: join(temporary, "native.mjs"), bundle: true, format: "esm", target: "es2022", legalComments: "none" });
    const native = await import(pathToFileURL(join(temporary, "native.mjs")).href);
    for (const key of requiredExports) assert.equal(typeof native[key], "function", key);
    await writeFile(join(temporary, "index.html"), `<!doctype html><html><body><canvas id="reference" width="${size}" height="${size}"></canvas><canvas id="native" width="${size}" height="${size}"></canvas><script type="module" src="index.mjs"></script></body></html>`);
    await writeFile(join(temporary, "index.mjs"), fixture);
    const files = new Map(await Promise.all(["index.html", "index.mjs", "native.mjs", "reference.mjs"].map(async key => [key, await readFile(join(temporary, key))])));
    server = createServer((request, response) => {
      const key = new URL(request.url, "http://127.0.0.1").pathname.slice(1) || "index.html";
      if (!files.has(key)) { response.writeHead(404); response.end(); return; }
      response.writeHead(200, { "Content-Type": key.endsWith(".html") ? "text/html" : "text/javascript", "Content-Security-Policy": "default-src 'self'; script-src 'self'" });
      response.end(files.get(key));
    });
    await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
    session = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", `fx-${canvasSHA(name).slice(0, 6)}-${identity}`], { cwd: root, env: environment, encoding: "utf8" })).stdout.trim();
    await browser("open", `http://127.0.0.1:${server.address().port}`);
    await browser("wait", "--fn", "document.documentElement.dataset.ready==='true'");
    const values = (await browser("eval", "window.results")).result;
    assert.equal(values.length, expectedCases);
    for (const value of values) {
      assert.match(value.kind, /^[a-zA-Z][a-zA-Z0-9-]+$/);
      assert.ok(Number.isInteger(value.index) && value.index >= 0);
      const images = {};
      for (const key of ["reference", "native"]) {
        const bytes = Buffer.from(value[key], "base64");
        const filename = `${value.kind}-${value.index}-${key}.png`;
        await writeFile(join(output, filename), bytes);
        images[key] = { path: filename, bytes: bytes.length, sha256: canvasSHA(bytes) };
      }
      const { reference: ignoredReference, native: ignoredNative, ...measurement } = value;
      report.cases.push({ ...measurement, images });
      assert.equal(value.handled, true, `${value.kind}: missing independent renderer`);
      assert.equal(value.unchanged, true, `${value.kind}: modified its rule input`);
      assert.equal(value.differences, 0, `${value.kind}#${value.index}: ${value.differences} RGBA channels differ`);
      if (value.expectedVisible) assert.ok(value.nonzero > 0, `${value.kind}: missing reference pixels`);
    }
    report.passed = true;
    return report;
  } catch (error) {
    report.error = error.stack ?? String(error);
    throw error;
  } finally {
    if (session) { try { await browser("close"); } catch {} }
    if (server) await new Promise(resolve => server.close(resolve));
    await rm(profile, { recursive: true, force: true });
    await rm(temporary, { recursive: true, force: true });
    await writeFile(join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
  }
}
