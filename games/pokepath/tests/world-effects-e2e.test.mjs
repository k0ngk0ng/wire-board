import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { mkdir, readFile, writeFile, access, rm } from "node:fs/promises";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { fileURLToPath, pathToFileURL } from "node:url";
import { join } from "node:path";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const execute = promisify(execFile);
const output = join(root, ".local/pokepath/verification/world-effects");
const temporary = join(output, "fixture");
const profile = join(root, ".local/pokepath/effects-browser-profile");
const environment = { ...process.env, TMPDIR: join(root, ".local/tmp"), AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), AGENT_BROWSER_PROFILE: profile, AGENT_BROWSER_NO_VERSION_CHECK: "1", AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter" };
const sha = bytes => createHash("sha256").update(bytes).digest("hex");

test("independent persistent fields match the fixed original renderer in real Canvas pixels", { timeout: 120000 }, async () => {
  await mkdir(temporary, { recursive: true });
  const report = { command: "node --test games/pokepath/tests/world-effects-e2e.test.mjs", passed: false, scope: "Isolated rendering comparison in a muted owned browser; not a full-game or authenticated performance claim", cases: [] };
  let server;
  let session;
  const browser = async (...args) => {
    const result = await execute("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env: environment, timeout: 45000, maxBuffer: 16 << 20 });
    const value = JSON.parse(result.stdout);
    assert.equal(value.success, true, JSON.stringify(value));
    return value.data;
  };
  try {
    await access(join(root, "games/pokepath/web/world-effects.ts"));
    const source = await readFile(join(root, ".local/pokepath/research/source/src/js/game/component/Tower.js"), "utf8");
    const ts = require("typescript");
    const parsed = ts.createSourceFile("Tower.js", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
    const declaration = parsed.statements.find(node => ts.isClassDeclaration(node) && node.name?.text === "SpikeZone");
    const texture = parsed.statements.find(node => ts.isVariableStatement(node) && node.declarationList.declarations.some(value => value.name?.text === "smokeTexture"));
    const tower = parsed.statements.find(node => ts.isClassDeclaration(node) && node.name?.text === "Tower");
    const shadow = tower?.members.find(node => ts.isMethodDeclaration(node) && node.name?.getText(parsed) === "drawShadowTagZones");
    assert.ok(declaration);
    assert.ok(texture);
    assert.ok(shadow);
    const projectileSource = await readFile(join(root, ".local/pokepath/research/source/src/js/game/component/Projectile.js"), "utf8");
    const projectile = ts.createSourceFile("Projectile.js", projectileSource, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
    const phaseBodies = {};
    const visit = node => {
      if (ts.isIfStatement(node)) for (const phase of ["waiting", "pulling"]) {
        if (node.expression.getText(projectile) === `s.phase === '${phase}'`) phaseBodies[phase] = node.thenStatement.getText(projectile);
      }
      ts.forEachChild(node, visit);
    };
    visit(projectile);
    for (const phase of ["waiting", "pulling"]) assert.ok(phaseBodies[phase]);
    const singularity = `export class SingularityRenderer{waiting(s){const simDelta=0,secs=0,shouldDraw=true;${phaseBodies.waiting}}pulling(s){const simDelta=0,secs=0,shouldDraw=true;${phaseBodies.pulling}}}`;
    const reference = ts.transpileModule(declaration.getText(parsed) + "\n" + texture.getText(parsed) + "\nexport class ShadowRenderer{" + shadow.getText(parsed) + "}\n" + singularity, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022, removeComments: true } }).outputText;
    await writeFile(join(temporary, "reference.mjs"), reference);
    await require("esbuild").build({ entryPoints: [join(root, "games/pokepath/web/world-effects.ts")], outfile: join(temporary, "native.mjs"), bundle: true, format: "esm", target: "es2022", legalComments: "none" });
    const nativeModule = await import(pathToFileURL(join(temporary, "native.mjs")).href);
    assert.equal(typeof nativeModule.paintSingularityOverlay, "function");
    await writeFile(join(temporary, "index.html"), '<!doctype html><html><body><canvas id="reference" width="256" height="256"></canvas><canvas id="native" width="256" height="256"></canvas><script type="module" src="index.mjs"></script></body></html>');
    await writeFile(join(temporary, "index.mjs"), await readFile(new URL("./world-effects-fixture.mjs", import.meta.url)));
    const files = new Map(await Promise.all(["index.html", "index.mjs", "native.mjs", "reference.mjs"].map(async name => [name, await readFile(join(temporary, name))])));
    server = createServer((request, response) => {
      const name = new URL(request.url, "http://127.0.0.1").pathname.slice(1) || "index.html";
      if (!files.has(name)) { response.writeHead(404); response.end(); return; }
      response.writeHead(200, { "Content-Type": name.endsWith(".html") ? "text/html" : "text/javascript", "Content-Security-Policy": "default-src 'self'; script-src 'self'" });
      response.end(files.get(name));
    });
    await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
    session = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "ppfx"], { cwd: root, env: environment, encoding: "utf8" })).stdout.trim();
    await browser("open", `http://127.0.0.1:${server.address().port}`);
    await browser("wait", "--fn", "document.documentElement.dataset.ready==='true'");
    const result = (await browser("eval", "window.results")).result;
    for (const value of result) {
      const images = {};
      for (const key of ["reference", "native"]) {
        const bytes = Buffer.from(value[key], "base64");
        const name = `${value.kind}-${value.index}-${key}.png`;
        await writeFile(join(output, name), bytes);
        images[key] = { path: name, bytes: bytes.length, sha256: sha(bytes) };
      }
      const { reference: ignoredReference, native: ignoredNative, ...measurement } = value;
      report.cases.push({ ...measurement, images });
      assert.equal(value.handled, true);
      assert.equal(value.unchanged, true);
      assert.equal(value.differences, 0, `${value.kind} at ${value.ageMS}: ${value.differences} RGBA channels differ`);
      if (value.expectedVisible) assert.ok(value.nonzero > 0);
    }
    report.reference = { files: [{ path: "src/js/game/component/Tower.js", sha256: sha(source) }, { path: "src/js/game/component/Projectile.js", sha256: sha(projectileSource) }], version: "1.7.3" };
    report.passed = true;
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
});
