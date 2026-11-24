import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

test(
  "all cover and login image requests start at the CDN without an application redirect",
  {
    skip:
      process.env.DIRECT_IMAGES_BROWSER !== "1" ||
      process.env.GITHUB_ACTIONS === "true",
    timeout: 120000,
  },
  async () => {
    const root = fileURLToPath(new URL("../../", import.meta.url));
    const out = path.join(root, ".local/_common/direct-images/verification");
    const temporary = path.join(out, "browser-temporary");
    const config = JSON.parse(
      await readFile(path.join(root, ".local/business.json"), "utf8"),
    );
    const env = {
      ...process.env,
      TMPDIR: path.join(temporary, "tmp"),
      AGENT_BROWSER_SOCKET_DIR: path.join(root, ".local/_common/s"),
      AGENT_BROWSER_PROFILE: path.join(temporary, "profile"),
      AGENT_BROWSER_SCREENSHOT_DIR: out,
      AGENT_BROWSER_NO_VERSION_CHECK: "1",
    };
    for (const folder of [
      temporary,
      env.TMPDIR,
      env.AGENT_BROWSER_SOCKET_DIR,
      env.AGENT_BROWSER_PROFILE,
    ])
      await mkdir(folder, { recursive: true });
    const require = createRequire(new URL("../package.json", import.meta.url));
    const execute = promisify(execFile);
    const session = (
      await execute(
        "agent-browser",
        ["session", "id", "--scope", "worktree", "--prefix", "di"],
        { env },
      )
    ).stdout.trim();
    const browser = async (...args) => {
      const response = await execute(
        "agent-browser",
        ["--session", session, ...args, "--json"],
        { env, timeout: 30000, maxBuffer: 4 << 20 },
      );
      const value = JSON.parse(response.stdout);
      assert.equal(value.success, true, response.stdout);
      return value.data;
    };
    const fixture = path.join(temporary, "fixture.tsx");
    await writeFile(
      fixture,
      'import {createRoot} from "react-dom/client";import {Auth} from "/src/ui/auth";import {gameModule} from "/src/game-api/registry";import covers from "/@fs/' +
        path.join(root, "assets/game-covers.json") +
        '";function Page(){return <><section style={{display:"grid",gridTemplateColumns:"repeat(6,160px)",gap:12}}>{Object.keys(covers.games).map(game=>{const Cover=gameModule(game)!.Cover!;return <div className="cover" style={{width:160,height:95}} key={game}><Cover/></div>})}</section><Auth busy={false} error="" invited={false} submit={()=>{}}/></>};createRoot(document.getElementById("root")!).render(<Page/>);',
    );
    let applicationImageRequests = 0;
    const server = await require("vite").createServer({
      configFile: false,
      root: path.join(root, "web"),
      publicDir: false,
      cacheDir: path.join(temporary, "vite"),
      esbuild: { jsx: "automatic" },
      define: {
        "import.meta.env.VITE_CDN_ORIGIN": JSON.stringify(config.CDN_ORIGIN),
      },
      optimizeDeps: {
        noDiscovery: true,
        include: [
          "react",
          "react-dom",
          "react-dom/client",
          "react/jsx-runtime",
          "react/jsx-dev-runtime",
          "lucide-react",
        ],
      },
      resolve: { dedupe: ["react", "react-dom", "lucide-react"] },
      server: { host: "127.0.0.1", port: 0, hmr: false, fs: { allow: [root] } },
      plugins: [
        {
          name: "direct-image-verification",
          configureServer(server) {
            server.middlewares.use((request, response, next) => {
              if (
                request.url.startsWith("/game-covers/") ||
                request.url.startsWith("/bga/game-covers/") ||
                request.url.startsWith("/_common/login/")
              ) {
                applicationImageRequests++;
                response.statusCode = 500;
                response.end(
                  "Application image aliases are forbidden in this acceptance",
                );
              } else if (request.url === "/") {
                response.setHeader("Content-Type", "text/html; charset=utf-8");
                response.end(
                  '<!doctype html><html lang="zh-CN"><meta charset="utf-8"><div id="root"></div><script type="module" src="/@fs/' +
                    fixture +
                    '"></script></html>',
                );
              } else next();
            });
          },
        },
      ],
    });
    const report = { passed: false, applicationImageRequests: 0 };
    try {
      await server.listen();
      await browser(
        "open",
        "http://127.0.0.1:" + server.httpServer.address().port,
      );
      await browser("wait", "--fn", "document.images.length===33");
      const images = (
        await browser(
          "eval",
          '[...document.images].map(image=>image.getAttribute("src"))',
        )
      ).result;
      assert.equal(images.length, 33);
      assert.ok(
        images.every((src) => src.startsWith(config.CDN_ORIGIN + "/bga/")),
        JSON.stringify(images),
      );
      await browser(
        "wait",
        "--fn",
        "[...document.images].every(image=>image.complete&&image.naturalWidth>0)",
      );
      const requests = (
        await browser(
          "eval",
          'performance.getEntriesByType("resource").filter(entry=>entry.initiatorType==="img").map(entry=>entry.name)',
        )
      ).result;
      assert.ok(requests.length >= 33);
      assert.ok(
        requests.every((url) => url.startsWith(config.CDN_ORIGIN + "/bga/")),
      );
      assert.equal(applicationImageRequests, 0);
      report.images = images;
      report.requests = requests;
      report.applicationImageRequests = applicationImageRequests;
      report.passed = true;
      await browser(
        "screenshot",
        "--full",
        path.join(out, "direct-images.png"),
      );
    } finally {
      await writeFile(
        path.join(out, "browser-result.json"),
        JSON.stringify(report, null, 2) + "\n",
      );
      await browser("close").catch(() => {});
      await server.close();
      await rm(temporary, { recursive: true, force: true });
    }
  },
);
