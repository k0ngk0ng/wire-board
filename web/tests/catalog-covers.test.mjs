import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";
import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../../", import.meta.url));
const output = path.join(root, ".local/_common/covers/verification");
const temporary = path.join(output, "temporary");
const require = createRequire(new URL("../package.json", import.meta.url));
const { createElement } = require("react");
const { renderToStaticMarkup } = require("react-dom/server");
const games = [
  "splendor",
  "rail",
  "catan",
  "carcassonne",
  "sanguosha",
  "basketball",
  "monopoly",
  "starfort",
  "doudizhu",
  "guandan",
  "shengji",
  "snooker",
  "pairs",
  "gomoku",
  "junqi",
  "chess",
  "xiangqi",
  "weiqi",
  "racing",
  "tetris",
  "battlecity",
  "bubbleparty",
  "fortressblue",
  "conquest",
  "tabletennis",
  "contra",
  "yuyuhakusho",
  "pvz",
  "pokepath",
  "hotlinemiami",
  "cs16",
  "pal",
];

test("replacement covers register game ownership and matching publication metadata", async () => {
  const manifest = JSON.parse(
    await readFile(path.join(root, "assets/game-covers.json"), "utf8"),
  );
  const resources = JSON.parse(
    await readFile(path.join(root, "assets/resources.json"), "utf8"),
  );
  const external = JSON.parse(
    await readFile(path.join(root, "assets/external-resources.json"), "utf8"),
  );
  assert.deepEqual(Object.keys(manifest.games).sort(), [...games].sort());
  for (const game of games) {
    const cover = manifest.games[game];
    assert.equal(cover.path, `/game-covers/${game}.png`);
    const file = `games/${game}/assets/public${cover.path}`;
    assert.equal(cover.width, 480);
    assert.equal(cover.height, 285);
    const record = resources.files[file];
    assert.equal(record.key, `bga/${game}/${record.sha256}.png`);
    assert.match(record.sha256, /^[a-f0-9]{64}$/);
    assert.equal(record.mime, "image/png");
    assert.ok(Number.isSafeInteger(record.bytes) && record.bytes > 0);
    assert.deepEqual(external.files[cover.path], record);
  }
});

test("catalog, legacy pages and loaded runtime render the replacement covers", async () => {
  await mkdir(temporary, { recursive: true });
  const server = await require("vite").createServer({
    configFile: false,
    root: path.join(root, "web"),
    cacheDir: path.join(temporary, "vite"),
    publicDir: false,
    esbuild: { jsx: "automatic" },
    optimizeDeps: { noDiscovery: true },
    resolve: {
      dedupe: ["react", "react-dom", "lucide-react", "three", "fflate"],
    },
    server: { middlewareMode: true, hmr: false, fs: { allow: [root] } },
  });
  const report = { passed: false, games: [], legacy: [], runtime: "" };
  try {
    const registry = await server.ssrLoadModule("/src/game-api/registry.ts");
    const { LegacyCover } = await server.ssrLoadModule("/src/legacy/cover.tsx");
    const { AssetsContext } = await server.ssrLoadModule(
      "/src/ui/board-host.tsx",
    );
    const render = (component, base, props = {}) =>
      renderToStaticMarkup(
        createElement(
          AssetsContext.Provider,
          { value: base },
          createElement(component, props),
        ),
      );
    for (const game of games) {
      for (const base of ["", "/bga"]) {
        const markup = render(registry.gameModule(game).Cover, base);
        assert.ok(
          markup.includes(`src="${base}/game-covers/${game}.png"`),
          game,
        );
        assert.ok(markup.includes('width="480" height="285"'), game);
        assert.ok(markup.includes("object-fit:cover"), game);
        assert.ok(!markup.includes("<svg"), game);
      }
      report.games.push(game);
    }
    for (const game of [
      "splendor",
      "rail",
      "catan",
      "carcassonne",
      "sanguosha",
    ]) {
      const markup = render(LegacyCover, "/bga", { kind: game, mini: true });
      assert.ok(markup.includes(`src="/bga/game-covers/${game}.png"`), game);
      assert.ok(markup.includes(`cover ${game} mini`), game);
      report.legacy.push(game);
    }
    assert.equal(registry.gameModule("escape"), undefined);
    await registry.loadGame("gomoku");
    assert.ok(
      render(registry.gameModule("gomoku").Cover, "/bga").includes(
        'src="/bga/game-covers/gomoku.png"',
      ),
    );
    report.runtime = "gomoku";
    report.passed = true;
  } catch (error) {
    report.error = String(error);
    throw error;
  } finally {
    await writeFile(
      path.join(output, "render-result.json"),
      JSON.stringify(report, null, 2) + "\n",
    );
    await server.close();
    await rm(temporary, { recursive: true, force: true });
  }
});
