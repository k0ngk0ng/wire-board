import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";
import { execFile, spawn } from "node:child_process";
import { promisify } from "node:util";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

test(
  "every game creates and keeps its first and other legal room capacities",
  {
    skip:
      process.env.ALL_ROOM_CAPACITIES !== "1" ||
      process.env.GITHUB_ACTIONS === "true",
    timeout: 600000,
  },
  async () => {
    const root = fileURLToPath(new URL("../../", import.meta.url));
    const output = path.join(root, ".local/_common/room-capacity/verification");
    const temporary = path.join(output, "temporary");
    const env = {
      ...process.env,
      GOTOOLCHAIN: "local",
      GOCACHE: path.join(root, ".local/go-build"),
      GOMODCACHE: path.join(root, ".local/go-mod"),
      GOTMPDIR: path.join(temporary, "go"),
      TMPDIR: path.join(temporary, "tmp"),
      AGENT_BROWSER_SOCKET_DIR: path.join(root, ".local/_common/s"),
      AGENT_BROWSER_PROFILE: path.join(temporary, "profile"),
      AGENT_BROWSER_NO_VERSION_CHECK: "1",
    };
    for (const directory of [
      output,
      temporary,
      env.GOCACHE,
      env.GOMODCACHE,
      env.GOTMPDIR,
      env.TMPDIR,
      env.AGENT_BROWSER_SOCKET_DIR,
      env.AGENT_BROWSER_PROFILE,
    ])
      await mkdir(directory, { recursive: true });
    const execute = promisify(execFile);
    const require = createRequire(new URL("../package.json", import.meta.url));
    let backend, frontend, session;
    let backendLog = "";
    const report = {
      passed: false,
      scope:
        "Actual application and all production game modules; local HTTP, SQLite, WebSocket and browser selections; no production mutations",
      games: [],
    };
    const browser = async (...args) => {
      const response = await execute(
        "agent-browser",
        ["--session", session, ...args, "--json"],
        { cwd: root, env, timeout: 35000, maxBuffer: 4 << 20 },
      );
      const result = JSON.parse(response.stdout);
      assert.equal(result.success, true);
      return result.data;
    };
    const evaluate = async (source) =>
      (await browser("eval", "-b", Buffer.from(source).toString("base64")))
        .result;
    const api = (suffix, body) =>
      evaluate(
        `(async()=>{const response=await fetch('/api'+${JSON.stringify(suffix)},{method:${JSON.stringify(body ? "POST" : "GET")},headers:{'Content-Type':'application/json'},body:${body ? JSON.stringify(JSON.stringify(body)) : "undefined"}});const value=await response.json();if(!response.ok)throw Error(JSON.stringify(value));return value})()`,
      );
    const room = async () => (await api("/state")).room;
    const wait = async (predicate) => {
      for (let attempt = 0; attempt < 100; attempt++) {
        if (await predicate()) return;
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
      throw Error("room state did not settle");
    };
    try {
      const serverFile = path.join(temporary, "server.go");
      await writeFile(
        serverFile,
        `package main
import("context";"fmt";"net";"net/http";"os";"testing/fstest";"github.com/k0ngk0ng/wire-board/internal/production";"github.com/k0ngk0ng/wire-board/internal/server")
func main(){registry,err:=production.Open(context.Background(),os.DirFS(os.Args[2]));if err!=nil{panic(err)};defer registry.Close(context.Background());app,err:=server.New(server.Config{DataDir:os.Args[1],InviteCode:"capacity-local",AdminUsername:"CapacityRoot",AdminPassword:"capacity-local-password",Modules:registry.Modules,ClientLoading:true},fstest.MapFS{"index.html":{Data:[]byte("local capacity acceptance")}});if err!=nil{panic(err)};defer app.Close();listener,err:=net.Listen("tcp","127.0.0.1:0");if err!=nil{panic(err)};fmt.Println("CAPACITY_URL=http://"+listener.Addr().String());if err:=http.Serve(listener,app.Handler());err!=nil{panic(err)}}`,
      );
      const receipt = JSON.parse(
        await readFile(
          path.join(root, "web/dist/contra-delivery.json"),
          "utf8",
        ),
      );
      const flags =
        "-X github.com/k0ngk0ng/wire-board/games/contra/runtime.CompiledRulesBuildID=" +
        receipt.rulesBuildID +
        " -X github.com/k0ngk0ng/wire-board/games/contra/runtime.CompiledRuntimeBuildID=" +
        receipt.runtimeBuildID;
      const binary = path.join(temporary, "server");
      await execute(
        "go",
        ["build", "-ldflags=" + flags, "-o", binary, serverFile],
        { cwd: root, env, timeout: 120000, maxBuffer: 4 << 20 },
      );
      backend = spawn(
        binary,
        [
          path.join(temporary, "data"),
          path.join(root, ".local/_common/room-capacity/runtime/pal"),
        ],
        { cwd: root, env, stdio: ["ignore", "pipe", "pipe"] },
      );
      backend.stdout.on("data", (data) => {
        backendLog += data;
      });
      backend.stderr.on("data", (data) => {
        backendLog += data;
      });
      await wait(() =>
        /CAPACITY_URL=http:\/\/127\.0\.0\.1:\d+/.test(backendLog),
      );
      const target = backendLog.match(
        /CAPACITY_URL=(http:\/\/127\.0\.0\.1:\d+)/,
      )[1];
      const config = JSON.parse(
        await readFile(path.join(root, ".local/business.json"), "utf8"),
      );
      frontend = await require("vite").createServer({
        configFile: false,
        root: path.join(root, "web"),
        publicDir: path.join(root, "web/public"),
        cacheDir: path.join(temporary, "vite"),
        esbuild: { jsx: "automatic" },
        define: {
          "import.meta.env.VITE_CDN_ORIGIN": JSON.stringify(config.CDN_ORIGIN),
        },
        resolve: {
          dedupe: ["react", "react-dom", "lucide-react", "three", "fflate"],
        },
        server: {
          host: "127.0.0.1",
          port: 0,
          hmr: false,
          fs: { allow: [root] },
          proxy: { "/api": { target, ws: true }, "/healthz": target },
        },
      });
      await frontend.listen();
      const origin = "http://127.0.0.1:" + frontend.httpServer.address().port;
      session = (
        await execute(
          "agent-browser",
          ["session", "id", "--scope", "worktree", "--prefix", "rc"],
          { cwd: root, env },
        )
      ).stdout.trim();
      await browser("open", origin);
      await browser("wait", 'input[name="name"]');
      await api("/login", {
        name: "CapacityRoot",
        password: "capacity-local-password",
      });
      await browser("reload");
      await browser("wait", ".room-list");
      const available = (await api("/state")).availableGames;
      const covers = JSON.parse(
        await readFile(path.join(root, "assets/game-covers.json"), "utf8"),
      );
      assert.deepEqual([...available].sort(), Object.keys(covers.games).sort());
      report.expectedGames = available.length;
      const games = [
        "tabletennis",
        ...available.filter((kind) => kind !== "tabletennis"),
      ];
      for (const width of [1440, 320]) {
        await browser("set", "viewport", String(width), "900");
        for (const kind of games) {
          const entry = {
            kind,
            width,
            passed: false,
            capacities: [],
            errors: [],
          };
          report.games.push(entry);
          try {
            await browser("click", ".lobby-create");
            await browser("wait", '[aria-label="搜索可选游戏"]');
            await browser("fill", '[aria-label="搜索可选游戏"]', kind);
            await browser("click", ".choose-games button");
            await browser("wait", '[aria-label="房间人数"]');
            await wait(async () => (await room())?.kind === kind);
            entry.created = (await room()).capacity;
            const counts = await evaluate(
              "[...document.querySelector('[aria-label=\"房间人数\"]').options].map(option=>({value:Number(option.value),disabled:option.disabled}))",
            );
            entry.counts = counts;
            assert.equal(
              entry.created,
              Math.max(...counts.map((option) => option.value)),
            );
            assert.ok(
              counts.length &&
                counts.every((option) => Number.isSafeInteger(option.value)),
            );
            const selector = '[aria-label="房间人数"]';
            const custom = {
              tabletennis: { bestOf: 3, court: "heritage", difficulty: "hard" },
              basketball: { mode: "versus" },
              junqi: { difficulty: "easy", turnSeconds: 90, setupSeconds: 60 },
              pvz: { roundSeconds: 180, prepSeconds: 10 },
            }[kind];
            if (custom) {
              const original = await room();
              await api("/rooms/" + original.id, {
                type: "room_settings",
                capacity: original.capacity,
                options: { ...original.options, ...custom },
                version: original.version,
                nonce: crypto.randomUUID(),
              });
              await wait(async () => {
                const updated = await room();
                return Object.entries(custom).every(
                  ([key, value]) => updated.options[key] === value,
                );
              });
              const ready = {
                tabletennis:
                  'document.querySelector(\'[aria-label="乒乓球场馆"]\')?.value==="heritage"',
                basketball:
                  'document.querySelector(".basketball-options select")?.value==="versus"',
                junqi:
                  'document.querySelector(\'[aria-label="军棋电脑难度"]\')?.value==="easy"',
                pvz: 'document.querySelector(\'[aria-label="植物大战僵尸每回合时限"]\')?.value==="180"',
              }[kind];
              await browser("wait", "--fn", ready);
              const configured = await room();
              entry.preserved =
                kind === "basketball"
                  ? {
                      teams: configured.options.teams,
                      characters: configured.options.characters,
                    }
                  : custom;
            }
            for (const value of [
              ...counts.map((option) => option.value),
              counts[0].value,
            ]) {
              if (counts.length > 1) {
                await browser("wait", selector + ":not(:disabled)");
                await browser("select", selector, String(value));
                await wait(
                  async () =>
                    !(await evaluate(
                      "document.querySelector('[aria-label=\"房间人数\"]').disabled",
                    )),
                );
              }
              await new Promise((resolve) => setTimeout(resolve, 200));
              const saved = await room();
              const shown = await evaluate(
                "Number(document.querySelector('[aria-label=\"房间人数\"]').value)",
              );
              const check = {
                requested: value,
                saved: saved.capacity,
                shown,
                seats: await evaluate(
                  'document.querySelectorAll(".waiting-seat").length',
                ),
              };
              if (entry.preserved) {
                for (const [key, value] of Object.entries(entry.preserved))
                  if (
                    JSON.stringify(saved.options[key]) !== JSON.stringify(value)
                  )
                    entry.errors.push({
                      option: key,
                      expected: value,
                      saved: saved.options[key],
                    });
                check.options = {
                  mode: saved.options.mode,
                  teams: saved.options.teams,
                };
              }
              entry.capacities.push(check);
              if (
                saved.capacity !== value ||
                shown !== value ||
                check.seats !== value
              )
                entry.errors.push(check);
            }
            await browser("reload");
            await browser("wait", selector);
            entry.afterReload = {
              saved: (await room()).capacity,
              shown: await evaluate(
                "Number(document.querySelector('[aria-label=\"房间人数\"]').value)",
              ),
            };
            if (
              entry.afterReload.saved !== counts[0].value ||
              entry.afterReload.shown !== counts[0].value
            )
              entry.errors.push(entry.afterReload);
            if (kind === "tabletennis")
              await browser(
                "screenshot",
                "--full",
                path.join(output, "tabletennis-" + width + ".png"),
              );
            entry.passed = entry.errors.length === 0;
          } catch (error) {
            entry.errors.push(String(error));
          }
          const current = await room();
          if (current)
            await api("/rooms/" + current.id, {
              type: "leave",
              version: current.version,
              nonce: crypto.randomUUID(),
            });
          await browser("reload");
          await browser("wait", ".room-list");
          await writeFile(
            path.join(output, "result.json"),
            JSON.stringify(report, null, 2) + "\n",
          );
        }
      }
      report.passed =
        report.games.length === games.length * 2 &&
        report.games.every((entry) => entry.passed);
      assert.ok(
        report.passed,
        JSON.stringify(
          report.games
            .filter((entry) => !entry.passed)
            .map((entry) => ({
              kind: entry.kind,
              width: entry.width,
              errors: entry.errors,
            })),
        ),
      );
    } finally {
      await writeFile(
        path.join(output, "result.json"),
        JSON.stringify(report, null, 2) + "\n",
      );
      await writeFile(path.join(output, "server.log"), backendLog);
      if (session) await browser("close").catch(() => {});
      await frontend?.close();
      if (backend) {
        backend.kill("SIGTERM");
        await new Promise((resolve) =>
          backend.exitCode !== null ? resolve() : backend.once("exit", resolve),
        );
      }
      await rm(temporary, { recursive: true, force: true });
    }
  },
);
