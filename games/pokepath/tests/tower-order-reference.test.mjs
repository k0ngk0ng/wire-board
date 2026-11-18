import test from "node:test";
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { createServer } from "node:http";
import { createHash, randomUUID } from "node:crypto";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { join, extname } from "node:path";
import { fileURLToPath } from "node:url";
import { isDeepStrictEqual, promisify } from "node:util";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const execute = promisify(execFile);
const hash = bytes => createHash("sha256").update(bytes).digest("hex");

test("actual original tower updates and independent battle steps preserve cross-owner order", { timeout: 300000 }, async () => {
  const identity = randomUUID().slice(0, 8);
  const output = join(root, ".local/pokepath/verification/tower-order", `run-${identity}`);
  const profile = join(output, "profile");
  await mkdir(output, { recursive: true });
  const research = join(root, ".local/pokepath/research");
  const source = join(research, "source");
  const media = join(root, "games/pokepath/assets/public/pokepath/v1");
  const css = join(root, ".local/pokepath/rejected-embedding/assets/src/css");
  const environment = { ...process.env, WIRE_BOARD_POKEPATH_TOWER_ORDER_OUT: join(output, "native.json"), TMPDIR: join(root, ".local/tmp"), AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), AGENT_BROWSER_PROFILE: profile, AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter", AGENT_BROWSER_NO_VERSION_CHECK: "1" };
  const fixture = await readFile(new URL("./tower-order-reference-fixture.mjs", import.meta.url));
  const html = (await readFile(join(research, "game.html"), "utf8")).replace(/<script\b[^>]*src=["'][^"']*["'][^>]*><\/script>/g, "").replace("</body>", '<script type="module" src="/fixture.mjs"></script></body>');
  const report = { command: "node --test games/pokepath/tests/tower-order-reference.test.mjs", scope: "Original v1.7.3 executes only in the local research browser; controlled legal deployments with fixed stats, positions and enemy health; includes impact-stage rounding, exact half-health boundaries, guaranteed criticals, extra half-power Maw shots and field/Technician ordering compared with the real independent Go Advance chain", passed: false, fixtureSHA256: hash(fixture), sources: {}, cases: [] };
  for (const path of ["game/Game.js", "game/component/Tower.js", "game/component/Enemy.js", "game/component/Projectile.js"]) report.sources[path] = hash(await readFile(join(source, "src/js", path)));
  const server = createServer(async (request, response) => {
    const path = new URL(request.url, "http://127.0.0.1").pathname;
    if (path === "/") { response.writeHead(200, { "Content-Type": "text/html; charset=utf-8" }); response.end(html); return; }
    if (path === "/fixture.mjs") { response.writeHead(200, { "Content-Type": "text/javascript" }); response.end(fixture); return; }
    if (!/^\/(?:styles\.css|src\/(?:assets|js|css)\/[a-zA-Z0-9_/.-]+)$/.test(path) || path.includes("..")) { response.writeHead(404); response.end(); return; }
    const file = path === "/styles.css" ? join(research, "styles.css") : path.startsWith("/src/js/") ? join(source, path) : path.startsWith("/src/css/") ? join(css, path.slice(9)) : join(media, path);
    try { const bytes = await readFile(file); response.writeHead(200, { "Content-Type": ({ ".js": "text/javascript", ".css": "text/css", ".png": "image/png", ".ttf": "font/ttf" })[extname(file)] ?? "application/octet-stream" }); response.end(bytes); }
    catch { response.writeHead(404); response.end(); }
  });
  let session;
  const browser = async (...args) => { const result = JSON.parse((await execute("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env: environment, timeout: 45000, maxBuffer: 8 << 20 })).stdout); assert.equal(result.success, true, JSON.stringify(result)); return result.data; };
  try {
    await execute("go", ["test", "./games/pokepath/engine", "-run", "^TestCoreTowerOrderingReferenceFixtureE2E$", "-count=1"], { cwd: root, env: environment, timeout: 45000 });
    const native = JSON.parse(await readFile(join(output, "native.json")));
    report.rulesVersion = native.rulesVersion;
    await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
    session = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", `tower-ref-${identity}`], { cwd: root, env: environment })).stdout.trim();
    await browser("open", "about:blank");
    await browser("set", "viewport", "1238", "674");
    for (const actual of native.cases) {
      await browser("open", `http://127.0.0.1:${server.address().port}/?${new URLSearchParams({ input: JSON.stringify(actual.input) })}`);
      await browser("wait", "--fn", "Boolean(window.towerReference)");
      const original = (await browser("eval", "window.towerReference")).result;
      const path = join(output, `${actual.input.name}.png`);
      await browser("screenshot", path);
      const positionsMatch = actual.positions.length === original.updatePositions.length && actual.positions.every((point, index) => point.tower === original.updatePositions[index].tower && Math.abs(point.x - original.updatePositions[index].x) <= 1e-6 && Math.abs(point.y - original.updatePositions[index].y) <= 1e-6);
      report.cases.push({ ...actual, original, eventsMatch: isDeepStrictEqual(actual.events, original.events), healthMatches: isDeepStrictEqual(actual.health, original.health), positionsMatch, screenshot: { path, sha256: hash(await readFile(path)) } });
    }
    report.mismatches = report.cases.filter(value => !value.eventsMatch || !value.healthMatches || !value.positionsMatch).map(value => value.input.name);
    assert.deepEqual(report.mismatches, []);
    report.passed = true;
  } finally {
    if (session) { try { await browser("close"); } catch {} }
    await new Promise(resolve => server.close(resolve));
    await rm(profile, { recursive: true, force: true });
    await writeFile(join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
    await writeFile(join(root, ".local/pokepath/verification/tower-order/latest.json"), JSON.stringify({ output, passed: report.passed }) + "\n");
  }
});
