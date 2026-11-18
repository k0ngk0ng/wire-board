import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { mkdir, readFile, writeFile, rm, access } from "node:fs/promises";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const execute = promisify(execFile);
const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = join(root, ".local/pokepath/verification/enemy-effects");
const temporary = join(output, "fixture");
const profile = join(output, "profile");
const environment = { ...process.env, TMPDIR: join(root, ".local/tmp"), AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), AGENT_BROWSER_PROFILE: profile, AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter", AGENT_BROWSER_NO_VERSION_CHECK: "1" };
const sha = bytes => createHash("sha256").update(bytes).digest("hex");

test("enemy status bars, particles and floating text match fixed original Canvas pixels", { timeout: 120000 }, async () => {
  await mkdir(temporary, { recursive: true });
  const report = { command: "node --test games/pokepath/tests/enemy-effects-e2e.test.mjs", passed: false, scope: "Muted real Canvas comparison of four isolated drawing methods; not original gameplay execution", cases: [] };
  let server;
  let session;
  const browser = async (...args) => { const raw = await execute("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env: environment, timeout: 45000, maxBuffer: 20 << 20 }); const result = JSON.parse(raw.stdout); assert.equal(result.success, true, JSON.stringify(result)); return result.data; };
  try {
    await access(join(root, "games/pokepath/web/enemy-effects.ts"));
    const source = await readFile(join(root, ".local/pokepath/research/source/src/js/game/component/Enemy.js"), "utf8");
    const ts = require("typescript");
    const parsed = ts.createSourceFile("Enemy.js", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
    const enemy = parsed.statements.find(node => ts.isClassDeclaration(node) && node.name?.text === "Enemy");
    const names = ["drawStatusEffects", "drawSlowIndicator", "drawStunIndicator", "drawFloatingTexts"];
    const methods = names.map(name => enemy.members.find(node => ts.isMethodDeclaration(node) && node.name?.getText(parsed) === name));
    assert.ok(methods.every(Boolean));
    const reference = ts.transpileModule(`export class ReferenceEnemyDrawing{${methods.map(method => method.getText(parsed)).join("\n")}}`, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022, removeComments: true } }).outputText;
    report.sourceSHA256 = sha(source);
    report.referenceMethods = names;
    await writeFile(join(temporary, "reference.mjs"), reference);
    await require("esbuild").build({ entryPoints: [join(root, "games/pokepath/web/enemy-effects.ts")], outfile: join(temporary, "native.mjs"), bundle: true, format: "esm", target: "es2022", legalComments: "none" });
    await writeFile(join(temporary, "index.mjs"), await readFile(new URL("enemy-effects-fixture.mjs", import.meta.url)));
    await writeFile(join(temporary, "index.html"), '<!doctype html><html><body><canvas id="reference" width="256" height="256"></canvas><canvas id="native" width="256" height="256"></canvas><script type="module" src="index.mjs"></script></body></html>');
    const files = new Map(await Promise.all(["reference.mjs", "native.mjs", "index.mjs", "index.html"].map(async name => [name, await readFile(join(temporary, name))])));
    server = createServer((request, response) => { const name = new URL(request.url, "http://127.0.0.1").pathname.slice(1) || "index.html"; if (!files.has(name)) { response.writeHead(404); response.end(); return; } response.writeHead(200, { "Content-Type": name.endsWith("html") ? "text/html" : "text/javascript", "Content-Security-Policy": "default-src 'self'; script-src 'self'" }); response.end(files.get(name)); });
    await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
    session = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "ppenemyfx"], { cwd: root, env: environment, encoding: "utf8" })).stdout.trim();
    await browser("open", `http://127.0.0.1:${server.address().port}`);
    await browser("wait", "--fn", "document.documentElement.dataset.ready==='true'");
    const values = (await browser("eval", "window.results")).result;
    for (const value of values) {
      const images = {};
      for (const key of ["reference", "native"]) { const bytes = Buffer.from(value[key], "base64"); const name = `${value.name}-${key}.png`; await writeFile(join(output, name), bytes); images[key] = { path: name, bytes: bytes.length, sha256: sha(bytes) }; }
      report.cases.push({ name: value.name, changedRGBABytes: value.changedRGBABytes, maximumChannelDifference: value.maximumChannelDifference, images });
    }
    assert.ok(report.cases.length >= 40);
    assert.ok(report.cases.every(value => value.changedRGBABytes === 0), JSON.stringify(report.cases.filter(value => value.changedRGBABytes)));
    report.passed = true;
  } catch (cause) { report.error = cause.stack ?? String(cause); throw cause; }
  finally { if (session) { try { await browser("close"); } catch {} } if (server) await new Promise(resolve => server.close(resolve)); await rm(profile, { force: true, recursive: true }); await rm(temporary, { force: true, recursive: true }); await writeFile(join(output, "result.json"), JSON.stringify(report, null, 2) + "\n"); }
});
