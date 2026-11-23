import test from "node:test";
import assert from "node:assert/strict";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { createHash } from "node:crypto";

test("racing and fortress fullscreen CSS uses the whole viewport and retains controls", async () => {
  const root = fileURLToPath(new URL("../../", import.meta.url));
  const kind = process.env.MOBILE_VIEWPORT_GAME;
  assert.ok(["racing", "fortressblue"].includes(kind));
  const out = path.join(root, ".local", kind, "fullscreen-viewport");
  const temporary = path.join(out, "temporary");
  const env = { ...process.env, TMPDIR: path.join(root, ".local", kind, "t"), AGENT_BROWSER_SOCKET_DIR: path.join(root, ".local", kind, "s"), AGENT_BROWSER_NO_VERSION_CHECK: "1", XDG_CONFIG_HOME: path.join(temporary, "config"), XDG_DATA_HOME: path.join(temporary, "data"), XDG_CACHE_HOME: path.join(temporary, "cache") };
  for (const directory of [out, temporary, env.TMPDIR, env.AGENT_BROWSER_SOCKET_DIR, env.XDG_CONFIG_HOME, env.XDG_DATA_HOME, env.XDG_CACHE_HOME]) await mkdir(directory, { recursive: true });
  const css = ["web/src/ui/room-fullscreen.css", `games/${kind}/web/style.css`];
  const contents = await Promise.all(css.map(file => readFile(path.join(root, file), "utf8")));
  const controls = '<button>左</button><button>右</button><button>油门</button><button>刹车</button>';
  const game = kind === "racing" ? `<div class="racing-board is-classic"><div class="racing-stage"><div data-viewport-canvas style="height:var(--racing-view-height);background:#294c53"></div><div class="racing-controls">${controls}</div></div></div>` : '<div class="fortressblue-board"><div class="fb-battlefield"><canvas class="fb-canvas" data-viewport-canvas></canvas></div><div class="fb-console"><div class="fb-dial-area"><svg class="fb-angle-dial"></svg></div><div class="fb-instruments"><span>生命 950 / 950</span><label>角度<input aria-label="角度" type="range" min="0" max="90" value="45"></label></div><div class="fb-controls"><button>移动</button><button>武器</button><button data-viewport-fire>蓄力发射</button></div></div></div>';
  await writeFile(path.join(temporary, "index.html"), `<meta name="viewport" content="width=device-width,initial-scale=1"><style>*{box-sizing:border-box}body{margin:0}button{min-height:44px}${contents.join("\n")}</style><div class="game-fullscreen-surface" data-room-fullscreen="page" data-fullscreen-toolbar-overlay><div class="game-fullscreen-board">${game}</div></div>`);
  const require = createRequire(new URL("../package.json", import.meta.url));
  const server = await require("vite").createServer({ configFile: false, root: temporary, cacheDir: path.join(temporary, "vite"), publicDir: false, server: { host: "127.0.0.1", port: 0 } });
  await server.listen();
  const execute = promisify(execFile);
  const session = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "vp"], { cwd: root, env })).stdout.trim();
  const browser = async (...args) => { const result = await execute("agent-browser", ["--session", session, "--profile", path.join(temporary, "profile"), ...args, "--json"], { cwd: root, env, timeout: 30000 }); const value = JSON.parse(result.stdout); assert.ok(value.success, result.stdout); return value.data; };
  const report = { passed: false, scope: "Targeted real-browser CSS geometry of production fullscreen/game selectors with explicit static layout shells; no complete match claim", sources: Object.fromEntries(css.map((file, index) => [file, createHash("sha256").update(contents[index]).digest("hex")])), checks: [] };
  try {
    await browser("open", server.resolvedUrls.local[0]);
    for (const [width, height] of [[844,390],[1180,744],[820,1080],[390,664],[320,568]]) {
      await browser("set", "viewport", String(width), String(height));
      const geometry = await browser("eval", "(()=>{const box=e=>{const r=e.getBoundingClientRect();return Object.fromEntries(['top','bottom','left','right','width','height'].map(k=>[k,Math.round(r[k]*1000)]))};const game=document.querySelector('.game-fullscreen-board').firstElementChild;return {game:box(game),canvas:box(document.querySelector('[data-viewport-canvas]')),controls:[...game.querySelectorAll('button,input')].map(box)}})()");
      const value = geometry.result ?? geometry;
      report.checks.push({ width, height, geometry: value, passed: value.game.top === 0 && value.game.left === 0 && value.game.right === width * 1000 && value.game.bottom === height * 1000 && value.canvas.bottom <= height * 1000 && value.controls.every(control => control.top >= 0 && control.bottom <= height * 1000 && control.right <= width * 1000) });
      await browser("screenshot", path.join(out, `${width}-${height}.png`));
    }
    report.passed = report.checks.every(check => check.passed);
    assert.ok(report.passed, JSON.stringify(report.checks.filter(check => !check.passed)));
  } finally {
    await writeFile(path.join(out, "result.json"), JSON.stringify(report, null, 2) + "\n");
    await browser("close").catch(() => {});
    await server.close();
    await rm(temporary, { recursive: true, force: true });
  }
});
