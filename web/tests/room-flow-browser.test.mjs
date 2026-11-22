import test from "node:test";
import assert from "node:assert/strict";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { execFile, spawn } from "node:child_process";
import { promisify } from "node:util";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { createHash, randomUUID } from "node:crypto";

test(
  "one-click rooms, dynamic settings and completed match tools work through the browser",
  { timeout: 300000 },
  async () => {
    const root = fileURLToPath(new URL("../../", import.meta.url));
    const require = createRequire(new URL("../package.json", import.meta.url));
    const execute = promisify(execFile);
    const common = (
      await execute(
        "git",
        ["rev-parse", "--path-format=absolute", "--git-common-dir"],
        { cwd: root },
      )
    ).stdout.trim();
    const workspace = path.dirname(common);
    const owned = path.join(root, ".local/room-flow");
    const out = path.join(
      owned,
      "verification",
      process.env.ROOM_FLOW_REPORT ?? "current",
    );
    assert.ok(out.startsWith(path.join(owned, "verification") + path.sep));
    const temporary = path.join(owned, "tmp", randomUUID());
    const environment = {
      ...process.env,
      GOTOOLCHAIN: "local",
      GOCACHE: path.join(root, ".local/go-build"),
      GOMODCACHE: path.join(root, ".local/go-mod"),
      GOTMPDIR: path.join(temporary, "go"),
      TEST_TELEMETRY_DIR: path.join(temporary, "telemetry"),
      TMPDIR: path.join(workspace, ".local/rft"),
      XDG_CACHE_HOME: path.join(temporary, "cache"),
      AGENT_BROWSER_SOCKET_DIR:
        process.env.AGENT_BROWSER_SOCKET_DIR ??
        path.join(workspace, ".local/room-flow/s"),
      AGENT_BROWSER_PROFILE: path.join(temporary, "profile"),
      AGENT_BROWSER_NO_VERSION_CHECK: "1",
      AGENT_BROWSER_ARGS: [
        "--mute-audio",
        "--disable-breakpad",
        "--disable-crash-reporter",
        "--blink-settings=primaryHoverType=2",
        ...(process.env.GITHUB_ACTIONS === "true" ? ["--no-sandbox"] : []),
      ].join(","),
    };
    for (const directory of [
      out,
      temporary,
      environment.GOCACHE,
      environment.GOMODCACHE,
      environment.GOTMPDIR,
      environment.TEST_TELEMETRY_DIR,
      environment.TMPDIR,
      environment.XDG_CACHE_HOME,
      environment.AGENT_BROWSER_SOCKET_DIR,
      environment.AGENT_BROWSER_PROFILE,
    ])
      await mkdir(directory, { recursive: true });
    const report = {
      passed: false,
      scope:
        "Full application, real local HTTP/SQLite/WebSocket, desktop mouse and Chromium 320px touch emulation; no physical phone",
      checks: [],
      sources: {},
      observations: {},
    };
    let frontend,
      backend,
      session,
      socket,
      attached,
      backendLog = "";
    const pending = new Map();
    let ordinal = 0;
    const browser = async (...args) => {
      const result = await execute(
        "agent-browser",
        ["--session", session, ...args, "--json"],
        { cwd: root, env: environment, timeout: 35000, maxBuffer: 4 << 20 },
      );
      const value = JSON.parse(result.stdout);
      assert.equal(value.success, true, JSON.stringify(value));
      return value.data;
    };
    const evaluate = async (code) =>
      (await browser("eval", "-b", Buffer.from(code).toString("base64")))
        .result;
    const check = (name, passed, detail) => {
      report.checks.push({
        name,
        passed: !!passed,
        ...(detail === undefined ? {} : { detail }),
      });
      assert.ok(
        passed,
        name + (detail === undefined ? "" : ": " + JSON.stringify(detail)),
      );
    };
    const call = (method, params = {}, sessionId = attached) =>
      new Promise((resolve, reject) => {
        const id = ++ordinal;
        const timer = setTimeout(() => {
          pending.delete(id);
          reject(Error(method + " timed out"));
        }, 10000);
        pending.set(id, { resolve, reject, timer });
        socket.send(
          JSON.stringify({
            id,
            method,
            params,
            ...(sessionId ? { sessionId } : {}),
          }),
        );
      });
    const activate = async (selector, touch = false) => {
      await browser("scrollintoview", selector);
      if (!touch) return browser("click", selector);
      const point = await evaluate(
        `(()=>{const r=document.querySelector(${JSON.stringify(selector)}).getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()`,
      );
      await call("Input.dispatchTouchEvent", {
        type: "touchStart",
        touchPoints: [{ ...point, id: 1 }],
      });
      await call("Input.dispatchTouchEvent", {
        type: "touchEnd",
        touchPoints: [],
      });
    };
    const api = async (route, body) => {
      const response = await evaluate(
        `fetch(${JSON.stringify("/api" + route)},${JSON.stringify(body === undefined ? {} : { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) })}).then(async r=>({status:r.status,data:await r.json()}))`,
      );
      assert.ok(
        response.status >= 200 && response.status < 300,
        JSON.stringify(response),
      );
      return response.data;
    };
    const current = async () => (await api("/state")).room;
    const command = async (kind, extra = {}) => {
      const room = await current();
      return api("/rooms/" + room.id, {
        type: kind,
        version: room.version,
        nonce: randomUUID(),
        ...extra,
      });
    };
    const client = (origin) => {
      let cookie = "";
      const request = async (route, body, expectedStatus) => {
        const response = await fetch(origin + "/api" + route, {
          headers: {
            "Content-Type": "application/json",
            ...(cookie ? { Cookie: cookie } : {}),
          },
          ...(body === undefined
            ? {}
            : { method: "POST", body: JSON.stringify(body) }),
        });
        if (response.headers.has("set-cookie"))
          cookie = response.headers.get("set-cookie").split(";")[0];
        const data = await response.json();
        if (expectedStatus === undefined)
          assert.ok(response.ok, JSON.stringify(data));
        else
          assert.equal(response.status, expectedStatus, JSON.stringify(data));
        return data;
      };
      return {
        request,
        command: async (kind, extra = {}) => {
          const room = (await request("/state")).room;
          return request("/rooms/" + room.id, {
            type: kind,
            version: room.version,
            nonce: randomUUID(),
            ...extra,
          });
        },
      };
    };
    const capture = async (name) => {
      check(
        name + " has no horizontal overflow",
        await evaluate("document.documentElement.scrollWidth<=innerWidth"),
      );
      const details = await evaluate(
        `(()=>{const luminance=color=>{const rgb=color.match(/[\\d.]+/g).slice(0,3).map(Number).map(value=>{const channel=value/255;return channel<=0.04045?channel/12.92:((channel+0.055)/1.055)**2.4});return rgb[0]*0.2126+rgb[1]*0.7152+rgb[2]*0.0722};return [...document.querySelectorAll('.match-result-modal h2,.match-result-modal p,.result-actions button,[aria-label="离开房间？"] p,[aria-label="离开房间？"] button,.room-tool-tooltip')].filter(element=>getComputedStyle(element).opacity==='1'&&getComputedStyle(element).visibility==='visible').map(element=>{const box=element.getBoundingClientRect();let parent=element,background;while(parent){background=getComputedStyle(parent).backgroundColor;if(background!=='rgba(0, 0, 0, 0)'&&background!=='transparent')break;parent=parent.parentElement}const foreground=luminance(getComputedStyle(element).color),back=luminance(background);return {text:element.textContent,contrast:(Math.max(foreground,back)+0.05)/(Math.min(foreground,back)+0.05),inside:box.left>=0&&box.top>=0&&box.right<=innerWidth&&box.bottom<=innerHeight}})})()`,
      );
      report.observations[name] = details;
      check(
        name + " new text is visible and readable",
        details.every((detail) => detail.inside && detail.contrast >= 4.5),
        details,
      );
      await browser("screenshot", path.join(out, name + ".png"));
      await writeFile(
        path.join(out, name + ".snapshot.json"),
        JSON.stringify(await browser("snapshot", "-i"), null, 2),
      );
    };
    const wait = (condition) => browser("wait", "--fn", condition);
    const shown = (selector) =>
      `!!document.querySelector(${JSON.stringify(selector)})`;
    try {
      const serverSource = `package main
import (
"fmt"
"net"
"net/http"
"os"
"testing/fstest"
gomoku "github.com/k0ngk0ng/wire-board/games/gomoku/server"
monopoly "github.com/k0ngk0ng/wire-board/games/monopoly/server"
cs16 "github.com/k0ngk0ng/wire-board/games/cs16/server"
hotlinemiami "github.com/k0ngk0ng/wire-board/games/hotlinemiami/server"
yuyuhakusho "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/server"
"github.com/k0ngk0ng/wire-board/internal/gameapi"
"github.com/k0ngk0ng/wire-board/internal/server"
)
func main() {
app, err := server.New(server.Config{DataDir:os.Args[1], InviteCode:"flow-invite", AdminUsername:"FlowRoot", AdminPassword:"flow-local-password", Modules:[]gameapi.Module{gomoku.Module(),monopoly.Module(),cs16.Module(),hotlinemiami.Module(),yuyuhakusho.Module()}}, fstest.MapFS{"index.html":{Data:[]byte("local flow acceptance")}})
if err != nil {panic(err)}
defer app.Close()
listener, err := net.Listen("tcp", "127.0.0.1:0")
if err != nil {panic(err)}
fmt.Println("FLOW_URL=http://"+listener.Addr().String())
if err := http.Serve(listener, app.Handler()); err != nil {panic(err)}
}
`;
      const serverFile = path.join(temporary, "server.go");
      await writeFile(serverFile, serverSource);
      const binary = path.join(temporary, "server");
      await execute("go", ["build", "-o", binary, serverFile], {
        cwd: root,
        env: environment,
        timeout: 120000,
        maxBuffer: 4 << 20,
      });
      backend = spawn(binary, [path.join(temporary, "data")], {
        cwd: root,
        env: environment,
        stdio: ["ignore", "pipe", "pipe"],
      });
      backend.stdout.on("data", (data) => {
        backendLog += data;
      });
      backend.stderr.on("data", (data) => {
        backendLog += data;
      });
      const backendOrigin = await new Promise((resolve, reject) => {
        const timer = setTimeout(
          () => reject(Error("backend start timed out: " + backendLog)),
          15000,
        );
        const inspect = (data) => {
          const match = String(data).match(
            /FLOW_URL=(http:\/\/127\.0\.0\.1:\d+)/,
          );
          if (match) {
            clearTimeout(timer);
            resolve(match[1]);
          }
        };
        backend.stdout.on("data", inspect);
        backend.once("exit", (code) => {
          clearTimeout(timer);
          reject(Error("backend exited: " + code + " " + backendLog));
        });
      });
      const { createServer } = await import(require.resolve("vite"));
      frontend = await createServer({
        configFile: false,
        root: path.join(root, "web"),
        cacheDir: path.join(temporary, "vite"),
        publicDir: path.join(root, "web/public"),
        esbuild: { jsx: "automatic" },
        resolve: {
          dedupe: ["react", "react-dom", "lucide-react", "three", "fflate"],
        },
        plugins: [
          {
            name: "room-flow-manifest",
            configureServer(server) {
              server.middlewares.use(
                "/game-load-manifest.json",
                (request, response) => {
                  response.setHeader("Content-Type", "application/json");
                  response.end(
                    JSON.stringify({
                      build: "room-flow-development",
                      entry: "/src/main.tsx",
                      games: { gomoku: [] },
                      media: {},
                    }),
                  );
                },
              );
            },
          },
        ],
        server: {
          host: "127.0.0.1",
          port: 0,
          fs: { allow: [root] },
          proxy: {
            "/api": { target: backendOrigin, ws: true },
            "/healthz": backendOrigin,
          },
        },
      });
      await frontend.listen();
      const origin = `http://127.0.0.1:${frontend.httpServer.address().port}`;
      session = (
        await execute(
          "agent-browser",
          ["session", "id", "--scope", "worktree", "--prefix", "rf"],
          { cwd: root, env: environment },
        )
      ).stdout.trim();
      await browser("open", origin);
      await browser("wait", "input[name=name]");
      const endpoint = await browser("get", "cdp-url");
      socket = new WebSocket(
        typeof endpoint === "string"
          ? endpoint
          : (endpoint.url ?? endpoint.cdpUrl ?? endpoint.value),
      );
      socket.onmessage = ({ data }) => {
        const message = JSON.parse(data),
          entry = pending.get(message.id);
        if (!entry) return;
        clearTimeout(entry.timer);
        pending.delete(message.id);
        if (message.error) entry.reject(Error(JSON.stringify(message.error)));
        else entry.resolve(message.result);
      };
      await new Promise((resolve, reject) => {
        socket.onopen = resolve;
        socket.onerror = reject;
      });
      const targets = await call("Target.getTargets", {}, undefined);
      const target = targets.targetInfos.find(
        (target) => target.type === "page" && target.url.startsWith(origin),
      );
      assert.ok(target);
      attached = (
        await call(
          "Target.attachToTarget",
          { targetId: target.targetId, flatten: true },
          undefined,
        )
      ).sessionId;
      report.browser = await call("Browser.getVersion", {}, undefined);
      for (const width of [1366, 320]) {
        const touch = width === 320;
        await browser("set", "viewport", String(width), "900");
        await call("Emulation.setDeviceMetricsOverride", {
          width,
          height: 900,
          deviceScaleFactor: 1,
          mobile: touch,
        });
        await call("Emulation.setTouchEmulationEnabled", {
          enabled: touch,
          maxTouchPoints: 5,
        });
        if (width === 320) await api("/logout", {});
        await api("/login", {
          name: "FlowRoot",
          password: "flow-local-password",
        });
        await browser("reload");
        await wait(shown(".lobby"));
        for (const [kind, title, minimum, maximum] of [
          ["splendor", "璀璨宝石", 2, 4],
          ["rail", "铁路环游", 2, 5],
          ["carcassonne", "卡卡颂", 2, 5],
          ["sanguosha", "三国杀", 4, 8],
          ["catan", "卡坦岛", 2, 6],
          ["monopoly", "强手棋", 2, 6],
          ["gomoku", "五子棋", 2, 2],
          ["cs16", "反恐精英", 2, 10],
          ["hotlinemiami", "迈阿密热线", 2, 4],
          ["yuyuhakusho", "魔强统一战", 2, 4],
        ]) {
          await browser(
            "find",
            "role",
            "button",
            "click",
            "--name",
            "创建房间",
            "--exact",
          );
          await browser("fill", '[aria-label="搜索可选游戏"]', title);
          check(
            "creation has no name field " + kind + width,
            await evaluate(
              '!document.querySelector(".create-room-modal input[name=name]")',
            ),
          );
          await activate(".choose-games button", touch);
          await wait(shown(".waiting-layout"));
          const opened = await current();
          check(
            "one click enters named room " + kind + width,
            opened.kind === kind &&
              opened.name === "FlowRoot的房间" &&
              !(await evaluate(shown(".create-room-modal"))),
          );
          check(
            "room defaults to maximum supported players " + kind + width,
            opened.capacity === maximum,
          );
          check(
            "room settings have no manual save action " + kind + width,
            await evaluate(
              "!document.querySelector('[data-room-settings-save]')",
            ),
          );
          if (kind === "gomoku") {
            const invalid = await evaluate(
              `fetch('/api/rooms/${opened.id}',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({type:'room_settings',capacity:3,version:${opened.version},nonce:crypto.randomUUID()})}).then(r=>r.status)`,
            );
            check(
              "fixed player count enforced " + width,
              invalid === 400 && (await current()).capacity === 2,
            );
          }
          await browser("select", '[aria-label="房间人数"]', String(maximum));
          await wait(
            `document.querySelector('[aria-label="房间人数"]').value==='${maximum}'&&document.querySelectorAll('.waiting-seat').length===${maximum}`,
          );
          check(
            "capacity persisted " + kind + width,
            (await current()).capacity === maximum,
          );
          if (kind === "catan") {
            check(
              "six player rules follow capacity " + width,
              (await current()).catanOptions.fiveSix,
            );
          }
          if (kind === "splendor") {
            await evaluate(
              `(()=>{const original=window.fetch;window.__roomFlowSettingsStatuses=[];window.fetch=async(...fetchArguments)=>{const response=await original(...fetchArguments);const body=fetchArguments[1]?.body;if(typeof body==='string'&&JSON.parse(body).type==='room_settings')window.__roomFlowSettingsStatuses.push(response.status);return response};return true})()`,
            );
            const rejectedPassword = "房".repeat(25);
            await browser("focus", '[aria-label="房间密码"]');
            await browser("fill", '[aria-label="房间密码"]', rejectedPassword);
            await browser("press", "Tab");
            await wait(
              "window.__roomFlowSettingsStatuses.includes(400)&&!document.querySelector('[aria-label=房间密码]').disabled",
            );
            check(
              "failed password edit remains available " + width,
              !(await current()).locked &&
                (await evaluate(
                  `document.querySelector('[aria-label="房间密码"]').value===${JSON.stringify(rejectedPassword)}`,
                )),
            );
            await browser(
              "fill",
              '[aria-label="房间密码"]',
              "flow-new-room-password",
            );
            await command("add_bot");
            await command("add_bot");
            await command("ready");
            check(
              "room updates preserve password draft " + width,
              await evaluate(
                'document.querySelector(\'[aria-label="房间密码"]\').value === "flow-new-room-password"',
              ),
            );
            const before = await current();
            const rejected = await evaluate(
              `fetch('/api/rooms/${before.id}',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({type:'room_settings',capacity:2,version:${before.version},nonce:crypto.randomUUID()})}).then(r=>r.status)`,
            );
            check(
              "occupied seats cannot be removed " + width,
              rejected === 400 && (await current()).capacity === 4,
            );
            await browser("press", "Tab");
            await wait(
              "!!document.querySelector('[data-room-password-remove]')&&document.querySelector('[aria-label=房间密码]').value===''",
            );
            check(
              "password commits when editing finishes " + width,
              (await current()).locked,
            );
            await browser("select", '[aria-label="房间人数"]', "3");
            await wait("document.querySelectorAll('.waiting-seat').length===3");
            const resized = await current();
            check(
              "resize preserves bots and resets human ready " + width,
              resized.capacity === 3 &&
                resized.locked &&
                !resized.seats[0].ready &&
                resized.seats.slice(1).every((seat) => seat.ready),
            );
            await browser("reload");
            await wait(shown('[aria-label="房间人数"]'));
            check(
              "room settings survive reload " + width,
              (await current()).capacity === 3 && (await current()).locked,
            );
            await command("room_settings", { capacity: 4 });
            check(
              "capacity change preserves password " + width,
              (await current()).locked,
            );
            const version = (await current()).version;
            const stale = await evaluate(
              `fetch('/api/rooms/${opened.id}',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({type:'room_settings',capacity:3,roomPassword:'',version:${version - 1},nonce:crypto.randomUUID()})}).then(r=>r.status)`,
            );
            check(
              "stale settings rejected atomically " + width,
              stale === 409 &&
                (await current()).capacity === 4 &&
                (await current()).locked,
            );
            await activate("[data-room-password-remove]", touch);
            await wait(
              "!document.querySelector('[data-room-password-remove]')",
            );
            check(
              "room password removable " + width,
              !(await current()).locked,
            );
            await browser("focus", '[aria-label="房间密码"]');
            await browser("fill", '[aria-label="房间密码"]', "flow-final-room-password");
            await activate("[data-add-bot]", touch);
            await wait("document.querySelectorAll('.waiting-seat.filled').length===4&&!!document.querySelector('[data-room-password-remove]')");
            check("finishing password and adding a bot takes one click " + width, (await current()).seats.length===4&&(await current()).locked);

          } else {
            await browser("select", '[aria-label="房间人数"]', String(minimum));
            await wait(
              `document.querySelectorAll('.waiting-seat').length===${minimum}`,
            );
            check(
              "capacity shrinks " + kind + width,
              (await current()).capacity === minimum,
            );
          }
          if (kind === "yuyuhakusho") {
            await browser("select", '[aria-label="房间人数"]', "4");
            await wait(
              "document.querySelectorAll('.waiting-seat').length===4&&document.querySelectorAll('.yuyu-role').length===4",
            );
            check(
              "four player preparation has one authoritative rule editor " +
                width,
              await evaluate(
                "document.querySelectorAll('.yuyu-room-rules').length===1&&document.querySelectorAll('[aria-label=赛制]').length===1&&!document.querySelector('[data-room-settings-save]')",
              ),
            );
            await browser("select", '[aria-label="赛制"]', "solo");
            await wait(
              "document.querySelectorAll('[data-yuyu-team]').length===2",
            );
            check(
              "one versus three applies without saving " + width,
              (await current()).options.oneVsThree === true,
            );
            await browser(
              "screenshot",
              path.join(out, `room-settings-yuyuhakusho-${width}.png`),
            );
          }
          if (kind === "catan") {
            for (const scenario of ["cities-knights", "shores", "rivers"]) {
              await command("catan_two_scenario", {
                catanTwoScenario: scenario,
              });
              for (const capacity of [3, 6, 2]) {
                await command("room_settings", { capacity });
                const adjusted = await current();
                check(
                  `catan ${scenario} resizes to ${capacity} ${width}`,
                  adjusted.capacity === capacity &&
                    (capacity === 2
                      ? !!adjusted.catanTwoRules
                      : !adjusted.catanTwoRules),
                );
              }
            }
          }
          if (["splendor", "monopoly", "catan"].includes(kind)) {
            check(
              "settings fit viewport " + kind + width,
              await evaluate(
                "document.documentElement.scrollWidth<=innerWidth",
              ),
            );
            await browser(
              "screenshot",
              path.join(out, `room-settings-${kind}-${width}.png`),
            );
          }
          await command("leave");
          await wait(shown(".lobby"));
        }
        const host = client(backendOrigin),
          peer = client(backendOrigin),
          spectator = client(backendOrigin);
        await host.request("/login", {
          name: "FlowRoot",
          password: "flow-local-password",
        });
        await peer.request("/register", {
          name: "FlowPeer" + width,
          password: "flow-local-password",
          invite: "flow-invite",
        });
        await spectator.request("/register", {
          name: "FlowWatcher" + width,
          password: "flow-local-password",
          invite: "flow-invite",
        });
        const created = await api("/rooms", {
          kind: "gomoku",
          name: "通用交互验收",
          capacity: 2,
          password: "flow-room-password",
          options: {
            rules: "freestyle",
            difficulty: "standard",
            turnSeconds: 120,
          },
        });
        await peer.request("/rooms/" + created.id, {
          type: "join",
          version: created.version,
          nonce: randomUUID(),
          password: "flow-room-password",
        });
        await peer.request(
          "/rooms/" + created.id,
          {
            type: "room_settings",
            capacity: 2,
            roomPassword: "",
            version: (await current()).version,
            nonce: randomUUID(),
          },
          400,
        );
        check("non-host settings rejected " + width, (await current()).locked);
        await command("ready");
        await peer.command("ready");
        await command("start");
        const started = await current();
        check(
          "playing room rejects settings " + width,
          (await evaluate(
            `fetch('/api/rooms/${started.id}',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({type:'room_settings',capacity:2,version:${started.version},nonce:crypto.randomUUID()})}).then(r=>r.status)`,
          )) === 400,
        );
        await browser("reload");
        await wait(
          "!!document.querySelector('[data-fullscreen-enter]:not(:disabled)')",
        );
        check(
          "all playing room actions are icon buttons " + width,
          await evaluate(
            "[...document.querySelectorAll('.room-tools button')].every(button=>button.classList.contains('room-tool-button')&&button.getAttribute('aria-label')&&button.querySelector('svg'))",
          ),
        );
        check(
          "real viewport width " + width,
          (await evaluate("innerWidth")) === width,
        );
        await peer.command("action", {
          action: {
            type: "resign",
            round: (await peer.request("/state")).room.game.data.round,
            ply: (await peer.request("/state")).room.game.data.ply,
          },
        });
        await wait(shown(".match-result-modal"));
        const finished = await current();
        check("automatic result " + width, finished.status === "finished");
        await call("Input.dispatchMouseEvent", {
          type: "mousePressed",
          x: 2,
          y: 2,
          button: "left",
          buttons: 1,
          clickCount: 1,
        });
        await call("Input.dispatchMouseEvent", {
          type: "mouseReleased",
          x: 2,
          y: 2,
          button: "left",
          buttons: 0,
          clickCount: 1,
        });
        check(
          "backdrop preserves result " + width,
          await evaluate(shown(".match-result-modal")),
        );
        await evaluate(
          "window.resultEscape=[];document.addEventListener('keydown',event=>{if(event.key==='Escape')window.resultEscape.push(event.defaultPrevented)});true",
        );
        await browser("press", "Escape");
        check(
          "Escape does not stop browser requests " + width,
          await evaluate("window.resultEscape.at(-1)===true"),
        );
        check(
          "Escape preserves result " + width,
          await evaluate(shown(".match-result-modal")),
        );
        check(
          "explicit result actions " + width,
          await evaluate(
            "!document.querySelector('.match-result-modal [aria-label=关闭]')&&[...document.querySelectorAll('.match-result-modal button')].some(e=>e.textContent==='返回房间')",
          ),
        );
        await capture("result-" + width);
        await activate(".result-review", touch);
        await wait(shown(".result-reopen"));
        await evaluate("window.dispatchEvent(new Event('focus'));true");
        await api("/state");
        check(
          "ordinary refresh keeps result collapsed " + width,
          await evaluate("!document.querySelector('.match-result-modal')"),
        );
        await activate(".result-reopen", touch);
        await wait(shown(".match-result-modal"));
        await browser("reload");
        await wait(shown(".match-result-modal"));
        await activate(".result-return", touch);
        await wait(
          "!document.querySelector('.match-result-modal')&&!!document.querySelector('.waiting-layout')",
        );
        const waiting = await current();
        check(
          "return preserves room and players " + width,
          waiting.id === finished.id &&
            waiting.name === finished.name &&
            waiting.locked === finished.locked &&
            JSON.stringify(waiting.options) ===
              JSON.stringify(finished.options) &&
            waiting.seats.length === finished.seats.length &&
            waiting.status === "waiting",
          waiting,
        );
        check(
          "waiting room hides fullscreen " + width,
          await evaluate("!document.querySelector('[data-fullscreen-enter]')"),
        );
        check(
          "all waiting room actions are icon buttons " + width,
          await evaluate(
            "[...document.querySelectorAll('.room-tools button')].every(button=>button.classList.contains('room-tool-button')&&button.getAttribute('aria-label')&&button.querySelector('svg'))",
          ),
        );
        await command("ready");
        await peer.command("ready");
        await command("start");
        await wait(
          "!!document.querySelector('[data-fullscreen-enter]:not(:disabled)')",
        );
        await spectator.request("/rooms/" + created.id + "/watch", {
          password: "flow-room-password",
        });
        await browser("press", "Tab");
        for (const label of [
          "全屏",
          "开启托管",
          "对局日志",
          "玩法速查",
          "邀请朋友",
          "结束牌桌",
        ]) {
          const selector = `.room-tools button[aria-label="${label}"]`;
          check(
            "icon-only accessible tool " + label + " " + width,
            await evaluate(
              `(()=>{const e=document.querySelector(${JSON.stringify(selector)});const r=e.getBoundingClientRect();return e.classList.contains('room-tool-button')&&!!e.querySelector('svg')&&r.width<=48&&r.width>=${touch ? 44 : 32}&&r.height>=${touch ? 44 : 32}})()`,
            ),
          );
          await browser("focus", selector);
          await wait(
            `getComputedStyle(document.querySelector(${JSON.stringify(selector + " .room-tool-tooltip")})).opacity==='1'`,
          );
          check(
            "fast focused tooltip " + label + " " + width,
            await evaluate(
              `parseFloat(getComputedStyle(document.querySelector(${JSON.stringify(selector + " .room-tool-tooltip")})).transitionDelay)<=0.15`,
            ),
          );
        }
        if (!touch) {
          check(
            "desktop browser exposes mouse hover " + width,
            await evaluate("matchMedia('(hover:hover)').matches"),
          );
          await activate(".room-heading h1", false);
          await browser("scrollintoview", ".room-tools [aria-label=玩法速查]");
          await browser("hover", ".room-tools [aria-label=玩法速查]");
          await wait(
            "getComputedStyle(document.querySelector('[aria-label=玩法速查] .room-tool-tooltip')).opacity==='1'",
          );
        }
        await capture("tools-" + width);
        await activate(".room-tools [aria-label=对局日志]", touch);
        await wait(
          "!!document.querySelector('[role=dialog][aria-label*=日志]')",
        );
        await activate(
          "[role=dialog][aria-label*=日志] [aria-label=关闭]",
          touch,
        );
        await activate(".room-tools [aria-label=邀请朋友]", touch);
        await wait("!!document.querySelector('.toast:not(.error)')");
        await activate(".toast [aria-label=关闭提示]", touch);
        await activate(".room-tools [aria-label=结束牌桌]", touch);
        await wait(
          "!!document.querySelector('[role=dialog][aria-label=\"结束当前牌桌？\"]')",
        );
        await browser(
          "find",
          "role",
          "button",
          "click",
          "--name",
          "继续游戏",
          "--exact",
        );
        await activate(".room-tools [aria-label=玩法速查]", touch);
        await wait(
          "!!document.querySelector('[role=dialog][aria-label*=玩法速查]')",
        );
        await activate(
          "[role=dialog][aria-label*=玩法速查] [aria-label=关闭]",
          touch,
        );
        await activate(".room-tools [aria-label=开启托管]", touch);
        await wait(
          "!!document.querySelector('.room-tools [aria-label=取消托管][aria-pressed=true].active')",
        );
        check(
          "autoplay server state on " + width,
          (await current()).seats[0].autoPlay === true,
        );
        await activate(".room-tools [aria-label=取消托管]", touch);
        await wait(
          "!!document.querySelector('.room-tools [aria-label=开启托管][aria-pressed=false]')",
        );
        await activate("[data-fullscreen-enter]", touch);
        await wait(shown("[data-room-fullscreen]"));
        await peer.command("action", {
          action: {
            type: "resign",
            round: (await peer.request("/state")).room.game.data.round,
            ply: (await peer.request("/state")).room.game.data.ply,
          },
        });
        await wait(shown(".match-result-modal"));
        check(
          "new match result inside fullscreen " + width,
          (await evaluate(
            "!!document.querySelector('[data-room-fullscreen] .match-result-modal')",
          )) && (await current()).result.id !== finished.result.id,
        );
        await capture("fullscreen-result-" + width);
        await activate(".result-review", touch);
        await activate("[aria-label=退出全屏]", touch);
        await wait("!document.querySelector('[data-room-fullscreen]')");
        await evaluate(
          "Element.prototype.requestFullscreen=()=>Promise.reject(Error('native fullscreen unavailable in acceptance'));true",
        );
        await activate("[data-fullscreen-enter]", touch);
        await wait(shown("[data-room-fullscreen=page]"));
        await activate(".result-reopen", touch);
        await wait(shown(".match-result-modal"));
        check(
          "fallback fullscreen keeps result in surface " + width,
          await evaluate(
            "!!document.querySelector('[data-room-fullscreen=page] .match-result-modal')",
          ),
        );
        await capture("fallback-result-" + width);
        await activate(".result-review", touch);
        await activate("[aria-label=退出全屏]", touch);
        await wait("!document.querySelector('[data-room-fullscreen]')");
        await api("/logout", {});
        await api("/login", {
          name: "FlowPeer" + width,
          password: "flow-local-password",
        });
        await browser("reload");
        await wait(shown(".match-result-modal"));
        check(
          "member waits for host " + width,
          await evaluate(
            "!document.querySelector('.result-return')&&document.querySelector('.match-result-modal').textContent.includes('等待房主开启下一局')",
          ),
        );
        await capture("member-result-" + width);
        await api("/logout", {});
        await api("/login", {
          name: "FlowWatcher" + width,
          password: "flow-local-password",
        });
        await browser("reload");
        await wait(shown(".match-result-modal"));
        check(
          "spectator only reviews result " + width,
          (await evaluate(
            "!document.querySelector('.result-return')&&!document.querySelector('.result-waiting')",
          )) && (await current()).spectating,
        );
        await activate(".result-review", touch);
        await browser(
          "find",
          "role",
          "button",
          "click",
          "--name",
          "离开观战",
          "--exact",
        );
        await wait("!document.querySelector('.room-page')");
        await api("/logout", {});
        await api("/login", {
          name: "FlowPeer" + width,
          password: "flow-local-password",
        });
        await browser("reload");
        await wait(shown(".match-result-modal"));
        await activate(".result-review", touch);
        await activate("[data-leave-room]", touch);
        await wait(shown("[aria-label='离开房间？']"));
        check(
          "member leave needs confirmation " + width,
          !!(await current()) &&
            (await evaluate(
              "document.activeElement.matches('[data-leave-cancel]')&&!document.querySelector('[aria-label=\"离开房间？\"]').textContent.includes('房间将关闭')",
            )),
        );
        await activate("[data-leave-cancel]", touch);
        await activate(".result-reopen", touch);
        await host.command("leave");
        await wait(shown(".result-return"));
        check(
          "host transfer updates result actions " + width,
          await evaluate("!document.querySelector('.result-waiting')"),
        );
        await activate(".result-review", touch);
        await activate("[data-leave-room]", touch);
        await wait(shown("[aria-label='离开房间？']"));
        check(
          "last member sees closure consequence " + width,
          await evaluate(
            "document.querySelector('[aria-label=\"离开房间？\"]').textContent.includes('房间将关闭')",
          ),
        );
        await capture("leave-confirm-" + width);
        await call("Input.dispatchMouseEvent", {
          type: "mousePressed",
          x: 2,
          y: 2,
          button: "left",
          buttons: 1,
          clickCount: 1,
        });
        await call("Input.dispatchMouseEvent", {
          type: "mouseReleased",
          x: 2,
          y: 2,
          button: "left",
          buttons: 0,
          clickCount: 1,
        });
        await wait("!document.querySelector('[aria-label=\"离开房间？\"]')");
        check(
          "dismissing leave confirmation retains room " + width,
          !!(await current()),
        );
        await activate("[data-leave-room]", touch);
        await wait(shown("[aria-label='离开房间？']"));
        await activate("[data-leave-confirm]", touch);
        await wait(
          "!!document.querySelector('.lobby')||!document.querySelector('.room-page')",
        );
        check("confirmed leave returns to lobby " + width, !(await current()));
      }
      check(
        "no browser runtime errors",
        !(await browser("errors")).errors.length,
      );
      const journal = await execute(
        "python3",
        [
          "-B",
          "-c",
          "import sqlite3,sys;db=sqlite3.connect(sys.argv[1]);print(db.execute(\"SELECT COUNT(*) FROM actions WHERE instr(CAST(action AS TEXT), 'flow-new-room-password') > 0\").fetchone()[0])",
          path.join(temporary, "data/wire-board.db"),
        ],
        { cwd: root, env: environment },
      );
      check(
        "room password absent from action journal",
        journal.stdout.trim() === "0",
      );
      report.passed = true;
    } catch (error) {
      report.error = error.stack;
      if (error.stdout || error.stderr)
        report.browserFailure = { stdout: error.stdout, stderr: error.stderr };
      if (session) {
        try {
          await browser("screenshot", path.join(out, "failure.png"), "--full");
          report.snapshot = await browser("snapshot", "-i");
        } catch {}
      }
      throw error;
    } finally {
      if (socket) socket.close();
      for (const entry of pending.values()) {
        clearTimeout(entry.timer);
        entry.reject(Error("test cleanup"));
      }
      if (session) {
        try {
          await browser("close");
        } catch {}
      }
      if (frontend) await frontend.close();
      if (backend) {
        backend.kill("SIGTERM");
        await new Promise((resolve) =>
          backend.exitCode !== null ? resolve() : backend.once("exit", resolve),
        );
      }
      for (const file of [
        "web/src/main.tsx",
        "web/src/game-api/host.tsx",
        "web/src/legacy/room-ui.tsx",
        "web/src/ui/game-choices.tsx",
        "web/src/ui/room-waiting.css",
        "web/src/ui/create-room.tsx",
        "web/src/ui/room-settings.tsx",
        "internal/server/room_settings.go",
        "internal/server/server.go",
        "web/src/ui/board-host.tsx",
        "web/src/ui/result-overlay.tsx",
        "web/src/ui/results.tsx",
        "web/src/ui/leave-room-control.tsx",
        "web/src/ui/room-tool-button.tsx",
        "web/src/ui/room-actions.css",
        "web/src/ui/saved-games.tsx",
        "web/src/ui/game-focus.ts",
        "web/src/ui/room-fullscreen.tsx",
        "web/tests/room-flow-browser.test.mjs",
      ]) {
        try {
          report.sources[file] = createHash("sha256")
            .update(await readFile(path.join(root, file)))
            .digest("hex");
        } catch {}
      }
      await writeFile(
        path.join(out, "result.json"),
        JSON.stringify(report, null, 2) + "\n",
      );
      await writeFile(path.join(out, "server.log"), backendLog);
      if (report.passed)
        await rm(path.join(out, "failure.png"), { force: true });
      await rm(temporary, { recursive: true, force: true });
      console.log("Room flow evidence:", path.join(out, "result.json"));
    }
  },
);
