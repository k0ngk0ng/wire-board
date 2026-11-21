import assert from "node:assert/strict";
import { promisify } from "node:util";
import { execFile } from "node:child_process";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const out = path.join(root, ".local/yuyuhakusho/match-options");
const temporary = path.join(out, "browser-temporary");
const sockets = path.join(root, ".local/yuyuhakusho/s");
for (const folder of [out, temporary, sockets])
  await mkdir(folder, { recursive: true });
const env = {
  ...process.env,
  TMPDIR: temporary,
  AGENT_BROWSER_PROFILE: path.join(temporary, "profile"),
  AGENT_BROWSER_SOCKET_DIR: sockets,
  AGENT_BROWSER_NO_VERSION_CHECK: "1",
};
const execute = promisify(execFile);
const browser = async (...args) => {
  const { stdout } = await execute(
    "agent-browser",
    ["--session", "yo", ...args, "--json"],
    { cwd: root, env, timeout: 30000 },
  );
  const value = JSON.parse(stdout);
  assert.equal(value.success, true, stdout);
  return value.data;
};
const evaluate = async (source) => (await browser("eval", source)).result;
const require = createRequire(
  new URL("../../../web/package.json", import.meta.url),
);
const { createServer } = require("vite");
const react = require("@vitejs/plugin-react").default;
await writeFile(
  path.join(temporary, "index.html"),
  `<meta name="viewport" content="width=device-width,initial-scale=1"><div id="root"></div><script type="module" src="/@fs/${path.join(root, "games/yuyuhakusho/tests/MatchOptionsFixture.tsx")}"></script>`,
);
const server = await createServer({
  configFile: false,
  root: temporary,
  cacheDir: path.join(temporary, "vite-cache"),
  publicDir: false,
  optimizeDeps: {
    include: ["react", "react-dom/client", "react/jsx-dev-runtime"],
  },
  plugins: [
    react(),
    {
      name: "single-game-registry",
      enforce: "pre",
      resolveId(source, importer) {
        if (
          (source.endsWith("/registry") && importer?.includes("/web/src/")) ||
          (source === "./registry" && importer?.includes("/web/src/game-api/"))
        )
          return "\0single-game-registry";
      },
      load(id) {
        if (id === "\0single-game-registry")
          return `import game from ${JSON.stringify(path.join(root, "games/yuyuhakusho/web/module.tsx"))}; export const gameModule=()=>game; export const gameName=()=>game.name; export const gameLoaded=()=>true; export const isIndependentGame=()=>true; export const loadGame=async()=>game; export const useGameRegistry=()=>0;`;
      },
    },
  ],
  resolve: {
    alias: {
      react: path.join(root, "web/node_modules/react"),
      "react-dom": path.join(root, "web/node_modules/react-dom"),
      "lucide-react": path.join(root, "web/node_modules/lucide-react"),
      fflate: path.join(root, "web/node_modules/fflate"),
    },
    dedupe: ["react", "react-dom"],
  },
  server: { host: "127.0.0.1", port: 0, fs: { allow: [root] } },
});
await server.listen();
const result = { passed: false, checks: [], screenshots: [] };
try {
  await browser("open", server.resolvedUrls.local[0]);
  await browser("wait", "[aria-label='赛制']");
  await browser("select", "[aria-label='赛制']", "solo");
  await browser("click", "[data-arena-picker] summary");
  await browser("click", "[data-arena-option='65535']");
  await browser("wait", "[data-arena='65535']");
  await browser("find", "text", "进入准备", "click");
  await browser("wait", ".yuyu-preparation");
  assert.deepEqual(
    await evaluate(
      "Array.from(document.querySelectorAll('[data-yuyu-team]')).map(node=>({label:node.getAttribute('aria-label'),names:Array.from(node.querySelectorAll('.yuyu-role > span')).map(name=>name.textContent)}))",
    ),
    [
      { label: "单人方", names: ["小明（你）"] },
      { label: "三人队", names: ["小红", "阿绿", "电脑"] },
    ],
  );
  assert.deepEqual(
    await evaluate(
      "Array.from(document.querySelectorAll('[data-player-group]')).map(node=>({label:node.getAttribute('aria-label'),seats:Array.from(node.querySelectorAll('[data-player-seat]')).map(player=>Number(player.dataset.playerSeat)),heading:getComputedStyle(node,'::before').content}))",
    ),
    [
      { label: "单人方", seats: [0], heading: '"单人方"' },
      { label: "三人队", seats: [1, 2, 3], heading: '"三人队"' },
    ],
  );
  await browser("click", "[data-character-seat='0'] summary");
  await browser("click", "[data-character-option='255']");
  await browser("wait", "[data-character-seat='0'][data-character='255']");
  await browser("click", "[data-character-seat='3'] summary");
  await browser("click", "[data-character-option='255']");
  assert.equal(
    await evaluate(
      "document.querySelector('[data-character-seat=\"1\"] summary').getAttribute('aria-disabled')",
    ),
    "true",
  );
  result.checks.push(
    "create and prepare random arena, self and computer characters; human seat ownership; one-versus-three groups",
  );
  for (const [width, height] of [
    [1280, 800],
    [320, 700],
    [844, 390],
  ]) {
    await browser("set", "viewport", String(width), String(height));
    await browser("click", "[data-character-seat='0'] summary");
    await browser("wait", "[data-character-option='255']");
    await browser("wait", "--fn", "Array.from(document.images).every(image=>image.complete)");
    const layout = await evaluate(
      "({overflow:document.documentElement.scrollWidth>innerWidth,menu:Array.from(document.querySelectorAll('.yuyu-choice-grid')).map(node=>({left:node.getBoundingClientRect().left,right:node.getBoundingClientRect().right,top:node.getBoundingClientRect().top,bottom:node.getBoundingClientRect().bottom})),width:innerWidth,height:innerHeight,brokenImages:Array.from(document.images).filter(image=>!image.complete||image.naturalWidth===0).length,randomImages:document.querySelectorAll('[data-character=\"255\"] img').length})",
    );
    assert.equal(layout.overflow, false, JSON.stringify(layout));
    assert.equal(await evaluate("Array.from(document.querySelectorAll('.yuyu-character-summary > span:nth-child(2)')).filter(node=>{const range=document.createRange();range.selectNodeContents(node);return range.getClientRects().length>1}).length"), 0, "selected names must stay on one line");
    assert.equal(layout.brokenImages, 0, JSON.stringify(layout));
    assert.equal(layout.randomImages, 0, JSON.stringify(layout));
    for (const menu of layout.menu) {
      assert.ok(
        menu.left >= 0 &&
          menu.right <= width &&
          menu.top >= 0 &&
          menu.bottom <= height,
        JSON.stringify(layout),
      );
    }
    const screenshot = `solo-${width}.png`;
    await browser("screenshot", path.join(out, screenshot));
    result.screenshots.push(screenshot);
    result.checks.push({ width, height, layout });
    await browser("press", "Escape");
    const members = `solo-members-${width}.png`;
    await browser("screenshot", "--full", path.join(out, members));
    result.screenshots.push(members);
  }
  await browser("set", "viewport", "1280", "800");
  await browser("select", "[aria-label='赛制']", "team");
  await browser("wait", "[data-yuyu-team='team-a']");
  assert.deepEqual(
    await evaluate(
      "Array.from(document.querySelectorAll('[data-player-group]')).map(node=>({label:node.getAttribute('aria-label'),seats:Array.from(node.querySelectorAll('[data-player-seat]')).map(player=>Number(player.dataset.playerSeat))}))",
    ),
    [
      { label: "A 队", seats: [0, 1] },
      { label: "B 队", seats: [2, 3] },
    ],
  );
  assert.deepEqual(
    await evaluate(
      "Array.from(document.querySelectorAll('[data-yuyu-team]')).map(node=>Array.from(node.querySelectorAll('.yuyu-role > span')).map(name=>name.textContent))",
    ),
    [
      ["小明（你）", "小红"],
      ["阿绿", "电脑"],
    ],
  );
  await browser("screenshot", path.join(out, "teams-1280.png"));
  result.screenshots.push("teams-1280.png");
  await browser("find", "text", "切换房主", "click");
  assert.equal(
    await evaluate("document.querySelector('[aria-label=赛制]').disabled"),
    true,
  );
  assert.equal(
    await evaluate(
      "document.querySelector('[data-arena-picker] summary').getAttribute('aria-disabled')",
    ),
    "true",
  );
  assert.equal(
    await evaluate(
      "document.querySelector('[data-character-seat=\"1\"] summary').getAttribute('aria-disabled')",
    ),
    "false",
  );
  await browser("find", "text", "切换房主", "click");
  await browser("select", "[aria-label='赛制']", "free");
  assert.equal(
    await evaluate("document.querySelectorAll('[data-yuyu-team]').length"),
    0,
  );
  await browser("select", "[aria-label='赛制']", "solo");
  await browser("find", "text", "创建选项", "click");
  await browser("find", "text", "切换人数", "click");
  await browser(
    "wait",
    "--fn",
    "JSON.parse(document.querySelector('#selection').textContent).characters.length===2",
  );
  const finalOptions = await evaluate(
    "JSON.parse(document.querySelector('#selection').textContent)",
  );
  assert.equal(finalOptions.teamBattle, false);
  assert.equal(!!finalOptions.oneVsThree, false);
  result.checks.push(
    "two-versus-two groups, guest permissions, free-for-all and capacity changes clear team mode",
  );
  result.passed = true;
} finally {
  await writeFile(
    path.join(out, "browser-result.json"),
    JSON.stringify(result, null, 2) + "\n",
  );
  await browser("close").catch(() => {});
  await server.close();
  await rm(temporary, { recursive: true, force: true });
}
process.stdout.write(JSON.stringify(result) + "\n");
