import { promisify } from "node:util";
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const out = path.join(root, ".local/bubbleparty/mobile-controls");
const temporary = path.join(out, "temporary");
const sockets = path.join(root, ".local/bubbleparty/s");
for (const folder of [out, temporary, sockets]) await mkdir(folder, { recursive: true });
const env = { ...process.env, TMPDIR: temporary, AGENT_BROWSER_PROFILE: path.join(temporary, "profile"), AGENT_BROWSER_SOCKET_DIR: sockets, AGENT_BROWSER_NO_VERSION_CHECK: "1" };
const execute = promisify(execFile);
const browser = async (...args) => {
  const { stdout: raw } = await execute("agent-browser", ["--session", "bc", ...args, "--json"], { cwd: root, env, encoding: "utf8", timeout: 30000 });
  const value = JSON.parse(raw);
  assert.equal(value.success, true, raw);
  return value.data;
};
const css = await readFile(path.join(root, "games/bubbleparty/web/style.css"), "utf8");
const server = createServer((request, response) => {
  response.setHeader("Content-Type", "text/html; charset=utf-8");
  response.end(`<meta name="viewport" content="width=device-width,initial-scale=1"><style>*{box-sizing:border-box}body{margin:0}${css}</style><div data-room-fullscreen><main class="bubble-board"><div class="bubble-status">泡泡堂</div><div class="bubble-teams">红队 · 蓝队</div><div class="bubble-stage" style="height:160px;background:#719875">战场</div><div class="bubble-controls" aria-label="操作"><div class="bubble-dpad bubble-joystick" data-bubbleparty-dpad><div class="bubble-stick-base"><span class="bubble-stick-thumb"></span></div></div><button class="bubble-bomb" data-bubbleparty-bomb>放水泡</button></div></main></div>`);
});
await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
const result = { passed: false, checks: [] };
try {
  await browser("open", `http://127.0.0.1:${server.address().port}`);
  await browser("set", "device", "iPhone 14");
  for (const [width, height] of [[320, 640], [390, 844], [844, 390], [1024, 600]]) {
    await browser("set", "viewport", String(width), String(height));
    const geometry = (await browser("eval", `JSON.stringify((()=>{const rect=s=>document.querySelector(s).getBoundingClientRect().toJSON();return {width:innerWidth,height:innerHeight,pad:rect('[data-bubbleparty-dpad]'),bomb:rect('[data-bubbleparty-bomb]')}})())`)).result;
    const value = typeof geometry === "string" ? JSON.parse(geometry) : geometry;
    result.checks.push(value);
    assert.ok(value.pad.x + value.pad.width / 2 < width / 2, `movement is not on the left: ${JSON.stringify(value)}`);
    assert.ok(value.bomb.x + value.bomb.width / 2 > width / 2, `bomb is not on the right: ${JSON.stringify(value)}`);
    assert.ok(value.pad.right <= value.bomb.x, `touch surfaces overlap: ${JSON.stringify(value)}`);
    assert.ok(value.pad.x >= 0 && value.bomb.right <= width, `controls leave viewport: ${JSON.stringify(value)}`);
    await browser("screenshot", path.join(out, `${width}.png`));
  }
  result.passed = true;
} finally {
  await writeFile(path.join(out, "result.json"), `${JSON.stringify(result, null, 2)}\n`);
  try { await browser("close"); } finally {
    await new Promise(resolve => server.close(resolve));
    await rm(temporary, { recursive: true, force: true });
  }
}
